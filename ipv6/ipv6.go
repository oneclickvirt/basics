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

// 获取第一个以 eth 或 en 开头的网络接口
func getInterface() (string, error) {
	cmd := exec.Command("sh", "-c", "ls /sys/class/net/ | grep -E '^(eth|en)' | head -n 1")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// 获取当前的公网 IPv6 地址
func getCurrentIPv6() (string, error) {
	cmd := exec.Command("curl", "-s", "-6", "-m", "5", "ipv6.ip.sb")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// Router Advertisement前缀选项类型
const (
	ICMPv6RouterAdvertisement = 134
	ICMPv6OptionPrefix        = 3
)

// ICMPv6报文头部结构
type ICMPv6Header struct {
	Type     uint8
	Code     uint8
	Checksum uint16
}

// Router Advertisement报文结构
type RouterAdvertisement struct {
	CurHopLimit    uint8
	Flags          uint8
	RouterLifetime uint16
	ReachableTime  uint32
	RetransTimer   uint32
	Options        []byte
}

// 前缀信息选项结构
type PrefixInfoOption struct {
	Type              uint8
	Length            uint8
	PrefixLength      uint8
	Flags             uint8
	ValidLifetime     uint32
	PreferredLifetime uint32
	Reserved          uint32
	Prefix            [16]byte
}

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
	go func() {
		// 实际应用中可能需要构造并发送Router Solicitation消息
		// 这里为简化代码，仅依赖网络上周期性的Router Advertisement
	}()
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

// 方法3：从配置文件获取前缀长度
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

// 获取 IPv6 子网掩码
// 获取 IPv6 子网掩码
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
		if language == "en" {
			return fmt.Sprintf(" IPv6 Mask           : /%s", prefixLen), nil
		}
		return fmt.Sprintf(" IPv6 子网掩码       : /%s", prefixLen), nil
	}
	// 优先级2：从ip命令获取前缀长度
	prefixLen, err = getPrefixFromIPCommand(interfaceName)
	if err == nil && prefixLen != "" {
		if language == "en" {
			return fmt.Sprintf(" IPv6 Mask           : /%s", prefixLen), nil
		}
		return fmt.Sprintf(" IPv6 子网掩码       : /%s", prefixLen), nil
	}
	// 优先级3：从配置文件获取前缀长度
	prefixLen, err = getPrefixFromConfigFiles()
	if err == nil && prefixLen != "" {
		if language == "en" {
			return fmt.Sprintf(" IPv6 Mask           : /%s", prefixLen), nil
		}
		return fmt.Sprintf(" IPv6 子网掩码       : /%s", prefixLen), nil
	}
	// 如果以上方法都失败但确实有公网IPv6，使用/128作为默认子网掩码
	if language == "en" {
		return " IPv6 Mask           : /128", nil
	}
	return " IPv6 子网掩码       : /128", nil
}
