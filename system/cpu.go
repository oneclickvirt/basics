package system

import (
	"fmt"
	"os"
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

func getCpuInfo(ret *model.SystemInfo, cpuType string) (*model.SystemInfo, error) {
	var aesFeature, virtFeature, hypervFeature string
	var st bool
	ci, err := cpu.Info()
	if err != nil {
		return nil, fmt.Errorf("cpu.Info error: %v", err.Error())
	} else {
		ret.CpuModel = ""
		for i := 0; i < len(ci); i++ {
			if len(ret.CpuModel) < len(ci[i].ModelName) {
				ret.CpuModel = ci[i].ModelName + fmt.Sprintf(" %d %s Core", len(ci), cpuType) + " @ " +
					strconv.FormatFloat(ci[i].Mhz, 'f', 2, 64) + " MHz"
				ret.CpuCores = fmt.Sprintf("%d vCPU(s)", runtime.NumCPU())
				if ci[i].CacheSize != 0 { // Windows查不到CPU的三缓
					ret.CpuCache = string(ci[i].CacheSize)
				}
			}
		}
	}
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
	// 查询CPU的三缓
	if runtime.GOOS == "windows" {
		ret.CpuCache = utils.GetCpuCache()
	}
	return ret, nil
}
