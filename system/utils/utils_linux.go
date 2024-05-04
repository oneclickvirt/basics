package utils

import (
	"bytes"
	"os/exec"
	"time"
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

// GetTimeZone 获取当前时区
func GetTimeZone() string {
	local := time.Now().Location()
	CurrentTimeZone := local.String()
	return CurrentTimeZone
}
