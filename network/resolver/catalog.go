package resolver

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
)

// Transport is the protocol used between the in-process resolver and a public
// upstream. Automatic fallback only ships encrypted DoH and DoT endpoints;
// UDP and TCP remain available for explicit embedding configurations.
type Transport string

const (
	TransportDoH Transport = "doh"
	TransportDoT Transport = "dot"
	TransportUDP Transport = "udp"
	TransportTCP Transport = "tcp"
)

type endpointManifest struct {
	SchemaVersion string                  `json:"schema_version"`
	GeneratedAt   string                  `json:"generated_at"`
	Endpoints     []endpointManifestEntry `json:"endpoints"`
}

type endpointManifestEntry struct {
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Addresses []string `json:"addresses"`
}

//go:embed endpoints_embed.json
var embeddedEndpointManifest []byte

var defaultCatalog struct {
	sync.Once
	endpoints []Endpoint
	bootstrap map[string][]string
}

// DefaultEndpoints returns a copy of the vetted built-in encrypted endpoint
// list. The generated manifest contains only unfiltered public resolvers so a
// fallback does not silently impose family, advertising, or threat filtering.
func DefaultEndpoints() []Endpoint {
	loadDefaultCatalog()
	return append([]Endpoint(nil), defaultCatalog.endpoints...)
}

// DefaultBootstrapHosts returns endpoint hostname to fixed-address mappings.
// They bootstrap only resolver upstreams; requested test hostnames always use
// DNS answers returned by the selected upstream.
func DefaultBootstrapHosts() map[string][]string {
	loadDefaultCatalog()
	result := make(map[string][]string, len(defaultCatalog.bootstrap))
	for host, addresses := range defaultCatalog.bootstrap {
		result[host] = append([]string(nil), addresses...)
	}
	return result
}

func loadDefaultCatalog() {
	defaultCatalog.Do(func() {
		manifest := endpointManifest{}
		if err := json.Unmarshal(embeddedEndpointManifest, &manifest); err != nil {
			manifest = fallbackEndpointManifest()
		}
		endpoints, bootstrap := manifestCatalog(manifest)
		if len(endpoints) == 0 {
			endpoints, bootstrap = manifestCatalog(fallbackEndpointManifest())
		}
		defaultCatalog.endpoints = endpoints
		defaultCatalog.bootstrap = bootstrap
	})
}

func manifestCatalog(manifest endpointManifest) ([]Endpoint, map[string][]string) {
	endpoints := make([]Endpoint, 0, len(manifest.Endpoints))
	bootstrap := make(map[string][]string)
	for _, entry := range manifest.Endpoints {
		endpoint := Endpoint{Name: strings.TrimSpace(entry.Name), URL: strings.TrimSpace(entry.URL)}
		parsed, transport, err := parseEndpoint(endpoint)
		if err != nil || (transport != TransportDoH && transport != TransportDoT) {
			continue
		}
		addresses := cleanAddresses(entry.Addresses)
		if len(addresses) == 0 {
			continue
		}
		host := strings.ToLower(parsed.Hostname())
		if endpoint.Name == "" {
			endpoint.Name = host
		}
		endpoints = append(endpoints, endpoint)
		bootstrap[host] = append(bootstrap[host], addresses...)
	}
	for host, addresses := range bootstrap {
		bootstrap[host] = cleanAddresses(addresses)
	}
	return endpoints, bootstrap
}

func cleanAddresses(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		if address := net.ParseIP(strings.TrimSpace(value)); address != nil {
			unique[address.String()] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := net.ParseIP(result[i]), net.ParseIP(result[j])
		if left.To4() != nil && right.To4() == nil {
			return true
		}
		if left.To4() == nil && right.To4() != nil {
			return false
		}
		return result[i] < result[j]
	})
	return result
}

func parseEndpoint(endpoint Endpoint) (*url.URL, Transport, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint.URL))
	if err != nil || parsed.Hostname() == "" {
		return nil, "", fmt.Errorf("invalid resolver endpoint %q", endpoint.URL)
	}
	transport := Transport("")
	switch strings.ToLower(parsed.Scheme) {
	case "https", "http":
		transport = TransportDoH
	case "tls":
		transport = TransportDoT
	case "udp":
		transport = TransportUDP
	case "tcp":
		transport = TransportTCP
	default:
		return nil, "", fmt.Errorf("unsupported resolver endpoint scheme %q", parsed.Scheme)
	}
	return parsed, transport, nil
}

func endpointPort(parsed *url.URL, transport Transport) string {
	if parsed == nil {
		return ""
	}
	if port := parsed.Port(); port != "" {
		return port
	}
	switch transport {
	case TransportDoH:
		if strings.EqualFold(parsed.Scheme, "http") {
			return "80"
		}
		return "443"
	case TransportDoT:
		return "853"
	default:
		return "53"
	}
}

func endpointsForMode(endpoints []Endpoint, mode Mode) []Endpoint {
	filtered := make([]Endpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		_, transport, err := parseEndpoint(endpoint)
		if err != nil {
			continue
		}
		switch mode {
		case ModeDoH:
			if transport != TransportDoH {
				continue
			}
		case ModeDoT:
			if transport != TransportDoT {
				continue
			}
		case ModeAuto:
			if transport != TransportDoH && transport != TransportDoT {
				continue
			}
		default:
			continue
		}
		filtered = append(filtered, endpoint)
	}
	return filtered
}

func fallbackEndpointManifest() endpointManifest {
	return endpointManifest{Endpoints: []endpointManifestEntry{
		{Name: "AliDNS", URL: "https://dns.alidns.com/dns-query", Addresses: []string{"223.5.5.5", "223.6.6.6"}},
		{Name: "AliDNS", URL: "tls://dns.alidns.com:853", Addresses: []string{"223.5.5.5", "223.6.6.6"}},
		{Name: "DNSPod", URL: "https://doh.pub/dns-query", Addresses: []string{"1.12.12.12", "120.53.53.53"}},
		{Name: "DNSPod", URL: "tls://dot.pub:853", Addresses: []string{"1.12.12.12", "120.53.53.53"}},
		{Name: "360 Public DNS", URL: "https://doh.360.cn/dns-query", Addresses: []string{"101.199.254.118", "112.65.69.15", "123.6.48.18"}},
		{Name: "360 Public DNS", URL: "tls://dot.360.cn:853", Addresses: []string{"101.199.254.118", "112.65.69.15", "123.6.48.18"}},
		{Name: "Cloudflare", URL: "https://cloudflare-dns.com/dns-query", Addresses: []string{"1.1.1.1", "1.0.0.1"}},
		{Name: "Cloudflare", URL: "tls://cloudflare-dns.com:853", Addresses: []string{"1.1.1.1", "1.0.0.1"}},
		{Name: "Google", URL: "https://dns.google/dns-query", Addresses: []string{"8.8.8.8", "8.8.4.4"}},
		{Name: "Google", URL: "tls://dns.google:853", Addresses: []string{"8.8.8.8", "8.8.4.4"}},
		{Name: "Quad9 Unsecured", URL: "https://dns10.quad9.net/dns-query", Addresses: []string{"9.9.9.10", "149.112.112.10"}},
		{Name: "Quad9 Unsecured", URL: "tls://dns10.quad9.net:853", Addresses: []string{"9.9.9.10", "149.112.112.10"}},
		{Name: "AdGuard Unfiltered", URL: "https://unfiltered.adguard-dns.com/dns-query", Addresses: []string{"94.140.14.140", "94.140.14.141"}},
		{Name: "AdGuard Unfiltered", URL: "tls://unfiltered.adguard-dns.com:853", Addresses: []string{"94.140.14.140", "94.140.14.141"}},
	}}
}
