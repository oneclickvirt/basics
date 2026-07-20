package system

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type Availability string

const (
	AvailabilityAvailable        Availability = "available"
	AvailabilityUnavailable      Availability = "unavailable"
	AvailabilityUnsupported      Availability = "unsupported"
	AvailabilityPermissionDenied Availability = "permission_denied"
	AvailabilityError            Availability = "error"
	AvailabilityCanceled         Availability = "canceled"
)

type ReportFileReader interface {
	ReadFile(path string) ([]byte, error)
	Glob(pattern string) ([]string, error)
}

type OSReportFileReader struct{}

func (OSReportFileReader) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (OSReportFileReader) Glob(pattern string) ([]string, error) { return filepath.Glob(pattern) }

type ReportSection struct {
	Availability Availability `json:"availability"`
	Error        string       `json:"error,omitempty"`
}

type CPUReport struct {
	ReportSection
	Model          string `json:"model,omitempty"`
	LogicalCPUs    *int   `json:"logical_cpus,omitempty"`
	PhysicalCores  *int   `json:"physical_cores,omitempty"`
	ThreadsPerCore *int   `json:"threads_per_core,omitempty"`
	Sockets        *int   `json:"sockets,omitempty"`
	CPUSet         string `json:"cpuset,omitempty"`
}

type MemoryReport struct {
	ReportSection
	TotalBytes     *int64 `json:"total_bytes,omitempty"`
	AvailableBytes *int64 `json:"available_bytes,omitempty"`
	SwapTotalBytes *int64 `json:"swap_total_bytes,omitempty"`
	SwapFreeBytes  *int64 `json:"swap_free_bytes,omitempty"`
}

type CgroupReport struct {
	ReportSection
	Version              string   `json:"version,omitempty"`
	CPUQuotaMicros       *int64   `json:"cpu_quota_micros,omitempty"`
	CPUPeriodMicros      *int64   `json:"cpu_period_micros,omitempty"`
	CPUQuotaCores        *float64 `json:"cpu_quota_cores,omitempty"`
	CPUSet               string   `json:"cpuset,omitempty"`
	MemoryLimitBytes     *int64   `json:"memory_limit_bytes,omitempty"`
	MemoryHighBytes      *int64   `json:"memory_high_bytes,omitempty"`
	MemoryCurrentBytes   *int64   `json:"memory_current_bytes,omitempty"`
	MemorySwapLimitBytes *int64   `json:"memory_swap_limit_bytes,omitempty"`
	PidsLimit            *int64   `json:"pids_limit,omitempty"`
}

type VirtualizationReport struct {
	ReportSection
	Type             string `json:"type,omitempty"`
	Container        bool   `json:"container"`
	ContainerRuntime string `json:"container_runtime,omitempty"`
}

type GPUReport struct {
	ReportSection
	Path     string `json:"path,omitempty"`
	VendorID string `json:"vendor_id,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
	Driver   string `json:"driver,omitempty"`
}

// PCIDeviceReport contains the non-identifying PCI topology exposed by Linux
// sysfs.  Serial numbers and other device-unique fields are intentionally not
// read or represented here.
type PCIDeviceReport struct {
	Address  string `json:"address,omitempty"`
	VendorID string `json:"vendor_id,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
	ClassID  string `json:"class_id,omitempty"`
	Driver   string `json:"driver,omitempty"`
}

type PCIReport struct {
	ReportSection
	Devices []PCIDeviceReport `json:"devices,omitempty"`
}

type DiskReport struct {
	ReportSection
	Name            string                `json:"name,omitempty"`
	SizeBytes       *int64                `json:"size_bytes,omitempty"`
	LogicalBytes    *int64                `json:"logical_block_bytes,omitempty"`
	Model           string                `json:"model,omitempty"`
	Vendor          string                `json:"vendor,omitempty"`
	Firmware        string                `json:"firmware,omitempty"`
	ControllerState string                `json:"controller_state,omitempty"`
	ReadOnly        *bool                 `json:"read_only,omitempty"`
	Rotational      *bool                 `json:"rotational,omitempty"`
	Health          DiskHealthReport      `json:"health"`
	Temperature     DiskTemperatureReport `json:"temperature"`
}

// DiskHealthReport contains passive health information. No self-test or write
// command is issued by the collectors.
type DiskHealthReport struct {
	ReportSection
	Protocol                string  `json:"protocol,omitempty"`
	Source                  string  `json:"source,omitempty"`
	Status                  string  `json:"status,omitempty"`
	CriticalWarning         *uint8  `json:"critical_warning,omitempty"`
	AvailableSparePct       *uint8  `json:"available_spare_percent,omitempty"`
	SpareThresholdPct       *uint8  `json:"spare_threshold_percent,omitempty"`
	PercentageUsed          *uint8  `json:"percentage_used,omitempty"`
	DataUnitsRead           *uint64 `json:"data_units_read,omitempty"`
	DataUnitsWritten        *uint64 `json:"data_units_written,omitempty"`
	DataUnitsReadDecimal    string  `json:"data_units_read_decimal,omitempty"`
	DataUnitsWrittenDecimal string  `json:"data_units_written_decimal,omitempty"`
	MediaErrors             *uint64 `json:"media_errors,omitempty"`
	PowerCycles             *uint64 `json:"power_cycles,omitempty"`
	PowerOnHours            *uint64 `json:"power_on_hours,omitempty"`
	UnsafeShutdowns         *uint64 `json:"unsafe_shutdowns,omitempty"`
	MediaErrorsDecimal      string  `json:"media_errors_decimal,omitempty"`
	PowerCyclesDecimal      string  `json:"power_cycles_decimal,omitempty"`
	PowerOnHoursDecimal     string  `json:"power_on_hours_decimal,omitempty"`
	UnsafeShutdownsDecimal  string  `json:"unsafe_shutdowns_decimal,omitempty"`
	CountersSaturated       bool    `json:"counters_saturated,omitempty"`
	ReallocatedSectors      *uint64 `json:"reallocated_sectors,omitempty"`
	PendingSectors          *uint64 `json:"pending_sectors,omitempty"`
	OfflineUncorrectable    *uint64 `json:"offline_uncorrectable,omitempty"`
}

type DiskTemperatureReport struct {
	ReportSection
	Celsius *float64 `json:"celsius,omitempty"`
	Source  string   `json:"source,omitempty"`
}

type diskHealthCollector interface {
	Collect(name string, files ReportFileReader) (DiskHealthReport, DiskTemperatureReport)
}

type NetworkTuningReport struct {
	ReportSection
	CongestionControl          string   `json:"congestion_control,omitempty"`
	AvailableCongestionControl []string `json:"available_congestion_control,omitempty"`
	DefaultQdisc               string   `json:"default_qdisc,omitempty"`
	TCPRMem                    []int64  `json:"tcp_rmem,omitempty"`
	TCPWMem                    []int64  `json:"tcp_wmem,omitempty"`
}

type FirmwareReport struct {
	ReportSection
	BoardVendor  string `json:"board_vendor,omitempty"`
	BoardName    string `json:"board_name,omitempty"`
	BoardVersion string `json:"board_version,omitempty"`
	BIOSVendor   string `json:"bios_vendor,omitempty"`
	BIOSVersion  string `json:"bios_version,omitempty"`
	BIOSDate     string `json:"bios_date,omitempty"`
}

type NUMANodeReport struct {
	Node     string `json:"node"`
	CPUSet   string `json:"cpuset,omitempty"`
	MemBytes *int64 `json:"memory_bytes,omitempty"`
}

type MemoryTopologyReport struct {
	ReportSection
	Nodes          []NUMANodeReport `json:"nodes,omitempty"`
	DIMMs          []DIMMReport     `json:"dimms,omitempty"`
	HugePagesTotal *int64           `json:"hugepages_total,omitempty"`
	HugePagesFree  *int64           `json:"hugepages_free,omitempty"`
	HugePageBytes  *int64           `json:"hugepage_bytes,omitempty"`
}

type DIMMReport struct {
	Locator            string `json:"locator,omitempty"`
	Bank               string `json:"bank,omitempty"`
	SizeBytes          *int64 `json:"size_bytes,omitempty"`
	Type               string `json:"type,omitempty"`
	Manufacturer       string `json:"manufacturer,omitempty"`
	PartNumber         string `json:"part_number,omitempty"`
	SpeedMTs           *int64 `json:"speed_mt_s,omitempty"`
	ConfiguredSpeedMTs *int64 `json:"configured_speed_mt_s,omitempty"`
	SerialRedacted     bool   `json:"serial_redacted"`
}

type RAIDArrayReport struct {
	Name       string   `json:"name"`
	Level      string   `json:"level,omitempty"`
	Members    []string `json:"members,omitempty"`
	State      string   `json:"state,omitempty"`
	Degraded   bool     `json:"degraded"`
	SyncAction string   `json:"sync_action,omitempty"`
}

type RAIDControllerReport struct {
	Address  string `json:"address,omitempty"`
	VendorID string `json:"vendor_id,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
	ClassID  string `json:"class_id,omitempty"`
	Driver   string `json:"driver,omitempty"`
}

type RAIDReport struct {
	ReportSection
	Arrays      []RAIDArrayReport      `json:"arrays,omitempty"`
	Controllers []RAIDControllerReport `json:"controllers,omitempty"`
}

type SystemReport struct {
	SchemaVersion  string               `json:"schema_version"`
	Availability   Availability         `json:"availability"`
	Error          string               `json:"error,omitempty"`
	CPU            CPUReport            `json:"cpu"`
	Memory         MemoryReport         `json:"memory"`
	Cgroup         CgroupReport         `json:"cgroup"`
	Virtualization VirtualizationReport `json:"virtualization"`
	GPUs           []GPUReport          `json:"gpus,omitempty"`
	PCI            PCIReport            `json:"pci"`
	Disks          []DiskReport         `json:"disks,omitempty"`
	Network        NetworkTuningReport  `json:"network"`
	Firmware       FirmwareReport       `json:"firmware"`
	MemoryTopology MemoryTopologyReport `json:"memory_topology"`
	RAID           RAIDReport           `json:"raid"`
}

func GetSystemReport() *SystemReport {
	return CollectSystemReport(context.Background())
}

func CollectSystemReport(ctx context.Context) *SystemReport {
	return collectSystemReport(ctx, OSReportFileReader{}, defaultDiskHealthCollector(), runtime.GOOS)
}

func CollectSystemReportFrom(ctx context.Context, files ReportFileReader, operatingSystem string) *SystemReport {
	return collectSystemReport(ctx, files, nil, operatingSystem)
}

// CollectSystemReportFromWithDiskHealth is the fixture-friendly entrypoint for
// callers that want to inject a passive health reader. The regular reader is
// intentionally kept separate so tests never need access to /dev devices.
func CollectSystemReportFromWithDiskHealth(ctx context.Context, files ReportFileReader, collector diskHealthCollector, operatingSystem string) *SystemReport {
	return collectSystemReport(ctx, files, collector, operatingSystem)
}

func collectSystemReport(ctx context.Context, files ReportFileReader, collector diskHealthCollector, operatingSystem string) *SystemReport {
	report := &SystemReport{SchemaVersion: "goecs.system/v1", Availability: AvailabilityAvailable}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		report.Availability = AvailabilityCanceled
		report.Error = err.Error()
		return report
	}
	report.CPU = collectCPUReport(files, operatingSystem)
	report.Memory = collectMemoryReport(files, operatingSystem)
	report.Cgroup = collectCgroupReport(files, operatingSystem)
	report.Virtualization = collectVirtualizationReport(files, operatingSystem)
	report.GPUs = collectGPUReports(files, operatingSystem)
	report.PCI = collectPCIReport(files, operatingSystem)
	report.Disks = collectDiskReports(files, operatingSystem, collector)
	report.Network = collectNetworkTuningReport(files, operatingSystem)
	report.Firmware = collectFirmwareReport(files, operatingSystem)
	report.MemoryTopology = collectMemoryTopologyReport(files, operatingSystem)
	report.RAID = collectRAIDReport(files, operatingSystem)
	report.RAID.Controllers = raidControllersFromPCI(report.PCI)
	if len(report.RAID.Controllers) > 0 && report.RAID.Availability != AvailabilityAvailable {
		report.RAID.Availability = AvailabilityAvailable
		report.RAID.Error = ""
	}
	if !hasAvailableSection(report.CPU.ReportSection, report.Memory.ReportSection, report.Cgroup.ReportSection, report.Virtualization.ReportSection) {
		report.Availability = AvailabilityUnavailable
	}
	return report
}

func raidControllersFromPCI(pci PCIReport) []RAIDControllerReport {
	result := make([]RAIDControllerReport, 0)
	for _, device := range pci.Devices {
		driver := strings.ToLower(strings.TrimSpace(device.Driver))
		hardwareRAIDDriver := false
		for _, name := range []string{"megaraid", "hpsa", "smartpqi", "aacraid", "arcmsr", "mpt3sas", "3w-"} {
			if strings.Contains(driver, name) {
				hardwareRAIDDriver = true
				break
			}
		}
		if device.ClassID != "0x010400" && !hardwareRAIDDriver {
			continue
		}
		result = append(result, RAIDControllerReport{
			Address: device.Address, VendorID: device.VendorID, DeviceID: device.DeviceID,
			ClassID: device.ClassID, Driver: device.Driver,
		})
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Address < result[j].Address })
	return result
}

func collectFirmwareReport(files ReportFileReader, operatingSystem string) FirmwareReport {
	result := FirmwareReport{ReportSection: ReportSection{Availability: AvailabilityUnsupported}}
	if operatingSystem != "linux" {
		return result
	}
	result.BoardVendor = strings.TrimSpace(readString(files, "/sys/class/dmi/id/board_vendor"))
	result.BoardName = strings.TrimSpace(readString(files, "/sys/class/dmi/id/board_name"))
	result.BoardVersion = strings.TrimSpace(readString(files, "/sys/class/dmi/id/board_version"))
	result.BIOSVendor = strings.TrimSpace(readString(files, "/sys/class/dmi/id/bios_vendor"))
	result.BIOSVersion = strings.TrimSpace(readString(files, "/sys/class/dmi/id/bios_version"))
	result.BIOSDate = strings.TrimSpace(readString(files, "/sys/class/dmi/id/bios_date"))
	if result.BoardVendor == "" && result.BoardName == "" && result.BIOSVendor == "" && result.BIOSVersion == "" {
		result.Availability = AvailabilityUnavailable
		result.Error = "DMI firmware fields are unavailable"
	} else {
		result.Availability = AvailabilityAvailable
	}
	return result
}

func collectMemoryTopologyReport(files ReportFileReader, operatingSystem string) MemoryTopologyReport {
	result := MemoryTopologyReport{ReportSection: ReportSection{Availability: AvailabilityUnsupported}}
	if operatingSystem != "linux" {
		return result
	}
	memInfo := parseMemInfo(readString(files, "/proc/meminfo"))
	result.HugePagesTotal = memInfo["HugePages_Total"]
	result.HugePagesFree = memInfo["HugePages_Free"]
	result.HugePageBytes = memInfo["Hugepagesize"]
	paths, _ := files.Glob("/sys/devices/system/node/node[0-9]*")
	sort.Strings(paths)
	for _, path := range paths {
		node := NUMANodeReport{Node: filepath.Base(path), CPUSet: strings.TrimSpace(readString(files, filepath.Join(path, "cpulist")))}
		values := parseMemInfo(readString(files, filepath.Join(path, "meminfo")))
		for key, value := range values {
			if strings.HasSuffix(key, "MemTotal") {
				node.MemBytes = value
				break
			}
		}
		result.Nodes = append(result.Nodes, node)
	}
	if dmi, err := files.ReadFile("/sys/firmware/dmi/tables/DMI"); err == nil {
		result.DIMMs = parseDMIType17(dmi)
	}
	if len(result.Nodes) == 0 && len(result.DIMMs) == 0 && result.HugePagesTotal == nil && result.HugePageBytes == nil {
		result.Availability = AvailabilityUnavailable
		result.Error = "NUMA and hugepage data are unavailable"
	} else {
		result.Availability = AvailabilityAvailable
	}
	return result
}

func parseDMIType17(data []byte) []DIMMReport {
	var result []DIMMReport
	for offset := 0; offset+4 <= len(data); {
		structureType, length := data[offset], int(data[offset+1])
		if length < 4 || offset+length > len(data) {
			break
		}
		stringsStart := offset + length
		end := stringsStart
		for end+1 < len(data) && !(data[end] == 0 && data[end+1] == 0) {
			end++
		}
		if end+1 >= len(data) {
			break
		}
		if structureType == 17 && length >= 0x1b {
			formatted := data[offset : offset+length]
			stringsTable := parseDMIStrings(data[stringsStart:end])
			sizeRaw := binary.LittleEndian.Uint16(formatted[0x0c:0x0e])
			if sizeRaw != 0 && sizeRaw != 0xffff {
				var sizeBytes int64
				if sizeRaw == 0x7fff && length >= 0x20 {
					sizeBytes = int64(binary.LittleEndian.Uint32(formatted[0x1c:0x20])) * 1024 * 1024
				} else if sizeRaw&0x8000 != 0 {
					sizeBytes = int64(sizeRaw&0x7fff) * 1024
				} else {
					sizeBytes = int64(sizeRaw) * 1024 * 1024
				}
				dimm := DIMMReport{
					Locator: dmiString(stringsTable, formatted[0x10]), Bank: dmiString(stringsTable, formatted[0x11]),
					SizeBytes: int64Ptr(sizeBytes), Type: memoryTypeName(formatted[0x12]),
					Manufacturer: dmiString(stringsTable, formatted[0x17]), PartNumber: strings.TrimSpace(dmiString(stringsTable, formatted[0x1a])),
					SerialRedacted: dmiString(stringsTable, formatted[0x18]) != "",
				}
				if speed := binary.LittleEndian.Uint16(formatted[0x15:0x17]); speed > 0 && speed != 0xffff {
					dimm.SpeedMTs = int64Ptr(int64(speed))
				}
				if length >= 0x22 {
					if speed := binary.LittleEndian.Uint16(formatted[0x20:0x22]); speed > 0 && speed != 0xffff {
						dimm.ConfiguredSpeedMTs = int64Ptr(int64(speed))
					}
				}
				result = append(result, dimm)
			}
		}
		offset = end + 2
		if structureType == 127 {
			break
		}
	}
	return result
}

func parseDMIStrings(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	parts := strings.Split(string(data), "\x00")
	result := make([]string, 0, len(parts))
	for _, value := range parts {
		result = append(result, strings.TrimSpace(value))
	}
	return result
}

func dmiString(values []string, index byte) string {
	if index == 0 || int(index) > len(values) {
		return ""
	}
	return values[int(index)-1]
}

func memoryTypeName(value byte) string {
	types := map[byte]string{0x12: "DDR", 0x13: "DDR2", 0x18: "DDR3", 0x1a: "DDR4", 0x1b: "LPDDR", 0x1c: "LPDDR2", 0x1d: "LPDDR3", 0x1e: "LPDDR4", 0x22: "DDR5", 0x23: "LPDDR5"}
	if name := types[value]; name != "" {
		return name
	}
	return "unknown"
}

func collectRAIDReport(files ReportFileReader, operatingSystem string) RAIDReport {
	result := RAIDReport{ReportSection: ReportSection{Availability: AvailabilityUnsupported}}
	if operatingSystem != "linux" {
		return result
	}
	content, err := files.ReadFile("/proc/mdstat")
	if err != nil {
		result.Availability = AvailabilityUnavailable
		result.Error = "mdstat is unavailable"
		return result
	}
	lines := strings.Split(string(content), "\n")
	for index, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 || !strings.HasPrefix(fields[0], "md") || fields[1] != ":" {
			continue
		}
		array := RAIDArrayReport{Name: fields[0], State: fields[2]}
		if len(fields) > 3 {
			array.Level = fields[3]
		}
		for _, member := range fields[4:] {
			if bracket := strings.IndexByte(member, '['); bracket > 0 {
				array.Members = append(array.Members, member[:bracket])
			}
		}
		if index+1 < len(lines) {
			statusLine := strings.TrimSpace(lines[index+1])
			array.Degraded = strings.Contains(statusLine, "_")
		}
		array.SyncAction = strings.TrimSpace(readString(files, filepath.Join("/sys/block", array.Name, "md/sync_action")))
		result.Arrays = append(result.Arrays, array)
	}
	result.Availability = AvailabilityAvailable
	return result
}

func collectNetworkTuningReport(files ReportFileReader, operatingSystem string) NetworkTuningReport {
	result := NetworkTuningReport{ReportSection: ReportSection{Availability: AvailabilityUnsupported}}
	if operatingSystem != "linux" {
		return result
	}
	result.CongestionControl = strings.TrimSpace(readString(files, "/proc/sys/net/ipv4/tcp_congestion_control"))
	result.AvailableCongestionControl = strings.Fields(readString(files, "/proc/sys/net/ipv4/tcp_available_congestion_control"))
	result.DefaultQdisc = strings.TrimSpace(readString(files, "/proc/sys/net/core/default_qdisc"))
	result.TCPRMem = parseInt64Fields(readString(files, "/proc/sys/net/ipv4/tcp_rmem"))
	result.TCPWMem = parseInt64Fields(readString(files, "/proc/sys/net/ipv4/tcp_wmem"))
	if result.CongestionControl == "" && result.DefaultQdisc == "" && len(result.TCPRMem) == 0 && len(result.TCPWMem) == 0 {
		result.Availability = AvailabilityUnavailable
		result.Error = "network tuning parameters are unavailable"
		return result
	}
	result.Availability = AvailabilityAvailable
	return result
}

func hasAvailableSection(sections ...ReportSection) bool {
	for _, section := range sections {
		if section.Availability == AvailabilityAvailable {
			return true
		}
	}
	return false
}

func collectCPUReport(files ReportFileReader, operatingSystem string) CPUReport {
	result := CPUReport{ReportSection: ReportSection{Availability: AvailabilityUnsupported}}
	if operatingSystem != "linux" {
		return result
	}
	content, err := files.ReadFile("/proc/cpuinfo")
	if err != nil {
		result.Availability = AvailabilityUnavailable
		result.Error = err.Error()
		return result
	}
	records := strings.Split(string(content), "\n\n")
	logical, physical, sockets := 0, make(map[string]struct{}), make(map[string]struct{})
	for _, record := range records {
		fields := parseKeyValues(record)
		if len(fields) == 0 {
			continue
		}
		logical++
		if result.Model == "" {
			result.Model = firstNonEmpty(fields["model name"], fields["Processor"], fields["machine"])
		}
		physicalID := firstNonEmpty(fields["physical id"], fields["package id"])
		coreID := firstNonEmpty(fields["core id"], fields["cpu number"])
		if physicalID != "" || coreID != "" {
			physical[physicalID+"/"+coreID] = struct{}{}
		}
		if physicalID != "" {
			sockets[physicalID] = struct{}{}
		}
	}
	if logical == 0 {
		result.Availability = AvailabilityUnavailable
		result.Error = "cpuinfo contains no processor records"
		return result
	}
	result.LogicalCPUs = intPtr(logical)
	if len(physical) > 0 {
		result.PhysicalCores = intPtr(len(physical))
	}
	if len(sockets) > 0 {
		result.Sockets = intPtr(len(sockets))
	}
	if result.PhysicalCores != nil && *result.PhysicalCores > 0 {
		result.ThreadsPerCore = intPtr(logical / *result.PhysicalCores)
	}
	if cpuset, ok := readFirst(files, "/sys/devices/system/cpu/online"); ok {
		result.CPUSet = strings.TrimSpace(string(cpuset))
		if count := countCPUSet(result.CPUSet); count > 0 {
			result.LogicalCPUs = intPtr(count)
		}
	}
	result.Availability = AvailabilityAvailable
	return result
}

func collectMemoryReport(files ReportFileReader, operatingSystem string) MemoryReport {
	result := MemoryReport{ReportSection: ReportSection{Availability: AvailabilityUnsupported}}
	if operatingSystem != "linux" {
		return result
	}
	content, err := files.ReadFile("/proc/meminfo")
	if err != nil {
		result.Availability = AvailabilityUnavailable
		result.Error = err.Error()
		return result
	}
	values := parseMemInfo(string(content))
	result.TotalBytes = values["MemTotal"]
	result.AvailableBytes = values["MemAvailable"]
	result.SwapTotalBytes = values["SwapTotal"]
	result.SwapFreeBytes = values["SwapFree"]
	if result.TotalBytes == nil {
		result.Availability = AvailabilityUnavailable
		result.Error = "meminfo contains no MemTotal"
		return result
	}
	result.Availability = AvailabilityAvailable
	return result
}

func collectCgroupReport(files ReportFileReader, operatingSystem string) CgroupReport {
	result := CgroupReport{ReportSection: ReportSection{Availability: AvailabilityUnsupported}}
	if operatingSystem != "linux" {
		return result
	}
	if _, err := files.ReadFile("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		result.Version = "v2"
		result.CPUQuotaMicros, result.CPUPeriodMicros = parseCPUQuota(readCgroupV2(files, "cpu.max"))
		result.CPUSet = strings.TrimSpace(readCgroupV2(files, "cpuset.cpus.effective"))
		if result.CPUSet == "" {
			result.CPUSet = strings.TrimSpace(readCgroupV2(files, "cpuset.cpus"))
		}
		result.MemoryLimitBytes = parseLimit(readCgroupV2(files, "memory.max"))
		result.MemoryHighBytes = parseLimit(readCgroupV2(files, "memory.high"))
		result.MemoryCurrentBytes = parseLimit(readCgroupV2(files, "memory.current"))
		result.MemorySwapLimitBytes = parseLimit(readCgroupV2(files, "memory.swap.max"))
		result.PidsLimit = parseLimit(readCgroupV2(files, "pids.max"))
		if result.CPUQuotaMicros == nil && result.CPUSet == "" && result.MemoryLimitBytes == nil && result.MemoryCurrentBytes == nil && result.PidsLimit == nil {
			result.Availability = AvailabilityUnavailable
			result.Error = "cgroup v2 controllers have no readable limits"
		} else {
			result.Availability = AvailabilityAvailable
		}
		setCPUQuotaCores(&result)
		return result
	}
	if quota, ok := readCgroupV1(files, "cpu", "cpu.cfs_quota_us"); ok || hasCgroupPath(files, "cpu") {
		result.Version = "v1"
		if ok {
			result.CPUQuotaMicros = parseSignedLimit(quota)
		}
		if period, found := readCgroupV1(files, "cpu", "cpu.cfs_period_us"); found {
			result.CPUPeriodMicros = parseSignedLimit(period)
		}
		if cpuset, found := readCgroupV1(files, "cpuset", "cpuset.cpus"); found {
			result.CPUSet = strings.TrimSpace(cpuset)
		}
		if memory, found := readCgroupV1(files, "memory", "memory.limit_in_bytes"); found {
			result.MemoryLimitBytes = parseSignedLimit(memory)
		}
		if memory, found := readCgroupV1(files, "memory", "memory.usage_in_bytes"); found {
			result.MemoryCurrentBytes = parseSignedLimit(memory)
		}
		if swap, found := readCgroupV1(files, "memory", "memory.memsw.limit_in_bytes"); found {
			result.MemorySwapLimitBytes = parseSignedLimit(swap)
		}
		if pids, found := readCgroupV1(files, "pids", "pids.max"); found {
			result.PidsLimit = parseLimit(pids)
		}
		result.Availability = AvailabilityAvailable
		setCPUQuotaCores(&result)
		return result
	}
	return result
}

func collectVirtualizationReport(files ReportFileReader, operatingSystem string) VirtualizationReport {
	result := VirtualizationReport{ReportSection: ReportSection{Availability: AvailabilityUnsupported}}
	if operatingSystem != "linux" {
		return result
	}
	if _, err := files.ReadFile("/.dockerenv"); err == nil {
		result.Container = true
		result.ContainerRuntime = "docker"
	}
	cgroup := strings.ToLower(readString(files, "/proc/1/cgroup"))
	for _, runtimeName := range []string{"docker", "containerd", "podman", "lxc", "kubepods"} {
		if strings.Contains(cgroup, runtimeName) {
			result.Container = true
			if result.ContainerRuntime == "" {
				result.ContainerRuntime = runtimeName
			}
		}
	}
	product := strings.TrimSpace(readString(files, "/sys/class/dmi/id/product_name"))
	if product == "" {
		product = strings.TrimSpace(readString(files, "/sys/devices/virtual/dmi/id/product_name"))
	}
	result.Type = normalizeVirtualizationType(product)
	if result.Type == "" && strings.Contains(strings.ToLower(readString(files, "/proc/cpuinfo")), " hypervisor") {
		result.Type = "virtualized"
	}
	if result.Container {
		result.Type = "container"
	}
	if result.Type == "" && !result.Container {
		result.Type = "bare-metal-or-unknown"
	}
	result.Availability = AvailabilityAvailable
	return result
}

func collectGPUReports(files ReportFileReader, operatingSystem string) []GPUReport {
	if operatingSystem != "linux" {
		return nil
	}
	paths, _ := files.Glob("/sys/class/drm/card[0-9]*")
	sort.Strings(paths)
	var reports []GPUReport
	for _, path := range paths {
		name := filepath.Base(path)
		if strings.Contains(name, "-") {
			continue
		}
		vendor := strings.TrimSpace(readString(files, filepath.Join(path, "device/vendor")))
		device := strings.TrimSpace(readString(files, filepath.Join(path, "device/device")))
		uevent := parseKeyValues(readString(files, filepath.Join(path, "device/uevent")))
		report := GPUReport{Path: path, VendorID: vendor, DeviceID: device, Driver: uevent["DRIVER"], ReportSection: ReportSection{Availability: AvailabilityAvailable}}
		if vendor == "" && device == "" && report.Driver == "" {
			report.Availability = AvailabilityUnavailable
		}
		reports = append(reports, report)
	}
	return reports
}

func collectDiskReports(files ReportFileReader, operatingSystem string, collector diskHealthCollector) []DiskReport {
	if operatingSystem != "linux" {
		return nil
	}
	paths, _ := files.Glob("/sys/block/*")
	sort.Strings(paths)
	var reports []DiskReport
	for _, path := range paths {
		name := filepath.Base(path)
		if !isPhysicalDiskName(name) {
			continue
		}
		sectors := parseUint(readString(files, filepath.Join(path, "size")))
		logical := parseUint(readString(files, filepath.Join(path, "queue/logical_block_size")))
		if logical == 0 {
			logical = 512
		}
		report := DiskReport{
			Name: name, LogicalBytes: int64Ptr(int64(logical)),
			Model:           strings.TrimSpace(readString(files, filepath.Join(path, "device/model"))),
			Vendor:          strings.TrimSpace(readString(files, filepath.Join(path, "device/vendor"))),
			Firmware:        strings.TrimSpace(readString(files, filepath.Join(path, "device/firmware_rev"))),
			ControllerState: strings.TrimSpace(readString(files, filepath.Join(path, "device/state"))),
			ReportSection:   ReportSection{Availability: AvailabilityAvailable},
			Health:          DiskHealthReport{ReportSection: ReportSection{Availability: AvailabilityUnavailable, Error: "passive health data unavailable"}, Protocol: storageProtocol(name)},
			Temperature:     DiskTemperatureReport{ReportSection: ReportSection{Availability: AvailabilityUnavailable}},
		}
		if collector != nil {
			report.Health, report.Temperature = collector.Collect(name, files)
		}
		if sectors > 0 && logical > 0 {
			maxInt64 := uint64(^uint64(0) >> 1)
			if sectors > maxInt64/logical {
				report.Availability = AvailabilityError
				report.Error = "disk size exceeds int64 range"
			} else {
				report.SizeBytes = int64Ptr(int64(sectors * logical))
			}
		}
		if value := parseBool(readString(files, filepath.Join(path, "ro"))); value != nil {
			report.ReadOnly = value
		}
		if value := parseBool(readString(files, filepath.Join(path, "queue/rotational"))); value != nil {
			report.Rotational = value
		}
		reports = append(reports, report)
	}
	return reports
}

func isPhysicalDiskName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for _, prefix := range []string{"loop", "ram", "fd", "sr", "dm-", "md", "zram", "nbd", "ublk"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "sd") ||
		strings.HasPrefix(name, "hd") || strings.HasPrefix(name, "vd") ||
		strings.HasPrefix(name, "xvd") || strings.HasPrefix(name, "mmcblk") ||
		strings.HasPrefix(name, "cciss")
}

func storageProtocol(name string) string {
	switch {
	case strings.HasPrefix(name, "nvme"):
		return "nvme"
	case strings.HasPrefix(name, "sd"), strings.HasPrefix(name, "hd"):
		return "ata_or_scsi"
	case strings.HasPrefix(name, "vd"):
		return "virtio"
	case strings.HasPrefix(name, "xvd"):
		return "xen"
	default:
		return "unknown"
	}
}

func readFirst(files ReportFileReader, paths ...string) ([]byte, bool) {
	for _, path := range paths {
		if content, err := files.ReadFile(path); err == nil {
			return content, true
		}
	}
	return nil, false
}

func readString(files ReportFileReader, path string) string {
	content, _ := files.ReadFile(path)
	return string(content)
}

func readCgroupV1(files ReportFileReader, controller, file string) (string, bool) {
	paths := []string{filepath.Join("/sys/fs/cgroup", controller, file)}
	if matches, err := files.Glob(filepath.Join("/sys/fs/cgroup", controller, "*", file)); err == nil {
		paths = append(paths, matches...)
	}
	for _, path := range paths {
		if content, err := files.ReadFile(path); err == nil {
			return string(content), true
		}
	}
	return "", false
}

func readCgroupV2(files ReportFileReader, name string) string {
	paths := make([]string, 0, 2)
	for _, line := range strings.Split(readString(files, "/proc/self/cgroup"), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && parts[0] == "0" && parts[1] == "" {
			relative := strings.TrimPrefix(filepath.Clean(parts[2]), "/")
			if relative != "." && relative != "" {
				paths = append(paths, filepath.Join("/sys/fs/cgroup", relative, name))
			}
			break
		}
	}
	paths = append(paths, filepath.Join("/sys/fs/cgroup", name))
	for _, path := range paths {
		if content, err := files.ReadFile(path); err == nil {
			return string(content)
		}
	}
	return ""
}

func hasCgroupPath(files ReportFileReader, controller string) bool {
	if _, err := files.ReadFile(filepath.Join("/sys/fs/cgroup", controller, "tasks")); err == nil {
		return true
	}
	matches, _ := files.Glob(filepath.Join("/sys/fs/cgroup", controller, "*"))
	return len(matches) > 0
}

func parseCPUQuota(value string) (*int64, *int64) {
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return nil, nil
	}
	var quota, period *int64
	if fields[0] != "max" {
		quota = parseSignedLimit(fields[0])
	}
	period = parseSignedLimit(fields[1])
	return quota, period
}

func parseLimit(value string) *int64 {
	value = strings.TrimSpace(value)
	if value == "" || value == "max" {
		return nil
	}
	return parseSignedLimit(value)
}

func parseSignedLimit(value string) *int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 0 || parsed >= 1<<60 {
		return nil
	}
	return &parsed
}

func setCPUQuotaCores(result *CgroupReport) {
	if result.CPUQuotaMicros == nil || result.CPUPeriodMicros == nil || *result.CPUPeriodMicros <= 0 {
		return
	}
	cores := float64(*result.CPUQuotaMicros) / float64(*result.CPUPeriodMicros)
	result.CPUQuotaCores = &cores
}

func parseKeyValues(content string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			continue
		}
		if parts = strings.SplitN(line, "=", 2); len(parts) == 2 {
			values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return values
}

func parseMemInfo(content string) map[string]*int64 {
	values := make(map[string]*int64)
	for key, value := range parseKeyValues(content) {
		fields := strings.Fields(value)
		if len(fields) == 0 {
			continue
		}
		parsed, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil || parsed < 0 {
			continue
		}
		if len(fields) > 1 && strings.EqualFold(fields[1], "kb") {
			parsed *= 1024
		}
		values[key] = &parsed
	}
	return values
}

func parseUint(value string) uint64 {
	parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return parsed
}

func parseInt64Fields(value string) []int64 {
	result := make([]int64, 0, 3)
	for _, field := range strings.Fields(value) {
		parsed, err := strconv.ParseInt(field, 10, 64)
		if err == nil && parsed >= 0 {
			result = append(result, parsed)
		}
	}
	return result
}

func parseBool(value string) *bool {
	value = strings.TrimSpace(value)
	if value != "0" && value != "1" {
		return nil
	}
	parsed := value == "1"
	return &parsed
}

func countCPUSet(value string) int {
	total := 0
	for _, part := range strings.Split(strings.TrimSpace(value), ",") {
		if part == "" {
			continue
		}
		bounds := strings.SplitN(part, "-", 2)
		start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
		if err != nil {
			continue
		}
		end := start
		if len(bounds) == 2 {
			end, err = strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil || end < start {
				continue
			}
		}
		total += end - start + 1
	}
	return total
}

func normalizeVirtualizationType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(value, "kvm"):
		return "kvm"
	case strings.Contains(value, "qemu"):
		return "qemu"
	case strings.Contains(value, "vmware"):
		return "vmware"
	case strings.Contains(value, "virtualbox"):
		return "virtualbox"
	case strings.Contains(value, "microsoft") || strings.Contains(value, "hyper-v"):
		return "hyper-v"
	case strings.Contains(value, "xen"):
		return "xen"
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func intPtr(value int) *int { return &value }

func int64Ptr(value int64) *int64 { return &value }
