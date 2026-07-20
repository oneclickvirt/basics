package system

import (
	"encoding/binary"
	"fmt"
	"math/big"
)

const nvmeSMARTLogSize = 512

func parseNVMeSMARTLog(data []byte) (DiskHealthReport, DiskTemperatureReport) {
	if len(data) < nvmeSMARTLogSize {
		err := fmt.Sprintf("NVMe SMART log is %d bytes; require %d", len(data), nvmeSMARTLogSize)
		return DiskHealthReport{ReportSection: ReportSection{Availability: AvailabilityError, Error: err}, Protocol: "nvme", Source: "nvme_smart_log"}, DiskTemperatureReport{ReportSection: ReportSection{Availability: AvailabilityError, Error: err}, Source: "nvme_smart_log"}
	}
	critical := data[0]
	availableSpare, spareThreshold, percentageUsed := data[3], data[4], data[5]
	read, readLegacy, readSaturated := parseNVMeCounter(data[32:48])
	written, writtenLegacy, writtenSaturated := parseNVMeCounter(data[48:64])
	cycles, cyclesLegacy, cyclesSaturated := parseNVMeCounter(data[112:128])
	hours, hoursLegacy, hoursSaturated := parseNVMeCounter(data[128:144])
	unsafe, unsafeLegacy, unsafeSaturated := parseNVMeCounter(data[144:160])
	media, mediaLegacy, mediaSaturated := parseNVMeCounter(data[160:176])
	health := DiskHealthReport{
		ReportSection: ReportSection{Availability: AvailabilityAvailable}, Protocol: "nvme", Source: "nvme_smart_log",
		CriticalWarning: &critical, AvailableSparePct: &availableSpare, SpareThresholdPct: &spareThreshold,
		PercentageUsed: &percentageUsed, DataUnitsRead: &readLegacy,
		DataUnitsWritten: &writtenLegacy, PowerCycles: &cyclesLegacy,
		PowerOnHours: &hoursLegacy, UnsafeShutdowns: &unsafeLegacy, MediaErrors: &mediaLegacy,
		DataUnitsReadDecimal: read, DataUnitsWrittenDecimal: written,
		PowerCyclesDecimal: cycles, PowerOnHoursDecimal: hours,
		UnsafeShutdownsDecimal: unsafe, MediaErrorsDecimal: media,
		CountersSaturated: readSaturated || writtenSaturated || cyclesSaturated || hoursSaturated || unsafeSaturated || mediaSaturated,
	}
	if critical == 0 {
		health.Status = "passed"
	} else {
		health.Status = "warning"
	}
	temperature := DiskTemperatureReport{ReportSection: ReportSection{Availability: AvailabilityUnavailable}, Source: "nvme_smart_log"}
	kelvin := binary.LittleEndian.Uint16(data[1:3])
	if kelvin != 0 && kelvin != 0xffff {
		celsius := float64(kelvin) - 273.15
		if celsius >= -50 && celsius <= 250 {
			temperature.Availability = AvailabilityAvailable
			temperature.Celsius = &celsius
		}
	}
	return health, temperature
}

func uint64Ptr(value uint64) *uint64 { return &value }

// parseNVMeCounter parses the 128-bit little-endian counters from the NVMe
// SMART log. Legacy uint64 fields are saturated when the high half is set;
// callers that need the full value should use the decimal field.
func parseNVMeCounter(data []byte) (decimal string, legacy uint64, saturated bool) {
	if len(data) < 16 {
		return "0", 0, false
	}
	legacy = binary.LittleEndian.Uint64(data[:8])
	if binary.LittleEndian.Uint64(data[8:16]) != 0 {
		legacy = ^uint64(0)
		saturated = true
	}
	bigEndian := make([]byte, 16)
	for index := range data[:16] {
		bigEndian[15-index] = data[index]
	}
	return new(big.Int).SetBytes(bigEndian).String(), legacy, saturated
}
