//go:build !linux

package system

type unsupportedDiskHealthCollector struct{}

func defaultDiskHealthCollector() diskHealthCollector { return unsupportedDiskHealthCollector{} }

func (unsupportedDiskHealthCollector) Collect(name string, _ ReportFileReader) (DiskHealthReport, DiskTemperatureReport) {
	return DiskHealthReport{
		ReportSection: ReportSection{Availability: AvailabilityUnsupported, Error: "passive disk health is unsupported on this platform"},
		Protocol:      storageProtocol(name),
	}, DiskTemperatureReport{ReportSection: ReportSection{Availability: AvailabilityUnsupported}}
}
