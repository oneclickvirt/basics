package system

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const reportLabelDisplayWidth = 20

// RenderSystemReportText adds a compact, non-identifying summary for fields
// that are not represented by the historical SystemInfo text.
func RenderSystemReportText(report *SystemReport, language string) string {
	if report == nil {
		return ""
	}
	zh := strings.EqualFold(strings.TrimSpace(language), "zh")
	var builder strings.Builder
	row := func(zhLabel, enLabel, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if zh {
			builder.WriteString(formatReportRow(zhLabel, value))
		} else {
			builder.WriteString(formatReportRow(enLabel, value))
		}
	}

	row("Cgroup 限制", "Cgroup Limits", cgroupSummary(report.Cgroup))
	row("TCP 缓冲/队列", "TCP Buffers/Qdisc", networkTuningSummary(report.Network))
	row("主板/BIOS", "Board/BIOS", firmwareSummary(report.Firmware))
	row("PCI/GPU", "PCI/GPU", pciGPUSummary(report.PCI, report.GPUs))
	row("内存拓扑", "Memory Topology", memoryTopologySummary(report.MemoryTopology))
	for index, disk := range report.Disks {
		if index >= 4 {
			row("物理盘其余", "Other Physical Disks", fmt.Sprintf("%d", len(report.Disks)-index))
			break
		}
		row(fmt.Sprintf("物理盘 %d", index+1), fmt.Sprintf("Physical Disk %d", index+1), diskSummary(disk))
	}
	row("RAID", "RAID", raidSummary(report.RAID))
	return builder.String()
}

func formatReportRow(label, value string) string {
	padding := reportLabelDisplayWidth - reportTextDisplayWidth(label)
	if padding < 0 {
		padding = 0
	}
	return " " + label + strings.Repeat(" ", padding) + ": " + value + "\n"
}

// reportTextDisplayWidth follows the terminal width used by the compact
// report. CJK and other wide runes occupy two cells, unlike len/rune counts.
func reportTextDisplayWidth(value string) int {
	width := 0
	for _, r := range value {
		switch {
		case r == '\t':
			width += 4
		case unicode.IsControl(r):
			continue
		case reportRuneIsWide(r):
			width += 2
		default:
			width++
		}
	}
	return width
}

func reportRuneIsWide(r rune) bool {
	return (r >= 0x1100 && r <= 0x115f) ||
		(r >= 0x2329 && r <= 0x232a) ||
		(r >= 0x2e80 && r <= 0xa4cf) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) ||
		(r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1faff)
}

func cgroupSummary(cgroup CgroupReport) string {
	if cgroup.Availability != AvailabilityAvailable {
		return ""
	}
	parts := make([]string, 0, 4)
	if cgroup.Version != "" {
		parts = append(parts, cgroup.Version)
	}
	if cgroup.CPUQuotaCores != nil {
		parts = append(parts, fmt.Sprintf("CPU %.2f", *cgroup.CPUQuotaCores))
	}
	if cgroup.CPUSet != "" {
		parts = append(parts, "cpuset "+cgroup.CPUSet)
	}
	if cgroup.MemoryCurrentBytes != nil && cgroup.MemoryLimitBytes != nil {
		parts = append(parts, "memory "+formatCompactBytes(*cgroup.MemoryCurrentBytes)+"/"+formatCompactBytes(*cgroup.MemoryLimitBytes))
	} else if cgroup.MemoryLimitBytes != nil {
		parts = append(parts, "memory limit "+formatCompactBytes(*cgroup.MemoryLimitBytes))
	}
	if cgroup.PidsLimit != nil {
		parts = append(parts, fmt.Sprintf("pids %d", *cgroup.PidsLimit))
	}
	return strings.Join(parts, " / ")
}

func networkTuningSummary(network NetworkTuningReport) string {
	if network.Availability != AvailabilityAvailable {
		return ""
	}
	parts := make([]string, 0, 3)
	if network.DefaultQdisc != "" {
		parts = append(parts, "qdisc "+network.DefaultQdisc)
	}
	if value := formatByteTuple(network.TCPRMem); value != "" {
		parts = append(parts, "rmem "+value)
	}
	if value := formatByteTuple(network.TCPWMem); value != "" {
		parts = append(parts, "wmem "+value)
	}
	return strings.Join(parts, " / ")
}

func firmwareSummary(firmware FirmwareReport) string {
	if firmware.Availability != AvailabilityAvailable {
		return ""
	}
	board := joinValues(firmware.BoardVendor, firmware.BoardName, firmware.BoardVersion)
	bios := joinValues(firmware.BIOSVendor, firmware.BIOSVersion, firmware.BIOSDate)
	if board != "" && bios != "" {
		return board + " / BIOS " + bios
	}
	return joinValues(board, bios)
}

func pciGPUSummary(pci PCIReport, gpus []GPUReport) string {
	if len(pci.Devices) == 0 && len(gpus) == 0 {
		return ""
	}
	parts := []string{fmt.Sprintf("PCI %d", len(pci.Devices)), fmt.Sprintf("GPU %d", len(gpus))}
	drivers := make(map[string]struct{})
	for _, gpu := range gpus {
		if gpu.Driver != "" {
			drivers[gpu.Driver] = struct{}{}
		}
	}
	for _, device := range pci.Devices {
		if device.Driver != "" {
			drivers[device.Driver] = struct{}{}
		}
	}
	values := sortedLimitedKeys(drivers, 4)
	if len(values) > 0 {
		parts = append(parts, "drivers "+strings.Join(values, ","))
	}
	return strings.Join(parts, " / ")
}

func memoryTopologySummary(topology MemoryTopologyReport) string {
	if topology.Availability != AvailabilityAvailable {
		return ""
	}
	parts := []string{fmt.Sprintf("NUMA %d", len(topology.Nodes)), fmt.Sprintf("DIMM %d", len(topology.DIMMs))}
	var dimmBytes int64
	for _, dimm := range topology.DIMMs {
		if dimm.SizeBytes != nil && *dimm.SizeBytes <= (1<<63-1)-dimmBytes {
			dimmBytes += *dimm.SizeBytes
		}
	}
	if dimmBytes > 0 {
		parts = append(parts, "DIMM total "+formatCompactBytes(dimmBytes))
	}
	if topology.HugePagesTotal != nil {
		huge := fmt.Sprintf("hugepages %d", *topology.HugePagesTotal)
		if topology.HugePagesFree != nil {
			huge += fmt.Sprintf("/%d free", *topology.HugePagesFree)
		}
		if topology.HugePageBytes != nil {
			huge += " @ " + formatCompactBytes(*topology.HugePageBytes)
		}
		parts = append(parts, huge)
	}
	return strings.Join(parts, " / ")
}

func diskSummary(disk DiskReport) string {
	parts := make([]string, 0, 3)
	protocol := disk.Health.Protocol
	if protocol == "" || protocol == "unknown" {
		protocol = storageProtocol(disk.Name)
	}
	if protocol != "" && protocol != "unknown" {
		parts = append(parts, protocol)
	}
	if disk.Health.Status != "" {
		parts = append(parts, disk.Health.Status)
	} else if disk.Health.Availability != "" && disk.Health.Availability != AvailabilityAvailable {
		parts = append(parts, string(disk.Health.Availability))
	}
	if disk.Temperature.Celsius != nil {
		parts = append(parts, fmt.Sprintf("%.1f C", *disk.Temperature.Celsius))
	}
	return strings.Join(parts, " / ")
}

func raidSummary(raid RAIDReport) string {
	if len(raid.Arrays) == 0 && len(raid.Controllers) == 0 {
		return ""
	}
	degraded := 0
	levels := make(map[string]struct{})
	for _, array := range raid.Arrays {
		if array.Degraded {
			degraded++
		}
		if array.Level != "" {
			levels[array.Level] = struct{}{}
		}
	}
	drivers := make(map[string]struct{})
	for _, controller := range raid.Controllers {
		if controller.Driver != "" {
			drivers[controller.Driver] = struct{}{}
		}
	}
	parts := []string{fmt.Sprintf("arrays %d", len(raid.Arrays)), fmt.Sprintf("controllers %d", len(raid.Controllers))}
	if values := sortedLimitedKeys(levels, 3); len(values) > 0 {
		parts = append(parts, "levels "+strings.Join(values, ","))
	}
	if degraded > 0 {
		parts = append(parts, fmt.Sprintf("degraded %d", degraded))
	}
	if values := sortedLimitedKeys(drivers, 3); len(values) > 0 {
		parts = append(parts, "drivers "+strings.Join(values, ","))
	}
	return strings.Join(parts, " / ")
}

func formatByteTuple(values []int64) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, formatCompactBytes(value))
	}
	return strings.Join(parts, "/")
}

func formatCompactBytes(value int64) string {
	if value < 0 {
		return ""
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	amount := float64(value)
	unit := 0
	for amount >= 1024 && unit < len(units)-1 {
		amount /= 1024
		unit++
	}
	if amount >= 10 || amount == float64(int64(amount)) {
		return fmt.Sprintf("%.0f %s", amount, units[unit])
	}
	return fmt.Sprintf("%.1f %s", amount, units[unit])
}

func joinValues(values ...string) string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return strings.Join(result, " ")
}

func sortedLimitedKeys(values map[string]struct{}, limit int) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	if len(result) > limit {
		result = append(result[:limit], fmt.Sprintf("+%d", len(result)-limit))
	}
	return result
}
