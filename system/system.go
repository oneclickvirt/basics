package system

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"

	"github.com/oneclickvirt/basics/model"
	"github.com/oneclickvirt/basics/system/utils"
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
	// 系统信息查询（必须先完成，后续 CPU/GPU 依赖 cpuType）
	cpuType, ret.Uptime, ret.Platform, ret.Kernel, ret.Arch, ret.VmType, ret.NatType, ret.TimeZone, err = getHostInfo()
	if err != nil && model.EnableLoger {
		Logger.Info(err.Error())
	}

	// CPU、GPU 需顺序执行（共享 ret 指针，内部直接写字段）
	// 先执行 CPU，再执行 GPU，避免数据竞争
	var cpuErr error
	ret, cpuErr = getCpuInfo(ret, cpuType)
	if cpuErr != nil && model.EnableLoger {
		Logger.Info(cpuErr.Error())
	}
	gpuRet, gpuErr := getGPUInfo(ret)
	if gpuErr != nil && model.EnableLoger {
		Logger.Info(gpuErr.Error())
	}
	ret = gpuRet

	// 硬盘、内存、负载、TCP加速 互不依赖，并发执行
	var wg sync.WaitGroup

	var (
		diskTotal     []string
		diskUsage     []string
		percentage    []string
		diskRealPath  []string
		bootPath      string
		memTotal      string
		memUsage      string
		swapTotal     string
		swapUsage     string
		virtioBalloon string
		ksm           string
		load1         float64
		load5         float64
		load15        float64
		tcpAccel      string
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				diskTotal, diskUsage, percentage, diskRealPath, bootPath = nil, nil, nil, nil, ""
				if model.EnableLoger {
					Logger.Info(fmt.Sprintf("panic in getDiskInfo: %v\n%s", r, string(debug.Stack())))
				}
			}
		}()
		diskTotal, diskUsage, percentage, diskRealPath, bootPath, _ = getDiskInfo()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				memTotal, memUsage, swapTotal, swapUsage, virtioBalloon, ksm = "", "", "", "", "", ""
				if model.EnableLoger {
					Logger.Info(fmt.Sprintf("panic in getMemoryInfo: %v\n%s", r, string(debug.Stack())))
				}
			}
		}()
		memTotal, memUsage, swapTotal, swapUsage, virtioBalloon, ksm = getMemoryInfo()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				load1, load5, load15 = 0, 0, 0
				if model.EnableLoger {
					Logger.Info(fmt.Sprintf("panic in getSystemLoad: %v\n%s", r, string(debug.Stack())))
				}
			}
		}()
		var e error
		load1, load5, load15, e = getSystemLoad()
		if e != nil {
			load1, load5, load15 = 0, 0, 0
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				tcpAccel = ""
				if model.EnableLoger {
					Logger.Info(fmt.Sprintf("panic in GetTCPAccelerateStatus: %v\n%s", r, string(debug.Stack())))
				}
			}
		}()
		tcpAccel = utils.GetTCPAccelerateStatus()
	}()

	wg.Wait()

	// 所有并发任务完成，将结果写入 ret
	ret.DiskTotal = diskTotal
	ret.DiskUsage = diskUsage
	ret.Percentage = percentage
	ret.DiskRealPath = diskRealPath
	ret.BootPath = bootPath
	ret.MemoryTotal = memTotal
	ret.MemoryUsage = memUsage
	ret.SwapTotal = swapTotal
	ret.SwapUsage = swapUsage
	ret.VirtioBalloon = virtioBalloon
	ret.KSM = ksm
	ret.Load = strconv.FormatFloat(load1, 'f', 2, 64) + " / " +
		strconv.FormatFloat(load5, 'f', 2, 64) + " / " +
		strconv.FormatFloat(load15, 'f', 2, 64)
	ret.TcpAccelerationMethod = tcpAccel
	return ret
}

func CheckSystemInfo(language string) string {
	ret := GetSystemInfo()
	report := CollectSystemReport(context.Background())
	var res string
	row := func(label, value string) { res += formatReportRow(label, value) }
	if language == "en" {
		row("CPU Model", ret.CpuModel)
		row("CPU Cores", ret.CpuCores)
		if ret.CpuCache != "" {
			row("CPU Cache", ret.CpuCache)
		}
		if ret.GpuModel != "" && ret.GpuModel != "unknown" {
			row("GPU Model", ret.GpuModel)
			if ret.GpuStats != "" && ret.GpuStats != "0" {
				row("GPU Stats", ret.GpuStats)
			}
		}
		if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
			row("AES-NI", ret.CpuAesNi)
		}
		if runtime.GOOS != "darwin" {
			row("VM-x/AMD-V/Hyper-V", ret.CpuVAH)
		}
		row("RAM", ret.MemoryUsage+" / "+ret.MemoryTotal)
		if ret.VirtioBalloon != "" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
			row("Virtio Balloon", ret.VirtioBalloon)
		}
		if ret.KSM != "" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
			row("KSM", ret.KSM)
		}
		if ret.SwapTotal == "" && ret.SwapUsage == "" {
			row("Swap", "[ no swap partition or swap file detected ]")
		} else if ret.SwapTotal != "" && ret.SwapUsage != "" {
			row("Swap", ret.SwapUsage+" / "+ret.SwapTotal)
		}
		for i := 0; i < len(ret.DiskUsage); i++ {
			var label string
			if i == 0 && len(ret.DiskUsage) == 1 {
				label = "Disk"
			} else {
				label = fmt.Sprintf("Disk %d", i+1)
			}
			value := fmt.Sprintf("%s / %s", ret.DiskUsage[i], ret.DiskTotal[i])
			if i < len(ret.Percentage) && ret.Percentage[i] != "" {
				value += fmt.Sprintf(" [%s]", ret.Percentage[i])
				if ret.DiskRealPath[i] != "" {
					value += " " + ret.DiskRealPath[i]
				}
			}
			row(label, value)
		}
		if ret.BootPath != "" {
			row("Boot Path", ret.BootPath)
		}
		row("OS Release", ret.Platform+" ["+ret.Arch+"]")
		if ret.Kernel != "" {
			row("Kernel", ret.Kernel)
		}
		row("Uptime", ret.Uptime)
		row("Current Time Zone", ret.TimeZone)
		row("Load", ret.Load)
		row("VM Type", ret.VmType)
		if ret.NatType != "" {
			row("NAT Type", ret.NatType)
		}
		tcpAcceleration := ret.TcpAccelerationMethod
		if tcpAcceleration == "" {
			tcpAcceleration = report.Network.CongestionControl
		}
		res += renderExtendedSystemReportText(report, language, tcpAcceleration)

	} else if language == "zh" {
		row("CPU 型号", ret.CpuModel)
		row("CPU 数量", ret.CpuCores)
		if ret.CpuCache != "" {
			row("CPU 缓存", ret.CpuCache)
		}
		if ret.GpuModel != "" && ret.GpuModel != "unknown" {
			row("GPU 型号", ret.GpuModel)
			if ret.GpuStats != "" && ret.GpuStats != "0" {
				row("GPU 状态", ret.GpuStats)
			}
		}
		if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
			row("AES-NI", ret.CpuAesNi)
		}
		if runtime.GOOS != "darwin" {
			row("VM-x/AMD-V/Hyper-V", ret.CpuVAH)
		}
		row("内存", ret.MemoryUsage+" / "+ret.MemoryTotal)
		if ret.VirtioBalloon != "" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
			row("气球驱动", ret.VirtioBalloon)
		}
		if ret.KSM != "" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
			row("内核页合并", ret.KSM)
		}
		if ret.SwapTotal == "" && ret.SwapUsage == "" {
			row("虚拟内存 Swap", "[ no swap partition or swap file detected ]")
		} else if ret.SwapTotal != "" && ret.SwapUsage != "" {
			row("虚拟内存 Swap", ret.SwapUsage+" / "+ret.SwapTotal)
		}
		for i := 0; i < len(ret.DiskUsage); i++ {
			var label string
			if i == 0 && len(ret.DiskUsage) == 1 {
				label = "硬盘空间"
			} else {
				label = fmt.Sprintf("硬盘空间 Disk %d", i+1)
			}
			value := fmt.Sprintf("%s / %s", ret.DiskUsage[i], ret.DiskTotal[i])
			if i < len(ret.Percentage) && ret.Percentage[i] != "" {
				value += fmt.Sprintf(" [%s]", ret.Percentage[i])
				if ret.DiskRealPath[i] != "" {
					value += " " + ret.DiskRealPath[i]
				}
			}
			row(label, value)
		}
		if ret.BootPath != "" {
			row("启动盘路径", ret.BootPath)
		}
		row("系统", ret.Platform+" ["+ret.Arch+"]")
		if ret.Kernel != "" {
			row("内核", ret.Kernel)
		}
		row("系统在线时间", ret.Uptime)
		row("时区", ret.TimeZone)
		row("负载", ret.Load)
		row("虚拟化架构", ret.VmType)
		if ret.NatType != "" {
			row("NAT类型", ret.NatType)
		}
		tcpAcceleration := ret.TcpAccelerationMethod
		if tcpAcceleration == "" {
			tcpAcceleration = report.Network.CongestionControl
		}
		res += renderExtendedSystemReportText(report, language, tcpAcceleration)
	}
	return res
}
