package system

import (
	"os/exec"
	"strings"

	"github.com/oneclickvirt/basics/model"
)

func isMacOS() bool {
	out, err := exec.Command("uname", "-a").Output()
	if err != nil {
		return false
	}
	systemName := strings.ToLower(string(out))
	return strings.Contains(systemName, "darwin")
}

func getMacOSInfo() {
	out, err := exec.Command("system_profiler", "SPHardwareDataType").Output()
	if err == nil && !strings.Contains(string(out), "error") {
		model.MacOSInfo = strings.Split(string(out), "\n")
	}
}
