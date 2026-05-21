package network

import (
	"errors"
	"testing"

	"github.com/oneclickvirt/basics/model"
)

func TestNetworkCheckIPv6UsesSecondResult(t *testing.T) {
	old := runIpCheck
	t.Cleanup(func() { runIpCheck = old })

	runIpCheck = func(checkType string) (*model.IpInfo, *model.IpInfo, error) {
		if checkType != "ipv6" {
			t.Fatalf("unexpected checkType: %s", checkType)
		}
		return nil, &model.IpInfo{Ip: "2001:db8::1"}, nil
	}

	ipv4, ipv6, _, _, err := NetworkCheck("ipv6", false, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ipv4 != "" {
		t.Fatalf("expected empty ipv4, got %s", ipv4)
	}
	if ipv6 != "2001:db8::1" {
		t.Fatalf("expected ipv6 to be second return value, got %s", ipv6)
	}
}

func TestNetworkCheckKeepsRunningWhenRunIpCheckErrors(t *testing.T) {
	old := runIpCheck
	t.Cleanup(func() { runIpCheck = old })

	runIpCheck = func(checkType string) (*model.IpInfo, *model.IpInfo, error) {
		return nil, nil, errors.New("upstream failed")
	}

	ipv4, ipv6, ipInfo, _, err := NetworkCheck("both", false, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ipv4 != "" || ipv6 != "" || ipInfo != "" {
		t.Fatalf("expected empty result when all providers fail, got ipv4=%q ipv6=%q ipInfo=%q", ipv4, ipv6, ipInfo)
	}
}
