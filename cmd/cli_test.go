package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/oneclickvirt/basics/network/resolver"
	"github.com/oneclickvirt/basics/utils"
)

func TestParseCLIOptions(t *testing.T) {
	opts, err := parseCLI([]string{"--json", "--timeout", "2s", "-l", "en", "-dns-mode", "doh"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if !opts.jsonOutput || opts.textOutput || opts.language != "en" || opts.timeout != 2*time.Second || opts.dnsMode != "doh" {
		t.Fatalf("unexpected options: %#v", opts)
	}
	textOpts, err := parseCLI([]string{"--text", "-l", "zh"})
	if err != nil || !textOpts.textOutput || textOpts.jsonOutput || textOpts.language != "zh" {
		t.Fatalf("unexpected text options: %#v, err=%v", textOpts, err)
	}
}

func TestHelpRetainsLegacyFlags(t *testing.T) {
	var output bytes.Buffer
	newFlagSet(&cliOptions{}, &output).PrintDefaults()
	for _, legacy := range []string{"-dns-mode string", "-h", "-l string", "-log", "-v"} {
		if !strings.Contains(output.String(), legacy) {
			t.Fatalf("help is missing legacy flag %q: %s", legacy, output.String())
		}
	}
}

func TestParseCLIRejectsNegativeTimeout(t *testing.T) {
	if _, err := parseCLI([]string{"--timeout", "-1s"}); err == nil {
		t.Fatal("expected negative timeout to be rejected")
	}
}

func TestParseCLIRejectsConflictingStructuredOutputs(t *testing.T) {
	if _, err := parseCLI([]string{"--json", "--text"}); err == nil {
		t.Fatal("expected conflicting structured output modes to be rejected")
	}
}

func TestParseCLIRejectsUnsupportedLanguageAndPositionalArguments(t *testing.T) {
	for _, args := range [][]string{{"-l", "fr"}, {"-dns-mode", "invalid"}, {"unexpected"}} {
		if _, err := parseCLI(args); err == nil {
			t.Fatalf("expected arguments %v to be rejected", args)
		}
	}
	opts, err := parseCLI([]string{"-l", " EN "})
	if err != nil || opts.language != "en" {
		t.Fatalf("language was not normalized: opts=%#v err=%v", opts, err)
	}
}

func TestPromoteEncryptedDNSConnectivity(t *testing.T) {
	originalStackType := utils.StackType
	t.Cleanup(func() { utils.StackType = originalStackType })
	preCheck := &utils.NetCheckResult{StackType: "None"}
	promoteEncryptedDNSConnectivity(preCheck, resolver.Status{Active: resolver.ModeDoT, Stack: "IPv6"})
	if !preCheck.Connected || !preCheck.HasIPv6 || preCheck.HasIPv4 || preCheck.StackType != "IPv6" {
		t.Fatalf("successful encrypted DNS did not promote network state: %#v", preCheck)
	}
}

func TestConfigureCLIResolverScopesBootstrapFallback(t *testing.T) {
	originalConfigure := cliDNSConfigureFn
	originalShutdown := cliDNSShutdownFn
	originalBootstrapReachable := cliDNSBootstrapReachableFn
	t.Cleanup(func() {
		cliDNSConfigureFn = originalConfigure
		cliDNSShutdownFn = originalShutdown
		cliDNSBootstrapReachableFn = originalBootstrapReachable
	})
	tests := []struct {
		name               string
		mode               resolver.Mode
		connected          bool
		bootstrapReachable bool
		configuredStatus   resolver.Status
		wantConfigure      int
		wantBootstrap      int
		wantShutdown       int
	}{
		{
			name:             "connected auto uses normal resolver path",
			mode:             resolver.ModeAuto,
			connected:        true,
			configuredStatus: resolver.Status{Requested: resolver.ModeAuto, Active: resolver.ModeSystem, SystemAvailable: true},
			wantConfigure:    1,
		},
		{
			name:          "offline auto stops when bootstrap is unreachable",
			mode:          resolver.ModeAuto,
			wantBootstrap: 1,
			wantShutdown:  1,
		},
		{
			name:               "offline auto retries after reachable bootstrap",
			mode:               resolver.ModeAuto,
			bootstrapReachable: true,
			configuredStatus:   resolver.Status{Requested: resolver.ModeAuto, Active: resolver.ModeDoH, DoHAvailable: true, Fallback: true, Stack: "IPv4"},
			wantConfigure:      1,
			wantBootstrap:      1,
		},
		{
			name:             "forced DoH bypasses bootstrap gate",
			mode:             resolver.ModeDoH,
			configuredStatus: resolver.Status{Requested: resolver.ModeDoH, Active: resolver.ModeDoH, DoHAvailable: true},
			wantConfigure:    1,
		},
		{
			name:             "forced DoT bypasses bootstrap gate",
			mode:             resolver.ModeDoT,
			configuredStatus: resolver.Status{Requested: resolver.ModeDoT, Active: resolver.ModeDoT, DoTAvailable: true},
			wantConfigure:    1,
		},
		{
			name:         "system mode never falls back",
			mode:         resolver.ModeSystem,
			wantShutdown: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configureCalls := 0
			bootstrapCalls := 0
			shutdownCalls := 0
			cliDNSConfigureFn = func(_ context.Context, config resolver.Config) resolver.Status {
				configureCalls++
				if config.Mode != test.mode {
					t.Fatalf("resolver mode = %q, want %q", config.Mode, test.mode)
				}
				return test.configuredStatus
			}
			cliDNSBootstrapReachableFn = func(_ context.Context, config resolver.Config) (string, bool) {
				bootstrapCalls++
				if config.Mode != resolver.ModeAuto {
					t.Fatalf("bootstrap mode = %q, want auto", config.Mode)
				}
				return "IPv4", test.bootstrapReachable
			}
			cliDNSShutdownFn = func() { shutdownCalls++ }

			status := configureCLIResolver(context.Background(), test.mode, &utils.NetCheckResult{Connected: test.connected})
			if configureCalls != test.wantConfigure || bootstrapCalls != test.wantBootstrap || shutdownCalls != test.wantShutdown {
				t.Fatalf("DNS calls = configure:%d bootstrap:%d shutdown:%d, want configure:%d bootstrap:%d shutdown:%d", configureCalls, bootstrapCalls, shutdownCalls, test.wantConfigure, test.wantBootstrap, test.wantShutdown)
			}
			if test.wantConfigure == 0 && status.Active != resolver.ModeUnavailable {
				t.Fatalf("status = %#v, want unavailable", status)
			}
		})
	}
}
