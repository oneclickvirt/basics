package system

import (
	"fmt"
	"runtime"
	"strconv"

	"github.com/oneclickvirt/basics/model"
	"github.com/oneclickvirt/basics/system/utils"
	precheckUtils "github.com/oneclickvirt/basics/utils"
	. "github.com/oneclickvirt/defaultset"
)

// GetSystemInfo 获取主机硬件信息
func GetSystemInfo() *model.SystemInfo {
	if model.EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	var ret = &model.SystemInfo{}
	var err error
	if runtime.GOOS == "darwin" {
		getMacOSInfo()
	}
	// 系统信息查询
	cpuType, ret.Uptime, ret.Platform, ret.Kernel, ret.Arch, ret.VmType, ret.NatType, ret.TimeZone, err = getHostInfo()
	if err != nil && model.EnableLoger {
		Logger.Info(err.Error())
	}
	// CPU信息查询
	ret, err = getCpuInfo(ret, cpuType)
	if err != nil && model.EnableLoger {
		Logger.Info(err.Error())
	}
	// GPU信息查询
	ret, err = getGPUInfo(ret)
	if err != nil && model.EnableLoger {
		Logger.Info(err.Error())
	}
	// 硬盘信息查询
	ret.DiskTotal, ret.DiskUsage, ret.Percentage, ret.DiskRealPath, ret.BootPath, err = getDiskInfo()
	if err != nil && model.EnableLoger {
		Logger.Info(err.Error())
	}
	// 内存信息查询
	ret.MemoryTotal, ret.MemoryUsage, ret.SwapTotal, ret.SwapUsage, ret.VirtioBalloon, ret.KSM = getMemoryInfo()
	// 获取负载信息
	load1, load5, load15, err := getSystemLoad()
	if err != nil {
		load1, load5, load15 = 0, 0, 0
	}
	ret.Load = strconv.FormatFloat(load1, 'f', 2, 64) + " / " +
		strconv.FormatFloat(load5, 'f', 2, 64) + " / " +
		strconv.FormatFloat(load15, 'f', 2, 64)
	// 获取TCP控制算法
	ret.TcpAccelerationMethod = utils.GetTCPAccelerateStatus()
	return ret
}

func CheckSystemInfo(language string) string {
	ret := GetSystemInfo()
	var res string
	if language == "en" {
		res += " CPU Model           : " + ret.CpuModel + "\n"
		res += " CPU Cores           : " + ret.CpuCores + "\n"
		if ret.CpuCache != "" {
			res += " CPU Cache           : " + ret.CpuCache + "\n"
		}
		if ret.GpuModel != "" && ret.GpuModel != "unknown" {
			res += " GPU Model           : " + ret.GpuModel + "\n"
			if ret.GpuStats != "" && ret.GpuStats != "0" {
				res += " GPU Stats           : " + ret.GpuStats + "\n"
			}
		}
		if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
			res += " AES-NI              : " + ret.CpuAesNi + "\n"
		}
		if runtime.GOOS != "darwin" {
			res += " VM-x/AMD-V/Hyper-V  : " + ret.CpuVAH + "\n"
		}
		res += " RAM                 : " + ret.MemoryUsage + " / " + ret.MemoryTotal + "\n"
		if ret.VirtioBalloon != "" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
			res += " Virtio Balloon      : " + ret.VirtioBalloon + "\n"
		}
		if ret.KSM != "" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
			res += " KSM                 : " + ret.KSM + "\n"
		}
		if ret.SwapTotal == "" && ret.SwapUsage == "" {
			res += " Swap                : [ no swap partition or swap file detected ]" + "\n"
		} else if ret.SwapTotal != "" && ret.SwapUsage != "" {
			res += " Swap                : " + ret.SwapUsage + " / " + ret.SwapTotal + "\n"
		}
		for i := 0; i < len(ret.DiskUsage); i++ {
			var label string
			if i == 0 && len(ret.DiskUsage) == 1 {
				label = "Disk"
			} else {
				label = fmt.Sprintf("Disk %d", i+1)
			}
			res += fmt.Sprintf(" %-20s: %s / %s", label, ret.DiskUsage[i], ret.DiskTotal[i])
			if i < len(ret.Percentage) && ret.Percentage[i] != "" {
				res += fmt.Sprintf(" [%s]", ret.Percentage[i])
				if ret.DiskRealPath[i] != "" {
					res += fmt.Sprintf(" %s\n", ret.DiskRealPath[i])
				} else {
					res += "\n"
				}
			} else {
				res += "\n"
			}
		}
		if ret.BootPath != "" {
			res += " Boot Path           : " + ret.BootPath + "\n"
		}
		res += " OS Release          : " + ret.Platform + " [" + ret.Arch + "] " + "\n"
		if ret.Kernel != "" {
			res += " Kernel              : " + ret.Kernel + "\n"
		}
		res += " Uptime              : " + ret.Uptime + "\n"
		res += " Current Time Zone   : " + ret.TimeZone + "\n"
		res += " Load                : " + ret.Load + "\n"
		res += " VM Type             : " + ret.VmType + "\n"
		if ret.NatType != "" {
			res += " NAT Type            : " + ret.NatType + "\n"
		}
		if ret.TcpAccelerationMethod != "" {
			res += " Tcp Accelerate      : " + ret.TcpAccelerationMethod + "\n"
		}

	} else if language == "zh" {
		res += " CPU 型号            : " + ret.CpuModel + "\n"
		res += " CPU 数量            : " + ret.CpuCores + "\n"
		if ret.CpuCache != "" {
			res += " CPU 缓存            : " + ret.CpuCache + "\n"
		}
		if ret.GpuModel != "" && ret.GpuModel != "unknown" {
			res += " GPU 型号            : " + ret.GpuModel + "\n"
			if ret.GpuStats != "" && ret.GpuStats != "0" {
				res += " GPU 状态            : " + ret.GpuStats + "\n"
			}
		}
		if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
			res += " AES-NI              : " + ret.CpuAesNi + "\n"
		}
		if runtime.GOOS != "darwin" {
			res += " VM-x/AMD-V/Hyper-V  : " + ret.CpuVAH + "\n"
		}
		res += " 内存                : " + ret.MemoryUsage + " / " + ret.MemoryTotal + "\n"
		if ret.VirtioBalloon != "" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
			res += " 气球驱动            : " + ret.VirtioBalloon + "\n"
		}
		if ret.KSM != "" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
			res += " 内核页合并          : " + ret.KSM + "\n"
		}
		if ret.SwapTotal == "" && ret.SwapUsage == "" {
			res += " 虚拟内存 Swap       : [ no swap partition or swap file detected ]" + "\n"
		} else if ret.SwapTotal != "" && ret.SwapUsage != "" {
			res += " 虚拟内存 Swap       : " + ret.SwapUsage + " / " + ret.SwapTotal + "\n"
		}
		for i := 0; i < len(ret.DiskUsage); i++ {
			var label string
			if i == 0 && len(ret.DiskUsage) == 1 {
				label = "硬盘空间"
			} else {
				label = fmt.Sprintf("硬盘空间 Disk %d", i+1)
			}
			res += fmt.Sprintf(" %-16s: %s / %s", label, ret.DiskUsage[i], ret.DiskTotal[i])
			if i < len(ret.Percentage) && ret.Percentage[i] != "" {
				res += fmt.Sprintf(" [%s]", ret.Percentage[i])
				if ret.DiskRealPath[i] != "" {
					res += fmt.Sprintf(" %s\n", ret.DiskRealPath[i])
				} else {
					res += "\n"
				}
			} else {
				res += "\n"
			}
		}
		if ret.BootPath != "" {
			res += " 启动盘路径          : " + ret.BootPath + "\n"
		}
		res += " 系统                : " + ret.Platform + " [" + ret.Arch + "] " + "\n"
		if ret.Kernel != "" {
			res += " 内核                : " + ret.Kernel + "\n"
		}
		res += " 系统在线时间        : " + ret.Uptime + "\n"
		res += " 时区                : " + ret.TimeZone + "\n"
		res += " 负载                : " + ret.Load + "\n"
		res += " 虚拟化架构          : " + ret.VmType + "\n"
		if ret.NatType != "" {
			res += " NAT类型             : " + ret.NatType + "\n"
		}
		if ret.TcpAccelerationMethod != "" {
			res += " TCP加速方式         : " + ret.TcpAccelerationMethod + "\n"
		}
	}
	return res
}
