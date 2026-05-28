package llm

import "fmt"

// ProviderFactory creates a Provider given an API key.
type ProviderFactory func(apiKey string) Provider

var providers = make(map[string]ProviderFactory)

// RegisterProvider registers a named provider factory.
func RegisterProvider(name string, factory ProviderFactory) {
	providers[name] = factory
}

// ModelConfigurable is optionally implemented by providers that support
// model selection. Use GetProviderWithModel to set the model at creation time.
type ModelConfigurable interface {
	SetModel(model string)
}

// GetProvider returns a Provider instance for the given name and API key.
func GetProvider(name, apiKey string) (Provider, error) {
	factory, ok := providers[name]
	if !ok {
		available := make([]string, 0, len(providers))
		for k := range providers {
			available = append(available, k)
		}
		return nil, fmt.Errorf("unknown provider %q (available: %v)", name, available)
	}
	return factory(apiKey), nil
}

// GetProviderWithModel returns a Provider and configures the model if the
// provider supports it. If model is empty, the provider's default is used.
func GetProviderWithModel(name, apiKey, model string) (Provider, error) {
	p, err := GetProvider(name, apiKey)
	if err != nil {
		return nil, err
	}
	if mc, ok := p.(ModelConfigurable); ok && model != "" {
		mc.SetModel(model)
	}
	return p, nil
}
