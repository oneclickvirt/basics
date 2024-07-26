package system

import (
	"os/exec"
	"strings"
)

// func checkSysctlVersion(path string) bool {
// 	out, err := exec.Command(path, "-h").Output()
// 	if err != nil {
// 		return false
// 	}
// 	if strings.Contains(string(out), "error") {
// 		return false
// 	}
// 	return true
// }

func getSysctlValue(path, key string) (string, error) {
	out, err := exec.Command(path, "-n", key).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
