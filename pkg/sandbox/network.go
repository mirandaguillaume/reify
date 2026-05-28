// Package sandbox — network policy enforcement.
//
// Precedence:
//  1. ForbiddenDomains (deny wins over everything)
//  2. AllowedDomains   (allow if listed; deny if non-empty and host not listed)
//  3. Default          (allow when AllowedDomains is empty and host not forbidden)
package sandbox

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"
)

// AuditFunc is called for every proxy decision (allow or deny).
// host is the target host (without port). allowed indicates the decision.
// reason is empty on allow and contains the denial reason on deny.
type AuditFunc func(host string, allowed bool, reason string)

// NetworkPolicy declares domain-level access rules for a step.
// Patterns are exact hostnames or glob wildcards (*.suffix).
type NetworkPolicy struct {
	AllowedDomains   []string `yaml:"allowed_domains,omitempty"`
	ForbiddenDomains []string `yaml:"forbidden_domains,omitempty"`
}

// UnmarshalYAML enforces strict field checking: any key other than
// allowed_domains or forbidden_domains is rejected with a clear error
// naming the unknown field and the valid keys. This satisfies AC6 without
// enabling KnownFields globally on all YAML parsing in the project.
func (p *NetworkPolicy) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("network_policy: expected a mapping, got %s", value.Tag)
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		if key != "allowed_domains" && key != "forbidden_domains" {
			return fmt.Errorf("network_policy: unknown field %q (valid fields: allowed_domains, forbidden_domains)", key)
		}
	}
	// type alias breaks UnmarshalYAML recursion.
	type plain NetworkPolicy
	return value.Decode((*plain)(p))
}

// CheckHost returns whether host is permitted and the reason on denial.
// host must be a bare hostname without port.
func (p *NetworkPolicy) CheckHost(host string) (allowed bool, reason string) {
	host = strings.ToLower(host)

	for _, pattern := range p.ForbiddenDomains {
		if hostMatch(pattern, host) {
			return false, fmt.Sprintf("domain matches forbidden pattern: %s", pattern)
		}
	}

	if len(p.AllowedDomains) == 0 {
		return true, ""
	}

	for _, pattern := range p.AllowedDomains {
		if hostMatch(pattern, host) {
			return true, ""
		}
	}

	return false, "domain not in allowed_domains"
}

// hostMatch reports whether pattern matches host (case-insensitive).
// Supports exact match and suffix wildcard: *.suffix matches x.suffix and
// a.b.suffix but NOT suffix itself.
func hostMatch(pattern, host string) bool {
	pattern = strings.ToLower(pattern)
	host = strings.ToLower(host)

	if pattern == host {
		return true
	}

	if !strings.HasPrefix(pattern, "*.") {
		return false
	}

	// *.suffix → host must end with .suffix and have at least one label before it.
	suffix := pattern[1:] // includes the leading "."
	return len(host) > len(suffix) && strings.HasSuffix(host, suffix)
}

// ProxyServer is an HTTP proxy that enforces a NetworkPolicy.
type ProxyServer struct {
	listener  net.Listener
	server    *http.Server
	transport *http.Transport // Proxy: nil prevents self-loop via HTTP_PROXY env var
	policy    *NetworkPolicy
	audit     AuditFunc
	url       string
}

// NewProxyServer starts an HTTP proxy on a kernel-assigned ephemeral port.
// audit may be nil (no audit recording).
func NewProxyServer(policy *NetworkPolicy, audit AuditFunc) (*ProxyServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("proxy: listen: %w", err)
	}

	// Proxy: nil is critical — prevents http.DefaultTransport from reading HTTP_PROXY
	// and routing forwarded requests back to this proxy (self-loop).
	ps := &ProxyServer{
		listener:  ln,
		transport: &http.Transport{Proxy: nil},
		policy:    policy,
		audit:     audit,
		url:       "http://" + ln.Addr().String(),
	}

	ps.server = &http.Server{Handler: ps}
	go ps.server.Serve(ln) //nolint:errcheck
	return ps, nil
}

// URL returns the proxy's listen address as an http:// URL.
func (ps *ProxyServer) URL() string { return ps.url }

// Close shuts down the proxy. The listener is closed first (immediate effect:
// new connections are refused), then the http.Server is closed to clean up
// any in-flight handlers and prevent goroutine leaks.
func (ps *ProxyServer) Close() error {
	err := ps.listener.Close()
	ps.server.Close() //nolint:errcheck — always returns ErrServerClosed
	return err
}

// ServeHTTP dispatches CONNECT (HTTPS tunnel) and plain HTTP requests.
func (ps *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		ps.handleCONNECT(w, r)
	} else {
		ps.handleHTTP(w, r)
	}
}

func (ps *ProxyServer) handleCONNECT(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		// CONNECT always requires host:port per HTTP spec; malformed → reject.
		http.Error(w, "400 Bad Request: malformed host in CONNECT", http.StatusBadRequest)
		return
	}

	allowed, reason := ps.policy.CheckHost(host)
	if ps.audit != nil {
		ps.audit(host, allowed, reason)
	}

	if !allowed {
		http.Error(w, fmt.Sprintf("403 Forbidden: %s", reason), http.StatusForbidden)
		return
	}

	// Assert Hijacker BEFORE committing WriteHeader(200) — a 502 after 200
	// is unrecoverable from the client's perspective.
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "502 Bad Gateway: hijacking not supported", http.StatusBadGateway)
		return
	}

	upstream, err := net.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		return
	}
	defer upstream.Close()

	// Commit 200 only after Hijacker and upstream Dial both succeed.
	w.WriteHeader(http.StatusOK)

	// buf.Reader may contain bytes the HTTP server already read from the wire
	// (pipelined data after the CONNECT headers). Using it prevents data loss.
	clientConn, buf, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer clientConn.Close()

	done := make(chan struct{}, 2)
	go func() { io.Copy(upstream, buf.Reader); done <- struct{}{} }()  //nolint:errcheck
	go func() { io.Copy(clientConn, upstream); done <- struct{}{} }()  //nolint:errcheck
	// Drain both goroutines before returning so defers close connections cleanly.
	<-done
	<-done
}

func (ps *ProxyServer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Hostname()
	if host == "" {
		http.Error(w, "400 Bad Request: missing host", http.StatusBadRequest)
		return
	}

	allowed, reason := ps.policy.CheckHost(host)
	if ps.audit != nil {
		ps.audit(host, allowed, reason)
	}

	if !allowed {
		http.Error(w, fmt.Sprintf("403 Forbidden: %s", reason), http.StatusForbidden)
		return
	}

	// Strip proxy-specific headers and forward via the dedicated transport
	// (Proxy: nil) so this handler never routes back to itself.
	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""

	resp, err := ps.transport.RoundTrip(outReq)
	if err != nil {
		http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy only non-hop-by-hop headers to avoid connection management confusion.
	hopByHop := map[string]bool{
		"Connection": true, "Keep-Alive": true, "Transfer-Encoding": true,
		"Upgrade": true, "Proxy-Authenticate": true, "Proxy-Authorization": true,
		"Te": true, "Trailers": true,
	}
	for k, vs := range resp.Header {
		if hopByHop[k] {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}
