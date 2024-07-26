package utils

import (
	"fmt"
	"math"
	"runtime"
	"strings"

	"github.com/oneclickvirt/basics/model"
	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows/registry"
)

// GetCpuCache 查询CPU三缓
func GetCpuCache() string {
	var L1CacheSizeStr, L2CacheSizeStr, L3CacheSizeStr string
	var cacheMemories []model.Win32CacheMemory
	var sizeStrList []string
	// Get-WmiObject -Query "Select * from Win32_CacheMemory"
	err := wmi.Query("SELECT * FROM Win32_CacheMemory", &cacheMemories)
	if err == nil {
		for _, cache := range cacheMemories {
			var sizeStr string
			if cache.InstalledSize >= 1024*1024 {
				sizeStr = fmt.Sprintf("%.2f GB", float64(cache.InstalledSize)/(1024*1024))
			} else if cache.InstalledSize >= 1024 {
				sizeStr = fmt.Sprintf("%.2f MB", float64(cache.InstalledSize)/1024)
			} else {
				sizeStr = fmt.Sprintf("%d KB", cache.InstalledSize)
			}
			sizeStrList = append(sizeStrList, sizeStr)
		}
		if len(sizeStrList) > 0 {
			L1CacheSizeStr = sizeStrList[0]
		}
		if len(sizeStrList) > 1 {
			L2CacheSizeStr = sizeStrList[1]
		}
		if len(sizeStrList) > 2 {
			L3CacheSizeStr = sizeStrList[2]
		}
	}
	if L1CacheSizeStr == "" || L2CacheSizeStr == "" || L3CacheSizeStr == "" {
		var processors []model.Win32_Processor
		err = wmi.Query("SELECT L2CacheSize, L3CacheSize FROM Win32_Processor", &processors)
		if err == nil {
			if len(processors) > 0 {
				if L1CacheSizeStr == "" {
					L1CacheSizeStr = "null"
				}
				if L2CacheSizeStr == "" {
					L2CacheSize := processors[0].L2CacheSize
					if L2CacheSize >= 1024*1024 {
						L2CacheSizeStr = fmt.Sprintf("%.2f GB", float64(L2CacheSize)/(1024*1024))
					} else if L2CacheSize >= 1024 {
						L2CacheSizeStr = fmt.Sprintf("%.2f MB", float64(L2CacheSize)/1024)
					} else {
						L2CacheSizeStr = fmt.Sprintf("%d KB", L2CacheSize)
					}
				}
				if L3CacheSizeStr == "" {
					L3CacheSize := processors[0].L3CacheSize
					if L3CacheSize >= 1024*1024 {
						L3CacheSizeStr = fmt.Sprintf("%.2f GB", float64(L3CacheSize)/(1024*1024))
					} else if L3CacheSize >= 1024 {
						L3CacheSizeStr = fmt.Sprintf("%.2f MB", float64(L3CacheSize)/1024)
					} else {
						L3CacheSizeStr = fmt.Sprintf("%d KB", L3CacheSize)
					}
				}
				return fmt.Sprintf("L1: %s / L2: %s / L3: %s", L1CacheSizeStr, L2CacheSizeStr, L3CacheSizeStr)
			}
		}
	} else {
		return fmt.Sprintf("L1: %s / L2: %s / L3: %s", L1CacheSizeStr, L2CacheSizeStr, L3CacheSizeStr)
	}
	return ""
}

func CheckCPUFeatureWindows(subkey string, value string) (string, bool) {
	// 从WMI获取信息
	if value == "hypervisor" {
		var dst []model.Win32_VFE
		// wmic cpu get VirtualizationFirmwareEnabled
		query := wmi.CreateQuery(&dst, "")
		err := wmi.Query(query, &dst)
		if err == nil {
			var body string
			for _, processor := range dst {
				body += fmt.Sprintf("%v \n", processor.VirtualizationFirmwareEnabled)
			}
			body = strings.ToLower(body)
			// fmt.Println(body)
			if strings.Contains(body, "true") {
				if runtime.GOOS == "windows" {
					return "[Y] Enabled", true
				} else {
					return "✔️ Enabled", true
				}
			} else if strings.Contains(body, "false") {
				if runtime.GOOS == "windows" {
					return "[N] Disabled", false
				} else {
					return "❌ Disabled", false
				}
			}
		}
	}
	// 从注册表获取信息
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, subkey, registry.READ)
	if err != nil {
		if runtime.GOOS == "windows" {
			return "[N] Disabled", false
		} else {
			return "❌ Disabled", false
		}
	}
	defer k.Close()
	val, _, err := k.GetStringValue(value)
	if err != nil {
		if runtime.GOOS == "windows" {
			return "[N] Disabled", false
		} else {
			return "❌ Disabled", false
		}
	}
	if strings.Contains(val, "1") {
		if runtime.GOOS == "windows" {
			return "[Y] Enabled", true
		} else {
			return "✔️ Enabled", true
		}
	}
	if runtime.GOOS == "windows" {
		return "[N] Disabled", false
	} else {
		return "❌ Disabled", false
	}
}

func CheckVMTypeWithWIMC() string {
	var (
		computers        []model.Win32_ComputerSystem
		operatingSystems []model.Win32_OperatingSystem
		VmType           string
	)
	err1 := wmi.Query("SELECT * FROM Win32_ComputerSystem", &computers)
	err2 := wmi.Query("SELECT * FROM Win32_OperatingSystem", &operatingSystems)
	if err1 == nil || err2 == nil {
		if len(computers) > 0 || len(operatingSystems) > 0 {
			switch {
			case (len(computers) > 0 && strings.Contains(computers[0].SystemType, "Multiprocessor Free")) ||
				(len(operatingSystems) > 0 && strings.Contains(operatingSystems[0].BuildType, "Multiprocessor Free")):
				VmType = "Physical-Machine" + "(" + "Multiprocessor Free" + ")"
			case (len(computers) > 0 && strings.Contains(computers[0].SystemType, "Virtual Machine")) ||
				(len(operatingSystems) > 0 && strings.Contains(operatingSystems[0].BuildType, "Virtual Machine")):
				VmType = "Hyper-V" + "(" + "Virtual Machine" + ")"
			case (len(computers) > 0 && strings.Contains(computers[0].SystemType, "VMware")) ||
				(len(operatingSystems) > 0 && strings.Contains(operatingSystems[0].BuildType, "VMware")):
				VmType = "VMware"
			default:
				if len(computers) > 0 && len(operatingSystems) > 0 {
					VmType = computers[0].SystemType + "(" + operatingSystems[0].BuildType + ")"
				} else if len(computers) > 0 {
					VmType = computers[0].SystemType
				} else if len(operatingSystems) > 0 {
					VmType = operatingSystems[0].BuildType
				}
			}
		}
	}
	return VmType
}

func GetLoad1() float64 {
	var load1 float64
	type Win32_Processor struct {
		LoadPercentage uint16
		NumberOfCores  uint32
	}
	var processors []Win32_Processor
	query := "SELECT LoadPercentage, NumberOfCores FROM Win32_Processor"
	err := wmi.Query(query, &processors)
	if err == nil {
		for _, processor := range processors {
			tempLoad := float64(processor.LoadPercentage) / float64(processor.NumberOfCores)
			load1 = math.Round(tempLoad*100) / 100
		}
	}
	return load1
}

// GetTCPAccelerateStatus 查询TCP控制算法
func GetTCPAccelerateStatus() string {
	return ""
}

// GetTimeZone 获取当前时区
func GetTimeZone() string {
	var timezone []model.Win32_TimeZone
	err := wmi.Query("SELECT * FROM Win32_TimeZone", &timezone)
	if err == nil {
		if len(timezone) > 0 {
			return timezone[0].Caption
		} else {
			return ""
		}
	} else {
		return ""
	}
}

// GetPATH 检测本机的PATH环境是否含有对应的命令
func GetPATH(key string) (string, bool) {
	return "", false
}
