package system

import (
	"os/exec"
	"regexp"
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
	excludeFsTypes = []string{
		"tmpfs", "devtmpfs", "sysfs", "proc", "devpts", "cgroup", "cgroup2",
		"pstore", "bpf", "tracefs", "debugfs", "mqueue", "hugetlbfs",
		"securityfs", "swap", "squashfs", "overlay", "aufs",
	}
	excludeMountPoints = []string{
		"/dev/shm", "/run", "/sys", "/proc", "/tmp", "/var/tmp",
		"/boot/efi", "/snap", "/var/lib/kubelet", "/var/lib/docker",
		"/var/lib/lxd", "/var/lib/incus", "/snap", "/vz/root",
	}
	cpuType string
)

type DiskSingelInfo struct {
	TotalStr      string
	UsageStr      string
	PercentageStr string
	BootPath      string
	MountPath     string
	TotalBytes    uint64
}

func getDiskInfo() ([]string, []string, []string, []string, string, error) {
	var bootPath string
	var diskInfos []DiskSingelInfo
	var currentDiskInfo *DiskSingelInfo
	if runtime.GOOS == "darwin" {
		diskInfos, currentDiskInfo, bootPath = getMacOSDisks()
	} else if runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" || runtime.GOOS == "netbsd" {
		diskInfos, currentDiskInfo, bootPath = getBSDDisks()
	} else {
		diskInfos, currentDiskInfo, bootPath = getLinuxDisks()
	}
	if currentDiskInfo == nil {
		currentDiskInfo = getFallbackDiskInfo()
	}
	finalDiskInfos := consolidateDiskInfos(diskInfos, currentDiskInfo)
	filteredDiskInfos := filterSmallDisks(finalDiskInfos)
	dedupedDiskInfos := deduplicatePhysicalDisks(filteredDiskInfos, currentDiskInfo)
	sort.Slice(dedupedDiskInfos, func(i, j int) bool {
		return dedupedDiskInfos[i].TotalBytes > dedupedDiskInfos[j].TotalBytes
	})
	var diskTotalStrs, diskUsageStrs, percentageStrs, diskRealPaths []string
	for _, info := range dedupedDiskInfos {
		diskTotalStrs = append(diskTotalStrs, info.TotalStr)
		diskUsageStrs = append(diskUsageStrs, info.UsageStr)
		percentageStrs = append(percentageStrs, info.PercentageStr)
		diskRealPaths = append(diskRealPaths, info.BootPath+" - "+info.MountPath)
	}
	return diskTotalStrs, diskUsageStrs, percentageStrs, diskRealPaths, bootPath, nil
}

func filterSmallDisks(diskInfos []DiskSingelInfo) []DiskSingelInfo {
	if len(diskInfos) <= 1 {
		return diskInfos
	}
	var filtered []DiskSingelInfo
	oneGB := uint64(1024 * 1024 * 1024)
	for _, info := range diskInfos {
		if info.TotalBytes >= oneGB {
			filtered = append(filtered, info)
		}
	}
	if len(filtered) == 0 {
		return diskInfos
	}
	return filtered
}

func deduplicatePhysicalDisks(diskInfos []DiskSingelInfo, currentDiskInfo *DiskSingelInfo) []DiskSingelInfo {
	if len(diskInfos) <= 1 {
		return diskInfos
	}
	physicalDisks := make(map[string][]DiskSingelInfo)
	for _, info := range diskInfos {
		physicalDisk := getPhysicalDiskName(info.BootPath)
		physicalDisks[physicalDisk] = append(physicalDisks[physicalDisk], info)
	}
	var result []DiskSingelInfo
	for physicalDisk, disks := range physicalDisks {
		var largest DiskSingelInfo = disks[0]
		for _, disk := range disks[1:] {
			if disk.TotalBytes > largest.TotalBytes {
				largest = disk
			}
		}
		hasCurrent := false
		if currentDiskInfo != nil {
			currentPhysical := getPhysicalDiskName(currentDiskInfo.BootPath)
			if physicalDisk == currentPhysical {
				for _, d := range disks {
					if d.BootPath == currentDiskInfo.BootPath && d.MountPath == currentDiskInfo.MountPath {
						result = append(result, d)
						hasCurrent = true
						break
					}
				}
			}
		}
		if largest.BootPath != currentDiskInfo.BootPath || largest.MountPath != currentDiskInfo.MountPath {
			result = append(result, largest)
		} else if !hasCurrent {
			result = append(result, largest)
		}
	}
	return result
}

func getPhysicalDiskName(device string) string {
	if strings.HasPrefix(device, "/dev/sd") && len(device) > 7 {
		return "/dev/sd" + string(device[7])
	}
	if strings.HasPrefix(device, "/dev/nvme") {
		parts := strings.Split(device, "p")
		if len(parts) > 1 {
			return parts[0]
		}
	}
	if strings.HasPrefix(device, "/dev/mmcblk") {
		parts := strings.Split(device, "p")
		if len(parts) > 1 {
			return parts[0]
		}
	}
	if strings.HasPrefix(device, "/dev/disk") {
		parts := strings.Split(device, "s")
		if len(parts) > 1 {
			return parts[0]
		}
	}
	return device
}

func getMacOSDisks() ([]DiskSingelInfo, *DiskSingelInfo, string) {
	var diskInfos []DiskSingelInfo
	var currentDiskInfo *DiskSingelInfo
	var bootPath string
	cmd := exec.Command("df", "-k", "/")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 6 {
				totalKB, err1 := strconv.ParseUint(fields[1], 10, 64)
				usedKB, err2 := strconv.ParseUint(fields[2], 10, 64)
				if err1 == nil && err2 == nil {
					totalBytes := totalKB * 1024
					usedBytes := usedKB * 1024
					diskInfo := createDiskInfo(totalBytes, usedBytes, fields[0], fields[5])
					bootPath = fields[0]
					currentDiskInfo = &diskInfo
					diskInfos = append(diskInfos, diskInfo)
				}
			}
		}
	}
	additionalDisks := getMacOSAdditionalDisks(bootPath)
	diskInfos = append(diskInfos, additionalDisks...)
	return diskInfos, currentDiskInfo, bootPath
}

func getMacOSAdditionalDisks(bootPath string) []DiskSingelInfo {
	var diskInfos []DiskSingelInfo
	devices := make(map[string]string)
	seenMountPoints := make(map[string]bool)
	diskList, _ := disk.Partitions(true)
	for _, d := range diskList {
		fsType := strings.ToLower(d.Fstype)
		if !isListContainsStr(expectDiskFsTypes, fsType) {
			continue
		}
		if !strings.HasPrefix(d.Mountpoint, "/Volumes") && shouldExcludeMountPoint(d.Mountpoint) {
			continue
		}
		if d.Device == bootPath {
			continue
		}
		if seenMountPoints[d.Mountpoint] {
			continue
		}
		seenMountPoints[d.Mountpoint] = true
		devices[d.Device] = d.Mountpoint
	}
	physicalDisks := make(map[string][]DiskSingelInfo)
	for device, mountPath := range devices {
		diskUsageOf, err := disk.Usage(mountPath)
		if err == nil && diskUsageOf.Total > 0 {
			diskInfo := createDiskInfo(diskUsageOf.Total, diskUsageOf.Used, device, mountPath)
			physicalDisk := extractDiskX(device)
			physicalDisks[physicalDisk] = append(physicalDisks[physicalDisk], diskInfo)
		}
	}
	for _, disks := range physicalDisks {
		if len(disks) == 1 {
			diskInfos = append(diskInfos, disks[0])
		} else {
			largest := disks[0]
			for _, disk := range disks[1:] {
				if disk.TotalBytes > largest.TotalBytes {
					largest = disk
				}
			}
			diskInfos = append(diskInfos, largest)
		}
	}
	return diskInfos
}

func extractDiskX(device string) string {
	re := regexp.MustCompile(`/dev/disk[0-9]+`)
	match := re.FindString(device)
	if match != "" {
		return match
	}
	return device
}

func getBSDDisks() ([]DiskSingelInfo, *DiskSingelInfo, string) {
	var diskInfos []DiskSingelInfo
	var currentDiskInfo *DiskSingelInfo
	var bootPath string
	cmd := exec.Command("df", "-h")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		seenDevices := make(map[string]bool)
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "" {
				continue
			}
			fields := strings.Fields(lines[i])
			if len(fields) >= 5 {
				device := fields[0]
				mountPoint := ""
				if len(fields) >= 6 {
					mountPoint = fields[5]
				}
				if seenDevices[device] || shouldExcludeMountPoint(mountPoint) {
					continue
				}
				seenDevices[device] = true
				totalBytes := parseSize(fields[1])
				percentageStr := fields[4]
				if percentageStr != "" && strings.Contains(percentageStr, "%") {
					percentageStr = strings.ReplaceAll(percentageStr, "%", "%%")
				}
				diskInfo := DiskSingelInfo{
					TotalStr:      fields[1],
					UsageStr:      fields[2],
					PercentageStr: percentageStr,
					BootPath:      device,
					MountPath:     mountPoint,
					TotalBytes:    totalBytes,
				}
				if mountPoint == "/" {
					bootPath = device
					currentDiskInfo = &diskInfo
				}
				diskInfos = append(diskInfos, diskInfo)
			}
		}
	}
	return diskInfos, currentDiskInfo, bootPath
}

func getLinuxDisks() ([]DiskSingelInfo, *DiskSingelInfo, string) {
	var diskInfos []DiskSingelInfo
	var currentDiskInfo *DiskSingelInfo
	var bootPath string
	devices := make(map[string]string)
	diskList, _ := disk.Partitions(false)
	for _, d := range diskList {
		fsType := strings.ToLower(d.Fstype)
		if shouldExcludeFsType(fsType) {
			continue
		}
		if shouldExcludeMountPoint(d.Mountpoint) {
			continue
		}
		if devices[d.Device] == "" && isListContainsStr(expectDiskFsTypes, fsType) {
			devices[d.Device] = d.Mountpoint
		}
	}
	for device, mountPath := range devices {
		diskUsageOf, err := disk.Usage(mountPath)
		if err == nil && diskUsageOf.Total > 0 {
			diskInfo := createDiskInfo(diskUsageOf.Total, diskUsageOf.Used, device, mountPath)
			if mountPath == "/" || (bootPath == "" && runtime.GOOS == "windows") {
				bootPath = device
				currentDiskInfo = &diskInfo
			}
			diskInfos = append(diskInfos, diskInfo)
		}
	}
	if currentDiskInfo == nil {
		overlayDisk := getOverlayDisk()
		if overlayDisk != nil {
			bootPath = overlayDisk.BootPath
			currentDiskInfo = overlayDisk
			diskInfos = append(diskInfos, *overlayDisk)
		}
	}
	return diskInfos, currentDiskInfo, bootPath
}

func getOverlayDisk() *DiskSingelInfo {
	cmd := exec.Command("df", "-x", "tmpfs", "/")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return nil
	}
	fields := strings.Split(strings.TrimSpace(lines[1]), " ")
	var nonEmptyFields []string
	for _, field := range fields {
		if field != "" {
			nonEmptyFields = append(nonEmptyFields, field)
		}
	}
	if len(nonEmptyFields) > 0 && strings.Contains(nonEmptyFields[0], "overlay") && len(nonEmptyFields) >= 5 {
		tpDiskTotal, err1 := strconv.Atoi(nonEmptyFields[1])
		tpDiskUsage, err2 := strconv.Atoi(nonEmptyFields[2])
		if err1 == nil && err2 == nil {
			totalBytes := uint64(tpDiskTotal) * 1024
			usedBytes := uint64(tpDiskUsage) * 1024
			percentageStr := nonEmptyFields[4]
			if percentageStr != "" && strings.Contains(percentageStr, "%") {
				percentageStr = strings.ReplaceAll(percentageStr, "%", "%%")
			}
			diskInfo := createDiskInfo(totalBytes, usedBytes, nonEmptyFields[0], "/")
			diskInfo.PercentageStr = percentageStr
			return &diskInfo
		}
	}
	return nil
}

func getFallbackDiskInfo() *DiskSingelInfo {
	tempDiskTotal, tempDiskUsage := getDiskTotalAndUsed()
	if tempDiskTotal > 0 {
		diskInfo := createDiskInfo(tempDiskTotal, tempDiskUsage, "/", "/")
		return &diskInfo
	}
	return nil
}

func consolidateDiskInfos(diskInfos []DiskSingelInfo, currentDiskInfo *DiskSingelInfo) []DiskSingelInfo {
	var finalDiskInfos []DiskSingelInfo
	if len(diskInfos) == 0 {
		if currentDiskInfo != nil {
			finalDiskInfos = append(finalDiskInfos, *currentDiskInfo)
		}
	} else {
		finalDiskInfos = diskInfos
		if currentDiskInfo != nil {
			currentInList := false
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
	return finalDiskInfos
}

func createDiskInfo(totalBytes, usedBytes uint64, bootPath, mountPath string) DiskSingelInfo {
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
	return DiskSingelInfo{
		TotalStr:      diskTotalStr,
		UsageStr:      diskUsageStr,
		PercentageStr: percentageStr,
		BootPath:      bootPath,
		MountPath:     mountPath,
		TotalBytes:    totalBytes,
	}
}

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
	diskList, _ := disk.Partitions(false)
	for _, d := range diskList {
		fsType := strings.ToLower(d.Fstype)
		if shouldExcludeFsType(fsType) {
			continue
		}
		if shouldExcludeMountPoint(d.Mountpoint) {
			continue
		}
		if devices[d.Device] == "" && isListContainsStr(expectDiskFsTypes, fsType) {
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
	if total == 0 && used == 0 {
		var cmd *exec.Cmd
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
						total = total * 1024
						used = used * 1024
						break
					}
				} else if len(info) == 5 && (runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" || runtime.GOOS == "netbsd") {
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

func shouldExcludeFsType(fsType string) bool {
	for _, excludeType := range excludeFsTypes {
		if strings.Contains(fsType, excludeType) {
			return true
		}
	}
	return false
}

func shouldExcludeMountPoint(mountPoint string) bool {
	for _, excludePoint := range excludeMountPoints {
		if strings.Contains(mountPoint, excludePoint) {
			if strings.Contains(mountPoint, "/run") {
				if usage, err := disk.Usage(mountPoint); err == nil {
					hundredGB := uint64(50 * 1024 * 1024 * 1024)
					if usage.Total > hundredGB {
						return false
					}
				}
			}
			return true
		}
	}
	return false
}

func isListContainsStr(list []string, str string) bool {
	for i := 0; i < len(list); i++ {
		if strings.Contains(str, list[i]) {
			return true
		}
	}
	return false
}
