package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestParseCLIOptions(t *testing.T) {
	opts, err := parseCLI([]string{"--json", "--timeout", "2s", "-l", "en"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if !opts.jsonOutput || opts.language != "en" || opts.timeout != 2*time.Second {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestHelpRetainsLegacyFlags(t *testing.T) {
	var output bytes.Buffer
	newFlagSet(&cliOptions{}, &output).PrintDefaults()
	for _, legacy := range []string{"-h", "-l string", "-log", "-v"} {
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
