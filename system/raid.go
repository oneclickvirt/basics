package system

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// RAID信息结构体
type RAIDInfo struct {
	Exists     bool
	Type       string
	DiskCount  int
	Controller string
	Details    string
}

func detectRAID() (RAIDInfo, error) {
	var info RAIDInfo
	var err error
	switch runtime.GOOS {
	case "windows":
		info, err = detectWindowsRAID()
	case "linux":
		info, err = detectLinuxRAID()
	case "darwin":
		info, err = detectMacOSRAID()
	default:
		err = fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
	return info, err
}

func detectWindowsRAID() (RAIDInfo, error) {
	info := RAIDInfo{}
	// 使用diskpart和wmic获取存储信息
	cmd := exec.Command("powershell", "-Command", "Get-WmiObject -Class Win32_DiskDrive | Select-Object Model, InterfaceType, MediaType | Format-Table -AutoSize")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return info, err
	}
	// 检查是否有控制器信息
	controllerCmd := exec.Command("powershell", "-Command", "Get-WmiObject -Class Win32_SCSIController | Select-Object Name, DriverName, PNPDeviceID | Format-Table -AutoSize")
	controllerOutput, err := controllerCmd.CombinedOutput()
	if err == nil {
		controllerStr := string(controllerOutput)
		if strings.Contains(strings.ToLower(controllerStr), "raid") {
			info.Exists = true
			info.Controller = extractControllerName(controllerStr)
		}
	}
	// 检查虚拟磁盘
	vDiskCmd := exec.Command("powershell", "-Command", "Get-WmiObject -Class Win32_DiskDrive | Where-Object {$_.MediaType -eq 'Fixed hard disk media'} | ForEach-Object {$disk = $_; Get-WmiObject -Class Win32_DiskPartition | Where-Object {$_.DiskIndex -eq $disk.Index}} | Format-Table -AutoSize")
	vDiskOutput, _ := vDiskCmd.CombinedOutput()
	// 分析输出确定RAID类型
	outputStr := string(output) + string(vDiskOutput)
	if info.Exists || strings.Contains(strings.ToLower(outputStr), "raid") {
		info.Exists = true
		info.Type = determineWindowsRAIDType(outputStr)
		info.DiskCount = estimateDiskCount(outputStr)
		info.Details = cleanOutput(outputStr)
	}
	return info, nil
}

func detectLinuxRAID() (RAIDInfo, error) {
	info := RAIDInfo{}
	// 检查软RAID (mdadm)
	mdstatCmd := exec.Command("cat", "/proc/mdstat")
	mdstatOutput, err := mdstatCmd.CombinedOutput()
	if err == nil && len(mdstatOutput) > 0 && !strings.Contains(string(mdstatOutput), "unused devices") {
		mdstatStr := string(mdstatOutput)
		if strings.Contains(mdstatStr, "md") {
			info.Exists = true
			info.Type = determineLinuxRAIDType(mdstatStr)
			info.DiskCount = countDevicesInMdstat(mdstatStr)
			info.Controller = "软件RAID (mdadm)"
			info.Details = cleanOutput(mdstatStr)
			return info, nil
		}
	}
	// 检查硬件RAID控制器
	lspciCmd := exec.Command("lspci")
	lspciOutput, err := lspciCmd.CombinedOutput()
	if err == nil {
		lspciStr := string(lspciOutput)
		if strings.Contains(strings.ToLower(lspciStr), "raid") {
			info.Exists = true
			info.Controller = extractLinuxControllerName(lspciStr)
			info.Type = "硬件RAID"
			info.Details = cleanOutput(lspciStr)
			// 尝试获取更多详细信息
			if strings.Contains(strings.ToLower(info.Controller), "lsi") || strings.Contains(strings.ToLower(info.Controller), "avago") || strings.Contains(strings.ToLower(info.Controller), "broadcom") {
				mptUtilCmd := exec.Command("sh", "-c", "which mpt-status && mpt-status || echo 'mpt-status not installed'")
				mptOutput, _ := mptUtilCmd.CombinedOutput()
				info.Details += "\n" + cleanOutput(string(mptOutput))
			} else if strings.Contains(strings.ToLower(info.Controller), "adaptec") {
				arcConfCmd := exec.Command("sh", "-c", "which arcconf && arcconf getconfig 1 || echo 'arcconf not installed'")
				arcOutput, _ := arcConfCmd.CombinedOutput()
				info.Details += "\n" + cleanOutput(string(arcOutput))
			}
		}
	}
	return info, nil
}

func detectMacOSRAID() (RAIDInfo, error) {
	info := RAIDInfo{}

	// 使用diskutil获取RAID信息
	cmd := exec.Command("diskutil", "appleRAID", "list")
	output, err := cmd.CombinedOutput()

	if err == nil {
		outputStr := string(output)
		if !strings.Contains(outputStr, "No RAID sets") {
			info.Exists = true
			info.Type = determineMacRAIDType(outputStr)
			info.DiskCount = countMacRAIDDisks(outputStr)
			info.Controller = "Apple Software RAID"
			info.Details = cleanOutput(outputStr)
		}
	}

	return info, nil
}

func determineWindowsRAIDType(output string) string {
	lowerOutput := strings.ToLower(output)

	if strings.Contains(lowerOutput, "raid 0") || strings.Contains(lowerOutput, "stripe") {
		return "RAID 0 (条带化)"
	} else if strings.Contains(lowerOutput, "raid 1") || strings.Contains(lowerOutput, "mirror") {
		return "RAID 1 (镜像)"
	} else if strings.Contains(lowerOutput, "raid 5") || strings.Contains(lowerOutput, "parity") {
		return "RAID 5 (分布式奇偶校验)"
	} else if strings.Contains(lowerOutput, "raid 6") {
		return "RAID 6 (双奇偶校验)"
	} else if strings.Contains(lowerOutput, "raid 10") {
		return "RAID 10 (镜像+条带)"
	} else {
		return "未知RAID类型"
	}
}

func determineLinuxRAIDType(mdstat string) string {
	if strings.Contains(mdstat, "raid0") {
		return "RAID 0 (条带化)"
	} else if strings.Contains(mdstat, "raid1") {
		return "RAID 1 (镜像)"
	} else if strings.Contains(mdstat, "raid4") {
		return "RAID 4 (专用奇偶校验)"
	} else if strings.Contains(mdstat, "raid5") {
		return "RAID 5 (分布式奇偶校验)"
	} else if strings.Contains(mdstat, "raid6") {
		return "RAID 6 (双奇偶校验)"
	} else if strings.Contains(mdstat, "raid10") {
		return "RAID 10 (镜像+条带)"
	} else {
		return "未知RAID类型"
	}
}

func determineMacRAIDType(output string) string {
	lowerOutput := strings.ToLower(output)

	if strings.Contains(lowerOutput, "stripe") {
		return "RAID 0 (条带化)"
	} else if strings.Contains(lowerOutput, "mirror") {
		return "RAID 1 (镜像)"
	} else if strings.Contains(lowerOutput, "concat") {
		return "JBOD (磁盘连接)"
	} else {
		return "未知RAID类型"
	}
}

func extractControllerName(output string) string {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), "raid") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return strings.Join(fields, " ")
			}
		}
	}
	return "未知RAID控制器"
}

func extractLinuxControllerName(lspci string) string {
	scanner := bufio.NewScanner(strings.NewReader(lspci))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), "raid") {
			return line
		}
	}
	return "未知RAID控制器"
}

func estimateDiskCount(output string) int {
	// 简单估计，实际情况可能需要更复杂的解析
	re := regexp.MustCompile(`Disk #\d+`)
	matches := re.FindAllString(output, -1)
	return len(matches)
}

func countDevicesInMdstat(mdstat string) int {
	scanner := bufio.NewScanner(strings.NewReader(mdstat))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "active") {
			re := regexp.MustCompile(`sd[a-z][0-9]*`)
			matches := re.FindAllString(line, -1)
			return len(matches)
		}
	}
	return 0
}

func countMacRAIDDisks(output string) int {
	re := regexp.MustCompile(`Disk: [0-9]`)
	matches := re.FindAllString(output, -1)
	return len(matches)
}

func cleanOutput(output string) string {
	// 删除ANSI颜色代码和控制字符
	re := regexp.MustCompile(`\x1B\[[0-9;]*[a-zA-Z]`)
	cleanStr := re.ReplaceAllString(output, "")
	// 限制输出长度
	if len(cleanStr) > 500 {
		return cleanStr[:500] + "..."
	}
	return cleanStr
}
