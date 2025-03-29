package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/oneclickvirt/basics/ipv6"
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
	basicsFlag.BoolVar(&model.EnableLoger, "log", false, "Enable logging")
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
		http.Get("https://hits.spiritlhl.net/basics.svg?action=hit&title=Hits&title_bg=%23555555&count_bg=%230eecf8&edge_flat=false")
	}()
	fmt.Println("项目地址:", "https://github.com/oneclickvirt/basics")
	ipInfo, _, _ := network.NetworkCheck("both", false, language)
	ipv6Info, err := ipv6.GetIPv6Mask(language)
	res := system.CheckSystemInfo(language)
	fmt.Println("--------------------------------------------------")
	temp := strings.ReplaceAll(res+ipInfo, "\n\n", "\n")
	if err == nil && res != "" {
		temp += ipv6Info
	}
	fmt.Printf(temp)
	fmt.Println("--------------------------------------------------")
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		fmt.Println("Press Enter to exit...")
		fmt.Scanln()
	}
}
