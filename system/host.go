package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/libp2p/go-nat"
	"github.com/oneclickvirt/basics/model"
	"github.com/oneclickvirt/basics/system/utils"
	precheckUtils "github.com/oneclickvirt/basics/utils"
	"github.com/shirou/gopsutil/v4/host"
)

// getVmType 匹配架构信息
func getVmType(vmType string) string {
	switch strings.TrimSpace(vmType) {
	case "kvm":
		return "KVM"
	case "xen":
		return "Xen Hypervisor"
	case "microsoft":
		return "Microsoft Hyper-V"
	case "vmware":
		return "VMware"
	case "oracle":
		return "Oracle VirtualBox"
	case "parallels":
		return "Parallels"
	case "qemu":
		return "QEMU"
	case "amazon":
		return "Amazon Virtualization"
	case "docker":
		return "Docker"
	case "openvz":
		return "OpenVZ (Virutozzo)"
	case "lxc":
		return "LXC"
	case "lxc-libvirt":
		return "LXC (Based on libvirt)"
	case "uml":
		return "User-mode Linux"
	case "systemd-nspawn":
		return "Systemd nspawn"
	case "bochs":
		return "BOCHS"
	case "rkt":
		return "RKT"
	case "zvm":
		return "S390 Z/VM"
	case "none":
		return ""
	}
	return ""
}

// 使用 systemd-detect-virt 查询虚拟化信息
func getVmTypeFromSDV(path string) string {
	cmd := exec.Command(path)
	output, err := cmd.Output()
	if err == nil {
		return getVmType(strings.ReplaceAll(string(output), "\n", ""))
	}
	return ""
}

// 使用 dmidecode -t system 查询虚拟化信息
func getVmTypeFromDMI(path string) string {
	cmd := exec.Command(path, "-t", "system")
	output, err := cmd.Output()
	if err == nil {
		// Check for 'family'
		for _, line := range strings.Split(strings.ToLower(string(output)), "\n") {
			if strings.Contains(line, "family") {
				familyLine := strings.ReplaceAll(line, "family", "")
				if vmType := getVmType(strings.TrimSpace(strings.ReplaceAll(familyLine, ":", ""))); vmType != "" {
					return vmType
				}
			}
		}
		// Fallback to 'manufacturer'
		// for _, line := range strings.Split(strings.ToLower(string(output)), "\n") {
		// 	if strings.Contains(line, "manufacturer") {
		// 		tempL := strings.Split(line, ":")
		// 		if len(tempL) == 2 {
		// 			return strings.TrimSpace(tempL[1])
		// 		}
		// 	}
		// }
	}
	return ""
}

func getHostInfo() (string, string, string, string, string, string, string, string, error) {
	var Platform, Kernal, Arch, VmType, NatType, CurrentTimeZone string
	var cachedBootTime time.Time
	hi, err := host.Info()
	if err != nil {
		fmt.Println("host.Info error:", err)
	} else {
		if hi.VirtualizationRole == "guest" {
			cpuType = "Virtual"
		} else {
			cpuType = "Physical"
		}
		Platform = hi.Platform + " " + hi.PlatformVersion
		Arch = hi.KernelArch
		// 查询虚拟化类型和内核
		if runtime.GOOS != "windows" {
			cmd := exec.Command("uname", "-r")
			output, err := cmd.Output()
			if err == nil {
				Kernal = strings.TrimSpace(strings.ReplaceAll(string(output), "\n", ""))
			}
			path, exit := utils.GetPATH("systemd-detect-virt")
			if exit {
				VmType = getVmTypeFromSDV(path)
			}
			if VmType == "" {
				_, err := os.Stat("/.dockerenv")
				if os.IsExist(err) {
					VmType = "Docker"
				}
				cgroupFile, err := os.Open("/proc/1/cgroup")
				defer cgroupFile.Close()
				if err == nil {
					scanner := bufio.NewScanner(cgroupFile)
					for scanner.Scan() {
						if strings.Contains(scanner.Text(), "docker") {
							VmType = "Docker"
						}
					}
				}
				_, err = os.Stat("/dev/lxss")
				if os.IsExist(err) {
					VmType = "Windows Subsystem for Linux"
				}
				path, exit = utils.GetPATH("dmidecode")
				if exit && VmType == "" {
					VmType = getVmTypeFromDMI(path)
				}
				if VmType == "" {
					VmType = "Dedicated (No visible signage)"
				}
			}
		} else {
			VmType = hi.VirtualizationSystem
		}
		// 系统运行时长查询 /proc/uptime
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
	// MAC需要额外获取信息进行判断
	if runtime.GOOS == "darwin" {
		if len(model.MacOSInfo) > 0 {
			var modelName, modelIdentifier, chip, vendor string
			for _, line := range model.MacOSInfo {
				if strings.Contains(line, "Model Name") {
					modelName = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				} else if strings.Contains(line, "Model Identifier") {
					modelIdentifier = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				} else if strings.Contains(line, "Chip") {
					chip = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				} else if strings.Contains(line, "Manufacturer") || strings.Contains(line, "Vendor") {
					vendor = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				}
			}
			allInfo := strings.ToLower(modelName + " " + modelIdentifier + " " + chip + " " + vendor)
			virtualKeywords := []string{"vmware", "virtualbox", "parallels", "qemu", "microsoft", "xen"}
			isVirtual := false
			for _, key := range virtualKeywords {
				if strings.Contains(allInfo, key) {
					isVirtual = true
					break
				}
			}
			physicalWhitelist := []string{"mac mini", "macbook pro", "macbook air", "imac", "mac studio", "mac pro"}
			if isVirtual {
				VmType = "virtual"
			} else {
				foundPhysical := false
				for _, p := range physicalWhitelist {
					if strings.HasPrefix(strings.ToLower(modelName), p) {
						foundPhysical = true
						break
					}
				}
				if foundPhysical {
					VmType = "Physical"
				} else if modelName == "" {
					VmType = "Unknown"
				} else {
					VmType = modelName
				}
			}
		}
	}
	// 查询NAT类型
	if precheckUtils.StackType != "" && precheckUtils.StackType != "None" {
		NatType = getNatType()
		if NatType == "Inconclusive" {
			ctx := context.Background()
			gateway, err := nat.DiscoverGateway(ctx)
			if err == nil {
				natType := gateway.Type()
				NatType = natType
			}
		}
	}
	// 获取当前系统的本地时区
	CurrentTimeZone = utils.GetTimeZone()
	return cpuType, uptimeFormatted, Platform, Kernal, Arch, VmType, NatType, CurrentTimeZone, nil
}
