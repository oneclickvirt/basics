//go:build windows
// +build windows

package ipv6

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Windows上获取前缀长度
func getPrefixFromNetsh(interfaceName string) (string, error) {
	// Windows上使用netsh命令获取IPv6配置
	cmd := exec.Command("netsh", "interface", "ipv6", "show", "addresses")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// 转换输出为字符串并按行分割
	lines := strings.Split(string(output), "\n")
	// 解析输出查找IPv6地址和前缀长度
	currentInterface := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 查找接口名称
		if strings.HasSuffix(line, ":") {
			currentInterface = strings.TrimSuffix(line, ":")
			continue
		}
		// 只处理匹配的接口
		if currentInterface == "" || !strings.Contains(strings.ToLower(currentInterface), strings.ToLower(interfaceName)) {
			continue
		}
		// 查找地址和前缀长度
		if strings.Contains(line, "Address") && strings.Contains(line, "Parameters") {
			re := regexp.MustCompile(`([a-fA-F0-9:]+)/(\d+)`)
			match := re.FindStringSubmatch(line)
			if len(match) >= 3 {
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
		}
	}
	return "", fmt.Errorf("未找到全局IPv6地址")
}

// Windows特有的PowerShell方法获取IPv6信息
func getPrefixFromPowerShell(interfaceName string) (string, error) {
	// 使用PowerShell获取网络适配器信息
	cmd := exec.Command("powershell", "-Command",
		"Get-NetIPAddress -AddressFamily IPv6 | Where-Object { $_.InterfaceAlias -like '*"+interfaceName+"*' -and $_.PrefixOrigin -ne 'WellKnown' } | Select-Object IPAddress, PrefixLength | ConvertTo-Json")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	jsonStr := string(output)
	rePrefixLength := regexp.MustCompile(`"PrefixLength"\s*:\s*(\d+)`)
	reIPAddress := regexp.MustCompile(`"IPAddress"\s*:\s*"([^"]+)"`)
	prefixMatches := rePrefixLength.FindAllStringSubmatch(jsonStr, -1)
	ipMatches := reIPAddress.FindAllStringSubmatch(jsonStr, -1)
	if len(prefixMatches) != len(ipMatches) {
		return "", fmt.Errorf("解析PowerShell输出失败")
	}
	for i := 0; i < len(ipMatches); i++ {
		ipv6Addr := ipMatches[i][1]
		// 排除非公网地址前缀
		if strings.HasPrefix(ipv6Addr, "fe80") || // 链路本地地址
			strings.HasPrefix(ipv6Addr, "::1") || // 回环地址
			strings.HasPrefix(ipv6Addr, "fc") || // 本地唯一地址
			strings.HasPrefix(ipv6Addr, "fd") || // 本地唯一地址
			strings.HasPrefix(ipv6Addr, "ff") { // 组播地址
			continue
		}
		return prefixMatches[i][1], nil
	}
	return "", fmt.Errorf("未找到全局IPv6地址")
}

// 获取 IPv6 子网掩码 - Windows 实现
func GetIPv6Mask(language string) (string, error) {
	publicIPv6, err := getCurrentIPv6()
	if err != nil || publicIPv6 == "" {
		return "", nil
	}
	interfaceName, err := getInterface()
	if err != nil || interfaceName == "" {
		return "", fmt.Errorf("获取网络接口失败: %v", err)
	}
	prefixLen, err := getPrefixFromNetsh(interfaceName)
	if err == nil && prefixLen != "" {
		return formatIPv6Mask(prefixLen, language), nil
	}
	prefixLen, err = getPrefixFromPowerShell(interfaceName)
	if err == nil && prefixLen != "" {
		return formatIPv6Mask(prefixLen, language), nil
	}
	return formatIPv6Mask("128", language), nil
}
