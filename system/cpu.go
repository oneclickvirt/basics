package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/oneclickvirt/basics/model"
	"github.com/oneclickvirt/basics/system/utils"
	"github.com/shirou/gopsutil/v4/cpu"
)

func checkCPUFeatureLinux(filename string, feature string) (string, bool) {
	if feature == "hypervisor" {
		cmd := exec.Command("lscpu", "-B")
		output, err := cmd.Output()
		if err == nil {
			lscpuLines := strings.Split(string(output), "\n")
			var virtualizationType string
			for _, l := range lscpuLines {
				if strings.Contains(l, "Hypervisor:") {
					for _, l := range lscpuLines {
						if strings.Contains(l, "Virtualization type:") {
							tp := strings.Split(l, ":")
							if len(tp) == 2 {
								virtualizationType = fmt.Sprintf(" (%s)", strings.TrimSpace(tp[1]))
							}
						}
					}
					return "✔️ Enabled" + virtualizationType, true
				}
			}
		}
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		return "Error reading file", false
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.Contains(line, feature) {
			return "✔️ Enabled", true
		}
	}
	return "❌ Disabled", false
}

func checkCPUFeature(filename string, feature string) (string, bool) {
	if runtime.GOOS == "windows" {
		return utils.CheckCPUFeatureWindows(filename, feature)
	} else if runtime.GOOS == "linux" {
		return checkCPUFeatureLinux(filename, feature)
	}
	return "Unsupported OS", false
}

func convertBytes(bytes int64) (string, int64) {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return "GB", bytes / GB
	case bytes >= MB:
		return "MB", bytes / MB
	case bytes >= KB:
		return "KB", bytes / KB
	default:
		return "Bytes", bytes
	}
}

func getCpuInfoFromProcCpuinfo(ret *model.SystemInfo) {
	cpuinfoFile, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return
	}
	defer cpuinfoFile.Close()
	scanner := bufio.NewScanner(cpuinfoFile)
	var modelNameFound bool
	var cpuMHz string
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ":")
		if len(fields) >= 2 {
			key := strings.TrimSpace(fields[0])
			value := strings.TrimSpace(strings.Join(fields[1:], " "))
			switch {
			case strings.Contains(key, "model name"):
				ret.CpuModel = value
				modelNameFound = true
			case strings.Contains(key, "cache size"):
				ret.CpuCache = value
			case strings.Contains(key, "cpu MHz"):
				cpuMHz = value
			}
		}
	}
	if modelNameFound && cpuMHz != "" && !strings.Contains(ret.CpuModel, "@") {
		ret.CpuModel += " @ " + cpuMHz + " MHz"
	}
}

func getCpuInfoFromLscpu(ret *model.SystemInfo) {
	cmd := exec.Command("lscpu", "-B")
	output, err := cmd.Output()
	if err != nil {
		return
	}
	var L1dcache, L1icache, L2cache, L3cache string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
		}
		value := strings.TrimSpace(strings.Join(fields[1:], " "))
		switch {
		case strings.Contains(fields[0], "Model name") && !strings.Contains(fields[0], "BIOS Model name") && ret.CpuModel == "":
			ret.CpuModel = value
		case strings.Contains(fields[0], "CPU MHz") && !strings.Contains(ret.CpuModel, "@"):
			ret.CpuModel += " @ " + value + " MHz"
		case strings.Contains(fields[0], "CPU static MHz") && !strings.Contains(ret.CpuModel, "@"):
			ret.CpuModel += " @ " + value + " static MHz"
		case strings.Contains(fields[0], "CPU dynamic MHz") && !strings.Contains(ret.CpuModel, "@"):
			ret.CpuModel += " @ " + value + " dynamic MHz"
		case strings.Contains(fields[0], "L1d cache") || strings.Contains(fields[0], "L1d"):
			L1dcache = value
		case strings.Contains(fields[0], "L1i cache") || strings.Contains(fields[0], "L1i"):
			L1icache = value
		case strings.Contains(fields[0], "L2 cache") || strings.Contains(fields[0], "L2"):
			L2cache = value
		case strings.Contains(fields[0], "L3 cache") || strings.Contains(fields[0], "L3"):
			L3cache = value
		}
	}
	// 在实在找不到型号的时候，直接输出CPU制造的厂商
	if strings.Contains(ret.CpuModel, "@") && len(ret.CpuModel[:strings.Index(ret.CpuModel, "@")]) < 3 {
		for _, line := range lines {
			fields := strings.Split(line, ":")
			if len(fields) < 2 {
				continue
			}
			if strings.Contains(fields[0], "Vendor ID") {
				newModel := strings.TrimSpace(strings.Join(fields[1:], " "))
				if strings.Contains(ret.CpuModel, "@") && len(ret.CpuModel[:strings.Index(ret.CpuModel, "@")]) < len(newModel) {
					freqPart := ret.CpuModel[strings.Index(ret.CpuModel, "@"):]
					ret.CpuModel = newModel + " " + freqPart
				} else {
					ret.CpuModel = newModel
				}
			}
		}
	}
	updateCpuCache(ret, L1dcache, L1icache, L2cache, L3cache)
}

func updateCpuCache(ret *model.SystemInfo, L1dcache, L1icache, L2cache, L3cache string) {
	if L1dcache == "" || L1icache == "" || L2cache == "" || L3cache == "" || strings.Contains(ret.CpuCache, "/") {
		return
	}
	bytes1, err1 := strconv.ParseInt(L1dcache, 10, 64)
	bytes2, err2 := strconv.ParseInt(L1icache, 10, 64)
	bytes4, err4 := strconv.ParseInt(L2cache, 10, 64)
	bytes5, err5 := strconv.ParseInt(L3cache, 10, 64)
	if err1 != nil || err2 != nil || err4 != nil || err5 != nil {
		return
	}
	L1unit, L1size := convertBytes(bytes1 + bytes2)
	L2unit, L2size := convertBytes(bytes4)
	L3unit, L3size := convertBytes(bytes5)
	ret.CpuCache = fmt.Sprintf("L1: %d %s / L2: %d %s / L3: %d %s",
		L1size, L1unit, L2size, L2unit, L3size, L3unit)
}

func updateSystemLoad(ret *model.SystemInfo) {
	if ret.Load != "" {
		return
	}
	var load string
	out, err := exec.Command("w").Output()
	if err == nil {
		loadFields := strings.Fields(string(out))
		load = strings.Join(loadFields[len(loadFields)-3:], " ")
	} else {
		out, err = exec.Command("uptime").Output()
		if err == nil {
			fields := strings.Fields(string(out))
			load = strings.Join(fields[len(fields)-3:], " ")
		}
	}
	if load != "" {
		ret.Load = load
	}
}

func getCpuInfo(ret *model.SystemInfo, cpuType string) (*model.SystemInfo, error) {
	if runtime.NumCPU() != 0 {
		ret.CpuCores = fmt.Sprintf("%d %s CPU(s)", runtime.NumCPU(), cpuType)
	}
	switch runtime.GOOS {
	case "windows":
		return getWindowsCpuInfo(ret)
	case "linux":
		return getLinuxCpuInfo(ret)
	case "darwin":
		return getDarwinCpuInfo(ret)
	default:
		return getDefaultCpuInfo(ret)
	}
}

func getWindowsCpuInfo(ret *model.SystemInfo) (*model.SystemInfo, error) {
	ci, err := cpu.Info()
	if err != nil {
		return nil, fmt.Errorf("cpu.Info error: %v", err.Error())
	}
	for i := 0; i < len(ci); i++ {
		if len(ret.CpuModel) < len(ci[i].ModelName) {
			ret.CpuModel = strings.TrimSpace(ci[i].ModelName)
		}
	}
	ret.CpuCache = utils.GetCpuCache()
	aesFeature := `HARDWARE\DESCRIPTION\System\CentralProcessor\0`
	virtFeature := `HARDWARE\DESCRIPTION\System\CentralProcessor\0`
	hypervFeature := `SYSTEM\CurrentControlSet\Control\Hypervisor\0`
	ret.CpuAesNi, _ = checkCPUFeature(aesFeature, "aes")
	var st bool
	ret.CpuVAH, st = checkCPUFeature(virtFeature, "vmx")
	if !st {
		ret.CpuVAH, _ = checkCPUFeature(hypervFeature, "hypervisor")
	}
	return ret, nil
}

func getLinuxCpuInfo(ret *model.SystemInfo) (*model.SystemInfo, error) {
	getCpuInfoFromProcCpuinfo(ret)
	getCpuInfoFromLscpu(ret)
	ci, err := cpu.Info()
	if err == nil {
		for i := 0; i < len(ci); i++ {
			newModel := strings.TrimSpace(ci[i].ModelName)
			if strings.Contains(ret.CpuModel, "@") && len(ret.CpuModel[:strings.Index(ret.CpuModel, "@")]) < len(ci[i].ModelName) {
				freqPart := ret.CpuModel[strings.Index(ret.CpuModel, "@"):]
				ret.CpuModel = newModel + " " + freqPart
			} else {
				ret.CpuModel = newModel
			}
		}
	}
	deviceTreeContent, err := os.ReadFile("/proc/device-tree")
	if err == nil && ret.CpuModel == "" {
		ret.CpuModel = string(deviceTreeContent)
	}
	ret.CpuAesNi, _ = checkCPUFeature("/proc/cpuinfo", "aes")
	var st bool
	ret.CpuVAH, st = checkCPUFeature("/proc/cpuinfo", "vmx")
	if !st {
		ret.CpuVAH, _ = checkCPUFeature("/proc/cpuinfo", "hypervisor")
	}
	updateSystemLoad(ret)
	return ret, nil
}

func getDarwinCpuInfo(ret *model.SystemInfo) (*model.SystemInfo, error) {
	if len(model.MacOSInfo) > 0 {
		for _, line := range model.MacOSInfo {
			if strings.Contains(line, "Chip") && ret.CpuModel == "" {
				ret.CpuModel = strings.TrimSpace(strings.Split(line, ":")[1])
			}
			if strings.Contains(line, "Total Number of Cores") && ret.CpuCores == "" {
				ret.CpuCores = strings.TrimSpace(strings.Split(line, ":")[1])
			}
			if strings.Contains(line, "Memory") && ret.MemoryTotal == "" {
				ret.MemoryTotal = strings.TrimSpace(strings.Split(line, ":")[1])
			}
		}
	}
	return ret, nil
}

func getDefaultCpuInfo(ret *model.SystemInfo) (*model.SystemInfo, error) {
	path, exists := utils.GetPATH("sysctl")
	if !exists {
		return ret, nil
	}
	updateSysctlCpuInfo(ret, path)
	updateSysctlFeatures(ret, path)
	updateSysctlUptime(ret, path)
	updateSystemLoad(ret)
	return ret, nil
}

func updateSysctlCpuInfo(ret *model.SystemInfo, sysctlPath string) {
	if ret.CpuModel == "" || len(ret.CpuModel) < 3 {
		cname, err := getSysctlValue(sysctlPath, "hw.model")
		if err == nil && !strings.Contains(cname, "cannot") {
			ret.CpuModel = cname
			freq, err := getSysctlValue(sysctlPath, "dev.cpu.0.freq")
			if err == nil && !strings.Contains(freq, "cannot") {
				ret.CpuModel += " @" + freq + "MHz"
			}
		}
	}
	if ret.CpuCores == "" {
		cores, err := getSysctlValue(sysctlPath, "hw.ncpu")
		if err == nil && !strings.Contains(cores, "cannot") {
			ret.CpuCores = cores + " CPU(s)"
		}
	}
	if ret.CpuCache == "" {
		ccache, err := getSysctlValue(sysctlPath, "hw.cacheconfig")
		if err == nil && !strings.Contains(ccache, "cannot") {
			ret.CpuCache = strings.TrimSpace(strings.Split(ccache, ":")[1])
		}
	}
}

func updateSysctlFeatures(ret *model.SystemInfo, sysctlPath string) {
	aesOut, err := exec.Command(sysctlPath, "-a").Output()
	if err != nil {
		return
	}
	if ret.CpuAesNi == "Unsupported OS" || ret.CpuAesNi == "" {
		updateAesFeature(ret, string(aesOut))
	}
	if ret.CpuVAH == "Unsupported OS" || ret.CpuVAH == "" {
		updateVirtualizationFeature(ret, string(aesOut))
	}
}

func updateAesFeature(ret *model.SystemInfo, output string) {
	aesReg := regexp.MustCompile(`crypto\.aesni\s*=\s*(\d)`)
	aesMatch := aesReg.FindStringSubmatch(output)
	if len(aesMatch) <= 1 {
		aesReg = regexp.MustCompile(`dev\.aesni\.0\.%desc:\s*(.+)`)
		aesMatch = aesReg.FindStringSubmatch(output)
	}
	if len(aesMatch) > 1 {
		ret.CpuAesNi = getFeatureStatus(true)
	} else {
		ret.CpuAesNi = getFeatureStatus(false)
	}
}

func updateVirtualizationFeature(ret *model.SystemInfo, output string) {
	virtReg := regexp.MustCompile(`(hw\.vmx|hw\.svm)\s*=\s*(\d)`)
	virtMatch := virtReg.FindStringSubmatch(output)
	if len(virtMatch) > 2 {
		ret.CpuVAH = getFeatureStatus(true)
	} else {
		ret.CpuVAH = getFeatureStatus(false)
	}
}

func getFeatureStatus(enabled bool) string {
	if enabled {
		if runtime.GOOS == "windows" {
			return "[Y] Enabled"
		}
		return "✔️ Enabled"
	}
	if runtime.GOOS == "windows" {
		return "[N] Disabled"
	}
	return "❌ Disabled"
}

func updateSysctlUptime(ret *model.SystemInfo, sysctlPath string) {
	if ret.Uptime != "" {
		return
	}
	boottimeStr, err := getSysctlValue(sysctlPath, "kern.boottime")
	if err != nil {
		return
	}
	boottimeReg := regexp.MustCompile(`sec = (\d+), usec = (\d+)`)
	boottimeMatch := boottimeReg.FindStringSubmatch(boottimeStr)
	if len(boottimeMatch) <= 1 {
		return
	}
	boottime, err := strconv.ParseInt(boottimeMatch[1], 10, 64)
	if err != nil {
		return
	}
	uptime := time.Now().Unix() - boottime
	days := uptime / 86400
	hours := (uptime % 86400) / 3600
	minutes := (uptime % 3600) / 60
	ret.Uptime = fmt.Sprintf("%d days, %d hours, %d minutes", days, hours, minutes)
}
