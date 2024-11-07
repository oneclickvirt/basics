package ipv6

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

// 获取第一个以 eth 或 en 开头的网络接口
func getInterface() (string, error) {
	cmd := exec.Command("sh", "-c", "ls /sys/class/net/ | grep -E '^(eth|en)' | head -n 1")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// 获取当前的 IPv6 地址
func getCurrentIPv6() (string, error) {
	cmd := exec.Command("curl", "-s", "-6", "-m", "5", "ipv6.ip.sb")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// 添加 IPv6 地址到指定接口
func addIPv6(interfaceName, ipv6 string) error {
	cmd := exec.Command("ip", "addr", "add", ipv6+"/128", "dev", interfaceName)
	return cmd.Run()
}

// 删除指定接口上的 IPv6 地址
func delIPv6(interfaceName, ipv6 string) error {
	cmd := exec.Command("ip", "addr", "del", ipv6+"/128", "dev", interfaceName)
	return cmd.Run()
}

// 获取接口的子网掩码前缀
func getIPv6PrefixLength(interfaceName string) (string, error) {
	cmd := exec.Command("ifconfig", interfaceName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// 匹配 inet6 地址和前缀长度
	re := regexp.MustCompile(`inet6 ([^ ]+) prefixlen (\d+)`)
	matches := re.FindAllStringSubmatch(string(output), -1)
	if len(matches) == 0 {
		return "", nil
	}
	var prefixLens []string
	for _, match := range matches {
		ipv6Addr := match[1]
		// 排除非公网地址前缀
		if strings.HasPrefix(ipv6Addr, "fe80") ||    // 链路本地地址
			strings.HasPrefix(ipv6Addr, "::1") ||     // 回环地址
			strings.HasPrefix(ipv6Addr, "fc") ||      // 本地唯一地址
			strings.HasPrefix(ipv6Addr, "fd") ||      // 本地唯一地址
			strings.HasPrefix(ipv6Addr, "ff") {       // 组播地址
			continue
		}
		// 提取 prefixlen
		if len(match) > 2 {
			prefixLens = append(prefixLens, match[2])
		}
	}
	if len(prefixLens) >= 2 {
		sort.Strings(prefixLens)
		return prefixLens[0], nil
	} else if len(prefixLens) == 1 {
		return prefixLens[0], nil
	}
	return "", nil
}

// 获取 IPv6 子网掩码
func GetIPv6Mask(language string) (string, error) {
	// 获取网络接口
	interfaceName, err := getInterface()
	if err != nil || interfaceName == "" {
		return "", fmt.Errorf("Failed to get network interface: %v", err)
	}

	// 获取当前 IPv6 地址
	currentIPv6, err := getCurrentIPv6()
	if err != nil || currentIPv6 == "" {
		return "", fmt.Errorf("Failed to get current IPv6 address: %v", err)
	}

	// 生成新的 IPv6 地址
	newIPv6 := currentIPv6[:strings.LastIndex(currentIPv6, ":")] + ":3"

	// 添加新的 IPv6 地址
	if err := addIPv6(interfaceName, newIPv6); err != nil {
		return "", fmt.Errorf("Failed to add IPv6 address: %v", err)
	}
	time.Sleep(5 * time.Second)

	// 获取更新后的 IPv6 地址
	updatedIPv6, err := getCurrentIPv6()
	if err != nil {
		return "", fmt.Errorf("Failed to get updated IPv6 address: %v", err)
	}

	// 删除添加的 IPv6 地址
	if err := delIPv6(interfaceName, newIPv6); err != nil {
		return "", fmt.Errorf("Failed to delete IPv6 address: %v", err)
	}
	time.Sleep(5 * time.Second)

	// 获取子网掩码前缀长度
	ipv6Prefixlen, err := getIPv6PrefixLength(interfaceName)
	if err != nil {
		return "", fmt.Errorf("Failed to get IPv6 prefix length: %v", err)
	}
	if ipv6Prefixlen == "" {
		return "", fmt.Errorf("get IPv6 prefix length is null")
	}

	// 输出结果
	if updatedIPv6 == currentIPv6 || updatedIPv6 == "" {
		if language == "en" {
			return " IPv6 Mask           : /128", nil
		}
		return " IPv6 子网掩码       : /128", nil
	}
	if language == "en" {
		return fmt.Sprintf(" IPv6 Mask           : /%s\n", ipv6Prefixlen), nil
	}
	return fmt.Sprintf(" IPv6 子网掩码       : /%s\n", ipv6Prefixlen), nil
}
