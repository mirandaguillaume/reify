package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRegistryYAML = `version: "2026.04"
latest_version: "2026.04"
update_url: "REPLACE_ME"
recommendations:
  - id: guardrails
    title: "Add guardrails section"
    citation: "Liu et al., 2024"
    paper: "Lost in the Middle"
    url: "https://arxiv.org/abs/2307.03172"
    finding: "Critical instructions at prompt start improve adherence"
    confidence: high
    detection_prompt: "Are there explicit behavioral constraints?"
    suggestion_prompt: "Add a guardrails section."
`

func testChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// newTLSTestServer creates an HTTPS test server and returns it with its client.
func newTLSTestServer(handler http.Handler) (*httptest.Server, *http.Client) {
	srv := httptest.NewTLSServer(handler)
	return srv, srv.Client()
}

func TestUpdate_Success(t *testing.T) {
	yamlData := []byte(testRegistryYAML)
	checksum := testChecksum(yamlData)

	srv, client := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry.yaml":
			w.Write(yamlData)
		case "/registry.yaml.sha256":
			w.Write([]byte(checksum))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	reg := &Registry{
		Version:   "2026.03",
		UpdateURL: srv.URL + "/registry.yaml",
	}

	targetDir := t.TempDir()
	err := update(reg, targetDir, client)
	require.NoError(t, err)

	// Verify file was written
	outPath := filepath.Join(targetDir, ".reify", "research-registry.yaml")
	written, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, yamlData, written)

	// F4: verify no .tmp file left behind
	_, statErr := os.Stat(outPath + ".tmp")
	assert.True(t, os.IsNotExist(statErr), "temp file should be cleaned up after atomic rename")
}

func TestUpdate_ChecksumMismatch(t *testing.T) {
	yamlData := []byte(testRegistryYAML)

	srv, client := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry.yaml":
			w.Write(yamlData)
		case "/registry.yaml.sha256":
			w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	reg := &Registry{
		Version:   "2026.03",
		UpdateURL: srv.URL + "/registry.yaml",
	}

	targetDir := t.TempDir()
	err := update(reg, targetDir, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum verification failed")

	// Verify file was NOT written
	outPath := filepath.Join(targetDir, ".reify", "research-registry.yaml")
	_, statErr := os.Stat(outPath)
	assert.True(t, os.IsNotExist(statErr))
}

func TestUpdate_NetworkError(t *testing.T) {
	// Use a closed server to simulate network failure
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	client := srv.Client()
	srv.Close()

	reg := &Registry{
		Version:   "2026.03",
		UpdateURL: srv.URL + "/registry.yaml",
	}

	targetDir := t.TempDir()
	err := update(reg, targetDir, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "download registry")
}

func TestUpdate_ChecksumNetworkError(t *testing.T) {
	yamlData := []byte(testRegistryYAML)

	srv, client := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry.yaml":
			w.Write(yamlData)
		case "/registry.yaml.sha256":
			http.Error(w, "not found", http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	reg := &Registry{
		Version:   "2026.03",
		UpdateURL: srv.URL + "/registry.yaml",
	}

	targetDir := t.TempDir()
	err := update(reg, targetDir, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "download checksum")
}

func TestUpdate_NoUpdateURL(t *testing.T) {
	reg := &Registry{Version: "2026.03"}
	err := Update(reg, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no update_url")
}

func TestUpdate_InvalidYAMLDownloaded(t *testing.T) {
	badYAML := []byte("{{invalid yaml content")
	checksum := testChecksum(badYAML)

	srv, client := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry.yaml":
			w.Write(badYAML)
		case "/registry.yaml.sha256":
			w.Write([]byte(checksum))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	reg := &Registry{
		Version:   "2026.03",
		UpdateURL: srv.URL + "/registry.yaml",
	}

	targetDir := t.TempDir()
	err := update(reg, targetDir, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid YAML")
}

func TestUpdate_ChecksumWithFilename(t *testing.T) {
	// sha256sum output format: "hash  filename"
	yamlData := []byte(testRegistryYAML)
	checksum := testChecksum(yamlData) + "  research-registry.yaml"

	srv, client := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry.yaml":
			w.Write(yamlData)
		case "/registry.yaml.sha256":
			w.Write([]byte(checksum))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	reg := &Registry{
		Version:   "2026.03",
		UpdateURL: srv.URL + "/registry.yaml",
	}

	targetDir := t.TempDir()
	err := update(reg, targetDir, client)
	require.NoError(t, err, "should handle 'hash  filename' format")
}

func TestUpdate_HTTPSRequired(t *testing.T) {
	// F3: plain HTTP URLs must be rejected
	reg := &Registry{
		Version:   "2026.03",
		UpdateURL: "http://example.com/registry.yaml",
	}

	err := Update(reg, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS scheme")
}

func TestUpdate_SizeLimitRegistry(t *testing.T) {
	// F1: registry download exceeding maxRegistryBytes must be rejected.
	// Use a small limit via httpGet directly to avoid allocating 10 MB in tests.
	oversized := strings.Repeat("x", 2048)

	srv, client := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(oversized))
	}))
	defer srv.Close()

	// Call httpGet directly with a 1 KB limit
	_, err := httpGet(client, srv.URL+"/big", 1024)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds size limit")
}

func TestUpdate_SizeLimitChecksum(t *testing.T) {
	// F1: checksum file exceeding maxChecksumBytes must be rejected
	oversized := strings.Repeat("a", 2048) // 2 KB > 1 KB limit

	srv, client := newTLSTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(oversized))
	}))
	defer srv.Close()

	_, err := httpGet(client, srv.URL+"/big.sha256", maxChecksumBytes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds size limit")
}

func TestNeedsUpdate_VersionMismatch(t *testing.T) {
	reg := &Registry{Version: "2026.03", LatestVersion: "2026.04"}
	assert.True(t, reg.NeedsUpdate())
}

func TestNeedsUpdate_VersionMatch(t *testing.T) {
	reg := &Registry{Version: "2026.03", LatestVersion: "2026.03"}
	assert.False(t, reg.NeedsUpdate())
}

func TestNeedsUpdate_NoLatestVersion(t *testing.T) {
	reg := &Registry{Version: "2026.03"}
	assert.False(t, reg.NeedsUpdate())
}

func TestNeedsUpdate_Embedded(t *testing.T) {
	// The embedded registry has matching versions, so NeedsUpdate should be false
	reg, err := Load("")
	require.NoError(t, err)
	assert.False(t, reg.NeedsUpdate(), "embedded registry should not need update")
}
