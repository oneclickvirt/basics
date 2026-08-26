package resolver

import "testing"

func TestEmbeddedCatalogContainsBootstrappedDoHAndDoT(t *testing.T) {
	endpoints := DefaultEndpoints()
	bootstrap := DefaultBootstrapHosts()
	if len(endpoints) == 0 {
		t.Fatal("embedded resolver catalog is empty")
	}
	var hasDoH, hasDoT bool
	for _, endpoint := range endpoints {
		parsed, transport, err := parseEndpoint(endpoint)
		if err != nil {
			t.Fatalf("endpoint %q is invalid: %v", endpoint.URL, err)
		}
		if transport != TransportDoH && transport != TransportDoT {
			t.Fatalf("embedded endpoint %q is not encrypted", endpoint.URL)
		}
		if len(bootstrap[parsed.Hostname()]) == 0 {
			t.Fatalf("endpoint %q has no fixed bootstrap address", endpoint.URL)
		}
		hasDoH = hasDoH || transport == TransportDoH
		hasDoT = hasDoT || transport == TransportDoT
	}
	if !hasDoH || !hasDoT {
		t.Fatalf("embedded transports: DoH=%t DoT=%t", hasDoH, hasDoT)
	}
}

func TestFallbackCatalogRetainsValidated360DoT(t *testing.T) {
	manifest := fallbackEndpointManifest()
	for _, endpoint := range manifest.Endpoints {
		if endpoint.Name == "360 Public DNS" && endpoint.URL == "tls://dot.360.cn:853" {
			if len(endpoint.Addresses) == 0 {
				t.Fatal("fallback 360 DoT endpoint has no bootstrap addresses")
			}
			return
		}
	}
	t.Fatal("fallback catalog is missing validated 360 DoT endpoint")
}
