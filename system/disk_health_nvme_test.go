package system

import (
	"encoding/binary"
	"testing"
)

func TestParseNVMeSMARTLog(t *testing.T) {
	data := make([]byte, nvmeSMARTLogSize)
	binary.LittleEndian.PutUint16(data[1:3], 313)
	data[3], data[4], data[5] = 98, 10, 7
	binary.LittleEndian.PutUint64(data[32:40], 100)
	binary.LittleEndian.PutUint64(data[40:48], 1)
	binary.LittleEndian.PutUint64(data[48:56], 200)
	binary.LittleEndian.PutUint64(data[112:120], 3)
	binary.LittleEndian.PutUint64(data[128:136], 400)
	binary.LittleEndian.PutUint64(data[144:152], 2)
	binary.LittleEndian.PutUint64(data[160:168], 1)
	health, temperature := parseNVMeSMARTLog(data)
	if health.Availability != AvailabilityAvailable || health.Status != "passed" || temperature.Celsius == nil || *temperature.Celsius < 39.8 || *temperature.Celsius > 39.9 {
		t.Fatalf("unexpected SMART result: health=%#v temperature=%#v", health, temperature)
	}
	if health.AvailableSparePct == nil || *health.AvailableSparePct != 98 || health.PercentageUsed == nil || *health.PercentageUsed != 7 || health.MediaErrors == nil || *health.MediaErrors != 1 {
		t.Fatalf("unexpected counters: %#v", health)
	}
	if health.DataUnitsRead == nil || *health.DataUnitsRead != ^uint64(0) || health.DataUnitsReadDecimal != "18446744073709551716" || !health.CountersSaturated {
		t.Fatalf("128-bit NVMe counter was truncated: %#v", health)
	}
}

func TestParseNVMeSMARTLogRejectsShortData(t *testing.T) {
	health, temperature := parseNVMeSMARTLog(make([]byte, 64))
	if health.Availability != AvailabilityError || health.Error == "" || temperature.Availability != AvailabilityError {
		t.Fatalf("short data was not rejected: health=%#v temperature=%#v", health, temperature)
	}
}

func TestStorageProtocol(t *testing.T) {
	for name, want := range map[string]string{"nvme0n1": "nvme", "sda": "ata_or_scsi", "vda": "virtio", "xvda": "xen"} {
		if got := storageProtocol(name); got != want {
			t.Errorf("%s: got %q want %q", name, got, want)
		}
	}
}
