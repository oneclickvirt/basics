package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestRepositorySourceManifestContainsBothEncryptedTransports(t *testing.T) {
	manifest, err := readSourceManifest(filepath.FromSlash("../../network/resolver/endpoints_sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	var hasDoH, hasDoT bool
	for _, endpoint := range manifest.Endpoints {
		_, mode, err := endpointMode(endpoint.URL)
		if err != nil {
			t.Fatal(err)
		}
		hasDoH = hasDoH || mode == "doh"
		hasDoT = hasDoT || mode == "dot"
	}
	if !hasDoH || !hasDoT {
		t.Fatalf("source transports: DoH=%t DoT=%t", hasDoH, hasDoT)
	}
}

func TestRefreshManifestValidatesDiscoveredAddresses(t *testing.T) {
	original := validateAddressFn
	t.Cleanup(func() { validateAddressFn = original })
	validateAddressFn = func(_ context.Context, _ sourceEndpoint, address string, _ time.Duration) ([]string, error) {
		switch address {
		case "203.0.113.1":
			return []string{"203.0.113.2"}, nil
		case "203.0.113.2":
			return nil, nil
		case "2001:db8::1":
			return []string{"2001:db8::2"}, nil
		case "2001:db8::2":
			return nil, nil
		default:
			return nil, errors.New("unexpected address")
		}
	}
	sources := sourceManifest{SchemaVersion: sourceSchemaVersion, Endpoints: []sourceEndpoint{
		{Name: "DoH", URL: "https://doh.test/dns-query", BootstrapAddresses: []string{"203.0.113.1"}},
		{Name: "DoT", URL: "tls://dot.test:853", BootstrapAddresses: []string{"2001:db8::1"}},
	}}
	refreshed, warnings, err := refreshManifest(context.Background(), sources, generatedManifest{SchemaVersion: generatedSchemaVersion}, time.Second, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(refreshed.Endpoints) != 2 {
		t.Fatalf("refresh = %#v, warnings = %v", refreshed, warnings)
	}
	if got := refreshed.Endpoints[0].Addresses; len(got) != 2 || got[0] != "203.0.113.1" || got[1] != "203.0.113.2" {
		t.Fatalf("DoH addresses = %v", got)
	}
	if got := refreshed.Endpoints[1].Addresses; len(got) != 2 || got[0] != "2001:db8::1" || got[1] != "2001:db8::2" {
		t.Fatalf("DoT addresses = %v", got)
	}
}

func TestRefreshManifestPreservesPreviousOptionalEndpoint(t *testing.T) {
	original := validateAddressFn
	t.Cleanup(func() { validateAddressFn = original })
	validateAddressFn = func(context.Context, sourceEndpoint, string, time.Duration) ([]string, error) {
		return nil, errors.New("temporary network failure")
	}
	sources := sourceManifest{SchemaVersion: sourceSchemaVersion, Endpoints: []sourceEndpoint{
		{Name: "DoH", URL: "https://doh.test/dns-query", BootstrapAddresses: []string{"203.0.113.1"}, Optional: true},
		{Name: "DoT", URL: "tls://dot.test:853", BootstrapAddresses: []string{"203.0.113.2"}, Optional: true},
	}}
	current := generatedManifest{SchemaVersion: generatedSchemaVersion, Endpoints: []generatedEndpoint{
		{Name: "DoH", URL: "https://doh.test/dns-query", Addresses: []string{"203.0.113.10"}},
		{Name: "DoT", URL: "tls://dot.test:853", Addresses: []string{"203.0.113.11"}},
	}}
	refreshed, warnings, err := refreshManifest(context.Background(), sources, current, time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !semanticEqual(current, refreshed) || len(warnings) != 2 {
		t.Fatalf("refresh = %#v, warnings = %v", refreshed, warnings)
	}
}
