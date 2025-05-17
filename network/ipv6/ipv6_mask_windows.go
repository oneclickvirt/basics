//go:build windows
// +build windows

package ipv6

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Windows上获取前缀长度
func getPrefixFromNetsh(interfaceName string) (string, error) {
	cmd := exec.Command("netsh", "interface", "ipv6", "show", "addresses")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(output), "\n")
	currentInterface := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ":") {
			currentInterface = strings.TrimSuffix(line, ":")
			continue
		}
		if currentInterface == "" || !strings.Contains(strings.ToLower(currentInterface), strings.ToLower(interfaceName)) {
			continue
		}
		if strings.Contains(line, "Address") && strings.Contains(line, "Parameters") {
			re := regexp.MustCompile(`([a-fA-F0-9:]+)/(\d+)`)
			match := re.FindStringSubmatch(line)
			if len(match) >= 3 {
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
		}
	}
	return "", fmt.Errorf("未找到全局IPv6地址")
}

// Windows特有的PowerShell方法获取IPv6信息
func getPrefixFromPowerShell(interfaceName string) (string, error) {
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
		if strings.HasPrefix(ipv6Addr, "fe80") || // 链路本地地址
			strings.HasPrefix(ipv6Addr, "::1") || // 回环地址
			strings.HasPrefix(ipv6Addr, "fc") || // 本地唯一地址
			strings.HasPrefix(ipv6Addr, "fd") || // 本地唯一地址
			strings.HasPrefix(ipv6Addr, "ff") { // 组播地址
			continue
		}
		prefixLen := prefixMatches[i][1]
		prefixLenInt, err := strconv.Atoi(prefixLen)
		if err != nil || prefixLenInt < 0 || prefixLenInt > 128 {
			continue
		}
		return prefixLen, nil
	}
	return "", fmt.Errorf("未找到全局IPv6地址")
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
