package system

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var nvmeControllerPattern = regexp.MustCompile(`^(nvme[0-9]+)`)

func detectStorageProtocol(name string, files ReportFileReader) string {
	value := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		readString(files, filepath.Join("/sys/class/block", name, "device/protocol")),
		readString(files, filepath.Join("/sys/block", name, "device/protocol")),
	)))
	switch {
	case strings.Contains(value, "nvme"):
		return "nvme"
	case strings.Contains(value, "sata"), strings.Contains(value, "ata"):
		return "ata"
	case strings.Contains(value, "sas"), strings.Contains(value, "scsi"):
		return "scsi"
	default:
		return storageProtocol(name)
	}
}

func collectDiskHWMonTemperature(name string, files ReportFileReader) DiskTemperatureReport {
	report := DiskTemperatureReport{ReportSection: ReportSection{Availability: AvailabilityUnavailable}, Source: "sysfs_hwmon"}
	patterns := []string{
		filepath.Join("/sys/class/block", name, "device/hwmon/hwmon*/temp*_input"),
		filepath.Join("/sys/block", name, "device/hwmon/hwmon*/temp*_input"),
	}
	if controller := nvmeControllerPattern.FindString(name); controller != "" {
		patterns = append(patterns,
			filepath.Join("/sys/class/nvme", controller, "device/hwmon/hwmon*/temp*_input"),
			filepath.Join("/sys/class/nvme", controller, "hwmon/hwmon*/temp*_input"),
		)
	}
	seen := make(map[string]struct{})
	var paths []string
	for _, pattern := range patterns {
		matches, _ := files.Glob(pattern)
		for _, path := range matches {
			if _, ok := seen[path]; !ok {
				seen[path] = struct{}{}
				paths = append(paths, path)
			}
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		millidegrees, err := strconv.ParseInt(strings.TrimSpace(readString(files, path)), 10, 64)
		if err != nil || millidegrees < -50000 || millidegrees > 250000 {
			continue
		}
		celsius := float64(millidegrees) / 1000
		report.Availability = AvailabilityAvailable
		report.Celsius = &celsius
		return report
	}
	return report
}
