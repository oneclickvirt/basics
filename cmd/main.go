package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/oneclickvirt/basics/model"
	"github.com/oneclickvirt/basics/network"
	"github.com/oneclickvirt/basics/system"
	"github.com/oneclickvirt/basics/utils"
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
	fmt.Println("Repo:", "https://github.com/oneclickvirt/basics")
	preCheck := utils.CheckPublicAccess(3 * time.Second)
	var ipInfo string
	if preCheck.Connected && preCheck.StackType == "DualStack" {
		_, _, ipInfo, _, _ = network.NetworkCheck("both", false, language)
	} else if preCheck.Connected && preCheck.StackType == "IPv4" {
		_, _, ipInfo, _, _ = network.NetworkCheck("ipv4", false, language)
	} else if preCheck.Connected && preCheck.StackType == "IPv6" {
		_, _, ipInfo, _, _ = network.NetworkCheck("ipv6", false, language)
	}
	res := system.CheckSystemInfo(language)
	fmt.Println("--------------------------------------------------")
	fmt.Print(strings.ReplaceAll(res+ipInfo, "\n\n", "\n"))
	fmt.Println("--------------------------------------------------")
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		fmt.Println("Press Enter to exit...")
		fmt.Scanln()
	}
}
