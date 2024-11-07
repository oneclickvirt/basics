package ipv6

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// GetIPv6Mask 匹配获取公网 IPV6 的掩码信息
func GetIPv6Mask(language string) (string, error) {
	interfaceName := getNetworkInterface()
	if interfaceName == "" {
		return "", fmt.Errorf("无法获取网络接口名称")
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipv6 := ipnet.IP.To16(); ipv6 != nil {
				if !ipv6.IsLinkLocalUnicast() && !isIPv6LinkLocal(ipv6) && !isIPv6SiteLocal(ipv6) {
					newIPv6 := generateNewIPv6(ipv6.String())
					addIPv6Address(interfaceName, newIPv6)
					defer removeIPv6Address(interfaceName, newIPv6)
					updatedAddrs, err := net.InterfaceAddrs()
					if err != nil {
						return "", err
					}
					if len(updatedAddrs) == len(addrs) {
						_, bits := ipnet.Mask.Size()
						if language == "en" {
							return fmt.Sprintf(" IPV6 Mask           : /%d\n", bits), nil
						} else {
							return fmt.Sprintf(" IPV6 子网掩码       : /%d\n", bits), nil
						}
					}
					for _, updatedAddr := range updatedAddrs {
						if updatedIPnet, ok := updatedAddr.(*net.IPNet); ok {
							if updatedIPv6 := updatedIPnet.IP.To16(); updatedIPv6 != nil {
								if !isIPv6LinkLocal(updatedIPv6) && !isIPv6SiteLocal(updatedIPv6) && updatedIPv6.String() != ipv6.String() {
									_, bits := updatedIPnet.Mask.Size()
									if language == "en" {
										return fmt.Sprintf(" IPV6 Mask           : /%d\n", bits), nil
									} else {
										return fmt.Sprintf(" IPV6 子网掩码       : /%d\n", bits), nil
									}
								} else if !isIPv6LinkLocal(updatedIPv6) && !isIPv6SiteLocal(updatedIPv6) && updatedIPv6.String() == ipv6.String() {
									if language == "en" {
										return " IPV6 Mask           : /128", nil
									} else {
										return " IPV6 子网掩码       : /128", nil
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return "", fmt.Errorf("无法获取公网 IPv6 地址")
}

func getNetworkInterface() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range ifaces {
		if strings.HasPrefix(iface.Name, "eth") || strings.HasPrefix(iface.Name, "en") {
			return iface.Name
		}
	}

	return ""
}

func generateNewIPv6(currentIPv6 string) string {
	parts := strings.Split(currentIPv6, ":")
	if len(parts) < 8 {
		return ""
	}
	return fmt.Sprintf("%s:%s", strings.Join(parts[:7], ":"), "3")
}

func addIPv6Address(interfaceName, ipv6Address string) {
	_, err := exec.Command("ip", "addr", "add", ipv6Address+"/128", "dev", interfaceName).Output()
	if err != nil {
		return
	}
}

func removeIPv6Address(interfaceName, ipv6Address string) {
	_, err := exec.Command("ip", "addr", "del", ipv6Address+"/128", "dev", interfaceName).Output()
	if err != nil {
		return
	}
}

func isIPv6LinkLocal(ip net.IP) bool {
	return strings.HasPrefix(ip.String(), "fe80:")
}

func isIPv6SiteLocal(ip net.IP) bool {
	return strings.HasPrefix(ip.String(), "fec0:")
}
