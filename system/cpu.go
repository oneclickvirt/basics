package system

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/oneclickvirt/basics/system/model"
	"github.com/oneclickvirt/basics/system/utils"
	"github.com/shirou/gopsutil/cpu"
)

func checkCPUFeatureLinux(filename string, feature string) (string, bool) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "Error reading file", false
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.Contains(line, feature) {
			return "✔️ Enabled", true
		}
	}
	return "❌ Disabled", false
}

func checkCPUFeature(filename string, feature string) (string, bool) {
	if runtime.GOOS == "windows" {
		return utils.CheckCPUFeatureWindows(filename, feature)
	} else if runtime.GOOS == "linux" {
		return checkCPUFeatureLinux(filename, feature)
	}
	return "Unsupported OS", false
}

// convertBytes 转换字节数
func convertBytes(bytes int64) (string, int64) {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return "GB", bytes / GB
	case bytes >= MB:
		return "MB", bytes / MB
	case bytes >= KB:
		return "KB", bytes / KB
	default:
		return "Bytes", bytes
	}
}

func getCpuInfo(ret *model.SystemInfo, cpuType string) (*model.SystemInfo, error) {
	var aesFeature, virtFeature, hypervFeature string
	var st bool
	ret.CpuCores = fmt.Sprintf("%d %s CPU(s)", runtime.NumCPU(), cpuType)
	if runtime.GOOS == "windows" {
		ci, err := cpu.Info()
		if err != nil {
			return nil, fmt.Errorf("cpu.Info error: %v", err.Error())
		} else {
			for i := 0; i < len(ci); i++ {
				if len(ret.CpuModel) < len(ci[i].ModelName) {
					ret.CpuModel = strings.TrimSpace(ci[i].ModelName)
				}
			}
		}
		ret.CpuCache = utils.GetCpuCache()
	} else {
		var L1dcache, L1icache, L1cache, L2cache, L3cache string
		cmd := exec.Command("lscpu", "-B") // 以字节数为单位查询
		output, err := cmd.Output()
		if err == nil {
			outputStr := string(output)
			lines := strings.Split(outputStr, "\n")
			for _, line := range lines {
				fields := strings.Split(line, ":")
				if len(fields) >= 2 {
					if strings.Contains(fields[0], "Model name") {
						ret.CpuModel = strings.TrimSpace(strings.Join(fields[1:], " "))
					} else if strings.Contains(fields[0], "L1d cache") {
						L1dcache = strings.TrimSpace(strings.Join(fields[1:], " "))
					} else if strings.Contains(fields[0], "L1i cache") {
						L1icache = strings.TrimSpace(strings.Join(fields[1:], " "))
					} else if strings.Contains(fields[0], "L2 cache") {
						L2cache = strings.TrimSpace(strings.Join(fields[1:], " "))
					} else if strings.Contains(fields[0], "L3 cache") {
						L3cache = strings.TrimSpace(strings.Join(fields[1:], " "))
					}
				}
			}
		}
		if L1dcache != "" && L1icache != "" && L2cache != "" && L3cache != "" {
			bytes1, err1 := strconv.ParseInt(L1dcache, 10, 64)
			bytes2, err2 := strconv.ParseInt(L1icache, 10, 64)
			if err1 == nil && err2 == nil {
				bytes3 := bytes1 + bytes2
				unit, size := convertBytes(bytes3)
				L1cache = fmt.Sprintf("L1: %d %s", size, unit)
			}
			bytes4, err4 := strconv.ParseInt(L2cache, 10, 64)
			if err4 == nil {
				unit, size := convertBytes(bytes4)
				L2cache = fmt.Sprintf("L2: %d %s", size, unit)
			}
			bytes5, err5 := strconv.ParseInt(L3cache, 10, 64)
			if err5 == nil {
				unit, size := convertBytes(bytes5)
				L3cache = fmt.Sprintf("L3: %d %s", size, unit)
			}
			if err1 == nil && err2 == nil && err4 == nil && err5 == nil {
				ret.CpuCache = L1cache + " / " + L2cache + " / " + L3cache
			}
		}
	}
	// 获取虚拟化架构
	if runtime.GOOS == "windows" {
		aesFeature = `HARDWARE\DESCRIPTION\System\CentralProcessor\0`
		virtFeature = `HARDWARE\DESCRIPTION\System\CentralProcessor\0`
		hypervFeature = `SYSTEM\CurrentControlSet\Control\Hypervisor\0`
	} else if runtime.GOOS == "linux" {
		aesFeature = "/proc/cpuinfo"
		virtFeature = "/proc/cpuinfo"
		hypervFeature = "/proc/cpuinfo"
	}
	ret.CpuAesNi, _ = checkCPUFeature(aesFeature, "aes")
	ret.CpuVAH, st = checkCPUFeature(virtFeature, "vmx")
	if !st {
		ret.CpuVAH, _ = checkCPUFeature(hypervFeature, "hypervisor")
	}
	return ret, nil
}
