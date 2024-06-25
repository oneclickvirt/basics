package main

import (
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/oneclickvirt/basics/model"
	"github.com/oneclickvirt/basics/network"
	"github.com/oneclickvirt/basics/system"
)

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&model.EnableLoger, "e", false, "Enable logging")
	languagePtr := flag.String("l", "", "Language parameter (en or zh)")
	flag.Parse()
	if showVersion {
		fmt.Println(model.BasicsVersion)
		return
	}
	var language string
	if *languagePtr == "" {
		language = "zh"
	} else {
		language = *languagePtr
	}
	language = strings.ToLower(language)
	go func() {
		http.Get("https://hits.seeyoufarm.com/api/count/incr/badge.svg?url=https%3A%2F%2Fgithub.com%2Foneclickvirt%2Fbasics&count_bg=%232EFFF8&title_bg=%23555555&icon=&icon_color=%23E7E7E7&title=hits&edge_flat=false")
	}()
	fmt.Println("项目地址:", "https://github.com/oneclickvirt/basics")
	ipInfo, _, _ := network.NetworkCheck("both", false, language)
	res := system.CheckSystemInfo(language)
	fmt.Println("--------------------------------------------------")
	fmt.Printf(strings.ReplaceAll(res+ipInfo, "\n\n", "\n"))
	fmt.Println("--------------------------------------------------")
}
