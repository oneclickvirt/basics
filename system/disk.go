package system

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

// getDiskInfo 获取硬盘信息
func getDiskInfo() (string, string, string, string, error) {
	var diskTotalStr, diskUsageStr, percentageStr, bootPath string
	// macOS 特殊适配
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("df", "-h", "/")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 5 {
					bootPath = fields[0]
					diskTotalStr = fields[1]
					diskUsageStr = fields[2]
					percentageStr = fields[4]
					if strings.Contains(percentageStr, "%") {
						percentageStr = strings.ReplaceAll(percentageStr, "%", "%%")
					}
					return diskTotalStr, diskUsageStr, percentageStr, bootPath, nil
				}
			}
		}
	}
	// BSD系统特殊处理
	if runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" || runtime.GOOS == "netbsd" {
		cmd := exec.Command("df", "-h", "/")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 5 {
					bootPath = fields[0]
					diskTotalStr = fields[1]
					diskUsageStr = fields[2]
					percentageStr = fields[4]
					if percentageStr != "" && strings.Contains(percentageStr, "%") {
						percentageStr = strings.ReplaceAll(percentageStr, "%", "%%")
					}
					return diskTotalStr, diskUsageStr, percentageStr, bootPath, nil
				}
			}
		}
	}
	tempDiskTotal, tempDiskUsage := getDiskTotalAndUsed()
	diskTotalGB := float64(tempDiskTotal) / (1024 * 1024 * 1024)
	diskUsageGB := float64(tempDiskUsage) / (1024 * 1024 * 1024)
	if diskTotalGB < 1 {
		diskTotalStr = strconv.FormatFloat(diskTotalGB*1024, 'f', 2, 64) + " MB"
	} else {
		diskTotalStr = strconv.FormatFloat(diskTotalGB, 'f', 2, 64) + " GB"
	}
	if diskUsageGB < 1 {
		diskUsageStr = strconv.FormatFloat(diskUsageGB*1024, 'f', 2, 64) + " MB"
	} else {
		diskUsageStr = strconv.FormatFloat(diskUsageGB, 'f', 2, 64) + " GB"
	}
	if runtime.GOOS == "windows" {
		parts, err := disk.Partitions(true)
		if err != nil {
			bootPath = ""
		} else {
			for _, part := range parts {
				if part.Fstype == "tmpfs" {
					continue
				}
				usageStat, err := disk.Usage(part.Mountpoint)
				if err != nil {
					continue
				}
				if usageStat.Total > 0 {
					bootPath = part.Mountpoint
					break
				}
			}
		}
	} else {
		// 特殊处理 docker、lxc 等虚拟化使用 overlay 挂载的情况
		cmd := exec.Command("df", "-x", "tmpfs", "/")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				fields := strings.Split(strings.TrimSpace(lines[1]), " ")
				var nonEmptyFields []string
				for _, field := range fields {
					if field != "" {
						nonEmptyFields = append(nonEmptyFields, field)
					}
				}
				if len(nonEmptyFields) > 0 && nonEmptyFields[0] != "" {
					bootPath = nonEmptyFields[0]
					if strings.Contains(bootPath, "overlay") && len(nonEmptyFields) >= 5 {
						tpDiskTotal, err1 := strconv.Atoi(nonEmptyFields[1])
						tpDiskUsage, err2 := strconv.Atoi(nonEmptyFields[2])
						if err1 == nil && err2 == nil {
							diskTotalGB = float64(tpDiskTotal) / (1024 * 1024)
							diskUsageGB = float64(tpDiskUsage) / (1024 * 1024)
							if diskTotalGB < 1 {
								diskTotalStr = strconv.FormatFloat(diskTotalGB*1024, 'f', 2, 64) + " MB"
								percentageStr = nonEmptyFields[4]
							} else {
								diskTotalStr = strconv.FormatFloat(diskTotalGB, 'f', 2, 64) + " GB"
								percentageStr = nonEmptyFields[4]
							}
							if diskUsageGB < 1 {
								diskUsageStr = strconv.FormatFloat(diskUsageGB*1024, 'f', 2, 64) + " MB"
							} else {
								diskUsageStr = strconv.FormatFloat(diskUsageGB, 'f', 2, 64) + " GB"
							}
						}
					}
				}
			}
		}
	}
	if percentageStr != "" && strings.Contains(percentageStr, "%") {
		percentageStr = strings.ReplaceAll(percentageStr, "%", "%%")
	}
	return diskTotalStr, diskUsageStr, percentageStr, bootPath, nil
}

func getDiskTotalAndUsed() (total uint64, used uint64) {
	// MacOS特殊处理，直接用 df -k /
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("df", "-k", "/")
		out, err := cmd.CombinedOutput()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 6 {
					totalKB, err1 := strconv.ParseUint(fields[1], 10, 64)
					usedKB, err2 := strconv.ParseUint(fields[2], 10, 64)
					if err1 == nil && err2 == nil {
						total = totalKB * 1024
						used = usedKB * 1024
						return
					}
				}
			}
		}
	}
	devices := make(map[string]string)
	// 使用默认过滤规则
	diskList, _ := disk.Partitions(false)
	for _, d := range diskList {
		fsType := strings.ToLower(d.Fstype)
		// 不统计 K8s 的虚拟挂载点：https://github.com/shirou/gopsutil/issues/1007
		if devices[d.Device] == "" && isListContainsStr(expectDiskFsTypes, fsType) && !strings.Contains(d.Mountpoint, "/var/lib/kubelet") {
			devices[d.Device] = d.Mountpoint
		}
	}
	for _, mountPath := range devices {
		diskUsageOf, err := disk.Usage(mountPath)
		if err == nil {
			total += diskUsageOf.Total
			used += diskUsageOf.Used
		}
	}
	// 回退到根路径的获取方法
	if total == 0 && used == 0 {
		var cmd *exec.Cmd
		// BSD系统使用特定参数
		if runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" || runtime.GOOS == "netbsd" {
			cmd = exec.Command("df", "-k", "/")
		} else {
			cmd = exec.Command("df")
		}
		out, err := cmd.CombinedOutput()
		if err == nil {
			s := strings.Split(string(out), "\n")
			for _, c := range s {
				info := strings.Fields(c)
				if len(info) == 6 {
					if info[5] == "/" {
						total, _ = strconv.ParseUint(info[1], 0, 64)
						used, _ = strconv.ParseUint(info[2], 0, 64)
						// 默认获取的是1K块为单位的.
						total = total * 1024
						used = used * 1024
						break
					}
				} else if len(info) == 5 && (runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" || runtime.GOOS == "netbsd") {
					// BSD系统df输出格式可能只有5列
					if info[4] == "/" {
						total, _ = strconv.ParseUint(info[1], 0, 64)
						used, _ = strconv.ParseUint(info[2], 0, 64)
						total = total * 1024
						used = used * 1024
						break
					}
				}
			}
		}
	}
	return
}

func isListContainsStr(list []string, str string) bool {
	for i := 0; i < len(list); i++ {
		if strings.Contains(str, list[i]) {
			return true
		}
	}
	return false
}
