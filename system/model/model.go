package model

type CpuInfo struct {
	CpuModel string
	CpuCores string
	CpuCache string
	CpuAesNi string
	CpuVAH   string
}

type MemoryInfo struct {
	MemoryUsage string
	MemoryTotal string
	SwapUsage   string
	SwapTotal   string
}

type DiskInfo struct {
	DiskUsage string
	DiskTotal string
	BootPath  string
}

type SystemInfo struct {
	CpuInfo
	MemoryInfo
	DiskInfo
	Platform              string // 系统名字 Distro1
	PlatformVersion       string // 系统版本 Distro2
	Kernel                string // 系统内核
	Arch                  string //
	Uptime                string // 正常运行时间
	TimeZone              string // 系统时区
	VmType                string // 虚拟化架构
	Load                  string // load1 load2 load3
	NatType               string // stun
	VirtioBalloon         string // 气球驱动
	KSM                   string // 内存合并
	TcpAccelerationMethod string // TCP拥塞控制
}

type Win32_Processor struct {
	L2CacheSize uint32
	L3CacheSize uint32
}

type Win32_ComputerSystem struct {
	SystemType string
}

type Win32_OperatingSystem struct {
	BuildType string
}

type Win32_TimeZone struct {
	Caption string
}
