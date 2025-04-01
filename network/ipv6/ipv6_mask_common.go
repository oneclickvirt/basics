package ipv6

import (
	"fmt"
	"net"
	"strings"
)

// 获取第一个以 eth 或 en 开头的网络接口
func getInterface() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	// 优先查找以 eth 或 en 开头的接口
	for _, iface := range interfaces {
		if strings.HasPrefix(iface.Name, "eth") || strings.HasPrefix(iface.Name, "en") {
			return iface.Name, nil
		}
	}
	// 如果没有找到，返回第一个非回环且启用的接口
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback == 0 && iface.Flags&net.FlagUp != 0 {
			return iface.Name, nil
		}
	}
	return "", fmt.Errorf("未找到合适的网络接口")
}

// 判断是否为非全局单播地址前缀
func isNonGlobalPrefix(prefix [16]byte) bool {
	// 链路本地地址 fe80::/10
	if prefix[0] == 0xfe && (prefix[1]&0xc0) == 0x80 {
		return true
	}
	// 唯一本地地址 fc00::/7
	if (prefix[0] & 0xfe) == 0xfc {
		return true
	}
	// 回环地址 ::1
	if prefix[0] == 0 && prefix[1] == 0 && prefix[15] == 1 {
		return true
	}
	// 组播地址 ff00::/8
	if prefix[0] == 0xff {
		return true
	}
	return false
}

// 格式化返回IPv6子网掩码
func formatIPv6Mask(prefixLen string, language string) string {
	if language == "en" {
		return fmt.Sprintf(" IPv6 Mask           : /%s\n", prefixLen)
	}
	return fmt.Sprintf(" IPv6 子网掩码       : /%s\n", prefixLen)
}
