package baseinfo

import (
	"fmt"
	"testing"
)

func TestNeighborCount(t *testing.T) {
	ip := "207.174.22.39" // 示例 IP
	neighborActive, neighborTotal, err := GetActiveIpsCount(MaskIP(ip), 24)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Neighbor Active: %d/%d\n", neighborActive, neighborTotal)
}

// #!/bin/bash
// subnet="207.174.22"
// total_count=256
// active_count=$(seq 1 254 | xargs -P 50 -I {} sh -c "ping -c 1 -W 1 $subnet.{} >/dev/null 2>&1 && echo {}" | wc -l)
// echo "活跃 IP 数量: $active_count / $total_count"
