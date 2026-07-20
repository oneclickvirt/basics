package main

import (
	"os"
	"testing"
)

func Test_main(t *testing.T) {
	args := os.Args
	os.Args = []string{args[0], "-h"}
	defer func() { os.Args = args }()
	main()
}
