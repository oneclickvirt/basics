package system

import (
	"path/filepath"
	"sort"
	"strings"
)

func collectPCIReport(files ReportFileReader, operatingSystem string) PCIReport {
	report := PCIReport{ReportSection: ReportSection{Availability: AvailabilityUnsupported}}
	if operatingSystem != "linux" {
		return report
	}
	paths, err := files.Glob("/sys/bus/pci/devices/*")
	if err != nil {
		report.Availability = AvailabilityError
		report.Error = "PCI sysfs enumeration failed"
		return report
	}
	sort.Strings(paths)
	readable := 0
	for _, path := range paths {
		uevent := parsePCIUEVent(readString(files, filepath.Join(path, "uevent")))
		vendor := normalizePCIHex(readString(files, filepath.Join(path, "vendor")), 4)
		device := normalizePCIHex(readString(files, filepath.Join(path, "device")), 4)
		classID := normalizePCIHex(readString(files, filepath.Join(path, "class")), 6)
		if pciID := strings.SplitN(uevent["PCI_ID"], ":", 2); len(pciID) == 2 {
			if vendor == "" {
				vendor = normalizePCIHex(pciID[0], 4)
			}
			if device == "" {
				device = normalizePCIHex(pciID[1], 4)
			}
		}
		if classID == "" {
			classID = normalizePCIHex(uevent["PCI_CLASS"], 6)
		}
		driver := strings.TrimSpace(uevent["DRIVER"])
		if vendor != "" || device != "" || classID != "" || driver != "" {
			readable++
		}
		report.Devices = append(report.Devices, PCIDeviceReport{
			Address: filepath.Base(filepath.Clean(path)), VendorID: vendor,
			DeviceID: device, ClassID: classID, Driver: driver,
		})
	}
	if len(report.Devices) == 0 {
		report.Availability = AvailabilityUnavailable
		report.Error = "PCI devices are unavailable"
		return report
	}
	if readable == 0 {
		report.Availability = AvailabilityUnavailable
		report.Error = "PCI device fields are unreadable"
		return report
	}
	report.Availability = AvailabilityAvailable
	return report
}

func parsePCIUEVent(content string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return values
}

func normalizePCIHex(value string, width int) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "0x")
	if value == "" || len(value) > width {
		return ""
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return ""
		}
	}
	return "0x" + strings.Repeat("0", width-len(value)) + value
}
