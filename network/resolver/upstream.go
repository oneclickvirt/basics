package resolver

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type endpointProbe struct {
	endpoint  Endpoint
	transport Transport
	latency   time.Duration
	err       error
}

var dotTLSConfig = func(serverName string) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}
}

func (resolver *Resolver) rankEncryptedEndpoints(ctx context.Context) ([]Endpoint, string, Transport, error) {
	if resolver == nil || len(resolver.config.Endpoints) == 0 {
		return nil, "", "", errors.New("no resolver endpoints configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	request := new(dns.Msg)
	request.SetQuestion(dns.Fqdn(resolver.config.ProbeHost), dns.TypeA)
	request.RecursionDesired = true
	probes := make(chan endpointProbe, len(resolver.config.Endpoints))
	for _, endpoint := range resolver.config.Endpoints {
		go func(endpoint Endpoint) {
			_, transport, err := parseEndpoint(endpoint)
			if err != nil || (transport != TransportDoH && transport != TransportDoT) {
				if err == nil {
					err = fmt.Errorf("endpoint %s is not encrypted", endpoint.Name)
				}
				probes <- endpointProbe{endpoint: endpoint, err: err}
				return
			}
			started := time.Now()
			response, err := resolver.queryEndpoint(ctx, endpoint, request.Copy())
			if err == nil {
				err = validProbeResponse(response)
			}
			probes <- endpointProbe{endpoint: endpoint, transport: transport, latency: time.Since(started), err: err}
		}(endpoint)
	}

	successes := make([]endpointProbe, 0, len(resolver.config.Endpoints))
	failed := make(map[string]struct{}, len(resolver.config.Endpoints))
	var errs []error
	for range resolver.config.Endpoints {
		probe := <-probes
		if probe.err == nil {
			successes = append(successes, probe)
			continue
		}
		failed[endpointKey(probe.endpoint)] = struct{}{}
		errs = append(errs, fmt.Errorf("%s: %w", probe.endpoint.Name, probe.err))
	}
	if len(successes) == 0 {
		return append([]Endpoint(nil), resolver.config.Endpoints...), "", "", errors.Join(errs...)
	}
	sort.SliceStable(successes, func(i, j int) bool {
		return successes[i].latency < successes[j].latency
	})
	ranked := make([]Endpoint, 0, len(resolver.config.Endpoints))
	for _, probe := range successes {
		ranked = append(ranked, probe.endpoint)
	}
	for _, endpoint := range resolver.config.Endpoints {
		if _, failed := failed[endpointKey(endpoint)]; failed {
			ranked = append(ranked, endpoint)
		}
	}
	fastest := successes[0]
	return ranked, fastest.endpoint.Name, fastest.transport, nil
}

func validProbeResponse(response *dns.Msg) error {
	if response == nil {
		return errors.New("empty DNS response")
	}
	if !response.Response {
		return errors.New("upstream returned a DNS query")
	}
	if response.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("upstream returned DNS rcode %d", response.Rcode)
	}
	if len(response.Answer) == 0 {
		return errors.New("upstream returned no probe answer")
	}
	return nil
}

func endpointKey(endpoint Endpoint) string {
	return endpoint.Name + "\x00" + endpoint.URL
}

func (resolver *Resolver) queryUpstream(ctx context.Context, request *dns.Msg) (*dns.Msg, string, Transport, error) {
	if request == nil {
		return nil, "", "", errors.New("empty DNS request")
	}
	var errs []error
	for index, endpoint := range resolver.config.Endpoints {
		endpointCtx, cancel := resolver.endpointContext(ctx, len(resolver.config.Endpoints)-index)
		response, err := resolver.queryEndpoint(endpointCtx, endpoint, request)
		cancel()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", endpoint.Name, err))
			continue
		}
		if response == nil || !response.Response {
			errs = append(errs, fmt.Errorf("%s: invalid DNS response", endpoint.Name))
			continue
		}
		_, transport, _ := parseEndpoint(endpoint)
		return response, endpoint.Name, transport, nil
	}
	return nil, "", "", errors.Join(errs...)
}

func (resolver *Resolver) queryEndpoint(ctx context.Context, endpoint Endpoint, request *dns.Msg) (*dns.Msg, error) {
	_, transport, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	switch transport {
	case TransportDoH:
		return resolver.queryDoHEndpoint(ctx, endpoint, request)
	case TransportDoT, TransportUDP, TransportTCP:
		return resolver.queryWireEndpoint(ctx, endpoint, transport, request)
	default:
		return nil, fmt.Errorf("unsupported resolver transport %q", transport)
	}
}

func (resolver *Resolver) queryDoHEndpoint(ctx context.Context, endpoint Endpoint, request *dns.Msg) (*dns.Msg, error) {
	wire, err := request.Pack()
	if err != nil {
		return nil, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(wire))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Accept", "application/dns-message")
	httpRequest.Header.Set("Content-Type", "application/dns-message")
	response, err := resolver.dohClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("DoH endpoint %s returned HTTP %d", endpoint.Name, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxDoHResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxDoHResponseBytes {
		return nil, fmt.Errorf("DoH endpoint %s returned an oversized response", endpoint.Name)
	}
	message := new(dns.Msg)
	if err := message.Unpack(body); err != nil {
		return nil, err
	}
	if !message.Response {
		return nil, fmt.Errorf("DoH endpoint %s returned a DNS query", endpoint.Name)
	}
	return message, nil
}

func (resolver *Resolver) queryWireEndpoint(ctx context.Context, endpoint Endpoint, transport Transport, request *dns.Msg) (*dns.Msg, error) {
	parsed, _, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	addresses := resolver.bootstrapAddresses(host)
	if len(addresses) == 0 {
		return nil, fmt.Errorf("no bootstrap address for resolver endpoint %q", host)
	}
	port := endpointPort(parsed, transport)
	var errs []error
	for _, address := range addresses {
		ip := net.ParseIP(address)
		if ip == nil {
			continue
		}
		// dns.Client otherwise constructs a net.Dialer that reads
		// net.DefaultResolver, even for a literal bootstrap address. Bind it to
		// the resolver captured before Configure installs the fallback so an
		// in-flight query cannot race a later process-resolver swap.
		client := &dns.Client{Dialer: &net.Dialer{Resolver: resolver.system}}
		switch transport {
		case TransportDoT:
			client.Net = "tcp-tls"
			client.TLSConfig = dotTLSConfig(host)
		case TransportUDP:
			client.Net = "udp"
		case TransportTCP:
			client.Net = "tcp"
		}
		response, _, queryErr := client.ExchangeContext(ctx, request.Copy(), net.JoinHostPort(ip.String(), port))
		if queryErr == nil {
			resolver.markBootstrapStack(ip)
			return response, nil
		}
		errs = append(errs, queryErr)
	}
	if len(errs) == 0 {
		return nil, fmt.Errorf("no valid bootstrap address for resolver endpoint %q", host)
	}
	return nil, errors.Join(errs...)
}

func (resolver *Resolver) bootstrapAddresses(host string) []string {
	host = strings.ToLower(strings.TrimSuffix(strings.Trim(host, "[]"), "."))
	if literal := net.ParseIP(host); literal != nil {
		return []string{literal.String()}
	}
	return append([]string(nil), resolver.config.BootstrapHosts[host]...)
}
