package system

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/oneclickvirt/basics/model"
	"github.com/shirou/gopsutil/v4/mem"
)

func getMemoryInfo() (string, string, string, string, string, string) {
	var memoryTotalStr, memoryUsageStr, swapTotalStr, swapUsageStr, virtioBalloonStatus, KernelSamepageMerging string
	mv, err := mem.VirtualMemory()
	if err != nil {
		println("mem.VirtualMemory error:", err)
	} else {
		memoryTotal := float64(mv.Total)
		memoryUsage := float64(mv.Total - mv.Available)
		if memoryTotal < 1024*1024*1024 {
			memoryTotalStr = fmt.Sprintf("%.2f MB", memoryTotal/(1024*1024))
		} else {
			memoryTotalStr = fmt.Sprintf("%.2f GB", memoryTotal/(1024*1024*1024))
		}
		if memoryUsage < 1024*1024*1024 {
			memoryUsageStr = fmt.Sprintf("%.2f MB", memoryUsage/(1024*1024))
		} else {
			memoryUsageStr = fmt.Sprintf("%.2f GB", memoryUsage/(1024*1024*1024))
		}
		if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
			swapTotal := float64(mv.SwapTotal)
			swapUsage := float64(mv.SwapTotal - mv.SwapFree)
			if swapTotal != 0 {
				if swapTotal < 1024*1024*1024 {
					swapTotalStr = fmt.Sprintf("%.2f MB", swapTotal/(1024*1024))
				} else {
					swapTotalStr = fmt.Sprintf("%.2f GB", swapTotal/(1024*1024*1024))
				}
				if swapUsage < 1024*1024*1024 {
					swapUsageStr = fmt.Sprintf("%.2f MB", swapUsage/(1024*1024))
				} else {
					swapUsageStr = fmt.Sprintf("%.2f GB", swapUsage/(1024*1024*1024))
				}
			}
		}
	}
	// macOS 特殊处理：内存和 swap
	if runtime.GOOS == "darwin" {
		if len(model.MacOSInfo) > 0 {
			for _, line := range model.MacOSInfo {
				if strings.Contains(line, "Memory") {
					memoryTotalStr = strings.TrimSpace(strings.Split(line, ":")[1])
				}
			}
		}
		output, err := exec.Command("sysctl", "vm.swapusage").Output()
		if err == nil {
			// 输出示例: "vm.swapusage: total = 2048.00M  used = 1021.25M  free = 1026.75M  (encrypted)"
			fields := strings.Fields(string(output))
			if len(fields) >= 7 {
				totalVal, err1 := strconv.ParseFloat(strings.TrimSuffix(fields[3], "M"), 64)
				usedVal, err2 := strconv.ParseFloat(strings.TrimSuffix(fields[6], "M"), 64)
				if err1 == nil && err2 == nil {
					if totalVal >= 1024 {
						swapTotalStr = fmt.Sprintf("%.2f GB", totalVal/1024)
					} else {
						swapTotalStr = fmt.Sprintf("%.2f MB", totalVal)
					}
					if usedVal >= 1024 {
						swapUsageStr = fmt.Sprintf("%.2f GB", usedVal/1024)
					} else {
						swapUsageStr = fmt.Sprintf("%.2f MB", usedVal)
					}
				}
			}
		}
	}
	// Windows 特殊处理 swap（gopsutil 的 VirtualMemory 在 Win 上不准确）
	if runtime.GOOS == "windows" {
		ms, err := mem.SwapMemory()
		if err != nil {
			println("mem.SwapMemory error:", err)
		} else {
			swapTotal := float64(ms.Total)
			swapUsage := float64(ms.Used)
			if swapTotal != 0 {
				if swapTotal < 1024*1024*1024 {
					swapTotalStr = fmt.Sprintf("%.2f MB", swapTotal/(1024*1024))
				} else {
					swapTotalStr = fmt.Sprintf("%.2f GB", swapTotal/(1024*1024*1024))
				}
				if swapUsage < 1024*1024*1024 {
					swapUsageStr = fmt.Sprintf("%.2f MB", swapUsage/(1024*1024))
				} else {
					swapUsageStr = fmt.Sprintf("%.2f GB", swapUsage/(1024*1024*1024))
				}
			}
		}
	}
	// virtio_balloon 检测（Linux）
	virtioBalloon, err := os.ReadFile("/proc/modules")
	if err == nil && strings.Contains(string(virtioBalloon), "virtio_balloon") {
		if runtime.GOOS == "windows" {
			virtioBalloonStatus = "[Y] Enabled"
		} else {
			virtioBalloonStatus = "✔️ Enabled"
		}
	}
	if virtioBalloonStatus == "" {
		if runtime.GOOS == "windows" {
			virtioBalloonStatus = "[N] Undetected"
		} else {
			virtioBalloonStatus = "❌ Undetected"
		}
	}
	// KSM 状态检测（Linux）
	ksmStatus, err := os.ReadFile("/sys/kernel/mm/ksm/run")
	if err == nil && strings.Contains(string(ksmStatus), "1") {
		if runtime.GOOS == "windows" {
			KernelSamepageMerging = "[Y] Enabled"
		} else {
			KernelSamepageMerging = "✔️ Enabled"
		}
	}
	if KernelSamepageMerging == "" {
		if runtime.GOOS == "windows" {
			KernelSamepageMerging = "[N] Undetected"
		} else {
			KernelSamepageMerging = "❌ Undetected"
		}
	}
	return memoryTotalStr, memoryUsageStr, swapTotalStr, swapUsageStr, virtioBalloonStatus, KernelSamepageMerging
}
