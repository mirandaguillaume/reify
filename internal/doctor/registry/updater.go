package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// maxRegistryBytes is the size limit for downloaded registry YAML (10 MB).
	maxRegistryBytes = 10 << 20
	// maxChecksumBytes is the size limit for downloaded checksum files (1 KB).
	maxChecksumBytes = 1 << 10
)

// Update downloads the latest registry from the active registry's UpdateURL,
// verifies its SHA-256 checksum, and writes it to targetDir/.reify/research-registry.yaml.
// The companion checksum file is fetched from <update_url>.sha256.
func Update(reg *Registry, targetDir string) error {
	return update(reg, targetDir, &http.Client{Timeout: 30 * time.Second})
}

func update(reg *Registry, targetDir string, client *http.Client) error {
	if reg.UpdateURL == "" {
		return fmt.Errorf("no update_url configured in registry")
	}

	url := reg.UpdateURL

	// F3: only allow HTTPS URLs for security
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("update_url must use HTTPS scheme, got: %s", url)
	}

	checksumURL := url + ".sha256"

	// NFR22: show URL before downloading
	fmt.Fprintf(os.Stderr, "Downloading registry from %s ...\n", url)

	// Download registry YAML (capped at 10 MB)
	body, err := httpGet(client, url, maxRegistryBytes)
	if err != nil {
		return fmt.Errorf("download registry: %w", err)
	}

	// Download checksum (capped at 1 KB)
	fmt.Fprintf(os.Stderr, "Verifying checksum from %s ...\n", checksumURL)
	checksumBody, err := httpGet(client, checksumURL, maxChecksumBytes)
	if err != nil {
		return fmt.Errorf("download checksum: %w", err)
	}

	// Verify SHA-256
	expected := strings.TrimSpace(string(checksumBody))
	// Handle "hash  filename" format (sha256sum output)
	if parts := strings.Fields(expected); len(parts) > 0 {
		expected = parts[0]
	}

	actual := sha256Hex(body)
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("checksum verification failed: expected %s, got %s", expected, actual)
	}

	// Parse to validate before writing
	newReg, err := parse(body)
	if err != nil {
		return fmt.Errorf("downloaded registry is invalid YAML: %w", err)
	}

	// Write to .reify/research-registry.yaml
	reifyDir := filepath.Join(targetDir, ".reify")
	if err := os.MkdirAll(reifyDir, 0o755); err != nil {
		return fmt.Errorf("create .reify directory: %w", err)
	}

	// F4: atomic write via temp file + rename (same filesystem)
	outPath := filepath.Join(reifyDir, "research-registry.yaml")
	tmpPath := outPath + ".tmp"
	if err := os.WriteFile(tmpPath, body, 0o644); err != nil {
		return fmt.Errorf("write temp registry to %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		os.Remove(tmpPath) // best-effort cleanup
		return fmt.Errorf("atomic rename to %s: %w", outPath, err)
	}

	fmt.Fprintf(os.Stderr, "Registry updated to version %s (%s)\n", newReg.Version, outPath)
	return nil
}

func httpGet(client *http.Client, url string, maxBytes int64) ([]byte, error) {
	resp, err := client.Get(url) //nolint:noctx // short-lived CLI, no long-running context needed
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	// F1: cap read size to prevent memory exhaustion from malicious servers
	limited := io.LimitReader(resp.Body, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read response body from %s: %w", url, err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("response from %s exceeds size limit (%d bytes)", url, maxBytes)
	}
	return body, nil
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
