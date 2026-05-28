package sandbox

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── hostMatch tests ───────────────────────────────────────────────────────

func TestHostMatch_ExactMatch(t *testing.T) {
	assert.True(t, hostMatch("api.github.com", "api.github.com"))
	assert.False(t, hostMatch("api.github.com", "api.gitlab.com"))
}

func TestHostMatch_GlobSuffix_MatchesSubdomain(t *testing.T) {
	assert.True(t, hostMatch("*.internal", "api.internal"))
	assert.True(t, hostMatch("*.internal", "db.cluster.internal"))
}

func TestHostMatch_GlobSuffix_DoesNotMatchBareDomain(t *testing.T) {
	assert.False(t, hostMatch("*.internal", "internal"))
}

func TestHostMatch_GlobSuffix_DoesNotMatchUnrelated(t *testing.T) {
	assert.False(t, hostMatch("*.internal", "api.external"))
}

func TestHostMatch_CaseInsensitive(t *testing.T) {
	assert.True(t, hostMatch("api.github.com", "API.GitHub.COM"))
	assert.True(t, hostMatch("*.INTERNAL", "api.internal"))
}

func TestHostMatch_NonGlobPattern_NoPartialMatch(t *testing.T) {
	assert.False(t, hostMatch("github.com", "api.github.com"))
}

// ─── NetworkPolicy.CheckHost tests ────────────────────────────────────────

func TestCheckHost_EmptyPolicy_AllowsAll(t *testing.T) {
	p := &NetworkPolicy{}
	allowed, reason := p.CheckHost("anything.example.com")
	assert.True(t, allowed)
	assert.Empty(t, reason)
}

func TestCheckHost_ForbiddenDomain_Denied(t *testing.T) {
	p := &NetworkPolicy{ForbiddenDomains: []string{"api.stripe.com"}}
	allowed, reason := p.CheckHost("api.stripe.com")
	assert.False(t, allowed)
	assert.Equal(t, "domain matches forbidden pattern: api.stripe.com", reason)
}

func TestCheckHost_ForbiddenGlob_Denied(t *testing.T) {
	p := &NetworkPolicy{ForbiddenDomains: []string{"*.internal"}}
	allowed, reason := p.CheckHost("api.internal")
	assert.False(t, allowed)
	assert.Equal(t, "domain matches forbidden pattern: *.internal", reason)
}

func TestCheckHost_AllowedList_AllowsListed(t *testing.T) {
	p := &NetworkPolicy{AllowedDomains: []string{"api.github.com"}}
	allowed, reason := p.CheckHost("api.github.com")
	assert.True(t, allowed)
	assert.Empty(t, reason)
}

func TestCheckHost_AllowedList_DeniesUnlisted(t *testing.T) {
	p := &NetworkPolicy{AllowedDomains: []string{"api.github.com"}}
	allowed, reason := p.CheckHost("api.gitlab.com")
	assert.False(t, allowed)
	assert.Equal(t, "domain not in allowed_domains", reason)
}

func TestCheckHost_DenyPrecedence(t *testing.T) {
	// Host in both allowed and forbidden → forbidden wins.
	p := &NetworkPolicy{
		AllowedDomains:   []string{"api.example.com"},
		ForbiddenDomains: []string{"api.example.com"},
	}
	allowed, reason := p.CheckHost("api.example.com")
	assert.False(t, allowed)
	assert.Equal(t, "domain matches forbidden pattern: api.example.com", reason)
}

func TestCheckHost_EmptyAllowNonEmptyForbid_AllowsNonForbidden(t *testing.T) {
	p := &NetworkPolicy{ForbiddenDomains: []string{"bad.example.com"}}
	allowed, _ := p.CheckHost("good.example.com")
	assert.True(t, allowed)
}

func TestCheckHost_NonEmptyAllowEmptyForbid_DeniesUnlisted(t *testing.T) {
	p := &NetworkPolicy{AllowedDomains: []string{"good.example.com"}}
	allowed, reason := p.CheckHost("other.example.com")
	assert.False(t, allowed)
	assert.Equal(t, "domain not in allowed_domains", reason)
}

// ─── ProxyServer lifecycle tests ──────────────────────────────────────────

func TestNewProxyServer_ListensOnEphemeralPort(t *testing.T) {
	p := &NetworkPolicy{}
	srv, err := NewProxyServer(p, nil)
	require.NoError(t, err)
	defer srv.Close()

	url := srv.URL()
	assert.True(t, strings.HasPrefix(url, "http://127.0.0.1:"), "URL should be %q", url)

	// Port should be > 0 (kernel-assigned).
	_, portStr, err := net.SplitHostPort(strings.TrimPrefix(url, "http://"))
	require.NoError(t, err)
	assert.NotEmpty(t, portStr)
}

func TestProxyServer_Close_StopsListener(t *testing.T) {
	p := &NetworkPolicy{}
	srv, err := NewProxyServer(p, nil)
	require.NoError(t, err)

	url := srv.URL()
	srv.Close()

	// After close, connecting to the saved port should fail.
	_, portStr, _ := net.SplitHostPort(strings.TrimPrefix(url, "http://"))
	conn, err := net.Dial("tcp", "127.0.0.1:"+portStr)
	if err == nil {
		conn.Close()
		t.Fatal("expected connection refused after proxy close, but connected")
	}
}

// ─── ProxyServer HTTP plain tests ─────────────────────────────────────────

func TestProxyServer_PlainHTTP_AllowedDomain_Forwarded(t *testing.T) {
	// Upstream server standing in for the allowed target.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello from upstream")
	}))
	defer upstream.Close()

	// AllowedDomains are bare hostnames — strip the port from upstream URL.
	hostPort := strings.TrimPrefix(upstream.URL, "http://")
	upstreamBareHost, _, _ := net.SplitHostPort(hostPort)

	var auditHost string
	var auditAllowed bool
	audit := func(host string, allowed bool, reason string) {
		auditHost = host
		auditAllowed = allowed
	}

	policy := &NetworkPolicy{AllowedDomains: []string{upstreamBareHost}}
	srv, err := NewProxyServer(policy, audit)
	require.NoError(t, err)
	defer srv.Close()

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(mustParseURL(srv.URL())),
		},
	}

	resp, err := client.Get("http://" + hostPort + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "hello from upstream")
	assert.Equal(t, upstreamBareHost, auditHost)
	assert.True(t, auditAllowed)
}

func TestProxyServer_PlainHTTP_ForbiddenDomain_Returns403(t *testing.T) {
	var auditAllowed bool
	var auditReason string
	audit := func(host string, allowed bool, reason string) {
		auditAllowed = allowed
		auditReason = reason
	}

	policy := &NetworkPolicy{ForbiddenDomains: []string{"bad.example.com"}}
	srv, err := NewProxyServer(policy, audit)
	require.NoError(t, err)
	defer srv.Close()

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(mustParseURL(srv.URL())),
		},
	}

	resp, err := client.Get("http://bad.example.com/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.False(t, auditAllowed)
	assert.Contains(t, auditReason, "bad.example.com")
}

// ─── ProxyServer CONNECT (HTTPS tunnel) tests ─────────────────────────────

func TestProxyServer_CONNECT_ForbiddenDomain_Returns403(t *testing.T) {
	var auditAllowed bool
	audit := func(host string, allowed bool, reason string) {
		auditAllowed = allowed
	}

	policy := &NetworkPolicy{ForbiddenDomains: []string{"forbidden.example.com"}}
	srv, err := NewProxyServer(policy, audit)
	require.NoError(t, err)
	defer srv.Close()

	// Dial the proxy and send a CONNECT request manually.
	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL(), "http://"))
	require.NoError(t, err)
	defer conn.Close()

	fmt.Fprintf(conn, "CONNECT forbidden.example.com:443 HTTP/1.1\r\nHost: forbidden.example.com:443\r\n\r\n")

	buf := make([]byte, 256)
	n, _ := conn.Read(buf)
	response := string(buf[:n])

	assert.Contains(t, response, "403", "expected 403 Forbidden response")
	assert.False(t, auditAllowed)
}

func TestProxyServer_CONNECT_AllowedDomain_EstablishesTunnel(t *testing.T) {
	// Set up a TCP server to act as the "upstream TLS endpoint".
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	upstreamAddr := listener.Addr().String()
	upstreamHost, upstreamPort, _ := net.SplitHostPort(upstreamAddr)
	_ = upstreamPort

	// Accept one connection and echo back a marker byte.
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.Write([]byte("TUNNEL_OK"))
	}()

	hostPort := upstreamHost + ":" + upstreamPort
	policy := &NetworkPolicy{AllowedDomains: []string{upstreamHost}}
	srv, err := NewProxyServer(policy, nil)
	require.NoError(t, err)
	defer srv.Close()

	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL(), "http://"))
	require.NoError(t, err)
	defer conn.Close()

	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", hostPort, hostPort)

	buf := make([]byte, 256)
	n, _ := conn.Read(buf)
	assert.Contains(t, string(buf[:n]), "200", "expected 200 Connection established")
}

// ─── Audit callback tests ─────────────────────────────────────────────────

func TestProxyServer_AuditCallback_InvokedOnDeny(t *testing.T) {
	var calls []struct {
		host    string
		allowed bool
		reason  string
	}
	audit := func(host string, allowed bool, reason string) {
		calls = append(calls, struct {
			host    string
			allowed bool
			reason  string
		}{host, allowed, reason})
	}

	policy := &NetworkPolicy{ForbiddenDomains: []string{"blocked.example.com"}}
	srv, err := NewProxyServer(policy, audit)
	require.NoError(t, err)
	defer srv.Close()

	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(mustParseURL(srv.URL()))},
	}
	resp, _ := client.Get("http://blocked.example.com/")
	if resp != nil {
		resp.Body.Close()
	}

	require.Len(t, calls, 1)
	assert.Equal(t, "blocked.example.com", calls[0].host)
	assert.False(t, calls[0].allowed)
	assert.Contains(t, calls[0].reason, "blocked.example.com")
}

func TestProxyServer_GlobForbidden_MatchesSubdomains(t *testing.T) {
	var auditHosts []string
	audit := func(host string, allowed bool, reason string) {
		if !allowed {
			auditHosts = append(auditHosts, host)
		}
	}

	policy := &NetworkPolicy{ForbiddenDomains: []string{"*.internal"}}
	srv, err := NewProxyServer(policy, audit)
	require.NoError(t, err)
	defer srv.Close()

	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(mustParseURL(srv.URL()))},
	}

	for _, host := range []string{"api.internal", "db.cluster.internal"} {
		resp, _ := client.Get("http://" + host + "/")
		if resp != nil {
			resp.Body.Close()
		}
	}

	assert.Len(t, auditHosts, 2)
}

// ─── helpers ──────────────────────────────────────────────────────────────

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
