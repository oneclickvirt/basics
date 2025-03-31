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
	"strings"
	"syscall"
	"time"
)

// Router Advertisement前缀选项类型
const (
	ICMPv6RouterAdvertisement = 134
	ICMPv6RouterSolicitation  = 133
	ICMPv6OptionPrefix        = 3
)

// 从RA报文中提取前缀长度信息
func extractPrefixFromRAOption(data []byte) []string {
	var prefixLengths []string
	// 跳过ICMPv6头部(4字节)和Router Advertisement基本信息(12字节)
	optionStart := 16
	// 遍历所有选项
	for optionStart < len(data) {
		// 确保有足够的数据可读
		if optionStart+2 > len(data) {
			break
		}
		optionType := data[optionStart]
		optionLen := data[optionStart+1] * 8 // 长度以8字节为单位
		// 确保选项长度有效
		if optionLen == 0 || optionStart+int(optionLen) > len(data) {
			break
		}
		// 解析前缀信息选项
		if optionType == ICMPv6OptionPrefix && optionLen >= 32 {
			// 前缀长度在选项开始后2字节处
			prefixLen := data[optionStart+2]
			// 前缀值从选项开始后16字节处
			prefixStart := optionStart + 16
			if prefixStart+16 <= len(data) {
				var prefix [16]byte
				copy(prefix[:], data[prefixStart:prefixStart+16])
				// 排除非全局单播地址
				if !isNonGlobalPrefix(prefix) {
					prefixLengths = append(prefixLengths, fmt.Sprintf("%d", prefixLen))
				}
			}
		}
		optionStart += int(optionLen)
	}
	return prefixLengths
}

// 发送Router Solicitation消息
func sendRouterSolicitation(fd int, interfaceName string) error {
	// 获取接口信息
	intf, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return fmt.Errorf("获取接口信息失败: %v", err)
	}
	// 构造ICMPv6 Router Solicitation消息
	// ICMPv6头部(4字节): 类型(1字节) + 代码(1字节) + 校验和(2字节)
	msg := make([]byte, 8)
	msg[0] = ICMPv6RouterSolicitation // 类型:Router Solicitation
	msg[1] = 0                        // 代码:0
	// msg[2]和msg[3]是校验和字段，暂时为0，稍后计算
	// 可选：添加Source Link-Layer Address选项（MAC地址）
	// 这对一些路由器来说可能是必要的
	if len(intf.HardwareAddr) == 6 { // 确保MAC地址有效
		// 添加源链路层地址选项
		// 选项类型(1) + 长度(1) + MAC地址(6)
		msg = append(msg, 1)                    // 选项类型:1 (Source Link-Layer Address)
		msg = append(msg, 1)                    // 长度:1 (以8字节为单位，这里是8字节)
		msg = append(msg, intf.HardwareAddr...) // MAC地址
		msg = append(msg, 0, 0)                 // 填充到8字节对齐
	}
	// 设置ICMPv6的目标地址为All Routers组播地址
	var allRoutersAddr [16]byte
	copy(allRoutersAddr[:], net.ParseIP("ff02::2").To16())
	// 构造sockaddr_in6结构
	var addr syscall.SockaddrInet6
	addr.ZoneId = uint32(intf.Index)
	copy(addr.Addr[:], allRoutersAddr[:])
	// 发送数据包
	if err := syscall.Sendto(fd, msg, 0, &addr); err != nil {
		return fmt.Errorf("发送Router Solicitation失败: %v", err)
	}
	return nil
}

// 方法1：尝试原生实现从Router Advertisement获取前缀长度
func getPrefixFromRA(interfaceName string) (string, error) {
	// 尝试先用radvdump，如果存在的话
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
				// 排除非公网地址前缀
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
	// 如果radvdump不可用或未找到有效前缀，使用原生实现
	intf, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return "", fmt.Errorf("获取网络接口失败: %v", err)
	}
	// 创建原始套接字接收ICMPv6消息
	fd, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_RAW, syscall.IPPROTO_ICMPV6)
	if err != nil {
		return "", fmt.Errorf("创建原始套接字失败: %v", err)
	}
	defer syscall.Close(fd)
	// 绑定到指定接口
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, intf.Index); err != nil {
		return "", fmt.Errorf("绑定套接字到接口失败: %v", err)
	}
	// 设置过滤器，只接收Router Advertisement消息
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_IPV6, syscall.IPV6_RECVPKTINFO, 1); err != nil {
		return "", fmt.Errorf("设置套接字选项失败: %v", err)
	}
	// 设置接收超时
	tv := syscall.Timeval{
		Sec:  5, // 5秒超时
		Usec: 0,
	}
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
		return "", fmt.Errorf("设置接收超时失败: %v", err)
	}
	// 创建上下文，用于发送Router Solicitation并等待Router Advertisement
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// 发送Router Solicitation
	err = sendRouterSolicitation(fd, interfaceName)
	if err != nil {
		return "", fmt.Errorf("发送Router Solicitation失败: %v", err)
	}
	// 接收Router Advertisement
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
			// 确保收到的是ICMPv6消息
			if n < 4 {
				continue
			}
			// 检查是否为Router Advertisement
			if buffer[0] == ICMPv6RouterAdvertisement {
				// 解析数据包提取前缀长度
				prefixLengths := extractPrefixFromRAOption(buffer[:n])
				if len(prefixLengths) > 0 {
					// 返回第一个有效前缀长度
					return prefixLengths[0], nil
				}
			}
		}
	}
}

// 方法2：从ip命令获取前缀长度
func getPrefixFromIPCommand(interfaceName string) (string, error) {
	cmd := exec.Command("ip", "-o", "-6", "addr", "show", interfaceName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// 匹配 inet6 地址和前缀长度
	re := regexp.MustCompile(`\s*inet6\s+([a-fA-F0-9:]+)/(\d+)\s+scope\s+global`)
	matches := re.FindAllStringSubmatch(string(output), -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("未找到全局IPv6地址")
	}
	var prefixLens []string
	for _, match := range matches {
		ipv6Addr := match[1]
		// 排除非公网地址前缀
		if strings.HasPrefix(ipv6Addr, "fe80") || // 链路本地地址
			strings.HasPrefix(ipv6Addr, "::1") || // 回环地址
			strings.HasPrefix(ipv6Addr, "fc") || // 本地唯一地址
			strings.HasPrefix(ipv6Addr, "fd") || // 本地唯一地址
			strings.HasPrefix(ipv6Addr, "ff") { // 组播地址
			continue
		}
		// 提取 prefixlen
		if len(match) > 2 {
			prefixLens = append(prefixLens, match[2])
		}
	}
	if len(prefixLens) >= 1 {
		sort.Strings(prefixLens)
		return prefixLens[0], nil
	}
	return "", fmt.Errorf("未找到有效的IPv6前缀长度")
}

// Linux平台专用的配置文件获取方法
func getPrefixFromConfigFiles() (string, error) {
	// 尝试从常见的网络配置文件中读取
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
			// 在配置文件中查找IPv6前缀长度
			re := regexp.MustCompile(`(?i)(prefix-length|prefixlen|netmask)[\s:=]+(\d+)`)
			matches := re.FindStringSubmatch(string(content))
			if len(matches) >= 3 {
				return matches[2], nil
			}
		}
	}
	return "", fmt.Errorf("在配置文件中未找到IPv6前缀长度信息")
}

// 获取 IPv6 子网掩码 - Linux 实现
func GetIPv6Mask(language string) (string, error) {
	// 首先检查是否有公网IPv6地址
	publicIPv6, err := getCurrentIPv6()
	if err != nil || publicIPv6 == "" {
		// 没有公网IPv6，返回空字符串
		return "", nil
	}
	// 获取网络接口
	interfaceName, err := getInterface()
	if err != nil || interfaceName == "" {
		return "", fmt.Errorf("获取网络接口失败: %v", err)
	}
	// 优先级1：尝试从RA报文获取前缀长度
	prefixLen, err := getPrefixFromRA(interfaceName)
	if err == nil && prefixLen != "" {
		return formatIPv6Mask(prefixLen, language), nil
	}
	// 优先级2：从ip命令获取前缀长度
	prefixLen, err = getPrefixFromIPCommand(interfaceName)
	if err == nil && prefixLen != "" {
		return formatIPv6Mask(prefixLen, language), nil
	}
	// 优先级3：从配置文件获取前缀长度
	prefixLen, err = getPrefixFromConfigFiles()
	if err == nil && prefixLen != "" {
		return formatIPv6Mask(prefixLen, language), nil
	}
	// 如果以上方法都失败但确实有公网IPv6，使用/128作为默认子网掩码
	return formatIPv6Mask("128", language), nil
}
