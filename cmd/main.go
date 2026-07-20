package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

type cliOptions struct {
	help, version, jsonOutput, log bool
	language                       string
	timeout                        time.Duration
}

func parseCLI(args []string) (cliOptions, error) {
	opts := cliOptions{}
	fs := newFlagSet(&opts, io.Discard)
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if opts.timeout < 0 {
		return opts, fmt.Errorf("timeout must not be negative")
	}
	return opts, nil
}

func newFlagSet(opts *cliOptions, output io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("basics", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.BoolVar(&opts.help, "h", false, "Show help information")
	fs.BoolVar(&opts.version, "v", false, "Show version")
	fs.BoolVar(&opts.log, "log", false, "Enable logging")
	fs.StringVar(&opts.language, "l", "", "Set language (en or zh)")
	fs.BoolVar(&opts.jsonOutput, "json", false, "Print the structured system report as JSON")
	fs.BoolVar(&opts.jsonOutput, "structured", false, "Print the structured system report as JSON")
	fs.DurationVar(&opts.timeout, "timeout", 0, "Structured report timeout (for example 10s)")
	return fs
}

func printCLIHelp(program string) {
	fmt.Printf("Usage: %s [options]\n", program)
	newFlagSet(&cliOptions{}, os.Stdout).PrintDefaults()
}

func main() {
	opts, err := parseCLI(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	model.EnableLoger = opts.log
	if opts.help {
		printCLIHelp(os.Args[0])
		return
	}
	if opts.version {
		fmt.Println(model.BasicsVersion)
		return
	}
	if opts.jsonOutput {
		timeout := opts.timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		report, marshalErr := json.Marshal(system.CollectSystemReport(ctx))
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, marshalErr)
			return
		}
		fmt.Println(string(report))
		return
	}
	language := opts.language
	if language == "" {
		language = "zh"
	}
	language = strings.ToLower(language)
	go func() {
		defer func() {
			_ = recover()
		}()
		resp, err := http.Get("https://hits.spiritlhl.net/basics.svg?action=hit&title=Hits&title_bg=%23555555&count_bg=%230eecf8&edge_flat=false")
		if err == nil && resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
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
