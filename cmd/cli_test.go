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
	if !opts.jsonOutput || opts.textOutput || opts.language != "en" || opts.timeout != 2*time.Second {
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

func TestParseCLIRejectsConflictingStructuredOutputs(t *testing.T) {
	if _, err := parseCLI([]string{"--json", "--text"}); err == nil {
		t.Fatal("expected conflicting structured output modes to be rejected")
	}
}

func TestParseCLIRejectsUnsupportedLanguageAndPositionalArguments(t *testing.T) {
	for _, args := range [][]string{{"-l", "fr"}, {"unexpected"}} {
		if _, err := parseCLI(args); err == nil {
			t.Fatalf("expected arguments %v to be rejected", args)
		}
	}
	opts, err := parseCLI([]string{"-l", " EN "})
	if err != nil || opts.language != "en" {
		t.Fatalf("language was not normalized: opts=%#v err=%v", opts, err)
	}
}
