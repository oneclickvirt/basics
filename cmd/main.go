package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/oneclickvirt/basics/model"
	"github.com/oneclickvirt/basics/network"
	"github.com/oneclickvirt/basics/system"
)

func main() {
	var showVersion, help bool
	var language string
	basicsFlag := flag.NewFlagSet("basics", flag.ContinueOnError)
	basicsFlag.BoolVar(&help, "h", false, "Show help information")
	basicsFlag.BoolVar(&showVersion, "v", false, "Show version")
	basicsFlag.BoolVar(&model.EnableLoger, "e", false, "Enable logging")
	basicsFlag.StringVar(&language, "l", "", "Set language (en or zh)")
	basicsFlag.Parse(os.Args[1:])
	if help {
		fmt.Printf("Usage: %s [options]\n", os.Args[0])
		basicsFlag.PrintDefaults()
		return
	}
	if showVersion {
		fmt.Println(model.BasicsVersion)
		return
	}
	if language == "" {
		language = "zh"
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
