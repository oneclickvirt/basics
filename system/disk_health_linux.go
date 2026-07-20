//go:build linux

package system

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

var diskDeviceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

var errATAPassthroughUnsupported = errors.New("ATA SMART passthrough unsupported")

type linuxDiskHealthCollector struct{}

type nvmeAdminCommand struct {
	Opcode      uint8
	Flags       uint8
	Reserved    uint16
	NamespaceID uint32
	Command2    uint32
	Command3    uint32
	Metadata    uint64
	Address     uint64
	MetadataLen uint32
	DataLen     uint32
	Command10   uint32
	Command11   uint32
	Command12   uint32
	Command13   uint32
	Command14   uint32
	Command15   uint32
	TimeoutMS   uint32
	Result      uint32
}

type sgIOHeader struct {
	InterfaceID       int32
	TransferDirection int32
	CommandLength     uint8
	MaxSenseLength    uint8
	IOVecCount        uint16
	TransferLength    uint32
	TransferPointer   uintptr
	CommandPointer    uintptr
	SensePointer      uintptr
	TimeoutMS         uint32
	Flags             uint32
	PackID            int32
	UserPointer       uintptr
	Status            uint8
	MaskedStatus      uint8
	MessageStatus     uint8
	SenseLength       uint8
	HostStatus        uint16
	DriverStatus      uint16
	Residual          int32
	Duration          uint32
	Info              uint32
}

var nvmeAdminIOCTL = uintptr((3 << 30) | (uint32(unsafe.Sizeof(nvmeAdminCommand{})) << 16) | (uint32('N') << 8) | 0x41)

const (
	sgIO                 = uintptr(0x2285)
	sgTransferFromDevice = int32(-3)
)

func defaultDiskHealthCollector() diskHealthCollector { return linuxDiskHealthCollector{} }

func (linuxDiskHealthCollector) Collect(name string, files ReportFileReader) (DiskHealthReport, DiskTemperatureReport) {
	protocol := detectStorageProtocol(name, files)
	hwmonTemperature := collectDiskHWMonTemperature(name, files)
	if protocol != "nvme" {
		if protocol != "ata" && protocol != "ata_or_scsi" && protocol != "scsi" {
			return DiskHealthReport{
				ReportSection: ReportSection{Availability: AvailabilityUnsupported, Error: "passive SMART health is unsupported for this storage protocol"},
				Protocol:      protocol,
			}, hwmonTemperature
		}
		return collectATASMART(name, hwmonTemperature)
	}
	controller := nvmeControllerPattern.FindString(name)
	if controller == "" {
		return DiskHealthReport{ReportSection: ReportSection{Availability: AvailabilityError, Error: "invalid NVMe device name"}, Protocol: protocol, Source: "nvme_smart_log"}, hwmonTemperature
	}
	file, err := os.Open("/dev/" + controller)
	if err != nil {
		availability := storageHealthAvailability(err)
		return DiskHealthReport{ReportSection: ReportSection{Availability: availability, Error: classifyStorageHealthError(err)}, Protocol: protocol, Source: "nvme_smart_log"}, hwmonTemperature
	}
	defer file.Close()
	buffer := make([]byte, nvmeSMARTLogSize)
	command := nvmeAdminCommand{
		Opcode: 0x02, NamespaceID: ^uint32(0), Address: uint64(uintptr(unsafe.Pointer(&buffer[0]))),
		DataLen: uint32(len(buffer)), Command10: 0x02 | (((uint32(len(buffer)) / 4) - 1) << 16), TimeoutMS: 2000,
	}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, file.Fd(), nvmeAdminIOCTL, uintptr(unsafe.Pointer(&command)))
	runtime.KeepAlive(buffer)
	if errno != 0 {
		availability := storageHealthAvailability(errno)
		return DiskHealthReport{ReportSection: ReportSection{Availability: availability, Error: classifyStorageHealthError(errno)}, Protocol: protocol, Source: "nvme_smart_log"}, hwmonTemperature
	}
	health, temperature := parseNVMeSMARTLog(buffer)
	if temperature.Availability != AvailabilityAvailable && hwmonTemperature.Availability == AvailabilityAvailable {
		temperature = hwmonTemperature
	}
	return health, temperature
}

func collectATASMART(name string, fallbackTemperature DiskTemperatureReport) (DiskHealthReport, DiskTemperatureReport) {
	if !diskDeviceNamePattern.MatchString(name) {
		return DiskHealthReport{ReportSection: ReportSection{Availability: AvailabilityError, Error: "invalid ATA device name"}, Protocol: "ata", Source: "ata_smart_attributes"}, fallbackTemperature
	}
	file, err := os.Open("/dev/" + name)
	if err != nil {
		availability := storageHealthAvailability(err)
		return DiskHealthReport{ReportSection: ReportSection{Availability: availability, Error: classifyStorageHealthError(err)}, Protocol: "ata", Source: "ata_smart_attributes"}, fallbackTemperature
	}
	defer file.Close()
	attributes, err := readATASMARTPage(file.Fd(), 0xd0)
	if err != nil {
		availability := storageHealthAvailability(err)
		return DiskHealthReport{ReportSection: ReportSection{Availability: availability, Error: classifyStorageHealthError(err)}, Protocol: "ata", Source: "ata_smart_attributes"}, fallbackTemperature
	}
	thresholds, _ := readATASMARTPage(file.Fd(), 0xd1)
	health, temperature := parseATASMARTData(attributes, thresholds)
	if temperature.Availability != AvailabilityAvailable && fallbackTemperature.Availability == AvailabilityAvailable {
		temperature = fallbackTemperature
	}
	return health, temperature
}

func readATASMARTPage(fd uintptr, feature byte) ([]byte, error) {
	data := make([]byte, ataSMARTPageSize)
	command := make([]byte, 16)
	sense := make([]byte, 32)
	command[0] = 0x85   // ATA PASS-THROUGH (16)
	command[1] = 4 << 1 // PIO data-in
	command[2] = 0x0e   // data-in, block transfer, sector-count length
	command[4] = feature
	command[6] = 1
	command[10] = 0x4f
	command[12] = 0xc2
	command[14] = 0xb0 // SMART
	header := sgIOHeader{
		InterfaceID: int32('S'), TransferDirection: sgTransferFromDevice,
		CommandLength: uint8(len(command)), MaxSenseLength: uint8(len(sense)),
		TransferLength: uint32(len(data)), TransferPointer: uintptr(unsafe.Pointer(&data[0])),
		CommandPointer: uintptr(unsafe.Pointer(&command[0])), SensePointer: uintptr(unsafe.Pointer(&sense[0])),
		TimeoutMS: 2000,
	}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, sgIO, uintptr(unsafe.Pointer(&header)))
	runtime.KeepAlive(data)
	runtime.KeepAlive(command)
	runtime.KeepAlive(sense)
	if errno != 0 {
		return nil, errno
	}
	if header.HostStatus != 0 || header.Status != 0 || header.DriverStatus != 0 || header.Info&1 != 0 {
		return nil, errATAPassthroughUnsupported
	}
	if header.Residual != 0 {
		return nil, fmt.Errorf("short ATA SMART response: %d bytes missing", header.Residual)
	}
	return data, nil
}

func storageHealthAvailability(err error) Availability {
	if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
		return AvailabilityPermissionDenied
	}
	if errors.Is(err, os.ErrNotExist) {
		return AvailabilityUnavailable
	}
	if errors.Is(err, errATAPassthroughUnsupported) || errors.Is(err, syscall.ENOTTY) || errors.Is(err, syscall.EOPNOTSUPP) {
		return AvailabilityUnsupported
	}
	return AvailabilityError
}

func classifyStorageHealthError(err error) string {
	switch {
	case errors.Is(err, syscall.EACCES), errors.Is(err, syscall.EPERM):
		return "permission denied"
	case errors.Is(err, os.ErrNotExist):
		return "controller device unavailable"
	case errors.Is(err, errATAPassthroughUnsupported), errors.Is(err, syscall.ENOTTY), errors.Is(err, syscall.EOPNOTSUPP):
		return "passive SMART passthrough unsupported"
	default:
		return "passive health read failed"
	}
}
