package main

import (
	"os"
	"strings"
	"testing"
)

func Test_main(t *testing.T) {
	args := os.Args
	os.Args = []string{args[0], "-h"}
	defer func() { os.Args = args }()
	main()
}

func TestParseCLIRejectsUnusedLegacyTimeout(t *testing.T) {
	_, err := parseCLI([]string{"-timeout", "3s"})
	if err == nil || !strings.Contains(err.Error(), "requires") {
		t.Fatalf("legacy timeout error = %v", err)
	}
	if _, err := parseCLI([]string{"-text", "-timeout", "3s"}); err != nil {
		t.Fatalf("structured text timeout rejected: %v", err)
	}
}
