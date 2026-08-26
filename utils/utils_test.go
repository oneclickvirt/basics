package utils

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestCheckPublicAccess(t *testing.T) {
	if os.Getenv("BASICS_INTEGRATION") != "1" {
		t.Skip("set BASICS_INTEGRATION=1 to run live public-access checks")
	}
	timeout := 3 * time.Second
	result := CheckPublicAccess(timeout)
	if result.Connected {
		fmt.Printf("✅ 本机有公网连接，类型: %s\n", result.StackType)
	} else {
		fmt.Println("❌ 本机未检测到公网连接")
	}
}
