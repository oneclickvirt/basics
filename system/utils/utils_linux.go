package utils

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

// GetCpuCache 查询CPU三缓
func GetCpuCache() string {
	return ""
}

func CheckCPUFeatureWindows(subkey string, value string) (string, bool) {
	return "", false
}

func CheckVMTypeWithWIMC() string {
	return ""
}

func GetLoad1() float64 {
	return 0
}

// GetTCPAccelerateStatus 查询TCP控制算法
func GetTCPAccelerateStatus() string {
	cmd := exec.Command("sysctl", "-n", "net.ipv4.tcp_congestion_control")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return ""
	} else {
		return out.String()
	}
}

// 解析时区信息
func parseTimeZone(output string) string {
	// 在输出中查找 Time zone 字符串
	index := strings.Index(output, "Time zone")
	if index != -1 {
		// 如果找到，则截取 Time zone 字符串后的部分
		output = output[index+len("Time zone")+1:]
		// 找到冒号后的第一个空格，分割字符串获取时区信息
		parts := strings.SplitN(output, " ", 2)
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// GetTimeZone 获取当前时区
func GetTimeZone() string {
	var CurrentTimeZone string
	output, err := exec.Command("timedatectl", "|", "grep", "Time zone").Output()
	if err == nil && strings.Contains(string(output), "Time zone") {
		timeZone := parseTimeZone(string(output))
		CurrentTimeZone = timeZone
	} else {
		output, err = exec.Command("date", "+%Z").Output()
		if err == nil {
			timeZone := strings.TrimSpace(string(output))
			CurrentTimeZone = timeZone
		}
	}
	return CurrentTimeZone
}

// GetPATH 检测本机的PATH环境是否含有对应的命令
func GetPATH(key string) (string, bool) {
	// 指定要搜索的目录列表
	dirs := []string{"/usr/local/bin", "/usr/local/sbin", "/usr/bin", "/usr/sbin", "/sbin", "/bin", "/snap/bin"}
	// 循环遍历每个目录
	for _, dir := range dirs {
		// 使用 filepath.Join 来构造文件路径
		filePath := dir + "/" + key
		// 检查文件是否存在且可执行
		if fileInfo, err := os.Stat(filePath); err == nil && fileInfo.Mode().IsRegular() && (fileInfo.Mode()&0111 != 0) {
			return filePath, true
		}
	}
	return "", false
}
