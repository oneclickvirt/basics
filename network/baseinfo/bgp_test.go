package baseinfo

import (
	"fmt"
	"testing"
)

func TestNeighborCount(t *testing.T) {
	ip := "54.92.128.123" // 示例 IP
	neighborActive, neighborTotal, err := GetActiveIpsCount(MaskIP(ip), 24)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Neighbor Active: %d/%d\n", neighborActive, neighborTotal)
}
