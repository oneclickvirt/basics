package system

import (
	"fmt"
	"github.com/oneclickvirt/basics/system/model"
	"strconv"
)

var (
	expectDiskFsTypes = []string{
		"apfs", "ext4", "ext3", "ext2", "f2fs", "reiserfs", "jfs", "btrfs",
		"fuseblk", "zfs", "simfs", "ntfs", "fat32", "exfat", "xfs", "fuse.rclone",
	}
	cpuType string
)

// GetHost 获取主机硬件信息
func GetHost() *model.SystemInfo {
	var ret = &model.SystemInfo{}
	// 系统信息查询
	cpuType, ret.Uptime, ret.Platform, ret.Kernel, ret.Arch, ret.VmType, ret.NatType, _ = getHostInfo()
	// CPU信息查询
	ret, _ = getCpuInfo(ret, cpuType)
	// 硬盘信息查询
	ret.DiskTotal, ret.DiskUsage, ret.BootPath, _ = getDiskInfo()
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
	ret.TcpAccelerationMethod = getTCPAccelerateStatus()
	return ret
}

func GetSystemInfo() {
	ret := GetHost()
	fmt.Println("Cpu Model          :", ret.CpuModel)
	fmt.Println("Cpu Cores          :", ret.CpuCores)
	if ret.CpuCache != "" {
		fmt.Println("Cpu Cache          :", ret.CpuCache)
	}
	fmt.Println("AES-NI             :", ret.CpuAesNi)
	fmt.Println("VM-x/AMD-V/Hyper-V :", ret.CpuVAH)
	fmt.Println("RAM                :", ret.MemoryUsage+" / "+ret.MemoryTotal)
	if ret.VirtioBalloon != "" {
		fmt.Println("Virtio Balloon     :", ret.VirtioBalloon)
	}
	if ret.KSM != "" {
		fmt.Println("KSM                :", ret.KSM)
	}
	if ret.SwapTotal == "" && ret.SwapUsage == "" {
		fmt.Println("Swap               : [ no swap partition or swap file detected ]")
	} else if ret.SwapTotal != "" && ret.SwapUsage != "" {
		fmt.Println("Swap               :", ret.SwapUsage+" / "+ret.SwapTotal)
	}
	fmt.Println("Disk               :", ret.DiskUsage+" / "+ret.DiskTotal)
	fmt.Println("Boot Path          :", ret.BootPath)
	fmt.Println("OS Release         :", ret.Platform+" ["+ret.Arch+"] ")
	if ret.Kernel != "" {
		fmt.Println("Kernel             :", ret.Kernel)
	}
	fmt.Println("Uptime             :", ret.Uptime)
	fmt.Println("Load               :", ret.Load)
	fmt.Println("VM Type            :", ret.VmType)
	fmt.Println("NAT Type           :", ret.NatType)
	if ret.TcpAccelerationMethod != "" {
		fmt.Println("Tcp Accelerate     :", ret.TcpAccelerationMethod)
	}
}
