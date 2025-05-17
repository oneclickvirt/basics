//go:build linux
// +build linux

package ipv6

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	ICMPv6RouterAdvertisement = 134
	ICMPv6RouterSolicitation  = 133
	ICMPv6OptionPrefix        = 3
)

func extractPrefixFromRAOption(data []byte) []string {
	var prefixLengths []string
	optionStart := 16
	for optionStart < len(data) {
		if optionStart+2 > len(data) {
			break
		}
		optionType := data[optionStart]
		optionLen := data[optionStart+1] * 8
		if optionLen == 0 || optionStart+int(optionLen) > len(data) {
			break
		}
		if optionType == ICMPv6OptionPrefix && optionLen >= 32 {
			prefixLen := data[optionStart+2]
			prefixStart := optionStart + 16
			if prefixStart+16 <= len(data) {
				var prefix [16]byte
				copy(prefix[:], data[prefixStart:prefixStart+16])
				if !isNonGlobalPrefix(prefix) && isPrefixLengthValid(int(prefixLen)) {
					prefixLengths = append(prefixLengths, fmt.Sprintf("%d", prefixLen))
				}
			}
		}
		optionStart += int(optionLen)
	}
	return prefixLengths
}

func isPrefixLengthValid(prefixLen int) bool {
	return prefixLen >= 1 && prefixLen <= 128
}

func sendRouterSolicitation(fd int, interfaceName string) error {
	intf, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return fmt.Errorf("获取接口信息失败: %v", err)
	}
	msg := make([]byte, 8)
	msg[0] = ICMPv6RouterSolicitation
	msg[1] = 0
	if len(intf.HardwareAddr) == 6 {
		msg = append(msg, 1)
		msg = append(msg, 1)
		msg = append(msg, intf.HardwareAddr...)
		msg = append(msg, 0, 0)
	}
	var allRoutersAddr [16]byte
	copy(allRoutersAddr[:], net.ParseIP("ff02::2").To16())
	var addr syscall.SockaddrInet6
	addr.ZoneId = uint32(intf.Index)
	copy(addr.Addr[:], allRoutersAddr[:])
	if err := syscall.Sendto(fd, msg, 0, &addr); err != nil {
		return fmt.Errorf("发送Router Solicitation失败: %v", err)
	}
	return nil
}

func getPrefixFromRA(interfaceName string) (string, error) {
	radvdumpPath, err := exec.LookPath("radvdump")
	if err == nil && radvdumpPath != "" {
		cmd := exec.Command("radvdump", "-i", interfaceName)
		output, err := cmd.Output()
		if err == nil {
			re := regexp.MustCompile(`(?i)prefix\s+([a-fA-F0-9:]+)/(\d+)`)
			matches := re.FindAllStringSubmatch(string(output), -1)
			for _, match := range matches {
				prefix := match[1]
				prefixLen := match[2]
				prefixLenInt, err := strconv.Atoi(prefixLen)
				if err != nil || !isPrefixLengthValid(prefixLenInt) {
					continue
				}
				if !strings.HasPrefix(prefix, "fe80") &&
					!strings.HasPrefix(prefix, "::1") &&
					!strings.HasPrefix(prefix, "fc") &&
					!strings.HasPrefix(prefix, "fd") &&
					!strings.HasPrefix(prefix, "ff") {
					return prefixLen, nil
				}
			}
		}
	}
	intf, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return "", fmt.Errorf("获取网络接口失败: %v", err)
	}
	fd, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_RAW, syscall.IPPROTO_ICMPV6)
	if err != nil {
		return "", fmt.Errorf("创建原始套接字失败: %v", err)
	}
	defer syscall.Close(fd)
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, intf.Index); err != nil {
		return "", fmt.Errorf("绑定套接字到接口失败: %v", err)
	}
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_IPV6, syscall.IPV6_RECVPKTINFO, 1); err != nil {
		return "", fmt.Errorf("设置套接字选项失败: %v", err)
	}
	tv := syscall.Timeval{
		Sec:  5,
		Usec: 0,
	}
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
		return "", fmt.Errorf("设置接收超时失败: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = sendRouterSolicitation(fd, interfaceName)
	if err != nil {
		return "", fmt.Errorf("发送Router Solicitation失败: %v", err)
	}
	buffer := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("等待Router Advertisement超时")
		default:
			n, _, err := syscall.Recvfrom(fd, buffer, 0)
			if err != nil {
				if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
					continue
				}
				return "", fmt.Errorf("接收数据失败: %v", err)
			}
			if n < 4 {
				continue
			}
			if buffer[0] == ICMPv6RouterAdvertisement {
				prefixLengths := extractPrefixFromRAOption(buffer[:n])
				if len(prefixLengths) > 0 {
					return prefixLengths[0], nil
				}
			}
		}
	}
}

func getPrefixFromIPCommand(interfaceName string) (string, error) {
	cmd := exec.Command("ip", "-o", "-6", "addr", "show", interfaceName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`\s*inet6\s+([a-fA-F0-9:]+)/(\d+)\s+scope\s+global`)
	matches := re.FindAllStringSubmatch(string(output), -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("未找到全局IPv6地址")
	}
	var prefixLens []int
	for _, match := range matches {
		ipv6Addr := match[1]
		if strings.HasPrefix(ipv6Addr, "fe80") ||
			strings.HasPrefix(ipv6Addr, "::1") ||
			strings.HasPrefix(ipv6Addr, "fc") ||
			strings.HasPrefix(ipv6Addr, "fd") ||
			strings.HasPrefix(ipv6Addr, "ff") {
			continue
		}
		if len(match) > 2 {
			prefixLen, err := strconv.Atoi(match[2])
			if err != nil || !isPrefixLengthValid(prefixLen) {
				continue
			}
			prefixLens = append(prefixLens, prefixLen)
		}
	}
	if len(prefixLens) >= 1 {
		sort.Ints(prefixLens)
		return strconv.Itoa(prefixLens[0]), nil
	}
	return "", fmt.Errorf("未找到有效的IPv6前缀长度")
}

func getPrefixFromConfigFiles() (string, error) {
	configFiles := []string{
		"/etc/network/interfaces",
		"/etc/netplan/01-netcfg.yaml",
		"/etc/netplan/50-cloud-init.yaml",
		"/etc/sysconfig/network-scripts/ifcfg-eth0",
	}
	for _, file := range configFiles {
		if _, err := os.Stat(file); err == nil {
			content, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			re := regexp.MustCompile(`(?i)(prefix-length|prefixlen|netmask)[\s:=]+(\d+)`)
			matches := re.FindStringSubmatch(string(content))
			if len(matches) >= 3 {
				prefixLen, err := strconv.Atoi(matches[2])
				if err != nil || !isPrefixLengthValid(prefixLen) {
					continue
				}
				return matches[2], nil
			}
		}
	}
	return "", fmt.Errorf("在配置文件中未找到IPv6前缀长度信息")
}

func GetIPv6Mask(publicIPv6, language string) (string, error) {
	if publicIPv6 == "" {
		return "", fmt.Errorf("无公网IPV6地址")
	}
	interfaceName, err := getInterface()
	if err != nil || interfaceName == "" {
		return "", fmt.Errorf("获取网络接口失败: %v", err)
	}
	var prefixLen string
	// 优先级1：从RA报文获取前缀长度
	prefixLen, err = getPrefixFromRA(interfaceName)
	if err == nil && prefixLen != "" {
		if len, err := strconv.Atoi(prefixLen); err == nil && isPrefixLengthValid(len) {
			return formatIPv6Mask(prefixLen, language), nil
		}
	}
	// 优先级2：从ip命令获取前缀长度
	prefixLen, err = getPrefixFromIPCommand(interfaceName)
	if err == nil && prefixLen != "" {
		if len, err := strconv.Atoi(prefixLen); err == nil && isPrefixLengthValid(len) {
			return formatIPv6Mask(prefixLen, language), nil
		}
	}
	// 优先级3：从配置文件获取前缀长度
	prefixLen, err = getPrefixFromConfigFiles()
	if err == nil && prefixLen != "" {
		if len, err := strconv.Atoi(prefixLen); err == nil && isPrefixLengthValid(len) {
			return formatIPv6Mask(prefixLen, language), nil
		}
	}
	return formatIPv6Mask("128", language), nil
}
