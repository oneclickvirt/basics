package system

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type reportFixture struct {
	files map[string]string
	globs map[string][]string
}

type cancelingReportFixture struct {
	reportFixture
	cancel context.CancelFunc
	reads  map[string]int
}

func (f *cancelingReportFixture) ReadFile(path string) ([]byte, error) {
	f.reads[path]++
	if path == "/proc/cpuinfo" {
		f.cancel()
	}
	return f.reportFixture.ReadFile(path)
}

func (f *cancelingReportFixture) Glob(pattern string) ([]string, error) {
	return f.reportFixture.Glob(pattern)
}

func (f reportFixture) ReadFile(path string) ([]byte, error) {
	value, ok := f.files[path]
	if !ok {
		return nil, errors.New("fixture file not found: " + path)
	}
	return []byte(value), nil
}

func (f reportFixture) Glob(pattern string) ([]string, error) {
	if values, ok := f.globs[pattern]; ok {
		return values, nil
	}
	var matches []string
	for path := range f.files {
		if matched, _ := filepath.Match(pattern, path); matched {
			matches = append(matches, path)
		}
	}
	return matches, nil
}

func TestCollectSystemReportCgroupV2Fixture(t *testing.T) {
	fixture := reportFixture{
		files: map[string]string{
			"/proc/cpuinfo":                                       "processor : 0\nmodel name : Fixture CPU\ncpu MHz : 2400.000\nflags : aes vmx sse\nphysical id : 0\ncore id : 0\n\nprocessor : 1\nmodel name : Fixture CPU\ncpu MHz : 2400.000\nflags : aes vmx sse\nphysical id : 0\ncore id : 1\n",
			"/proc/meminfo":                                       "MemTotal:       1048576 kB\nMemAvailable:    524288 kB\nSwapTotal:       262144 kB\nSwapFree:        131072 kB\n",
			"/sys/devices/system/cpu/online":                      "0-1\n",
			"/sys/fs/cgroup/cgroup.controllers":                   "cpu cpuset memory pids\n",
			"/sys/fs/cgroup/cpu.max":                              "200000 100000\n",
			"/sys/fs/cgroup/cpuset.cpus.effective":                "0-1\n",
			"/sys/fs/cgroup/memory.max":                           "1073741824\n",
			"/sys/fs/cgroup/memory.high":                          "max\n",
			"/sys/fs/cgroup/memory.current":                       "1048576\n",
			"/sys/fs/cgroup/memory.swap.max":                      "536870912\n",
			"/sys/fs/cgroup/pids.max":                             "128\n",
			"/.dockerenv":                                         "",
			"/proc/1/cgroup":                                      "0::/docker/fixture\n",
			"/sys/class/dmi/id/product_name":                      "KVM\n",
			"/sys/class/drm/card0/device/vendor":                  "0x1234\n",
			"/sys/class/drm/card0/device/device":                  "0xabcd\n",
			"/sys/class/drm/card0/device/uevent":                  "DRIVER=virtio-pci\n",
			"/sys/bus/pci/devices/0000:00:02.0/vendor":            "0x1234\n",
			"/sys/bus/pci/devices/0000:00:02.0/device":            "0xabcd\n",
			"/sys/bus/pci/devices/0000:00:02.0/class":             "0x030000\n",
			"/sys/bus/pci/devices/0000:00:02.0/uevent":            "DRIVER=virtio-pci\nSERIAL=must-not-leak\n",
			"/sys/block/vda/size":                                 "2048\n",
			"/sys/block/vda/queue/logical_block_size":             "512\n",
			"/sys/block/vda/device/model":                         "Fixture Disk\n",
			"/sys/block/vda/device/vendor":                        "Fixture Vendor\n",
			"/sys/block/vda/ro":                                   "0\n",
			"/sys/block/vda/queue/rotational":                     "0\n",
			"/proc/sys/net/ipv4/tcp_congestion_control":           "bbr\n",
			"/proc/sys/net/ipv4/tcp_available_congestion_control": "reno cubic bbr\n",
			"/proc/sys/net/core/default_qdisc":                    "fq\n",
			"/proc/sys/net/ipv4/tcp_rmem":                         "4096 131072 6291456\n",
			"/proc/sys/net/ipv4/tcp_wmem":                         "4096 16384 4194304\n",
			"/sys/class/dmi/id/board_vendor":                      "Fixture Corp\n",
			"/sys/class/dmi/id/board_name":                        "Fixture Board\n",
			"/sys/class/dmi/id/bios_vendor":                       "Fixture BIOS\n",
			"/sys/class/dmi/id/bios_version":                      "1.0\n",
			"/sys/devices/system/node/node0/cpulist":              "0-1\n",
			"/sys/devices/system/node/node0/meminfo":              "Node 0 MemTotal: 1048576 kB\n",
			"/proc/mdstat":                                        "Personalities : [raid1]\nmd0 : active raid1 vda1[0] vdb1[1]\n      1024 blocks [2/1] [U_]\n",
			"/sys/block/md0/md/sync_action":                       "idle\n",
		},
		globs: map[string][]string{
			"/sys/class/drm/card[0-9]*":           {"/sys/class/drm/card0"},
			"/sys/bus/pci/devices/*":              {"/sys/bus/pci/devices/0000:00:02.0"},
			"/sys/block/*":                        {"/sys/block/vda"},
			"/sys/devices/system/node/node[0-9]*": {"/sys/devices/system/node/node0"},
		},
	}
	report := CollectSystemReportFrom(context.Background(), fixture, "linux")
	if report.Availability != AvailabilityAvailable {
		t.Fatalf("report availability = %q", report.Availability)
	}
	if report.Cgroup.Version != "v2" || report.Cgroup.CPUQuotaCores == nil || *report.Cgroup.CPUQuotaCores != 2 {
		t.Fatalf("unexpected cgroup report: %+v", report.Cgroup)
	}
	if report.Cgroup.MemoryLimitBytes == nil || *report.Cgroup.MemoryLimitBytes != 1073741824 {
		t.Fatalf("unexpected memory limit: %+v", report.Cgroup.MemoryLimitBytes)
	}
	if report.CPU.FrequencyMHz == nil || *report.CPU.FrequencyMHz != 2400 || report.CPU.AESNI == nil || !*report.CPU.AESNI || report.CPU.VirtualizationSupported == nil || !*report.CPU.VirtualizationSupported {
		t.Fatalf("unexpected CPU capabilities: %+v", report.CPU)
	}
	if !report.Virtualization.Container || report.Virtualization.ContainerRuntime != "docker" || report.Virtualization.Type != "container" {
		t.Fatalf("unexpected virtualization report: %+v", report.Virtualization)
	}
	if len(report.GPUs) != 1 || report.GPUs[0].Driver != "virtio-pci" {
		t.Fatalf("unexpected gpu report: %+v", report.GPUs)
	}
	if report.PCI.Availability != AvailabilityAvailable || len(report.PCI.Devices) != 1 || report.PCI.Devices[0].ClassID != "0x030000" {
		t.Fatalf("unexpected PCI report: %+v", report.PCI)
	}
	if len(report.Disks) != 1 || report.Disks[0].SizeBytes == nil || *report.Disks[0].SizeBytes != 1048576 {
		t.Fatalf("unexpected disk report: %+v", report.Disks)
	}
	if report.Memory.TotalBytes == nil || *report.Memory.TotalBytes != 1073741824 {
		t.Fatalf("unexpected memory report: %+v", report.Memory)
	}
	if report.Network.Availability != AvailabilityAvailable || report.Network.CongestionControl != "bbr" || report.Network.DefaultQdisc != "fq" || len(report.Network.TCPRMem) != 3 {
		t.Fatalf("unexpected network tuning report: %+v", report.Network)
	}
	if report.Firmware.Availability != AvailabilityAvailable || report.Firmware.BoardName != "Fixture Board" {
		t.Fatalf("unexpected firmware report: %+v", report.Firmware)
	}
	if len(report.MemoryTopology.Nodes) != 1 || report.MemoryTopology.Nodes[0].MemBytes == nil {
		t.Fatalf("unexpected memory topology: %+v", report.MemoryTopology)
	}
	if len(report.RAID.Arrays) != 1 || !report.RAID.Arrays[0].Degraded || report.RAID.Arrays[0].Level != "raid1" {
		t.Fatalf("unexpected RAID report: %+v", report.RAID)
	}
}

func TestCollectPCIReportStableAndRedacted(t *testing.T) {
	fixture := reportFixture{
		files: map[string]string{
			"/sys/bus/pci/devices/0000:02:00.0/vendor": "0x10de\n",
			"/sys/bus/pci/devices/0000:02:00.0/device": "0x1db6\n",
			"/sys/bus/pci/devices/0000:02:00.0/class":  "0x030200\n",
			"/sys/bus/pci/devices/0000:02:00.0/uevent": "DRIVER=nvidia\nSERIAL=private-device-id\n",
			"/sys/bus/pci/devices/0000:02:00.0/serial": "private-device-id\n",
			"/sys/bus/pci/devices/0000:00:1f.6/uevent": "PCI_ID=8086:15be\nPCI_CLASS=020000\nDRIVER=e1000e\n",
		},
		globs: map[string][]string{
			"/sys/bus/pci/devices/*": {"/sys/bus/pci/devices/0000:02:00.0", "/sys/bus/pci/devices/0000:00:1f.6"},
		},
	}
	report := collectPCIReport(fixture, "linux")
	if report.Availability != AvailabilityAvailable || len(report.Devices) != 2 {
		t.Fatalf("unexpected PCI report: %+v", report)
	}
	if report.Devices[0].Address != "0000:00:1f.6" || report.Devices[0].VendorID != "0x8086" || report.Devices[0].ClassID != "0x020000" || report.Devices[1].Driver != "nvidia" {
		t.Fatalf("PCI records are not stable or complete: %+v", report.Devices)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-device-id") || strings.Contains(string(encoded), "serial") {
		t.Fatalf("PCI report exposed a device serial: %s", encoded)
	}
	unsupported := collectPCIReport(fixture, "windows")
	if unsupported.Availability != AvailabilityUnsupported || len(unsupported.Devices) != 0 {
		t.Fatalf("non-Linux PCI result = %+v", unsupported)
	}
}

func TestCollectCPUReportRecognizesARMFeatures(t *testing.T) {
	report := collectCPUReport(reportFixture{files: map[string]string{
		"/proc/cpuinfo":                  "processor : 0\nProcessor : ARMv8 Fixture\nFeatures : fp asimd aes\n",
		"/sys/devices/system/cpu/online": "0\n",
	}}, "linux")
	if report.AESNI == nil || !*report.AESNI {
		t.Fatalf("ARM AES feature was not recognized: %+v", report)
	}
	if report.VirtualizationSupported != nil {
		t.Fatalf("ARM virtualization support should remain unknown without an explicit flag: %+v", report)
	}
}

func TestRenderSystemReportTextIsCompactAndRedacted(t *testing.T) {
	report := &SystemReport{
		Cgroup:         CgroupReport{ReportSection: ReportSection{Availability: AvailabilityAvailable}, Version: "v2", CPUQuotaCores: float64Ptr(2), CPUSet: "0-1", MemoryCurrentBytes: int64Ptr(512 << 20), MemoryLimitBytes: int64Ptr(1 << 30), PidsLimit: int64Ptr(128)},
		Network:        NetworkTuningReport{ReportSection: ReportSection{Availability: AvailabilityAvailable}, DefaultQdisc: "fq", TCPRMem: []int64{4096, 131072, 6291456}, TCPWMem: []int64{4096, 16384, 4194304}},
		Firmware:       FirmwareReport{ReportSection: ReportSection{Availability: AvailabilityAvailable}, BoardVendor: "Fixture", BoardName: "Board", BIOSVendor: "BIOS", BIOSVersion: "1.0"},
		PCI:            PCIReport{ReportSection: ReportSection{Availability: AvailabilityAvailable}, Devices: []PCIDeviceReport{{Address: "private-address", Driver: "virtio-pci"}}},
		GPUs:           []GPUReport{{Path: "/private/gpu/path", Driver: "virtio-pci"}},
		MemoryTopology: MemoryTopologyReport{ReportSection: ReportSection{Availability: AvailabilityAvailable}, Nodes: []NUMANodeReport{{Node: "node0"}}, DIMMs: []DIMMReport{{PartNumber: "private-part", SerialRedacted: true, SizeBytes: int64Ptr(8 << 30)}}, HugePagesTotal: int64Ptr(16), HugePagesFree: int64Ptr(8), HugePageBytes: int64Ptr(2 << 20)},
		Disks:          []DiskReport{{Name: "private-disk", Health: DiskHealthReport{ReportSection: ReportSection{Availability: AvailabilityAvailable}, Protocol: "nvme", Status: "passed"}, Temperature: DiskTemperatureReport{ReportSection: ReportSection{Availability: AvailabilityAvailable}, Celsius: float64Ptr(42)}}},
		RAID:           RAIDReport{ReportSection: ReportSection{Availability: AvailabilityAvailable}, Arrays: []RAIDArrayReport{{Name: "private-array", Level: "raid1", Members: []string{"private-member"}, Degraded: true}}, Controllers: []RAIDControllerReport{{Address: "private-controller", Driver: "megaraid_sas"}}},
	}
	text := RenderSystemReportText(report, "zh")
	for _, want := range []string{"Cgroup 限制", "TCP 缓冲/队列", "主板/BIOS", "PCI/GPU", "内存拓扑", "物理盘 1", "RAID", "nvme", "42.0 C", "degraded 1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q:\n%s", want, text)
		}
	}
	english := RenderSystemReportText(report, "en")
	for _, want := range []string{"Cgroup Limits", "TCP Buffers/Qdisc", "Board/BIOS", "Memory Topology", "Physical Disk 1"} {
		if !strings.Contains(english, want) {
			t.Fatalf("English summary missing %q:\n%s", want, english)
		}
	}
	for _, forbidden := range []string{"private-address", "/private/gpu/path", "private-part", "private-disk", "private-array", "private-member", "private-controller"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("summary leaked %q:\n%s", forbidden, text)
		}
	}
	legacy := " CPU 型号            : Fixture CPU\n"
	combined := appendSystemReportText(legacy, report, "zh")
	if !strings.HasPrefix(combined, legacy) || !strings.Contains(combined, "Cgroup 限制") {
		t.Fatalf("legacy lines were not preserved before the summary:\n%s", combined)
	}
}

func TestRAIDControllersFromPCIFindsHardwareControllers(t *testing.T) {
	controllers := raidControllersFromPCI(PCIReport{Devices: []PCIDeviceReport{
		{Address: "0000:03:00.0", ClassID: "0x010400", Driver: "megaraid_sas", VendorID: "0x1000", DeviceID: "0x005d"},
		{Address: "0000:00:17.0", ClassID: "0x010601", Driver: "ahci"},
	}})
	if len(controllers) != 1 || controllers[0].Address != "0000:03:00.0" || controllers[0].Driver != "megaraid_sas" {
		t.Fatalf("unexpected RAID controllers: %+v", controllers)
	}
}

func TestCollectDiskHWMonTemperatureFixture(t *testing.T) {
	path := "/sys/class/block/sda/device/hwmon/hwmon2/temp1_input"
	fixture := reportFixture{
		files: map[string]string{path: "42000\n"},
		globs: map[string][]string{
			"/sys/class/block/sda/device/hwmon/hwmon*/temp*_input": {path},
		},
	}
	report := collectDiskHWMonTemperature("sda", fixture)
	if report.Availability != AvailabilityAvailable || report.Celsius == nil || *report.Celsius != 42 || report.Source != "sysfs_hwmon" {
		t.Fatalf("unexpected hwmon temperature: %+v", report)
	}
	if got := detectStorageProtocol("sda", reportFixture{files: map[string]string{"/sys/class/block/sda/device/protocol": "SATA\n"}}); got != "ata" {
		t.Fatalf("detected protocol = %q", got)
	}
}

func TestCollectSystemReportCgroupV1Fixture(t *testing.T) {
	fixture := reportFixture{
		files: map[string]string{
			"/proc/cpuinfo":                               "processor : 0\nmodel name : V1 CPU\nphysical id : 0\ncore id : 0\n",
			"/proc/meminfo":                               "MemTotal: 2048 kB\n",
			"/sys/fs/cgroup/cpu/cpu.cfs_quota_us":         "50000\n",
			"/sys/fs/cgroup/cpu/cpu.cfs_period_us":        "100000\n",
			"/sys/fs/cgroup/cpuset/cpuset.cpus":           "2-3\n",
			"/sys/fs/cgroup/memory/memory.limit_in_bytes": "2097152\n",
			"/sys/fs/cgroup/memory/memory.usage_in_bytes": "1048576\n",
			"/sys/fs/cgroup/pids/pids.max":                "32\n",
		},
	}
	report := CollectSystemReportFrom(context.Background(), fixture, "linux")
	if report.Cgroup.Version != "v1" || report.Cgroup.CPUQuotaCores == nil || *report.Cgroup.CPUQuotaCores != 0.5 {
		t.Fatalf("unexpected v1 cgroup report: %+v", report.Cgroup)
	}
	if report.Cgroup.MemoryLimitBytes == nil || *report.Cgroup.MemoryLimitBytes != 2097152 {
		t.Fatalf("unexpected v1 memory limit: %+v", report.Cgroup.MemoryLimitBytes)
	}
}

func TestCollectSystemReportCgroupV2NestedPath(t *testing.T) {
	fixture := reportFixture{files: map[string]string{
		"/proc/cpuinfo":                             "processor : 0\nmodel name : Nested CPU\n",
		"/proc/meminfo":                             "MemTotal: 1024 kB\n",
		"/sys/fs/cgroup/cgroup.controllers":         "cpu memory\n",
		"/proc/self/cgroup":                         "0::/tenant/workload\n",
		"/sys/fs/cgroup/tenant/workload/cpu.max":    "50000 100000\n",
		"/sys/fs/cgroup/tenant/workload/memory.max": "1048576\n",
		"/proc/mdstat":                              "Personalities :\n",
	}}
	report := CollectSystemReportFrom(context.Background(), fixture, "linux")
	if report.Cgroup.CPUQuotaCores == nil || *report.Cgroup.CPUQuotaCores != 0.5 || report.Cgroup.MemoryLimitBytes == nil || *report.Cgroup.MemoryLimitBytes != 1048576 {
		t.Fatalf("unexpected nested cgroup report: %+v", report.Cgroup)
	}
}

func TestCollectSystemReportCancellationAndUnsupported(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report := CollectSystemReportFrom(ctx, reportFixture{}, "linux")
	if report.Availability != AvailabilityCanceled {
		t.Fatalf("availability = %q", report.Availability)
	}
	report = CollectSystemReportFrom(context.Background(), reportFixture{}, "windows")
	if report.CPU.Availability != AvailabilityUnsupported || report.Cgroup.Availability != AvailabilityUnsupported {
		t.Fatalf("unsupported sections = cpu:%q cgroup:%q", report.CPU.Availability, report.Cgroup.Availability)
	}
}

func TestCollectSystemReportStopsBetweenCollectorsWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fixture := &cancelingReportFixture{
		reportFixture: reportFixture{files: map[string]string{
			"/proc/cpuinfo":                  "processor : 0\nmodel name : Fixture CPU\n",
			"/sys/devices/system/cpu/online": "0\n",
			"/proc/meminfo":                  "MemTotal: 1024 kB\n",
		}},
		cancel: cancel,
		reads:  make(map[string]int),
	}
	report := CollectSystemReportFrom(ctx, fixture, "linux")
	if report.Availability != AvailabilityCanceled || report.Error != context.Canceled.Error() {
		t.Fatalf("canceled report = %+v", report)
	}
	if fixture.reads["/proc/meminfo"] != 0 {
		t.Fatalf("memory collector ran after cancellation: reads=%v", fixture.reads)
	}
}

func TestCollectSystemReportStopsBetweenCollectorsAfterDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	fixture := &cancelingReportFixture{
		reportFixture: reportFixture{files: map[string]string{
			"/proc/cpuinfo":                  "processor : 0\nmodel name : Fixture CPU\n",
			"/sys/devices/system/cpu/online": "0\n",
			"/proc/meminfo":                  "MemTotal: 1024 kB\n",
		}},
		cancel: func() { <-ctx.Done() },
		reads:  make(map[string]int),
	}
	report := CollectSystemReportFrom(ctx, fixture, "linux")
	if report.Availability != AvailabilityCanceled || report.Error != context.DeadlineExceeded.Error() {
		t.Fatalf("timed-out report = %+v", report)
	}
	if fixture.reads["/proc/meminfo"] != 0 {
		t.Fatalf("memory collector ran after timeout: reads=%v", fixture.reads)
	}
}

func TestParseDMIType17(t *testing.T) {
	formatted := make([]byte, 0x22)
	formatted[0] = 17
	formatted[1] = byte(len(formatted))
	binary.LittleEndian.PutUint16(formatted[0x0c:0x0e], 8192)
	formatted[0x10], formatted[0x11], formatted[0x12] = 1, 2, 0x1a
	binary.LittleEndian.PutUint16(formatted[0x15:0x17], 3200)
	formatted[0x17], formatted[0x18], formatted[0x1a] = 3, 4, 5
	binary.LittleEndian.PutUint16(formatted[0x20:0x22], 2933)
	data := append(formatted, []byte("DIMM_A1\x00BANK 0\x00Vendor\x00Serial\x00Part-123\x00\x00")...)
	data = append(data, []byte{127, 4, 0, 0, 0, 0}...)
	dimms := parseDMIType17(data)
	if len(dimms) != 1 || dimms[0].SizeBytes == nil || *dimms[0].SizeBytes != 8<<30 || dimms[0].Type != "DDR4" || dimms[0].Manufacturer != "Vendor" || dimms[0].PartNumber != "Part-123" || !dimms[0].SerialRedacted || dimms[0].ConfiguredSpeedMTs == nil || *dimms[0].ConfiguredSpeedMTs != 2933 {
		t.Fatalf("unexpected DIMM report: %+v", dimms)
	}
}

func TestCollectDiskReportsFiltersLogicalDevices(t *testing.T) {
	fixture := reportFixture{
		files: map[string]string{
			"/sys/block/sda/size":                     "2048\n",
			"/sys/block/sda/queue/logical_block_size": "512\n",
			"/sys/block/dm-0/size":                    "4096\n",
			"/sys/block/md0/size":                     "4096\n",
			"/sys/block/zram0/size":                   "4096\n",
		},
		globs: map[string][]string{
			"/sys/block/*": {"/sys/block/dm-0", "/sys/block/md0", "/sys/block/sda", "/sys/block/zram0"},
		},
	}
	reports := collectDiskReports(fixture, "linux", nil)
	if len(reports) != 1 || reports[0].Name != "sda" || reports[0].SizeBytes == nil || *reports[0].SizeBytes != 1048576 {
		t.Fatalf("logical devices were not filtered: %+v", reports)
	}
}

func TestCollectDiskReportsRejectsSizeOverflow(t *testing.T) {
	fixture := reportFixture{
		files: map[string]string{
			"/sys/block/sda/size":                     "18446744073709551615\n",
			"/sys/block/sda/queue/logical_block_size": "4096\n",
		},
		globs: map[string][]string{"/sys/block/*": {"/sys/block/sda"}},
	}
	reports := collectDiskReports(fixture, "linux", nil)
	if len(reports) != 1 || reports[0].Availability != AvailabilityError || reports[0].Error == "" {
		t.Fatalf("overflow was not reported: %+v", reports)
	}
}
