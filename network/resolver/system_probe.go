package resolver

import (
	"context"
	"errors"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
)

const systemDNSInconclusiveReason = "system DNS probe inconclusive; preserving system resolver"

var defaultSystemProbeHosts = []string{
	"example.com",
	"cloudflare.com",
	"google.com",
}

// systemResolverConfigMissingFn is kept as a variable so resolver tests can
// exercise probe classifications without depending on the host's DNS setup.
var systemResolverConfigMissingFn = unixResolverConfigMissing

type systemProbeResult uint8

const (
	systemProbeAvailable systemProbeResult = iota
	systemProbeUnavailable
	systemProbeInconclusive
)

func (resolver *Resolver) probeSystem(ctx context.Context) systemProbeResult {
	if resolver == nil || resolver.system == nil {
		return systemProbeUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, resolver.config.ProbeTimeout)
	defer cancel()

	type result struct {
		addresses []net.IP
		err       error
	}
	hosts := resolver.systemProbeHosts()
	results := make(chan result, len(hosts))
	var wg sync.WaitGroup
	for _, host := range hosts {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			addresses, err := resolver.system.LookupIP(probeCtx, "ip", host)
			results <- result{addresses: addresses, err: err}
		}(host)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	hardFailures := 0
	inconclusiveFailures := 0
	for outcome := range results {
		if outcome.err == nil && len(outcome.addresses) > 0 {
			return systemProbeAvailable
		}
		if isDNSResponse(outcome.err) {
			// NXDOMAIN or a missing RR still proves that the configured resolver
			// responded. Auto mode must not silently change a filtered policy.
			return systemProbeAvailable
		}
		if isConfirmedLocalDNSFailure(outcome.err) {
			hardFailures++
		} else {
			inconclusiveFailures++
		}
	}
	if len(hosts) > 0 && hardFailures == len(hosts) {
		return systemProbeUnavailable
	}
	if systemResolverConfigMissingFn() && hardFailures+inconclusiveFailures == len(hosts) {
		return systemProbeUnavailable
	}
	return systemProbeInconclusive
}

func (resolver *Resolver) systemProbeHosts() []string {
	values := append([]string(nil), resolver.config.SystemProbeHosts...)
	if host := strings.TrimSpace(resolver.config.ProbeHost); host != "" {
		values = append([]string{host}, values...)
	}
	values = append(values, defaultSystemProbeHosts...)
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, host := range values {
		host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		result = append(result, host)
	}
	return result
}

func isDNSResponse(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

func isConfirmedLocalDNSFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && (dnsErr.IsNotFound || dnsErr.IsTimeout) {
		return false
	}
	message := strings.ToLower(err.Error())
	if dnsErr != nil {
		message = strings.ToLower(dnsErr.Err + " " + message)
	}
	return isLoopbackResolverRefusal(message) ||
		strings.Contains(message, "no dns servers") ||
		strings.Contains(message, "resolver unavailable") ||
		strings.Contains(message, "error reading dns config")
}

// isLoopbackResolverRefusal accepts only errors that identify a local DNS
// stub. A refusal from a remote resolver can also be caused by packet loss,
// filtering, or a transient upstream condition, so auto mode must retain the
// system resolver in that case.
func isLoopbackResolverRefusal(message string) bool {
	if !strings.Contains(message, "connection refused") {
		return false
	}
	for _, candidate := range strings.FieldsFunc(message, func(r rune) bool {
		return !((r >= '0' && r <= '9') ||
			(r >= 'a' && r <= 'f') ||
			(r >= 'A' && r <= 'F') ||
			r == '.' || r == ':' || r == '[' || r == ']')
	}) {
		candidate = strings.TrimRight(candidate, ":")
		if host, port, err := net.SplitHostPort(candidate); err == nil && port != "" {
			candidate = host
		}
		candidate = strings.Trim(candidate, "[]")
		if address := net.ParseIP(candidate); address != nil && address.IsLoopback() {
			return true
		}
	}
	return false
}

func unixResolverConfigMissing() bool {
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
	default:
		return false
	}
	contents, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return true
	}
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.EqualFold(fields[0], "nameserver") && net.ParseIP(fields[1]) != nil {
			return false
		}
	}
	return true
}
