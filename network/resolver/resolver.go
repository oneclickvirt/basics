// Package resolver provides a process-local encrypted DNS fallback for
// applications that can reach the public Internet but have no usable local
// DNS configuration. It never changes host DNS configuration files.
package resolver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Mode selects how DNS queries are handled.
type Mode string

const (
	// ModeAuto keeps the system resolver unless independent probes confirm it
	// is unavailable. It then selects the fastest validated encrypted endpoint.
	ModeAuto Mode = "auto"
	// ModeSystem disables the in-process encrypted fallback.
	ModeSystem Mode = "system"
	// ModeDoH always uses built-in DNS-over-HTTPS endpoints.
	ModeDoH Mode = "doh"
	// ModeDoT always uses built-in DNS-over-TLS endpoints.
	ModeDoT Mode = "dot"
	// ModeUnavailable indicates that the selected resolver path could not be
	// validated during startup.
	ModeUnavailable Mode = "unavailable"
)

const (
	defaultProbeHost    = "example.com"
	defaultProbeTimeout = 1500 * time.Millisecond
	defaultQueryTimeout = 4 * time.Second
	maxDoHResponseBytes = 1 << 20
)

// Endpoint identifies a public DNS upstream. URL uses https:// for DoH and
// tls:// for DoT. Bootstrap addresses are kept separately so resolving an
// endpoint never needs the system resolver that it may replace.
type Endpoint struct {
	Name string
	URL  string
}

// Config configures one resolver instance. Nil or empty endpoint/bootstrap
// settings use the maintained built-in defaults. SystemResolver is mainly
// useful for embedding applications that already own a resolver; nil uses the
// process resolver that was active when the instance was created.
type Config struct {
	Mode             Mode
	Endpoints        []Endpoint
	BootstrapHosts   map[string][]string
	ProbeHost        string
	SystemProbeHosts []string
	ProbeTimeout     time.Duration
	QueryTimeout     time.Duration
	SystemResolver   *net.Resolver
}

// Status describes the startup decision without exposing query contents.
// DoHAvailable is retained for consumers that predate DoT support.
type Status struct {
	Requested       Mode
	Active          Mode
	SystemAvailable bool
	DoHAvailable    bool
	DoTAvailable    bool
	Fallback        bool
	Provider        string
	// Stack is the address family used by the successful encrypted bootstrap.
	Stack  string
	Reason string
}

type cacheEntry struct {
	message   *dns.Msg
	expiresAt time.Time
}

// Resolver owns an optional loopback DNS server. Its net.Resolver can be
// installed as net.DefaultResolver without changing resolver files on disk.
type Resolver struct {
	config Config
	system *net.Resolver

	mu      sync.RWMutex
	status  Status
	udpAddr string
	tcpAddr string
	udp     *dns.Server
	tcp     *dns.Server
	udpConn net.PacketConn
	tcpLn   net.Listener
	closed  bool
	stack   string

	cacheMu sync.Mutex
	cache   map[string]cacheEntry

	dohClient *http.Client
	stdlib    *net.Resolver
}

// New creates an uninstalled resolver. Call Activate before using Resolver
// with the standard library.
func New(config Config) (*Resolver, error) {
	config, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}
	system := config.SystemResolver
	if system == nil {
		system = net.DefaultResolver
	}
	resolver := &Resolver{
		config: config,
		system: system,
		status: Status{Requested: config.Mode, Active: ModeUnavailable},
		cache:  make(map[string]cacheEntry),
	}
	resolver.dohClient = &http.Client{
		Timeout: config.QueryTimeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           resolver.bootstrapDialContext,
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   config.QueryTimeout,
			ResponseHeaderTimeout: config.QueryTimeout,
			ExpectContinueTimeout: time.Second,
			IdleConnTimeout:       30 * time.Second,
			MaxIdleConns:          len(config.Endpoints),
			MaxIdleConnsPerHost:   1,
		},
	}
	resolver.stdlib = &net.Resolver{PreferGo: true, Dial: resolver.dial}
	return resolver, nil
}

func normalizeConfig(config Config) (Config, error) {
	config.Mode = ParseMode(string(config.Mode))
	if config.ProbeHost = strings.TrimSuffix(strings.TrimSpace(config.ProbeHost), "."); config.ProbeHost == "" {
		config.ProbeHost = defaultProbeHost
	}
	if config.ProbeTimeout <= 0 {
		config.ProbeTimeout = defaultProbeTimeout
	}
	if config.QueryTimeout <= 0 {
		config.QueryTimeout = defaultQueryTimeout
	}
	if len(config.Endpoints) == 0 {
		config.Endpoints = DefaultEndpoints()
	} else {
		config.Endpoints = append([]Endpoint(nil), config.Endpoints...)
	}
	for index := range config.Endpoints {
		endpoint := &config.Endpoints[index]
		endpoint.Name = strings.TrimSpace(endpoint.Name)
		endpoint.URL = strings.TrimSpace(endpoint.URL)
		parsed, _, err := parseEndpoint(*endpoint)
		if err != nil {
			return Config{}, err
		}
		if endpoint.Name == "" {
			endpoint.Name = parsed.Hostname()
		}
	}
	if config.Mode != ModeSystem {
		config.Endpoints = endpointsForMode(config.Endpoints, config.Mode)
		if len(config.Endpoints) == 0 {
			return Config{}, fmt.Errorf("no resolver endpoints match DNS mode %q", config.Mode)
		}
	}
	bootstrap := DefaultBootstrapHosts()
	for host, addresses := range config.BootstrapHosts {
		host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
		if host == "" {
			continue
		}
		if clean := cleanAddresses(addresses); len(clean) > 0 {
			bootstrap[host] = clean
		}
	}
	config.BootstrapHosts = bootstrap
	return config, nil
}

// ParseMode normalizes a user-provided mode. Invalid values safely use auto.
func ParseMode(value string) Mode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(ModeSystem):
		return ModeSystem
	case string(ModeDoH):
		return ModeDoH
	case string(ModeDoT):
		return ModeDoT
	default:
		return ModeAuto
	}
}

// BootstrapReachable validates at least one configured encrypted endpoint via
// its fixed bootstrap address. It does not modify net.DefaultResolver. Unlike
// a TCP-only check, this validates TLS and an actual DNS response.
func BootstrapReachable(ctx context.Context, config Config) (string, bool) {
	resolver, err := New(config)
	if err != nil {
		return "", false
	}
	defer resolver.Close()
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, resolver.config.ProbeTimeout)
	defer cancel()
	_, _, _, err = resolver.rankEncryptedEndpoints(probeCtx)
	return resolver.bootstrapStack(), err == nil
}

// Activate probes the requested resolver path. On encrypted fallback success
// it starts a loopback DNS server; callers can then install StdlibResolver as
// net.DefaultResolver.
func (resolver *Resolver) Activate(ctx context.Context) Status {
	if resolver == nil {
		return Status{Requested: ModeAuto, Active: ModeUnavailable, Reason: "resolver unavailable"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	switch resolver.config.Mode {
	case ModeSystem:
		switch resolver.probeSystem(ctx) {
		case systemProbeAvailable:
			return resolver.setAndReturn(Status{Requested: ModeSystem, Active: ModeSystem, SystemAvailable: true})
		case systemProbeInconclusive:
			return resolver.setAndReturn(Status{Requested: ModeSystem, Active: ModeSystem, Reason: systemDNSInconclusiveReason})
		default:
			return resolver.setAndReturn(Status{Requested: ModeSystem, Active: ModeUnavailable, Reason: "system DNS probe confirmed unavailable"})
		}
	case ModeAuto:
		switch resolver.probeSystem(ctx) {
		case systemProbeAvailable:
			return resolver.setAndReturn(Status{Requested: ModeAuto, Active: ModeSystem, SystemAvailable: true})
		case systemProbeInconclusive:
			return resolver.setAndReturn(Status{Requested: ModeAuto, Active: ModeSystem, Reason: systemDNSInconclusiveReason})
		}
	case ModeDoH, ModeDoT:
		// Explicit encrypted modes deliberately bypass the system probe.
	default:
		return resolver.setAndReturn(Status{Requested: resolver.config.Mode, Active: ModeUnavailable, Reason: "invalid DNS mode"})
	}

	provider, transport, stack, err := resolver.activateEncrypted(ctx)
	if err != nil {
		reason := "encrypted DNS probe failed"
		if resolver.config.Mode == ModeAuto {
			reason = "system DNS confirmed unavailable; encrypted DNS probe failed"
		}
		return resolver.setAndReturn(Status{Requested: resolver.config.Mode, Active: ModeUnavailable, Reason: reason})
	}
	active := modeForTransport(transport)
	status := Status{
		Requested:    resolver.config.Mode,
		Active:       active,
		DoHAvailable: transport == TransportDoH,
		DoTAvailable: transport == TransportDoT,
		Fallback:     resolver.config.Mode == ModeAuto,
		Provider:     provider,
		Stack:        stack,
	}
	if status.Fallback {
		status.Reason = "system DNS confirmed unavailable; encrypted fallback active"
	}
	return resolver.setAndReturn(status)
}

func (resolver *Resolver) activateEncrypted(ctx context.Context) (string, Transport, string, error) {
	probeCtx, cancel := context.WithTimeout(ctx, resolver.config.ProbeTimeout)
	defer cancel()
	ranked, provider, transport, err := resolver.rankEncryptedEndpoints(probeCtx)
	if err != nil {
		return "", "", "", err
	}
	resolver.config.Endpoints = ranked
	if err := resolver.start(); err != nil {
		return "", "", "", err
	}
	return provider, transport, resolver.bootstrapStack(), nil
}

func modeForTransport(transport Transport) Mode {
	switch transport {
	case TransportDoH:
		return ModeDoH
	case TransportDoT:
		return ModeDoT
	default:
		return ModeUnavailable
	}
}

func (resolver *Resolver) setAndReturn(status Status) Status {
	resolver.setStatus(status)
	return status
}

// StdlibResolver returns the resolver that forwards standard DNS wire queries
// to the in-process encrypted fallback. Activate must report an encrypted mode
// before callers install it as net.DefaultResolver.
func (resolver *Resolver) StdlibResolver() *net.Resolver {
	if resolver == nil {
		return nil
	}
	return resolver.stdlib
}

// Status returns a snapshot of the latest activation or query result.
func (resolver *Resolver) Status() Status {
	if resolver == nil {
		return Status{Requested: ModeAuto, Active: ModeUnavailable, Reason: "resolver unavailable"}
	}
	resolver.mu.RLock()
	defer resolver.mu.RUnlock()
	return resolver.status
}

func (resolver *Resolver) setStatus(status Status) {
	resolver.mu.Lock()
	resolver.status = status
	resolver.mu.Unlock()
}

func (resolver *Resolver) markProvider(provider string, transport Transport) {
	resolver.mu.Lock()
	resolver.status.Provider = provider
	resolver.status.DoHAvailable = transport == TransportDoH
	resolver.status.DoTAvailable = transport == TransportDoT
	resolver.mu.Unlock()
}

func (resolver *Resolver) markBootstrapStack(address net.IP) {
	stack := "IPv6"
	if address.To4() != nil {
		stack = "IPv4"
	}
	resolver.mu.Lock()
	resolver.stack = stack
	if resolver.status.Active == ModeDoH || resolver.status.Active == ModeDoT {
		resolver.status.Stack = stack
	}
	resolver.mu.Unlock()
}

func (resolver *Resolver) bootstrapStack() string {
	resolver.mu.RLock()
	defer resolver.mu.RUnlock()
	return resolver.stack
}

func (resolver *Resolver) start() error {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	if resolver.closed {
		return errors.New("resolver is closed")
	}
	if resolver.udpConn != nil && resolver.tcpLn != nil {
		return nil
	}
	udpConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := udpConn.LocalAddr().(*net.UDPAddr).Port
	tcpLn, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port)))
	if err != nil {
		_ = udpConn.Close()
		return err
	}
	resolver.udpConn = udpConn
	resolver.tcpLn = tcpLn
	resolver.udpAddr = udpConn.LocalAddr().String()
	resolver.tcpAddr = tcpLn.Addr().String()
	handler := dns.HandlerFunc(resolver.handleDNS)
	resolver.udp = &dns.Server{PacketConn: udpConn, Handler: handler}
	resolver.tcp = &dns.Server{Listener: tcpLn, Handler: handler}
	go func(server *dns.Server) { _ = server.ActivateAndServe() }(resolver.udp)
	go func(server *dns.Server) { _ = server.ActivateAndServe() }(resolver.tcp)
	return nil
}

func (resolver *Resolver) dial(ctx context.Context, network, _ string) (net.Conn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	resolver.mu.RLock()
	udpAddr, tcpAddr, closed := resolver.udpAddr, resolver.tcpAddr, resolver.closed
	resolver.mu.RUnlock()
	if closed || udpAddr == "" || tcpAddr == "" {
		return nil, errors.New("encrypted DNS fallback is not active")
	}
	// The target is loopback, but net.Dialer still snapshots DefaultResolver
	// before recognizing a literal address. Keep this internal path bound to
	// the resolver captured before installation so Configure never races with
	// its own background HTTP work while swapping the global pointer.
	dialer := net.Dialer{Resolver: resolver.system}
	if strings.HasPrefix(strings.ToLower(network), "tcp") {
		return dialer.DialContext(ctx, "tcp4", tcpAddr)
	}
	return dialer.DialContext(ctx, "udp4", udpAddr)
}

func (resolver *Resolver) handleDNS(writer dns.ResponseWriter, request *dns.Msg) {
	response := resolver.resolve(request)
	if response == nil {
		response = new(dns.Msg)
		response.SetRcode(request, dns.RcodeServerFailure)
	}
	response.Id = request.Id
	response.Response = true
	response.RecursionAvailable = true
	_ = writer.WriteMsg(response)
}

func (resolver *Resolver) resolve(request *dns.Msg) *dns.Msg {
	if request == nil || len(request.Question) == 0 {
		response := new(dns.Msg)
		if request != nil {
			response.SetRcode(request, dns.RcodeFormatError)
		} else {
			response.Rcode = dns.RcodeFormatError
			response.Response = true
		}
		return response
	}
	key := cacheKey(request)
	if cached := resolver.cached(key); cached != nil {
		cached.Id = request.Id
		return cached
	}
	ctx, cancel := context.WithTimeout(context.Background(), resolver.config.QueryTimeout)
	defer cancel()
	response, provider, transport, err := resolver.queryUpstream(ctx, request)
	if err != nil {
		failed := new(dns.Msg)
		failed.SetRcode(request, dns.RcodeServerFailure)
		return failed
	}
	response.Id = request.Id
	resolver.markProvider(provider, transport)
	resolver.storeCache(key, response)
	return response
}

// endpointContext reserves part of a caller's remaining deadline for each
// provider still to be tried. Without this, one unreachable endpoint can use
// the whole query budget and prevent later built-in fallbacks from being tried.
func (resolver *Resolver) endpointContext(parent context.Context, remaining int) (context.Context, context.CancelFunc) {
	timeout := resolver.config.QueryTimeout
	if deadline, ok := parent.Deadline(); ok {
		budget := time.Until(deadline)
		if remaining > 1 {
			budget /= time.Duration(remaining)
		}
		if budget < timeout {
			timeout = budget
		}
	}
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

func (resolver *Resolver) bootstrapDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses := resolver.bootstrapAddresses(host)
	if len(addresses) == 0 {
		return nil, fmt.Errorf("no bootstrap address for resolver endpoint %q", host)
	}
	var errs []error
	for _, address := range addresses {
		ip := net.ParseIP(address)
		if ip == nil {
			continue
		}
		dialNetwork := network
		if strings.HasPrefix(network, "tcp") {
			if ip.To4() != nil {
				dialNetwork = "tcp4"
			} else {
				dialNetwork = "tcp6"
			}
		}
		// Do not let the upstream transport observe net.DefaultResolver while
		// Configure is installing the fallback. The destination is a literal IP.
		dialer := &net.Dialer{Resolver: resolver.system}
		connection, dialErr := dialer.DialContext(ctx, dialNetwork, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			resolver.markBootstrapStack(ip)
			return connection, nil
		}
		errs = append(errs, dialErr)
	}
	if len(errs) == 0 {
		return nil, fmt.Errorf("no valid bootstrap address for resolver endpoint %q", host)
	}
	return nil, errors.Join(errs...)
}

func cacheKey(message *dns.Msg) string {
	if message == nil || len(message.Question) == 0 {
		return ""
	}
	question := message.Question[0]
	return strings.ToLower(question.Name) + fmt.Sprintf("/%d/%d", question.Qtype, question.Qclass)
}

func (resolver *Resolver) cached(key string) *dns.Msg {
	if key == "" {
		return nil
	}
	resolver.cacheMu.Lock()
	defer resolver.cacheMu.Unlock()
	entry, ok := resolver.cache[key]
	if !ok {
		return nil
	}
	if time.Now().After(entry.expiresAt) {
		delete(resolver.cache, key)
		return nil
	}
	return cloneMessage(entry.message)
}

func (resolver *Resolver) storeCache(key string, message *dns.Msg) {
	if key == "" || message == nil || message.Rcode != dns.RcodeSuccess {
		return
	}
	var ttl uint32
	for _, record := range message.Answer {
		recordTTL := record.Header().Ttl
		if recordTTL == 0 {
			continue
		}
		if ttl == 0 || recordTTL < ttl {
			ttl = recordTTL
		}
	}
	if ttl == 0 {
		return
	}
	if ttl < 15 {
		ttl = 15
	}
	if ttl > 600 {
		ttl = 600
	}
	resolver.cacheMu.Lock()
	resolver.cache[key] = cacheEntry{message: cloneMessage(message), expiresAt: time.Now().Add(time.Duration(ttl) * time.Second)}
	resolver.cacheMu.Unlock()
}

func cloneMessage(message *dns.Msg) *dns.Msg {
	if message == nil {
		return nil
	}
	wire, err := message.Pack()
	if err != nil {
		return nil
	}
	copy := new(dns.Msg)
	if err := copy.Unpack(wire); err != nil {
		return nil
	}
	return copy
}

// Close releases loopback listeners. It does not mutate net.DefaultResolver;
// callers that installed the resolver should use Shutdown.
func (resolver *Resolver) Close() {
	if resolver == nil {
		return
	}
	resolver.mu.Lock()
	if resolver.closed {
		resolver.mu.Unlock()
		return
	}
	resolver.closed = true
	udp, tcp, udpConn, tcpLn := resolver.udp, resolver.tcp, resolver.udpConn, resolver.tcpLn
	resolver.mu.Unlock()
	if udp != nil {
		_ = udp.Shutdown()
	}
	if tcp != nil {
		_ = tcp.Shutdown()
	}
	if udpConn != nil {
		_ = udpConn.Close()
	}
	if tcpLn != nil {
		_ = tcpLn.Close()
	}
	if resolver.dohClient != nil {
		if transport, ok := resolver.dohClient.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
}

var globalState struct {
	sync.Mutex
	installed *Resolver
	previous  *net.Resolver
	status    Status
}

// Configure creates, probes, and conditionally installs the process resolver.
// It is intended for one application startup path and is safe to call again
// before work begins, such as after an interactive configuration change.
func Configure(ctx context.Context, config Config) Status {
	globalState.Lock()
	defer globalState.Unlock()
	if globalState.installed != nil {
		if net.DefaultResolver == globalState.installed.StdlibResolver() {
			net.DefaultResolver = globalState.previous
		}
		globalState.installed.Close()
		globalState.installed = nil
	}
	globalState.previous = net.DefaultResolver
	if config.SystemResolver == nil {
		config.SystemResolver = globalState.previous
	}
	resolver, err := New(config)
	if err != nil {
		globalState.status = Status{Requested: ParseMode(string(config.Mode)), Active: ModeUnavailable, Reason: "invalid DNS configuration"}
		return globalState.status
	}
	status := resolver.Activate(ctx)
	if status.Active == ModeDoH || status.Active == ModeDoT {
		net.DefaultResolver = resolver.StdlibResolver()
		globalState.installed = resolver
	} else {
		resolver.Close()
	}
	globalState.status = status
	return status
}

// CurrentStatus returns the result from the most recent Configure call.
func CurrentStatus() Status {
	globalState.Lock()
	defer globalState.Unlock()
	return globalState.status
}

// Shutdown restores the resolver that Configure replaced and closes any
// in-process loopback listeners. It is mainly useful for embedding programs
// and tests; process exit also releases all of these resources.
func Shutdown() {
	globalState.Lock()
	defer globalState.Unlock()
	if globalState.installed != nil {
		if net.DefaultResolver == globalState.installed.StdlibResolver() {
			net.DefaultResolver = globalState.previous
		}
		globalState.installed.Close()
		globalState.installed = nil
	}
	globalState.status = Status{}
}
