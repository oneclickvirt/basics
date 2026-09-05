package resolver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestConfigureFallsBackToDoHForDefaultHTTPDialer(t *testing.T) {
	Shutdown()
	t.Cleanup(Shutdown)
	previous := net.DefaultResolver
	const dohBootstrapHost = "doh-bootstrap.test"

	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Host == "" {
			t.Fatal("request host was lost")
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	targetHost, targetPort, err := net.SplitHostPort(target.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	doh := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/dns-message" {
			t.Fatalf("unexpected DoH request: method=%s content-type=%q", request.Method, request.Header.Get("Content-Type"))
		}
		query := new(dns.Msg)
		if err := query.Unpack(readRequestBody(t, request)); err != nil {
			t.Fatal(err)
		}
		response := new(dns.Msg)
		response.SetReply(query)
		for _, question := range query.Question {
			switch question.Qtype {
			case dns.TypeA:
				response.Answer = append(response.Answer, &dns.A{Hdr: dns.RR_Header{Name: question.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP(targetHost)})
			case dns.TypeAAAA:
				response.Answer = append(response.Answer, &dns.AAAA{Hdr: dns.RR_Header{Name: question.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60}, AAAA: net.ParseIP(targetHost)})
			}
		}
		payload, err := response.Pack()
		if err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Content-Type", "application/dns-message")
		_, _ = writer.Write(payload)
	}))
	defer doh.Close()
	dohHost, dohPort, err := net.SplitHostPort(doh.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	missingSystemResolver := &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("system resolver unavailable")
	}}
	status := Configure(context.Background(), Config{
		Mode:           ModeAuto,
		Endpoints:      []Endpoint{{Name: "test", URL: "http://" + dohBootstrapHost + ":" + dohPort + "/dns-query"}},
		BootstrapHosts: map[string][]string{dohBootstrapHost: {dohHost}},
		SystemResolver: missingSystemResolver,
		ProbeHost:      "bootstrap.test",
		ProbeTimeout:   time.Second,
		QueryTimeout:   time.Second,
	})
	if status.Active != ModeDoH || !status.Fallback || status.Provider != "test" || status.Stack != "IPv4" {
		t.Fatalf("status = %#v, want active DoH fallback", status)
	}
	if net.DefaultResolver == previous {
		t.Fatal("DoH fallback did not replace the process resolver")
	}

	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	response, err := client.Get("http://target.test:" + targetPort + "/")
	if err != nil {
		t.Fatalf("default HTTP resolver did not use DoH fallback: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	Shutdown()
	if net.DefaultResolver != previous {
		t.Fatal("Shutdown did not restore the previous process resolver")
	}
}

func TestConfigureAutoKeepsHealthySystemResolver(t *testing.T) {
	Shutdown()
	previous := net.DefaultResolver
	t.Cleanup(func() {
		Shutdown()
		net.DefaultResolver = previous
	})
	// Register the process-resolver restoration before the fixture cleanup so
	// the DNS listener is closed first. This prevents a lingering stdlib DNS
	// lookup goroutine from racing the global resolver assignment under -race.
	healthy := healthySystemResolver(t)
	net.DefaultResolver = healthy
	var dohRequests atomic.Int32
	doh := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		dohRequests.Add(1)
		http.Error(writer, "DoH must not be used while system DNS is healthy", http.StatusInternalServerError)
	}))
	defer doh.Close()

	status := Configure(context.Background(), Config{
		Mode:         ModeAuto,
		Endpoints:    []Endpoint{{Name: "test", URL: doh.URL}},
		ProbeHost:    "healthy.test",
		ProbeTimeout: time.Second,
		QueryTimeout: time.Second,
	})
	if status.Active != ModeSystem || !status.SystemAvailable || status.Fallback {
		t.Fatalf("status = %#v, want active system resolver", status)
	}
	if got := dohRequests.Load(); got != 0 {
		t.Fatalf("DoH requests = %d, want 0", got)
	}
	if net.DefaultResolver != healthy {
		t.Fatal("healthy system DNS unexpectedly replaced the process resolver")
	}
	addresses, err := net.DefaultResolver.LookupIP(context.Background(), "ip4", "healthy.test")
	if err != nil || len(addresses) != 1 || addresses[0].String() != "192.0.2.53" {
		t.Fatalf("healthy system lookup = %v, %v", addresses, err)
	}
}

func TestConfigureSystemNeverFallsBackToDoH(t *testing.T) {
	Shutdown()
	t.Cleanup(Shutdown)
	previous := net.DefaultResolver
	var dohRequests atomic.Int32
	doh := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		dohRequests.Add(1)
		http.Error(writer, "system mode must not use DoH", http.StatusInternalServerError)
	}))
	defer doh.Close()

	missingSystemResolver := &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("system resolver unavailable")
	}}
	status := Configure(context.Background(), Config{
		Mode:           ModeSystem,
		Endpoints:      []Endpoint{{Name: "test", URL: doh.URL}},
		SystemResolver: missingSystemResolver,
		ProbeHost:      "missing.test",
		ProbeTimeout:   time.Second,
		QueryTimeout:   time.Second,
	})
	if status.Active != ModeUnavailable || status.Requested != ModeSystem {
		t.Fatalf("status = %#v, want unavailable system-only resolver", status)
	}
	if got := dohRequests.Load(); got != 0 {
		t.Fatalf("DoH requests = %d, want 0", got)
	}
	if net.DefaultResolver != previous {
		t.Fatal("system-only failure unexpectedly replaced the process resolver")
	}
}

func TestBootstrapReachableUsesFixedAddressWithoutSystemDNS(t *testing.T) {
	var queries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		queries.Add(1)
		query := new(dns.Msg)
		if err := query.Unpack(readRequestBody(t, request)); err != nil {
			t.Fatal(err)
		}
		response := new(dns.Msg)
		response.SetReply(query)
		response.Answer = append(response.Answer, &dns.A{Hdr: dns.RR_Header{Name: query.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("192.0.2.9")})
		payload, err := response.Pack()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = writer.Write(payload)
	}))
	defer server.Close()
	host, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	previous := net.DefaultResolver
	missingSystemResolver := &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("system resolver unavailable")
	}}
	stack, reachable := BootstrapReachable(context.Background(), Config{
		Mode:           ModeDoH,
		Endpoints:      []Endpoint{{Name: "test", URL: "http://bootstrap.test:" + port + "/dns-query"}},
		BootstrapHosts: map[string][]string{"bootstrap.test": {host}},
		ProbeTimeout:   time.Second,
		SystemResolver: missingSystemResolver,
	})
	if !reachable || stack != "IPv4" {
		t.Fatalf("bootstrap reachability = %q, %t; want IPv4, true", stack, reachable)
	}
	if net.DefaultResolver != previous {
		t.Fatal("bootstrap reachability changed the process resolver")
	}
	if queries.Load() == 0 {
		t.Fatal("bootstrap reachability did not issue a DNS query through the configured fixed address")
	}
}

func TestConfigureAutoKeepsSystemResolverAfterTransientTimeout(t *testing.T) {
	Shutdown()
	t.Cleanup(Shutdown)
	previous := net.DefaultResolver
	var upstreamQueries atomic.Int32
	doh := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamQueries.Add(1)
		http.Error(writer, "auto mode must not use encrypted fallback after a transient timeout", http.StatusInternalServerError)
	}))
	defer doh.Close()

	transientSystemResolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _ string, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	status := Configure(context.Background(), Config{
		Mode:             ModeAuto,
		Endpoints:        []Endpoint{{Name: "test", URL: doh.URL}},
		SystemResolver:   transientSystemResolver,
		ProbeTimeout:     40 * time.Millisecond,
		QueryTimeout:     time.Second,
		SystemProbeHosts: []string{"one.test", "two.test", "three.test"},
	})
	if status.Active != ModeSystem || status.Fallback || status.Reason != systemDNSInconclusiveReason {
		t.Fatalf("status = %#v, want inconclusive system resolver", status)
	}
	if got := upstreamQueries.Load(); got != 0 {
		t.Fatalf("encrypted upstream queries = %d, want 0", got)
	}
	if net.DefaultResolver != previous {
		t.Fatal("transient system DNS failure unexpectedly replaced the process resolver")
	}
}

func TestConfirmedLocalDNSFailureIsConservative(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "network loss", err: errors.New("network is unreachable")},
		{name: "remote refusal", err: errors.New("dial udp 198.51.100.53:53: connect: connection refused")},
		{name: "remote refusal with numeric query label", err: errors.New("lookup 127.0.0.1.example on 198.51.100.53:53: connect: connection refused")},
		{name: "loopback refusal", err: errors.New("dial udp 127.0.0.53:53: connect: connection refused"), want: true},
		{name: "wrapped remote refusal", err: &net.DNSError{Err: "connection refused", Server: "198.51.100.53:53"}},
		{name: "wrapped loopback refusal", err: &net.DNSError{Err: "connection refused", Server: "127.0.0.53:53"}, want: true},
		{name: "explicit resolver unavailable", err: errors.New("system resolver unavailable"), want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isConfirmedLocalDNSFailure(test.err); got != test.want {
				t.Fatalf("isConfirmedLocalDNSFailure(%v) = %t, want %t", test.err, got, test.want)
			}
		})
	}
}

func TestConfigureForcedDoTUsesTLSAndSNI(t *testing.T) {
	Shutdown()
	t.Cleanup(Shutdown)
	previous := net.DefaultResolver
	certificate := testCertificate(t, "dot.test")
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	originalTLSConfig := dotTLSConfig
	dotTLSConfig = func(serverName string) *tls.Config {
		return &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName, RootCAs: roots}
	}
	t.Cleanup(func() { dotTLSConfig = originalTLSConfig })

	var receivedServerName atomic.Value
	rawListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			receivedServerName.Store(hello.ServerName)
			return nil, nil
		},
	}
	listener := tls.NewListener(rawListener, serverTLSConfig)
	server := &dns.Server{
		Listener: listener,
		Net:      "tcp",
		Handler: dns.HandlerFunc(func(writer dns.ResponseWriter, request *dns.Msg) {
			response := new(dns.Msg)
			response.SetReply(request)
			for _, question := range request.Question {
				if question.Qtype == dns.TypeA {
					response.Answer = append(response.Answer, &dns.A{Hdr: dns.RR_Header{Name: question.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("192.0.2.77")})
				}
			}
			_ = writer.WriteMsg(response)
		}),
	}
	go func() { _ = server.ActivateAndServe() }()
	t.Cleanup(func() {
		_ = server.Shutdown()
		_ = listener.Close()
		_ = rawListener.Close()
	})
	host, port, err := net.SplitHostPort(rawListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	missingSystemResolver := &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("system resolver unavailable")
	}}
	status := Configure(context.Background(), Config{
		Mode:           ModeDoT,
		Endpoints:      []Endpoint{{Name: "test-dot", URL: "tls://dot.test:" + port}},
		BootstrapHosts: map[string][]string{"dot.test": {host}},
		SystemResolver: missingSystemResolver,
		ProbeTimeout:   time.Second,
		QueryTimeout:   time.Second,
	})
	if status.Active != ModeDoT || !status.DoTAvailable || status.DoHAvailable || status.Provider != "test-dot" {
		t.Fatalf("status = %#v, want active DoT resolver", status)
	}
	addresses, err := net.DefaultResolver.LookupIP(context.Background(), "ip4", "target.test")
	if err != nil || len(addresses) != 1 || addresses[0].String() != "192.0.2.77" {
		t.Fatalf("DoT lookup = %v, %v", addresses, err)
	}
	value := receivedServerName.Load()
	if value == nil || value.(string) != "dot.test" {
		t.Fatalf("DoT TLS SNI = %#v, want dot.test", value)
	}
	Shutdown()
	if net.DefaultResolver != previous {
		t.Fatal("Shutdown did not restore the previous process resolver")
	}
}

func TestDoHResponseCacheAvoidsDuplicateQueries(t *testing.T) {
	var queries int
	doh := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		queries++
		query := new(dns.Msg)
		if err := query.Unpack(readRequestBody(t, request)); err != nil {
			t.Fatal(err)
		}
		response := new(dns.Msg)
		response.SetReply(query)
		response.Answer = append(response.Answer, &dns.A{Hdr: dns.RR_Header{Name: query.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("192.0.2.10")})
		payload, err := response.Pack()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = writer.Write(payload)
	}))
	defer doh.Close()

	resolver, err := New(Config{Mode: ModeDoH, Endpoints: []Endpoint{{Name: "test", URL: doh.URL}}, ProbeTimeout: time.Second, QueryTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Close()
	if status := resolver.Activate(context.Background()); status.Active != ModeDoH {
		t.Fatalf("status = %#v", status)
	}
	client := resolver.StdlibResolver()
	for range 2 {
		addresses, err := client.LookupIP(context.Background(), "ip4", "cache.test")
		if err != nil || len(addresses) != 1 || addresses[0].String() != "192.0.2.10" {
			t.Fatalf("lookup = %v, %v", addresses, err)
		}
	}
	if queries != 2 { // one startup probe and one cached cache.test query
		t.Fatalf("DoH requests = %d, want 2", queries)
	}
}

func TestActivateReservesTimeForLaterFallbackEndpoints(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(700 * time.Millisecond)
	}))
	defer slow.Close()
	healthy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query := new(dns.Msg)
		if err := query.Unpack(readRequestBody(t, request)); err != nil {
			t.Fatal(err)
		}
		response := new(dns.Msg)
		response.SetReply(query)
		response.Answer = append(response.Answer, &dns.A{Hdr: dns.RR_Header{Name: query.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("192.0.2.1")})
		payload, err := response.Pack()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = writer.Write(payload)
	}))
	defer healthy.Close()

	resolver, err := New(Config{
		Mode:         ModeDoH,
		Endpoints:    []Endpoint{{Name: "slow", URL: slow.URL}, {Name: "healthy", URL: healthy.URL}},
		ProbeTimeout: 600 * time.Millisecond,
		QueryTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Close()
	status := resolver.Activate(context.Background())
	if status.Active != ModeDoH || status.Provider != "healthy" {
		t.Fatalf("status = %#v, want later healthy fallback", status)
	}
}

func TestParseModeDefaultsToAuto(t *testing.T) {
	for _, value := range []string{"", "invalid", " AUTO ", "system", "doh", "dot"} {
		mode := ParseMode(value)
		if value == "system" && mode != ModeSystem {
			t.Fatalf("ParseMode(%q) = %q", value, mode)
		}
		if value == "doh" && mode != ModeDoH {
			t.Fatalf("ParseMode(%q) = %q", value, mode)
		}
		if value == "dot" && mode != ModeDoT {
			t.Fatalf("ParseMode(%q) = %q", value, mode)
		}
	}
}

func testCertificate(t *testing.T, dnsName string) tls.Certificate {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: dnsName},
		DNSNames:     []string{dnsName},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	certificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return certificate
}

func readRequestBody(t *testing.T, request *http.Request) []byte {
	t.Helper()
	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func healthySystemResolver(t *testing.T) *net.Resolver {
	t.Helper()
	packetConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &dns.Server{
		PacketConn: packetConn,
		Handler: dns.HandlerFunc(func(writer dns.ResponseWriter, request *dns.Msg) {
			response := new(dns.Msg)
			response.SetReply(request)
			for _, question := range request.Question {
				switch question.Qtype {
				case dns.TypeA:
					response.Answer = append(response.Answer, &dns.A{
						Hdr: dns.RR_Header{Name: question.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
						A:   net.ParseIP("192.0.2.53"),
					})
				case dns.TypeAAAA:
					response.Answer = append(response.Answer, &dns.AAAA{
						Hdr:  dns.RR_Header{Name: question.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
						AAAA: net.ParseIP("2001:db8::53"),
					})
				}
			}
			_ = writer.WriteMsg(response)
		}),
	}
	go func() { _ = server.ActivateAndServe() }()
	t.Cleanup(func() {
		_ = server.Shutdown()
		_ = packetConn.Close()
	})
	address := packetConn.LocalAddr().String()
	return &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
}
