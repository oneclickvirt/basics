//go:build darwin
// +build darwin

package ipv6

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// macOS上获取前缀长度
func getPrefixFromIfconfig(interfaceName string) (string, error) {
	cmd := exec.Command("ifconfig", interfaceName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`inet6\s+([a-fA-F0-9:]+)%?\w*\s+prefixlen\s+(\d+)`)
	matches := re.FindAllStringSubmatch(string(output), -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		ipv6Addr := match[1]
		prefixLen := match[2]
		if strings.HasPrefix(ipv6Addr, "fe80") || // 链路本地地址
			strings.HasPrefix(ipv6Addr, "::1") || // 回环地址
			strings.HasPrefix(ipv6Addr, "fc") || // 本地唯一地址
			strings.HasPrefix(ipv6Addr, "fd") || // 本地唯一地址
			strings.HasPrefix(ipv6Addr, "ff") { // 组播地址
			continue
		}
		prefixLenInt, err := strconv.Atoi(prefixLen)
		if err != nil || prefixLenInt < 0 || prefixLenInt > 128 {
			continue
		}
		return prefixLen, nil
	}
	return "", fmt.Errorf("未找到全局IPv6地址")
}

// macOS平台上使用networksetup命令获取更多信息
func getPrefixFromNetworksetup(interfaceName string) (string, error) {
	cmd := exec.Command("networksetup", "-listallhardwareports")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(output), "\n")
	var serviceName string
	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], "Device: "+interfaceName) && i > 0 {
			serviceNameLine := lines[i-1]
			if strings.HasPrefix(serviceNameLine, "Hardware Port: ") {
				serviceName = strings.TrimPrefix(serviceNameLine, "Hardware Port: ")
				break
			}
		}
	}
	if serviceName == "" {
		return "", fmt.Errorf("未找到网络接口对应的服务名称")
	}
	cmd = exec.Command("networksetup", "-getinfo", serviceName)
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`IPv6:\s*Automatic\s*\nIPv6\s*Address:\s*([a-fA-F0-9:]+)\s*\nIPv6\s*Prefix\s*Length:\s*(\d+)`)
	match := re.FindStringSubmatch(string(output))
	if len(match) >= 3 {
		prefixLen := match[2]
		prefixLenInt, err := strconv.Atoi(prefixLen)
		if err != nil || prefixLenInt < 0 || prefixLenInt > 128 {
			return "", fmt.Errorf("无效的IPv6前缀长度: %s", prefixLen)
		}
		return prefixLen, nil
	}
	return "", fmt.Errorf("未找到IPv6前缀长度信息")
}

// 获取 IPv6 子网掩码
func GetIPv6Mask(publicIPv6, language string) (string, error) {
	if publicIPv6 == "" {
		return "", fmt.Errorf("无公网IPV6地址")
	}
	interfaceName, err := getInterface()
	if err != nil || interfaceName == "" {
		return "", fmt.Errorf("获取网络接口失败: %v", err)
	}
	// 方法1：从ifconfig获取前缀长度
	prefixLen, err := getPrefixFromIfconfig(interfaceName)
	if err == nil && prefixLen != "" {
		return formatIPv6Mask(prefixLen, language), nil
	}
	// 方法2：从networksetup获取前缀长度
	prefixLen, err = getPrefixFromNetworksetup(interfaceName)
	if err == nil && prefixLen != "" {
		return formatIPv6Mask(prefixLen, language), nil
	}
	return formatIPv6Mask("128", language), nil
}
