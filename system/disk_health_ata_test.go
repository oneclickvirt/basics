package system

import (
	"encoding/binary"
	"testing"
)

func TestParseATASMARTData(t *testing.T) {
	attributes := newATASMARTFixturePage()
	putATASMARTAttribute(attributes, 0, 5, 1, 5, 5, 7)
	putATASMARTAttribute(attributes, 1, 9, 0, 99, 99, 12345)
	putATASMARTAttribute(attributes, 2, 12, 0, 99, 99, 21)
	putATASMARTAttribute(attributes, 3, 192, 0, 99, 99, 2)
	putATASMARTAttribute(attributes, 4, 194, 0, 70, 60, 33)
	putATASMARTAttribute(attributes, 5, 197, 0, 100, 100, 3)
	putATASMARTAttribute(attributes, 6, 198, 0, 100, 100, 1)
	finalizeATASMARTFixturePage(attributes)
	thresholds := newATASMARTFixturePage()
	thresholds[2], thresholds[3] = 5, 10
	finalizeATASMARTFixturePage(thresholds)

	health, temperature := parseATASMARTData(attributes, thresholds)
	if health.Availability != AvailabilityAvailable || health.Status != "failed" || health.Protocol != "ata" {
		t.Fatalf("unexpected ATA SMART health: %+v", health)
	}
	if health.ReallocatedSectors == nil || *health.ReallocatedSectors != 7 || health.PendingSectors == nil || *health.PendingSectors != 3 || health.OfflineUncorrectable == nil || *health.OfflineUncorrectable != 1 {
		t.Fatalf("unexpected ATA counters: %+v", health)
	}
	if health.PowerOnHours == nil || *health.PowerOnHours != 12345 || health.PowerCycles == nil || *health.PowerCycles != 21 || health.UnsafeShutdowns == nil || *health.UnsafeShutdowns != 2 {
		t.Fatalf("unexpected ATA lifetime counters: %+v", health)
	}
	if temperature.Availability != AvailabilityAvailable || temperature.Celsius == nil || *temperature.Celsius != 33 {
		t.Fatalf("unexpected ATA temperature: %+v", temperature)
	}
}

func TestParseATASMARTDataWithoutThresholdsDoesNotClaimPassed(t *testing.T) {
	attributes := newATASMARTFixturePage()
	putATASMARTAttribute(attributes, 0, 5, 1, 100, 100, 0)
	finalizeATASMARTFixturePage(attributes)
	health, _ := parseATASMARTData(attributes, nil)
	if health.Availability != AvailabilityAvailable || health.Status != "attributes_available" {
		t.Fatalf("health without threshold data = %+v", health)
	}
}

func TestParseATASMARTDataRejectsCorruptPage(t *testing.T) {
	attributes := newATASMARTFixturePage()
	putATASMARTAttribute(attributes, 0, 5, 1, 100, 100, 0)
	finalizeATASMARTFixturePage(attributes)
	attributes[20]++
	health, temperature := parseATASMARTData(attributes, nil)
	if health.Availability != AvailabilityError || health.Error == "" || temperature.Availability != AvailabilityUnavailable {
		t.Fatalf("corrupt ATA SMART page accepted: health=%+v temperature=%+v", health, temperature)
	}
	health, _ = parseATASMARTData(make([]byte, 64), nil)
	if health.Availability != AvailabilityError || health.Error == "" {
		t.Fatalf("short ATA SMART page accepted: %+v", health)
	}
}

func newATASMARTFixturePage() []byte {
	page := make([]byte, ataSMARTPageSize)
	binary.LittleEndian.PutUint16(page[:2], 1)
	return page
}

func putATASMARTAttribute(page []byte, index int, id byte, flags uint16, current, worst byte, raw uint64) {
	offset := 2 + index*12
	page[offset] = id
	binary.LittleEndian.PutUint16(page[offset+1:offset+3], flags)
	page[offset+3], page[offset+4] = current, worst
	for byteIndex := 0; byteIndex < 6; byteIndex++ {
		page[offset+5+byteIndex] = byte(raw >> (8 * byteIndex))
	}
}

func finalizeATASMARTFixturePage(page []byte) {
	page[ataSMARTPageSize-1] = 0
	var sum byte
	for _, value := range page[:ataSMARTPageSize-1] {
		sum += value
	}
	page[ataSMARTPageSize-1] = -sum
}
