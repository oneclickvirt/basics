package system

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const reportLabelDisplayWidth = 20

// RenderSystemReportText adds non-identifying fields that are not represented
// by the historical SystemInfo text. Each row describes one property, while
// rows for the same entity remain adjacent.
func RenderSystemReportText(report *SystemReport, language string) string {
	if report == nil {
		return ""
	}
	return renderExtendedSystemReportText(report, language, report.Network.CongestionControl)
}

func renderHardwareReportText(report *SystemReport, language string) string {
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

	renderCgroupRows(row, report.Cgroup)
	renderFirmwareRows(row, report.Firmware)
	renderPCIGPURows(row, report.PCI, report.GPUs)
	renderMemoryTopologyRows(row, report.MemoryTopology)
	for index, disk := range report.Disks {
		if index >= 4 {
			row("物理盘其余", "Other Physical Disks", fmt.Sprintf("%d", len(report.Disks)-index))
			break
		}
		renderDiskRows(row, index+1, disk)
	}
	renderRAIDRows(row, report.RAID)
	return builder.String()
}

func renderExtendedSystemReportText(report *SystemReport, language, tcpAcceleration string) string {
	zh := strings.EqualFold(strings.TrimSpace(language), "zh")
	var builder strings.Builder
	tcpAcceleration = strings.TrimSpace(tcpAcceleration)
	if tcpAcceleration != "" {
		if zh {
			builder.WriteString(formatReportRow("TCP加速方式", tcpAcceleration))
		} else {
			builder.WriteString(formatReportRow("Tcp Accelerate", tcpAcceleration))
		}
	}
	builder.WriteString(renderNetworkReportText(report, language))
	builder.WriteString(renderHardwareReportText(report, language))
	return builder.String()
}

func renderNetworkReportText(report *SystemReport, language string) string {
	if report == nil || report.Network.Availability != AvailabilityAvailable {
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
	row("TCP队列规则", "TCP Queue Discipline", report.Network.DefaultQdisc)
	row("TCP接收缓冲", "TCP Receive Buffer", formatByteTuple(report.Network.TCPRMem))
	row("TCP发送缓冲", "TCP Send Buffer", formatByteTuple(report.Network.TCPWMem))
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

func renderCgroupRows(row func(string, string, string), cgroup CgroupReport) {
	if cgroup.Availability != AvailabilityAvailable {
		return
	}
	row("Cgroup版本", "Cgroup Version", cgroup.Version)
	if cgroup.CPUQuotaCores != nil {
		row("Cgroup CPU配额", "Cgroup CPU Quota", fmt.Sprintf("%.2f cores", *cgroup.CPUQuotaCores))
	}
	row("Cgroup CPU集合", "Cgroup CPU Set", cgroup.CPUSet)
	if cgroup.MemoryCurrentBytes != nil {
		row("Cgroup内存使用", "Cgroup Memory Usage", formatCompactBytes(*cgroup.MemoryCurrentBytes))
	}
	if cgroup.MemoryLimitBytes != nil {
		row("Cgroup内存上限", "Cgroup Memory Limit", formatCompactBytes(*cgroup.MemoryLimitBytes))
	}
	if cgroup.MemoryHighBytes != nil {
		row("Cgroup内存高水位", "Cgroup Memory High", formatCompactBytes(*cgroup.MemoryHighBytes))
	}
	if cgroup.MemorySwapLimitBytes != nil {
		row("Cgroup交换上限", "Cgroup Swap Limit", formatCompactBytes(*cgroup.MemorySwapLimitBytes))
	}
	if cgroup.PidsLimit != nil {
		row("Cgroup进程上限", "Cgroup PID Limit", fmt.Sprintf("%d", *cgroup.PidsLimit))
	}
}

func renderFirmwareRows(row func(string, string, string), firmware FirmwareReport) {
	if firmware.Availability != AvailabilityAvailable {
		return
	}
	row("主板厂商", "Board Vendor", firmware.BoardVendor)
	row("主板型号", "Board Name", firmware.BoardName)
	row("主板版本", "Board Version", firmware.BoardVersion)
	row("BIOS厂商", "BIOS Vendor", firmware.BIOSVendor)
	row("BIOS版本", "BIOS Version", firmware.BIOSVersion)
	row("BIOS日期", "BIOS Date", firmware.BIOSDate)
}

func renderPCIGPURows(row func(string, string, string), pci PCIReport, gpus []GPUReport) {
	if len(pci.Devices) == 0 && len(gpus) == 0 {
		return
	}
	pciDrivers := make(map[string]struct{})
	gpuDrivers := make(map[string]struct{})
	for _, gpu := range gpus {
		if gpu.Driver != "" {
			gpuDrivers[gpu.Driver] = struct{}{}
		}
	}
	for _, device := range pci.Devices {
		if device.Driver != "" {
			pciDrivers[device.Driver] = struct{}{}
		}
	}
	row("PCI设备数量", "PCI Device Count", fmt.Sprintf("%d", len(pci.Devices)))
	row("PCI驱动", "PCI Drivers", strings.Join(sortedLimitedKeys(pciDrivers, 4), ","))
	row("GPU设备数量", "GPU Device Count", fmt.Sprintf("%d", len(gpus)))
	row("GPU驱动", "GPU Drivers", strings.Join(sortedLimitedKeys(gpuDrivers, 4), ","))
}

func renderMemoryTopologyRows(row func(string, string, string), topology MemoryTopologyReport) {
	if topology.Availability != AvailabilityAvailable {
		return
	}
	row("NUMA节点数量", "NUMA Node Count", fmt.Sprintf("%d", len(topology.Nodes)))
	row("DIMM数量", "DIMM Count", fmt.Sprintf("%d", len(topology.DIMMs)))
	if topology.HugePagesTotal != nil {
		row("HugePages总数", "HugePages Total", fmt.Sprintf("%d", *topology.HugePagesTotal))
	}
	if topology.HugePagesFree != nil {
		row("HugePages空闲", "HugePages Free", fmt.Sprintf("%d", *topology.HugePagesFree))
	}
	if topology.HugePageBytes != nil {
		row("HugePage大小", "HugePage Size", formatCompactBytes(*topology.HugePageBytes))
	}
}

func renderDiskRows(row func(string, string, string), index int, disk DiskReport) {
	zhPrefix := fmt.Sprintf("物理盘 %d", index)
	enPrefix := fmt.Sprintf("Disk %d", index)
	protocol := disk.Health.Protocol
	if protocol == "" || protocol == "unknown" {
		protocol = storageProtocol(disk.Name)
	}
	if protocol == "unknown" {
		protocol = ""
	}
	row(zhPrefix+" 协议", enPrefix+" Protocol", protocol)
	health := disk.Health.Status
	if health == "" && disk.Health.Availability != "" && disk.Health.Availability != AvailabilityAvailable {
		health = string(disk.Health.Availability)
	}
	row(zhPrefix+" 健康", enPrefix+" Health", health)
	if disk.Temperature.Celsius != nil {
		row(zhPrefix+" 温度", enPrefix+" Temperature", fmt.Sprintf("%.1f C", *disk.Temperature.Celsius))
	}
}

func renderRAIDRows(row func(string, string, string), raid RAIDReport) {
	if len(raid.Arrays) == 0 && len(raid.Controllers) == 0 {
		return
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
	row("RAID阵列数量", "RAID Arrays", fmt.Sprintf("%d", len(raid.Arrays)))
	row("RAID级别", "RAID Levels", strings.Join(sortedLimitedKeys(levels, 3), ","))
	if degraded > 0 {
		row("RAID降级阵列", "RAID Degraded Arrays", fmt.Sprintf("%d", degraded))
	}
	row("RAID控制器数量", "RAID Controllers", fmt.Sprintf("%d", len(raid.Controllers)))
	row("RAID驱动", "RAID Drivers", strings.Join(sortedLimitedKeys(drivers, 3), ","))
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
