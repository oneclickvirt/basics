package main

import (
	"fmt"
	"net/http"

	"github.com/oneclickvirt/basics/network"
	"github.com/oneclickvirt/basics/system"
)

func main() {
	go func() {
		http.Get("https://hits.seeyoufarm.com/api/count/incr/badge.svg?url=https%3A%2F%2Fgithub.com%2Foneclickvirt%2Fbasics&count_bg=%232EFFF8&title_bg=%23555555&icon=&icon_color=%23E7E7E7&title=hits&edge_flat=false")
	}()
	fmt.Println("项目地址:", "https://github.com/oneclickvirt/basics")
	ipInfo, _, _ := network.NetworkCheck("both", false, "zh")
	fmt.Println("--------------------------------------------------")
	system.CheckSystemInfo()
	fmt.Printf(ipInfo)
	fmt.Println("--------------------------------------------------")
}
