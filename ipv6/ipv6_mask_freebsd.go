//go:build freebsd
// +build freebsd

package ipv6

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// FreeBSD 上获取前缀长度
func getPrefixFromIfconfig(interfaceName string) (string, error) {
	cmd := exec.Command("ifconfig", interfaceName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// 匹配 inet6 地址和前缀长度
	re := regexp.MustCompile(`inet6\s+([a-fA-F0-9:]+)%?\w*\s+prefixlen\s+(\d+)`)
	matches := re.FindAllStringSubmatch(string(output), -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		ipv6Addr := match[1]
		prefixLen := match[2]
		// 排除非公网地址前缀
		if strings.HasPrefix(ipv6Addr, "fe80") || // 链路本地地址
			strings.HasPrefix(ipv6Addr, "::1") || // 回环地址
			strings.HasPrefix(ipv6Addr, "fc") || // 本地唯一地址
			strings.HasPrefix(ipv6Addr, "fd") || // 本地唯一地址
			strings.HasPrefix(ipv6Addr, "ff") { // 组播地址
			continue
		}
		return prefixLen, nil
	}
	return "", fmt.Errorf("未找到全局IPv6地址")
}

// FreeBSD平台专用的配置文件获取方法
func getPrefixFromConfigFiles() (string, error) {
	// FreeBSD 的网络配置文件
	configFiles := []string{
		"/etc/rc.conf",
		"/etc/rc.conf.local",
	}
	for _, file := range configFiles {
		if _, err := os.Stat(file); err == nil {
			content, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			// 在配置文件中查找IPv6前缀长度
			re := regexp.MustCompile(`(?i)ipv6_prefix(len)?="?(\d+)"?`)
			matches := re.FindStringSubmatch(string(content))
			if len(matches) >= 3 {
				return matches[2], nil
			}
		}
	}
	return "", fmt.Errorf("在配置文件中未找到IPv6前缀长度信息")
}

// 获取 IPv6 子网掩码 - FreeBSD 实现
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
	// 方法1：从ifconfig获取前缀长度
	prefixLen, err := getPrefixFromIfconfig(interfaceName)
	if err == nil && prefixLen != "" {
		return formatIPv6Mask(prefixLen, language), nil
	}
	// 方法2：从配置文件获取前缀长度
	prefixLen, err = getPrefixFromConfigFiles()
	if err == nil && prefixLen != "" {
		return formatIPv6Mask(prefixLen, language), nil
	}
	// 如果以上方法都失败但确实有公网IPv6，使用/128作为默认子网掩码
	return formatIPv6Mask("128", language), nil
}
