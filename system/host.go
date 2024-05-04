package system

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/libp2p/go-nat"
	"github.com/oneclickvirt/basics/system/utils"
	"github.com/shirou/gopsutil/host"
)

func getHostInfo() (string, string, string, string, string, string, string, string, error) {
	var Platform, Kernal, Arch, VmType, NatType, CurrentTimeZone string
	var cachedBootTime time.Time
	hi, err := host.Info()
	if err != nil {
		println("host.Info error:", err)
	} else {
		if hi.VirtualizationRole == "guest" {
			cpuType = "Virtual"
		} else {
			cpuType = "Physical"
		}
		Platform = hi.Platform + " " + hi.PlatformVersion
		Arch = hi.KernelArch
		// 查询虚拟化类型
		VmType = hi.VirtualizationSystem
		// 系统运行时长查询
		cachedBootTime = time.Unix(int64(hi.BootTime), 0)
	}
	uptimeDuration := time.Since(cachedBootTime)
	days := int(uptimeDuration.Hours() / 24)
	uptimeDuration -= time.Duration(days*24) * time.Hour
	hours := int(uptimeDuration.Hours())
	uptimeDuration -= time.Duration(hours) * time.Hour
	minutes := int(uptimeDuration.Minutes())
	uptimeFormatted := fmt.Sprintf("%d days, %02d hours, %02d minutes", days, hours, minutes)
	// windows 查询虚拟化类型 使用 wmic
	if VmType == "" && runtime.GOOS == "windows" {
		VmType = utils.CheckVMTypeWithWIMC()
	}
	// 查询NAT类型
	ctx := context.Background()
	gateway, err := nat.DiscoverGateway(ctx)
	if err == nil {
		natType := gateway.Type()
		NatType = natType
	}
	// 获取当前系统的本地时区
	CurrentTimeZone = utils.GetTimeZone()
	return cpuType, uptimeFormatted, Platform, Kernal, Arch, VmType, NatType, CurrentTimeZone, nil
}
