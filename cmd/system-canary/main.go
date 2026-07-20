package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/oneclickvirt/basics/system"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	report := system.CollectSystemReport(ctx)
	encoded, err := json.Marshal(report)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}
