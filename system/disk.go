package system

import (
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

var (
	expectDiskFsTypes = []string{
		"apfs", "ext4", "ext3", "ext2", "f2fs", "reiserfs", "jfs", "bcachefs", "btrfs",
		"fuseblk", "zfs", "simfs", "ntfs", "fat32", "exfat", "xfs", "fuse.rclone",
	}
	cpuType string
)

type DiskSingelInfo struct {
	TotalStr      string
	UsageStr      string
	PercentageStr string
	BootPath      string
	TotalBytes    uint64
}

// getDiskInfo 获取硬盘信息
func getDiskInfo() ([]string, []string, []string, string, error) {
	var bootPath string
	var diskInfos []DiskSingelInfo
	var currentDiskInfo *DiskSingelInfo
	// macOS 特殊适配
	if runtime.GOOS == "darwin" {
		// 获取所有APFS卷的挂载点
		mountPoints := getMacOSMountPoints()
		for _, mountPoint := range mountPoints {
			cmd := exec.Command("df", "-k", mountPoint)
			output, err := cmd.Output()
			if err != nil {
				continue
			}
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 6 {
					totalKB, err1 := strconv.ParseUint(fields[1], 10, 64)
					usedKB, err2 := strconv.ParseUint(fields[2], 10, 64)
					if err1 == nil && err2 == nil {
						totalBytes := totalKB * 1024
						usedBytes := usedKB * 1024
						diskTotalGB := float64(totalBytes) / (1024 * 1024 * 1024)
						diskUsageGB := float64(usedBytes) / (1024 * 1024 * 1024)
						var diskTotalStr, diskUsageStr string
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
						percentage := float64(usedBytes) / float64(totalBytes) * 100
						percentageStr := strconv.FormatFloat(percentage, 'f', 1, 64) + "%%"
						diskInfo := DiskSingelInfo{
							TotalStr:      diskTotalStr,
							UsageStr:      diskUsageStr,
							PercentageStr: percentageStr,
							BootPath:      fields[0],
							TotalBytes:    totalBytes,
						}
						if mountPoint == "/" {
							bootPath = fields[0]
							currentDiskInfo = &diskInfo
						}
						if totalBytes >= 200*1024*1024*1024 {
							diskInfos = append(diskInfos, diskInfo)
						}
					}
				}
			}
		}
	} else if runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" || runtime.GOOS == "netbsd" {
		// BSD系统特殊处理
		cmd := exec.Command("df", "-h")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for i := 1; i < len(lines); i++ {
				if strings.TrimSpace(lines[i]) == "" {
					continue
				}
				fields := strings.Fields(lines[i])
				if len(fields) >= 5 {
					totalStr := fields[1]
					totalBytes := parseSize(totalStr)
					percentageStr := fields[4]
					if percentageStr != "" && strings.Contains(percentageStr, "%") {
						percentageStr = strings.ReplaceAll(percentageStr, "%", "%%")
					}
					diskInfo := DiskSingelInfo{
						TotalStr:      fields[1],
						UsageStr:      fields[2],
						PercentageStr: percentageStr,
						BootPath:      fields[0],
						TotalBytes:    totalBytes,
					}
					if len(fields) >= 6 && fields[5] == "/" {
						bootPath = fields[0]
						currentDiskInfo = &diskInfo
					}
					if totalBytes >= 200*1024*1024*1024 {
						diskInfos = append(diskInfos, diskInfo)
					}
				}
			}
		}
	} else {
		// 其他系统使用gopsutil
		devices := make(map[string]string)
		diskList, _ := disk.Partitions(false)
		for _, d := range diskList {
			fsType := strings.ToLower(d.Fstype)
			if devices[d.Device] == "" && isListContainsStr(expectDiskFsTypes, fsType) && !strings.Contains(d.Mountpoint, "/var/lib/kubelet") {
				devices[d.Device] = d.Mountpoint
			}
		}
		for device, mountPath := range devices {
			diskUsageOf, err := disk.Usage(mountPath)
			if err == nil && diskUsageOf.Total > 0 {
				diskTotalGB := float64(diskUsageOf.Total) / (1024 * 1024 * 1024)
				diskUsageGB := float64(diskUsageOf.Used) / (1024 * 1024 * 1024)
				var diskTotalStr, diskUsageStr string
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
				percentageStr := strconv.FormatFloat(float64(diskUsageOf.Used)/float64(diskUsageOf.Total)*100, 'f', 1, 64) + "%%"
				diskInfo := DiskSingelInfo{
					TotalStr:      diskTotalStr,
					UsageStr:      diskUsageStr,
					PercentageStr: percentageStr,
					BootPath:      device,
					TotalBytes:    diskUsageOf.Total,
				}
				if mountPath == "/" || (bootPath == "" && runtime.GOOS == "windows") {
					bootPath = device
					currentDiskInfo = &diskInfo
				}
				if diskUsageOf.Total >= 200*1024*1024*1024 {
					diskInfos = append(diskInfos, diskInfo)
				}
			}
		}
		// 特殊处理 docker、lxc 等虚拟化使用 overlay 挂载的情况
		if currentDiskInfo == nil && runtime.GOOS == "linux" {
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
						if strings.Contains(nonEmptyFields[0], "overlay") && len(nonEmptyFields) >= 5 {
							tpDiskTotal, err1 := strconv.Atoi(nonEmptyFields[1])
							tpDiskUsage, err2 := strconv.Atoi(nonEmptyFields[2])
							if err1 == nil && err2 == nil {
								totalBytes := uint64(tpDiskTotal) * 1024
								diskTotalGB := float64(tpDiskTotal) / (1024 * 1024)
								diskUsageGB := float64(tpDiskUsage) / (1024 * 1024)
								var diskTotalStr, diskUsageStr string
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
								percentageStr := nonEmptyFields[4]
								if percentageStr != "" && strings.Contains(percentageStr, "%") {
									percentageStr = strings.ReplaceAll(percentageStr, "%", "%%")
								}
								diskInfo := DiskSingelInfo{
									TotalStr:      diskTotalStr,
									UsageStr:      diskUsageStr,
									PercentageStr: percentageStr,
									BootPath:      nonEmptyFields[0],
									TotalBytes:    totalBytes,
								}
								bootPath = nonEmptyFields[0]
								currentDiskInfo = &diskInfo
								if totalBytes >= 200*1024*1024*1024 {
									diskInfos = append(diskInfos, diskInfo)
								}
							}
						}
					}
				}
			}
		}
	}
	if currentDiskInfo == nil {
		tempDiskTotal, tempDiskUsage := getDiskTotalAndUsed()
		if tempDiskTotal > 0 {
			diskTotalGB := float64(tempDiskTotal) / (1024 * 1024 * 1024)
			diskUsageGB := float64(tempDiskUsage) / (1024 * 1024 * 1024)
			var diskTotalStr, diskUsageStr string
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
			percentageStr := strconv.FormatFloat(float64(tempDiskUsage)/float64(tempDiskTotal)*100, 'f', 1, 64) + "%%"
			diskInfo := DiskSingelInfo{
				TotalStr:      diskTotalStr,
				UsageStr:      diskUsageStr,
				PercentageStr: percentageStr,
				BootPath:      "/",
				TotalBytes:    tempDiskTotal,
			}
			currentDiskInfo = &diskInfo
			if tempDiskTotal >= 200*1024*1024*1024 {
				diskInfos = append(diskInfos, diskInfo)
			}
		}
	}
	// 应用逻辑：
	// 1. 如果完全没有盘大于200GB，返回当前磁盘
	// 2. 如果有盘大于200GB但当前磁盘不大于200GB，返回大于200GB的磁盘+当前磁盘
	// 3. 如果当前磁盘和其他磁盘都大于200GB，返回所有大于200GB的磁盘
	var finalDiskInfos []DiskSingelInfo
	if len(diskInfos) == 0 {
		// 情况1：没有大于200GB的磁盘，返回当前磁盘
		if currentDiskInfo != nil {
			finalDiskInfos = append(finalDiskInfos, *currentDiskInfo)
		}
	} else {
		// 情况2和3：有大于200GB的磁盘
		finalDiskInfos = diskInfos
		// 如果当前磁盘不在大于200GB列表中，需要添加
		currentInList := false
		if currentDiskInfo != nil {
			for _, info := range diskInfos {
				if info.BootPath == currentDiskInfo.BootPath {
					currentInList = true
					break
				}
			}
			if !currentInList {
				finalDiskInfos = append(finalDiskInfos, *currentDiskInfo)
			}
		}
	}
	// 按容量从大到小排序
	sort.Slice(finalDiskInfos, func(i, j int) bool {
		return finalDiskInfos[i].TotalBytes > finalDiskInfos[j].TotalBytes
	})
	// 提取切片
	var diskTotalStrs, diskUsageStrs, percentageStrs []string
	for _, info := range finalDiskInfos {
		diskTotalStrs = append(diskTotalStrs, info.TotalStr)
		diskUsageStrs = append(diskUsageStrs, info.UsageStr)
		percentageStrs = append(percentageStrs, info.PercentageStr)
	}
	return diskTotalStrs, diskUsageStrs, percentageStrs, bootPath, nil
}

// getMacOSMountPoints 获取macOS所有APFS卷的挂载点
func getMacOSMountPoints() []string {
	var mountPoints []string
	mountPoints = append(mountPoints, "/")
	cmd := exec.Command("diskutil", "list")
	output, err := cmd.Output()
	if err != nil {
		return mountPoints
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "APFS Volume") && !strings.Contains(line, "Preboot") && !strings.Contains(line, "Recovery") && !strings.Contains(line, "VM") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				volumeName := fields[3]
				if volumeName == "Macintosh" || volumeName == "Data" {
					continue
				}
				mountPoint := "/Volumes/" + volumeName
				mountPoints = append(mountPoints, mountPoint)
			}
		}
	}
	return mountPoints
}

// parseSize 解析尺寸字符串为字节数
func parseSize(sizeStr string) uint64 {
	sizeStr = strings.TrimSpace(sizeStr)
	if sizeStr == "" {
		return 0
	}
	var multiplier uint64 = 1
	sizeStr = strings.ToUpper(sizeStr)
	if strings.HasSuffix(sizeStr, "KB") || strings.HasSuffix(sizeStr, "K") {
		multiplier = 1024
		if strings.HasSuffix(sizeStr, "KB") {
			sizeStr = strings.TrimSuffix(sizeStr, "KB")
		} else {
			sizeStr = strings.TrimSuffix(sizeStr, "K")
		}
	} else if strings.HasSuffix(sizeStr, "MB") || strings.HasSuffix(sizeStr, "M") {
		multiplier = 1024 * 1024
		if strings.HasSuffix(sizeStr, "MB") {
			sizeStr = strings.TrimSuffix(sizeStr, "MB")
		} else {
			sizeStr = strings.TrimSuffix(sizeStr, "M")
		}
	} else if strings.HasSuffix(sizeStr, "GB") || strings.HasSuffix(sizeStr, "G") {
		multiplier = 1024 * 1024 * 1024
		if strings.HasSuffix(sizeStr, "GB") {
			sizeStr = strings.TrimSuffix(sizeStr, "GB")
		} else {
			sizeStr = strings.TrimSuffix(sizeStr, "G")
		}
	} else if strings.HasSuffix(sizeStr, "TB") || strings.HasSuffix(sizeStr, "T") {
		multiplier = 1024 * 1024 * 1024 * 1024
		if strings.HasSuffix(sizeStr, "TB") {
			sizeStr = strings.TrimSuffix(sizeStr, "TB")
		} else {
			sizeStr = strings.TrimSuffix(sizeStr, "T")
		}
	}
	value, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		return 0
	}
	return uint64(value * float64(multiplier))
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