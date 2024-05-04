package system

import (
	"bytes"
	"os/exec"
)

// getTCPAccelerateStatus 查询TCP控制算法
func getTCPAccelerateStatus() string {
    cmd := exec.Command("sysctl", "-n", "net.ipv4.tcp_congestion_control")
    var out bytes.Buffer
    cmd.Stdout = &out
    err := cmd.Run()
    if err != nil {
        return ""
    } else {
        return out.String()
    }
}
