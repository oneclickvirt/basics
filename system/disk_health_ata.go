package system

import (
	"encoding/binary"
	"fmt"
)

const ataSMARTPageSize = 512

type ataSMARTAttribute struct {
	id      byte
	flags   uint16
	current byte
	raw     uint64
}

func parseATASMARTData(data, thresholds []byte) (DiskHealthReport, DiskTemperatureReport) {
	health := DiskHealthReport{
		ReportSection: ReportSection{Availability: AvailabilityError},
		Protocol:      "ata",
		Source:        "ata_smart_attributes",
	}
	temperature := DiskTemperatureReport{
		ReportSection: ReportSection{Availability: AvailabilityUnavailable},
		Source:        "ata_smart_attributes",
	}
	if err := validateATASMARTPage(data, "attribute"); err != nil {
		health.Error = err.Error()
		return health, temperature
	}
	attributes := parseATASMARTAttributes(data)
	if len(attributes) == 0 {
		health.Availability = AvailabilityUnavailable
		health.Error = "ATA SMART attribute page contains no attributes"
		return health, temperature
	}
	health.Availability = AvailabilityAvailable
	health.Status = "attributes_available"
	if attribute, ok := attributes[5]; ok {
		health.ReallocatedSectors = uint64Ptr(attribute.raw)
	}
	if attribute, ok := attributes[197]; ok {
		health.PendingSectors = uint64Ptr(attribute.raw)
	}
	if attribute, ok := attributes[198]; ok {
		health.OfflineUncorrectable = uint64Ptr(attribute.raw)
	}
	if attribute, ok := attributes[9]; ok {
		health.PowerOnHours = uint64Ptr(attribute.raw)
	}
	if attribute, ok := attributes[12]; ok {
		health.PowerCycles = uint64Ptr(attribute.raw)
	}
	if attribute, ok := attributes[192]; ok {
		health.UnsafeShutdowns = uint64Ptr(attribute.raw)
	}
	if parsedThresholds, ok := parseATASMARTThresholds(thresholds); ok {
		health.Status = "passed"
		for id, threshold := range parsedThresholds {
			attribute, found := attributes[id]
			if found && attribute.flags&1 != 0 && threshold > 0 && attribute.current <= threshold {
				health.Status = "failed"
				break
			}
		}
	}
	for _, id := range []byte{194, 190} {
		attribute, ok := attributes[id]
		if !ok {
			continue
		}
		celsius := float64(byte(attribute.raw))
		if celsius > 0 && celsius <= 150 {
			temperature.Availability = AvailabilityAvailable
			temperature.Celsius = &celsius
			break
		}
	}
	return health, temperature
}

func validateATASMARTPage(data []byte, pageType string) error {
	if len(data) < ataSMARTPageSize {
		return fmt.Errorf("ATA SMART %s page is %d bytes; require %d", pageType, len(data), ataSMARTPageSize)
	}
	revision := binary.LittleEndian.Uint16(data[:2])
	if revision == 0 || revision == 0xffff {
		return fmt.Errorf("ATA SMART %s page has invalid revision", pageType)
	}
	var checksum byte
	for _, value := range data[:ataSMARTPageSize] {
		checksum += value
	}
	if checksum != 0 {
		return fmt.Errorf("ATA SMART %s page checksum mismatch", pageType)
	}
	return nil
}

func parseATASMARTAttributes(data []byte) map[byte]ataSMARTAttribute {
	attributes := make(map[byte]ataSMARTAttribute)
	for offset := 2; offset+12 <= 362; offset += 12 {
		entry := data[offset : offset+12]
		id := entry[0]
		if id == 0 || id == 0xff {
			continue
		}
		var raw uint64
		for index := 0; index < 6; index++ {
			raw |= uint64(entry[5+index]) << (8 * index)
		}
		attributes[id] = ataSMARTAttribute{
			id: id, flags: binary.LittleEndian.Uint16(entry[1:3]), current: entry[3], raw: raw,
		}
	}
	return attributes
}

func parseATASMARTThresholds(data []byte) (map[byte]byte, bool) {
	if validateATASMARTPage(data, "threshold") != nil {
		return nil, false
	}
	thresholds := make(map[byte]byte)
	for offset := 2; offset+12 <= 362; offset += 12 {
		id := data[offset]
		if id != 0 && id != 0xff {
			thresholds[id] = data[offset+1]
		}
	}
	return thresholds, len(thresholds) > 0
}
