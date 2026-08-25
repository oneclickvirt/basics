// Command update-resolver-endpoints validates the reviewed resolver candidates
// through fixed addresses and refreshes the embedded fallback catalog.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/oneclickvirt/basics/network/resolver"
)

const (
	sourceSchemaVersion    = "goecs.resolver-endpoint-sources/v1"
	generatedSchemaVersion = "goecs.resolver-endpoints/v1"
)

type sourceManifest struct {
	SchemaVersion string           `json:"schema_version"`
	Endpoints     []sourceEndpoint `json:"endpoints"`
}

type sourceEndpoint struct {
	Name               string   `json:"name"`
	URL                string   `json:"url"`
	BootstrapAddresses []string `json:"bootstrap_addresses"`
	Optional           bool     `json:"optional"`
}

type generatedManifest struct {
	SchemaVersion string              `json:"schema_version"`
	GeneratedAt   string              `json:"generated_at"`
	Endpoints     []generatedEndpoint `json:"endpoints"`
}

type generatedEndpoint struct {
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Addresses []string `json:"addresses"`
}

type probeResult struct {
	address    string
	discovered []string
	err        error
}

var validateAddressFn = validateAddress

func main() {
	input := flag.String("input", "network/resolver/endpoints_sources.json", "reviewed candidate source file")
	output := flag.String("output", "network/resolver/endpoints_embed.json", "generated embedded endpoint file")
	timeout := flag.Duration("timeout", 3*time.Second, "per-address validation timeout")
	concurrency := flag.Int("concurrency", 6, "maximum simultaneous endpoint validations")
	check := flag.Bool("check", false, "fail instead of writing when the generated endpoint list changed")
	flag.Parse()

	if err := run(context.Background(), *input, *output, *timeout, *concurrency, *check); err != nil {
		fmt.Fprintln(os.Stderr, "update resolver endpoints:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, input, output string, timeout time.Duration, concurrency int, check bool) error {
	if timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if concurrency < 1 {
		return errors.New("concurrency must be at least one")
	}
	sources, err := readSourceManifest(input)
	if err != nil {
		return err
	}
	current, err := readGeneratedManifest(output)
	if err != nil {
		return err
	}
	refreshed, warnings, err := refreshManifest(ctx, sources, current, timeout, concurrency)
	if err != nil {
		return err
	}
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", warning)
	}
	if semanticEqual(current, refreshed) {
		fmt.Println("embedded resolver endpoint list is current")
		return nil
	}
	if check {
		return errors.New("embedded resolver endpoint list needs refresh")
	}
	refreshed.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	encoded, err := json.MarshalIndent(refreshed, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(output, append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("updated embedded resolver endpoint list with %d endpoints\n", len(refreshed.Endpoints))
	return nil
}

func readSourceManifest(path string) (sourceManifest, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return sourceManifest{}, err
	}
	manifest := sourceManifest{}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return sourceManifest{}, err
	}
	if manifest.SchemaVersion != sourceSchemaVersion {
		return sourceManifest{}, fmt.Errorf("unsupported source schema %q", manifest.SchemaVersion)
	}
	if len(manifest.Endpoints) == 0 {
		return sourceManifest{}, errors.New("source manifest has no endpoints")
	}
	seen := make(map[string]struct{}, len(manifest.Endpoints))
	for index := range manifest.Endpoints {
		endpoint, err := normalizeSourceEndpoint(manifest.Endpoints[index])
		if err != nil {
			return sourceManifest{}, err
		}
		key := endpointKey(endpoint.Name, endpoint.URL)
		if _, exists := seen[key]; exists {
			return sourceManifest{}, fmt.Errorf("duplicate source endpoint %q", endpoint.URL)
		}
		seen[key] = struct{}{}
		manifest.Endpoints[index] = endpoint
	}
	return manifest, nil
}

func readGeneratedManifest(path string) (generatedManifest, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return generatedManifest{SchemaVersion: generatedSchemaVersion}, nil
	}
	if err != nil {
		return generatedManifest{}, err
	}
	manifest := generatedManifest{}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return generatedManifest{}, err
	}
	if manifest.SchemaVersion != generatedSchemaVersion {
		return generatedManifest{}, fmt.Errorf("unsupported generated schema %q", manifest.SchemaVersion)
	}
	for index := range manifest.Endpoints {
		endpoint := &manifest.Endpoints[index]
		endpoint.Name = strings.TrimSpace(endpoint.Name)
		endpoint.URL = strings.TrimSpace(endpoint.URL)
		endpoint.Addresses = cleanAddresses(endpoint.Addresses)
	}
	return manifest, nil
}

func normalizeSourceEndpoint(endpoint sourceEndpoint) (sourceEndpoint, error) {
	endpoint.Name = strings.TrimSpace(endpoint.Name)
	endpoint.URL = strings.TrimSpace(endpoint.URL)
	parsed, mode, err := endpointMode(endpoint.URL)
	if err != nil {
		return sourceEndpoint{}, err
	}
	if mode != resolver.ModeDoH && mode != resolver.ModeDoT {
		return sourceEndpoint{}, fmt.Errorf("endpoint %q is not encrypted", endpoint.URL)
	}
	if endpoint.Name == "" {
		endpoint.Name = parsed.Hostname()
	}
	endpoint.BootstrapAddresses = cleanAddresses(endpoint.BootstrapAddresses)
	if len(endpoint.BootstrapAddresses) == 0 {
		return sourceEndpoint{}, fmt.Errorf("endpoint %q has no bootstrap addresses", endpoint.URL)
	}
	return endpoint, nil
}

func refreshManifest(ctx context.Context, sources sourceManifest, current generatedManifest, timeout time.Duration, concurrency int) (generatedManifest, []string, error) {
	currentByKey := make(map[string]generatedEndpoint, len(current.Endpoints))
	for _, endpoint := range current.Endpoints {
		currentByKey[endpointKey(endpoint.Name, endpoint.URL)] = endpoint
	}
	refreshed := generatedManifest{SchemaVersion: generatedSchemaVersion, GeneratedAt: current.GeneratedAt}
	warnings := make([]string, 0)
	for _, source := range sources.Endpoints {
		key := endpointKey(source.Name, source.URL)
		previous := currentByKey[key]
		candidates := append(append([]string(nil), source.BootstrapAddresses...), previous.Addresses...)
		accepted, discovered := validateAddresses(ctx, source, candidates, timeout, concurrency)
		if len(discovered) > 0 {
			extraAccepted, _ := validateAddresses(ctx, source, discovered, timeout, concurrency)
			accepted = append(accepted, extraAccepted...)
		}
		accepted = cleanAddresses(accepted)
		if len(accepted) == 0 {
			if len(previous.Addresses) == 0 {
				if source.Optional {
					warnings = append(warnings, fmt.Sprintf("skipping optional endpoint %s after no address validated", source.URL))
					continue
				}
				return generatedManifest{}, warnings, fmt.Errorf("no validated address for new endpoint %s", source.URL)
			}
			accepted = append(accepted, source.BootstrapAddresses...)
			accepted = append(accepted, previous.Addresses...)
			accepted = cleanAddresses(accepted)
			warnings = append(warnings, fmt.Sprintf("preserving previous addresses for %s after an inconclusive refresh", source.URL))
		} else {
			// A partial refresh is not enough evidence to remove a reviewed or
			// previously validated address. For example, GitHub's IPv4-only
			// runners cannot validate IPv6 even when it is useful to releases on
			// dual-stack hosts. Keep known bootstrap addresses and add newly
			// validated discoveries; a brand-new endpoint still requires a real
			// successful TLS/DNS validation before it can enter the catalog.
			accepted = append(accepted, source.BootstrapAddresses...)
			accepted = append(accepted, previous.Addresses...)
			accepted = cleanAddresses(accepted)
		}
		refreshed.Endpoints = append(refreshed.Endpoints, generatedEndpoint{Name: source.Name, URL: source.URL, Addresses: accepted})
	}
	if !hasEncryptedTransport(refreshed.Endpoints, resolver.ModeDoH) || !hasEncryptedTransport(refreshed.Endpoints, resolver.ModeDoT) {
		return generatedManifest{}, warnings, errors.New("refreshed catalog must contain both DoH and DoT endpoints")
	}
	return refreshed, warnings, nil
}

func validateAddresses(ctx context.Context, source sourceEndpoint, candidates []string, timeout time.Duration, concurrency int) ([]string, []string) {
	candidates = cleanAddresses(candidates)
	if len(candidates) == 0 {
		return nil, nil
	}
	if concurrency > len(candidates) {
		concurrency = len(candidates)
	}
	jobs := make(chan string)
	results := make(chan probeResult, len(candidates))
	var workers sync.WaitGroup
	for range concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for address := range jobs {
				discovered, err := validateAddressFn(ctx, source, address, timeout)
				results <- probeResult{address: address, discovered: discovered, err: err}
			}
		}()
	}
	go func() {
		for _, address := range candidates {
			jobs <- address
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()
	accepted := make([]string, 0, len(candidates))
	discovered := make([]string, 0)
	for result := range results {
		if result.err != nil {
			continue
		}
		accepted = append(accepted, result.address)
		discovered = append(discovered, result.discovered...)
	}
	return cleanAddresses(accepted), cleanAddresses(discovered)
}

func validateAddress(parent context.Context, source sourceEndpoint, address string, timeout time.Duration) ([]string, error) {
	parsed, mode, err := endpointMode(source.URL)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	endpointResolver, err := resolver.New(resolver.Config{
		Mode:           mode,
		Endpoints:      []resolver.Endpoint{{Name: source.Name, URL: source.URL}},
		BootstrapHosts: map[string][]string{parsed.Hostname(): {address}},
		ProbeTimeout:   timeout,
		QueryTimeout:   timeout,
	})
	if err != nil {
		return nil, err
	}
	defer endpointResolver.Close()
	status := endpointResolver.Activate(ctx)
	if status.Active != mode {
		return nil, fmt.Errorf("endpoint did not validate as %s", mode)
	}
	addresses, err := endpointResolver.StdlibResolver().LookupIP(ctx, "ip", parsed.Hostname())
	if err != nil {
		return nil, nil
	}
	result := make([]string, 0, len(addresses))
	for _, resolved := range addresses {
		result = append(result, resolved.String())
	}
	return cleanAddresses(result), nil
}

func endpointMode(rawURL string) (*url.URL, resolver.Mode, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, "", fmt.Errorf("invalid endpoint URL %q", rawURL)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return parsed, resolver.ModeDoH, nil
	case "tls":
		return parsed, resolver.ModeDoT, nil
	default:
		return nil, "", fmt.Errorf("unsupported encrypted endpoint URL %q", rawURL)
	}
}

func hasEncryptedTransport(endpoints []generatedEndpoint, wanted resolver.Mode) bool {
	for _, endpoint := range endpoints {
		_, mode, err := endpointMode(endpoint.URL)
		if err == nil && mode == wanted {
			return true
		}
	}
	return false
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
	sort.Slice(result, func(left, right int) bool {
		leftIP, rightIP := net.ParseIP(result[left]), net.ParseIP(result[right])
		if leftIP.To4() != nil && rightIP.To4() == nil {
			return true
		}
		if leftIP.To4() == nil && rightIP.To4() != nil {
			return false
		}
		return result[left] < result[right]
	})
	return result
}

func endpointKey(name, rawURL string) string {
	return strings.TrimSpace(name) + "\x00" + strings.TrimSpace(rawURL)
}

func semanticEqual(left, right generatedManifest) bool {
	if len(left.Endpoints) != len(right.Endpoints) {
		return false
	}
	for index := range left.Endpoints {
		leftEndpoint, rightEndpoint := left.Endpoints[index], right.Endpoints[index]
		if leftEndpoint.Name != rightEndpoint.Name || leftEndpoint.URL != rightEndpoint.URL || len(leftEndpoint.Addresses) != len(rightEndpoint.Addresses) {
			return false
		}
		for addressIndex := range leftEndpoint.Addresses {
			if leftEndpoint.Addresses[addressIndex] != rightEndpoint.Addresses[addressIndex] {
				return false
			}
		}
	}
	return true
}
