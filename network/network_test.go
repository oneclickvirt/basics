package network

import (
	"fmt"
	"testing"
)

// 本文件夹 network 修改需要同步 https://github.com/oneclickvirt/security 否则 goecs 无法使用
func TestIpv4SecurityCheck(t *testing.T) {
	// 全项测试
	ipInfo, _, _ := NetworkCheck("both", false, "zh")
	fmt.Println("--------------------------------------------------")
	fmt.Printf(ipInfo)
	fmt.Println("--------------------------------------------------")
}
