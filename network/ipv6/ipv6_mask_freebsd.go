//go:build freebsd
// +build freebsd

package ipv6

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// FreeBSD 上获取前缀长度
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

// FreeBSD平台专用的配置文件获取方法
func getPrefixFromConfigFiles() (string, error) {
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
			re := regexp.MustCompile(`(?i)ipv6_prefix(len)?="?(\d+)"?`)
			matches := re.FindStringSubmatch(string(content))
			if len(matches) >= 3 {
				prefixLen := matches[2]
				prefixLenInt, err := strconv.Atoi(prefixLen)
				if err != nil || prefixLenInt < 0 || prefixLenInt > 128 {
					continue
				}
				return prefixLen, nil
			}
		}
	}
	return "", fmt.Errorf("在配置文件中未找到IPv6前缀长度信息")
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
	// 方法2：从配置文件获取前缀长度
	prefixLen, err = getPrefixFromConfigFiles()
	if err == nil && prefixLen != "" {
		return formatIPv6Mask(prefixLen, language), nil
	}
	return formatIPv6Mask("128", language), nil
}
