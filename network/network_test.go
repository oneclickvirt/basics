package network

import (
	"fmt"
	"testing"
)

func TestIpv4SecurityCheck(t *testing.T) {
	// 全项测试
	ipInfo, _, _ := NetworkCheck("both", false, "zh")
	fmt.Println("--------------------------------------------------")
	fmt.Printf(ipInfo)
	fmt.Println("--------------------------------------------------")
}
