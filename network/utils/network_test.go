package utils

import "testing"

func TestNormalizeNetwork(t *testing.T) {
	cases := map[string]string{
		"": "", "auto": "", "tcp": "", "tcp4": "tcp4", "ipv4": "tcp4", "4": "tcp4",
		"tcp6": "tcp6", "ipv6": "tcp6", "6": "tcp6",
	}
	for input, want := range cases {
		got, err := NormalizeNetwork(input)
		if err != nil || got != want {
			t.Fatalf("NormalizeNetwork(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := NormalizeNetwork("udp4"); err == nil {
		t.Fatal("invalid network was accepted")
	}
}
