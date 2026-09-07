package utils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

func TestFamilyDialContextPinsRequestedFamilyWhenAAAAComesFirst(t *testing.T) {
	cases := []struct {
		name   string
		family string
	}{
		{name: "IPv4", family: "tcp4"},
		{name: "IPv6", family: "tcp6"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var gotNetwork, gotAddress string
			wantErr := errors.New("stop after capturing the dial")
			dial := familyDialContext(func(_ context.Context, network, address string) (net.Conn, error) {
				gotNetwork, gotAddress = network, address
				return nil, wantErr
			}, test.family)
			_, err := dial(context.Background(), "tcp", "aaaa-first.example:443")
			if !errors.Is(err, wantErr) {
				t.Fatalf("dial error = %v, want capture error", err)
			}
			if gotNetwork != test.family {
				t.Fatalf("dial network = %q, want %q; a preferred AAAA record must not change the requested family", gotNetwork, test.family)
			}
			if gotAddress != "aaaa-first.example:443" {
				t.Fatalf("dial address = %q", gotAddress)
			}
		})
	}
}

func TestCheckPublicAccess(t *testing.T) {
	if os.Getenv("BASICS_INTEGRATION") != "1" {
		t.Skip("set BASICS_INTEGRATION=1 to run live public-access checks")
	}
	timeout := 3 * time.Second
	result := CheckPublicAccess(timeout)
	if result.Connected {
		fmt.Printf("✅ 本机有公网连接，类型: %s\n", result.StackType)
	} else {
		fmt.Println("❌ 本机未检测到公网连接")
	}
}
