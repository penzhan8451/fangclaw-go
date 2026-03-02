// Package llm provides LLM provider integrations.
package llm

import (
	"fmt"
)

// NewDriver creates a new LLM driver based on the provider name.
func NewDriver(provider, apiKey, model string) (Driver, error) {
	switch provider {
	case "anthropic":
		return NewAnthropic(apiKey, model), nil
	case "openai":
		return NewOpenAI(apiKey, model), nil
	case "groq":
		return NewGroq(apiKey, model), nil
	case "gemini":
		return NewGemini(apiKey, model), nil
	case "openrouter":
		return NewOpenRouter(apiKey, model), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// NewDriverFromConfig creates a new LLM driver from config.
func NewDriverFromConfig(config *Config, provider, model string) (Driver, error) {
	switch provider {
	case "anthropic":
		return NewAnthropic(config.Anthropic.APIKey, model), nil
	case "openai":
		return NewOpenAI(config.OpenAI.APIKey, model), nil
	case "groq":
		return NewGroq(config.Groq.APIKey, model), nil
	case "gemini":
		return NewGemini(config.Gemini.APIKey, model), nil
	case "openrouter":
		return NewOpenRouter(config.OpenRouter.APIKey, model), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// SupportedProviders returns the list of supported providers.
func SupportedProviders() []string {
	return []string{"anthropic", "openai", "groq", "gemini", "openrouter"}
}
