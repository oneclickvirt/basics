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

func hasFrequency(model string) bool {
	matched, _ := regexp.MatchString(`@\s*\d+\.?\d*\s*(GHz|MHz)`, model)
	return matched
}

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
	var cpuMHz, cpuGHz string
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ":")
		if len(fields) >= 2 {
			key := strings.TrimSpace(fields[0])
			value := strings.TrimSpace(strings.Join(fields[1:], " "))
			switch {
			case strings.Contains(key, "model name"):
				ret.CpuModel = strings.Join(strings.Fields(value), " ")
				modelNameFound = true
			case strings.Contains(key, "cache size"):
				ret.CpuCache = value
			case strings.Contains(key, "cpu MHz"):
				cpuMHz = value
			case strings.Contains(key, "cpu GHz"):
				cpuGHz = value
			}
		}
	}
	if modelNameFound && cpuMHz != "" && !hasFrequency(ret.CpuModel) {
		ret.CpuModel = strings.Join(strings.Fields(ret.CpuModel+" @ "+cpuMHz+" MHz"), " ")
	}
	if modelNameFound && cpuGHz != "" && !hasFrequency(ret.CpuModel) {
		ret.CpuModel = strings.Join(strings.Fields(ret.CpuModel+" @ "+cpuGHz+" GHz"), " ")
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
			ret.CpuModel = strings.Join(strings.Fields(value), " ")
		case strings.Contains(fields[0], "CPU MHz") && !hasFrequency(ret.CpuModel) && !strings.Contains(ret.CpuModel, "@"):
			ret.CpuModel = strings.Join(strings.Fields(ret.CpuModel+" @ "+value+" MHz"), " ")
		case strings.Contains(fields[0], "CPU static MHz") && !hasFrequency(ret.CpuModel) && !strings.Contains(ret.CpuModel, "@"):
			ret.CpuModel += " @ " + value + " static MHz"
		case strings.Contains(fields[0], "CPU dynamic MHz") && !hasFrequency(ret.CpuModel) && !strings.Contains(ret.CpuModel, "@"):
			ret.CpuModel += " @ " + value + " dynamic MHz"
		case strings.Contains(fields[0], "L1d cache") || strings.Contains(fields[0], "L1d"):
			L1dcache = strings.Split(value, " ")[0]
		case strings.Contains(fields[0], "L1i cache") || strings.Contains(fields[0], "L1i"):
			L1icache = strings.Split(value, " ")[0]
		case strings.Contains(fields[0], "L2 cache") || strings.Contains(fields[0], "L2"):
			L2cache = strings.Split(value, " ")[0]
		case strings.Contains(fields[0], "L3 cache") || strings.Contains(fields[0], "L3"):
			L3cache = strings.Split(value, " ")[0]
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

func getCpuCoreDetails(ret *model.SystemInfo) {
	// 默认值：使用runtime.NumCPU()作为逻辑核心数
	ret.CpuLogicalCores = runtime.NumCPU()
	ret.CpuPhysicalCores = 0 // 初始化为0，如果获取失败则保持为0
	ret.CpuThreadsPerCore = 1
	ret.CpuSockets = 1

	// 首先尝试使用gopsutil获取核心信息（跨平台支持）
	if !getCpuCoreDetailsFromGopsutil(ret) {
		// 如果gopsutil失败，根据不同操作系统使用特定方法
		switch runtime.GOOS {
		case "linux":
			getCpuCoreDetailsLinux(ret)
		case "windows":
			getCpuCoreDetailsWindows(ret)
		case "darwin":
			getCpuCoreDetailsDarwin(ret)
		}
	}

	// 最终降级处理：如果没有成功获取物理核心数，设置为逻辑核心数
	if ret.CpuPhysicalCores <= 0 {
		ret.CpuPhysicalCores = ret.CpuLogicalCores
		ret.CpuThreadsPerCore = 1
	}
}

func getCpuCoreDetailsFromGopsutil(ret *model.SystemInfo) bool {
	// 获取逻辑核心数
	logicalCores, err := cpu.Counts(true)
	if err == nil && logicalCores > 0 {
		ret.CpuLogicalCores = logicalCores
	} else {
		return false
	}

	// 获取物理核心数
	physicalCores, err := cpu.Counts(false)
	if err == nil && physicalCores > 0 {
		ret.CpuPhysicalCores = physicalCores
	} else {
		return false
	}

	// 计算每核心线程数
	if ret.CpuPhysicalCores > 0 && ret.CpuLogicalCores >= ret.CpuPhysicalCores {
		ret.CpuThreadsPerCore = ret.CpuLogicalCores / ret.CpuPhysicalCores
	}

	// 尝试从cpu.Info()获取socket信息
	cpuInfo, err := cpu.Info()
	if err == nil && len(cpuInfo) > 0 {
		physicalIDs := make(map[string]bool)
		for _, info := range cpuInfo {
			if info.PhysicalID != "" {
				physicalIDs[info.PhysicalID] = true
			}
		}
		if len(physicalIDs) > 0 {
			ret.CpuSockets = len(physicalIDs)
		}
	}

	return ret.CpuPhysicalCores > 0
}

func getCpuCoreDetailsLinux(ret *model.SystemInfo) {
	var foundDetails bool
	var coresPerSocket int

	// 尝试从lscpu获取信息
	cmd := exec.Command("lscpu", "-B")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			fields := strings.Split(line, ":")
			if len(fields) < 2 {
				continue
			}
			key := strings.TrimSpace(fields[0])
			value := strings.TrimSpace(fields[1])

			switch key {
			case "CPU(s)":
				if v, err := strconv.Atoi(value); err == nil && v > 0 {
					ret.CpuLogicalCores = v
					foundDetails = true
				}
			case "Thread(s) per core":
				if v, err := strconv.Atoi(value); err == nil && v > 0 {
					ret.CpuThreadsPerCore = v
					foundDetails = true
				}
			case "Core(s) per socket":
				if v, err := strconv.Atoi(value); err == nil && v > 0 {
					coresPerSocket = v
					foundDetails = true
				}
			case "Socket(s)":
				if v, err := strconv.Atoi(value); err == nil && v > 0 {
					ret.CpuSockets = v
					foundDetails = true
				}
			}
		}

		// 计算总物理核心数 = 每插槽核心数 × 插槽数
		if coresPerSocket > 0 && ret.CpuSockets > 0 {
			ret.CpuPhysicalCores = coresPerSocket * ret.CpuSockets
		}
	}

	// 如果lscpu没有获取到完整信息，尝试从/proc/cpuinfo获取
	if !foundDetails || ret.CpuPhysicalCores <= 0 {
		getCpuCoreDetailsFromProcCpuinfo(ret)
	}
}

func getCpuCoreDetailsFromProcCpuinfo(ret *model.SystemInfo) {
	content, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return
	}

	physicalIDs := make(map[string]bool)
	var siblings, cpuCores int
	var foundInfo bool

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])

		switch key {
		case "physical id":
			physicalIDs[value] = true
			foundInfo = true
		case "siblings":
			if v, err := strconv.Atoi(value); err == nil && siblings == 0 {
				siblings = v
				foundInfo = true
			}
		case "cpu cores":
			if v, err := strconv.Atoi(value); err == nil && cpuCores == 0 {
				cpuCores = v
				foundInfo = true
			}
		}
	}

	// 只有在成功获取到信息时才更新
	if foundInfo {
		if len(physicalIDs) > 0 {
			ret.CpuSockets = len(physicalIDs)
		}
		if cpuCores > 0 && len(physicalIDs) > 0 {
			ret.CpuPhysicalCores = cpuCores * len(physicalIDs)
		} else if cpuCores > 0 {
			// 如果没有physical id信息，至少使用cpu cores
			ret.CpuPhysicalCores = cpuCores
		}
		if siblings > 0 && cpuCores > 0 && siblings >= cpuCores {
			ret.CpuThreadsPerCore = siblings / cpuCores
		}
	}
}

func getCpuCoreDetailsWindows(ret *model.SystemInfo) {
	// Windows下尝试使用wmic获取信息
	cmd := exec.Command("wmic", "cpu", "get", "NumberOfCores,NumberOfLogicalProcessors", "/format:list")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	var totalCores, totalLogical int
	for _, line := range lines {
		if strings.Contains(line, "NumberOfCores=") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				if v, err := strconv.Atoi(value); err == nil && v > 0 {
					totalCores += v
				}
			}
		} else if strings.Contains(line, "NumberOfLogicalProcessors=") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				if v, err := strconv.Atoi(value); err == nil && v > 0 {
					totalLogical += v
				}
			}
		}
	}

	// 只有在成功获取到信息时才更新
	if totalCores > 0 {
		ret.CpuPhysicalCores = totalCores
	}
	if totalLogical > 0 {
		ret.CpuLogicalCores = totalLogical
	}
	if totalCores > 0 && totalLogical > 0 && totalLogical >= totalCores {
		ret.CpuThreadsPerCore = totalLogical / totalCores
	}
}

func getCpuCoreDetailsDarwin(ret *model.SystemInfo) {
	// macOS下尝试使用sysctl获取信息
	path, exists := utils.GetPATH("sysctl")
	if !exists {
		return
	}

	// 获取物理核心数
	if out, err := exec.Command(path, "-n", "hw.physicalcpu").Output(); err == nil {
		value := strings.TrimSpace(string(out))
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			ret.CpuPhysicalCores = v
		}
	}

	// 获取逻辑核心数
	if out, err := exec.Command(path, "-n", "hw.logicalcpu").Output(); err == nil {
		value := strings.TrimSpace(string(out))
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			ret.CpuLogicalCores = v
		}
	}

	// 计算每核心线程数
	if ret.CpuPhysicalCores > 0 && ret.CpuLogicalCores > 0 && ret.CpuLogicalCores >= ret.CpuPhysicalCores {
		ret.CpuThreadsPerCore = ret.CpuLogicalCores / ret.CpuPhysicalCores
	}
}

func formatCpuCores(ret *model.SystemInfo, cpuType string, lang string) string {
	if ret.CpuLogicalCores == 0 {
		return ""
	}

	// 检查是否成功获取了详细信息（物理核心数与逻辑核心数不同，或者有多线程）
	hasDetailedInfo := (ret.CpuPhysicalCores > 0 &&
		(ret.CpuPhysicalCores != ret.CpuLogicalCores || ret.CpuThreadsPerCore > 1))

	// 降级处理：如果没有获取到详细信息，使用简单格式
	if !hasDetailedInfo {
		if lang == "zh" || lang == "cn" || lang == "chinese" {
			return fmt.Sprintf("%d %s CPU(s)", ret.CpuLogicalCores, cpuType)
		} else {
			return fmt.Sprintf("%d %s CPU(s)", ret.CpuLogicalCores, cpuType)
		}
	}

	// 检测是否为混合架构（简单判断：如果线程数不是整数倍）
	isHybrid := (ret.CpuLogicalCores != ret.CpuPhysicalCores*ret.CpuThreadsPerCore)

	if lang == "zh" || lang == "cn" || lang == "chinese" {
		if isHybrid {
			// 混合架构，可能有大小核
			return fmt.Sprintf("%d 插槽, %d 物理核心, %d 逻辑线程 (混合架构)",
				ret.CpuSockets, ret.CpuPhysicalCores, ret.CpuLogicalCores)
		} else if ret.CpuThreadsPerCore > 1 {
			// 有超线程
			return fmt.Sprintf("%d 插槽, %d 物理核心, %d 逻辑线程",
				ret.CpuSockets, ret.CpuPhysicalCores, ret.CpuLogicalCores)
		} else {
			// 无超线程
			return fmt.Sprintf("%d 插槽, %d 物理核心", ret.CpuSockets, ret.CpuPhysicalCores)
		}
	} else {
		if isHybrid {
			// Hybrid architecture
			return fmt.Sprintf("%d Socket(s), %d Physical Core(s), %d Logical Thread(s) (Hybrid)",
				ret.CpuSockets, ret.CpuPhysicalCores, ret.CpuLogicalCores)
		} else if ret.CpuThreadsPerCore > 1 {
			// With hyper-threading
			return fmt.Sprintf("%d Socket(s), %d Physical Core(s), %d Logical Thread(s)",
				ret.CpuSockets, ret.CpuPhysicalCores, ret.CpuLogicalCores)
		} else {
			// No hyper-threading
			return fmt.Sprintf("%d Socket(s), %d Physical Core(s)", ret.CpuSockets, ret.CpuPhysicalCores)
		}
	}
}

func getCpuInfo(ret *model.SystemInfo, cpuType string) (*model.SystemInfo, error) {
	// 获取详细的核心信息
	getCpuCoreDetails(ret)

	// 根据系统语言格式化输出
	lang := os.Getenv("LANG")
	if strings.Contains(strings.ToLower(lang), "zh") {
		ret.CpuCores = formatCpuCores(ret, cpuType, "zh")
	} else {
		ret.CpuCores = formatCpuCores(ret, cpuType, "en")
	}

	// 如果格式化失败，使用默认格式
	if ret.CpuCores == "" && runtime.NumCPU() != 0 {
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
			ret.CpuModel = strings.Join(strings.Fields(strings.TrimSpace(ci[i].ModelName)), " ")
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
			// 如果当前型号没有频率信息，且已有型号包含频率
			if !hasFrequency(newModel) && hasFrequency(ret.CpuModel) {
				// 保留现有带频率的型号
				continue
			}
			// 如果新型号更完整
			if len(newModel) > len(ret.CpuModel) || hasFrequency(newModel) {
				ret.CpuModel = strings.Join(strings.Fields(newModel), " ")
			}
		}
	}
	ret.CpuModel = strings.ReplaceAll(ret.CpuModel, "  ", " ")
	deviceTreeContent, err := os.ReadFile("/proc/device-tree")
	if err == nil && ret.CpuModel == "" {
		ret.CpuModel = strings.Join(strings.Fields(string(deviceTreeContent)), " ")
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
			ret.CpuModel = strings.Join(strings.Fields(cname), " ")
			freq, err := getSysctlValue(sysctlPath, "dev.cpu.0.freq")
			if err == nil && !strings.Contains(freq, "cannot") {
				ret.CpuModel += " @ " + freq + "MHz"
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
