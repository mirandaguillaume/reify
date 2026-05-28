package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mirandaguillaume/reify/internal/llm"
)

// selectProvider chooses an LLM provider using the fallback chain:
// 1. Explicit --provider flag
// 2. Ollama (if running locally)
// 3. ANTHROPIC_REIFY_API_KEY → anthropic
// 4. OPENROUTER_REIFY_API_KEY → openrouter
// 5. OPENROUTER_API_KEY env var → openrouter
// 6. ANTHROPIC_API_KEY env var  → anthropic
// 7. REIFY_API_KEY env var    → anthropic
// 8. Error (no provider available)
func selectProvider(providerFlag, modelFlag string, debug bool) (llm.Provider, string, error) {
	reifyKey := os.Getenv("REIFY_API_KEY")
	reifyAnthropic := os.Getenv("ANTHROPIC_REIFY_API_KEY")

	if providerFlag != "" {
		apiKey := ""
		switch providerFlag {
		case "ollama":
			// No API key needed
		case "openrouter":
			apiKey = firstNonEmpty(os.Getenv("OPENROUTER_REIFY_API_KEY"), os.Getenv("OPENROUTER_API_KEY"), reifyKey)
			if apiKey == "" {
				return nil, "", fmt.Errorf("OPENROUTER_REIFY_API_KEY or OPENROUTER_API_KEY not set")
			}
		case "anthropic":
			apiKey = firstNonEmpty(reifyAnthropic, os.Getenv("ANTHROPIC_API_KEY"), reifyKey)
			if apiKey == "" {
				return nil, "", fmt.Errorf("ANTHROPIC_API_KEY or REIFY_ANTHROPIC not set")
			}
		default:
			return nil, "", fmt.Errorf("unknown provider %q (available: ollama, openrouter, anthropic)", providerFlag)
		}
		p, err := llm.GetProviderWithModel(providerFlag, apiKey, modelFlag)
		if err != nil {
			return nil, "", err
		}
		return p, providerFlag, nil
	}

	if ollamaRunning(debug) {
		p, err := llm.GetProvider("ollama", "")
		if err == nil {
			return p, "ollama (auto-detected)", nil
		}
	}

	if reifyAnthropic != "" {
		p, err := llm.GetProvider("anthropic", reifyAnthropic)
		if err == nil {
			return p, "anthropic (via ANTHROPIC_REIFY_API_KEY)", nil
		}
	}

	if key := firstNonEmpty(os.Getenv("OPENROUTER_REIFY_API_KEY"), os.Getenv("OPENROUTER_API_KEY"), reifyKey); key != "" {
		p, err := llm.GetProvider("openrouter", key)
		if err == nil {
			return p, "openrouter (via OPENROUTER_API_KEY)", nil
		}
	}

	if key := firstNonEmpty(os.Getenv("ANTHROPIC_API_KEY"), reifyKey); key != "" {
		p, err := llm.GetProvider("anthropic", key)
		if err == nil {
			return p, "anthropic (via ANTHROPIC_API_KEY)", nil
		}
	}

	return nil, "", fmt.Errorf("no LLM provider available. Install Ollama or set ANTHROPIC_REIFY_API_KEY / OPENROUTER_REIFY_API_KEY / ANTHROPIC_API_KEY")
}

func ollamaRunning(debug bool) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Ollama not detected: %v\n", err)
		}
		return false
	}
	defer resp.Body.Close()

	if debug {
		var tags struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.NewDecoder(resp.Body).Decode(&tags) == nil {
			var names []string
			for _, m := range tags.Models {
				names = append(names, m.Name)
			}
			fmt.Fprintf(os.Stderr, "[DEBUG] Ollama running, models: %v\n", names)
		}
	}

	return resp.StatusCode == http.StatusOK
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// isOllamaProvider returns true if the provider name indicates Ollama.
func isOllamaProvider(name string) bool {
	return strings.HasPrefix(name, "ollama")
}

// canonicalProvider extracts the short provider name from display names
// like "ollama (auto-detected)" or "openrouter (via OPENROUTER_API_KEY)".
func canonicalProvider(name string) string {
	if idx := strings.Index(name, " ("); idx > 0 {
		return name[:idx]
	}
	return name
}
