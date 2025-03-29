package baseinfo

import (
	"fmt"
	"testing"
)

func TestNeighborCount(t *testing.T) {
	ip := "54.92.128.0" // 示例 IP
	prefixNum := 24     // 示例前缀
	neighborActive, neighborTotal, err := GetNeighborCount(ip, prefixNum)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Neighbor Active: %d/%d\n", neighborActive, neighborTotal)
}
