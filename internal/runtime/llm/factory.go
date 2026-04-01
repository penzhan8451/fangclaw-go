// Package llm provides LLM provider integrations.
package llm

import (
	"fmt"
	"os"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// Provider API key environment variable names
var ProviderEnvKeys = map[string]string{
	"deepseek":   "DEEPSEEK_API_KEY",
	"qwen":       "DASHSCOPE_API_KEY",
	"zhipu":      "ZHIPU_API_KEY",
	"moonshot":   "MOONSHOT_API_KEY",
	"minimax":    "MINIMAX_API_KEY",
	"qianfan":    "QIANFAN_API_KEY",
	"volcengine": "VOLCENGINE_API_KEY",
	"anthropic":  "ANTHROPIC_API_KEY",
	"openai":     "OPENAI_API_KEY",
	"groq":       "GROQ_API_KEY",
	"gemini":     "GEMINI_API_KEY",
	"openrouter": "OPENROUTER_API_KEY",
}

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
	case "deepseek":
		if apiKey == "" {
			// NOTE: Commented out for multi-tenant mode - use NewDriverWithSecrets instead
			// apiKey = os.Getenv("DEEPSEEK_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.DeepSeekBaseURL), nil
	case "qwen":
		if apiKey == "" {
			// NOTE: Commented out for multi-tenant mode - use NewDriverWithSecrets instead
			// apiKey = os.Getenv("DASHSCOPE_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.QwenBaseURL), nil
	case "zhipu":
		if apiKey == "" {
			// NOTE: Commented out for multi-tenant mode - use NewDriverWithSecrets instead
			// apiKey = os.Getenv("ZHIPU_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.ZhipuBaseURL), nil
	case "moonshot":
		if apiKey == "" {
			// NOTE: Commented out for multi-tenant mode - use NewDriverWithSecrets instead
			// apiKey = os.Getenv("MOONSHOT_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.MoonshotBaseURL), nil
	case "minimax":
		if apiKey == "" {
			// NOTE: Commented out for multi-tenant mode - use NewDriverWithSecrets instead
			// apiKey = os.Getenv("MINIMAX_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.MinimaxBaseURL), nil
	case "qianfan":
		if apiKey == "" {
			// NOTE: Commented out for multi-tenant mode - use NewDriverWithSecrets instead
			// apiKey = os.Getenv("QIANFAN_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.QianfanBaseURL), nil
	case "volcengine":
		if apiKey == "" {
			// NOTE: Commented out for multi-tenant mode - use NewDriverWithSecrets instead
			// apiKey = os.Getenv("VOLCENGINE_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.VolcEngineBaseURL), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// NewDriverWithSecrets creates a new LLM driver with secrets map for API key lookup.
func NewDriverWithSecrets(provider, model string, secrets map[string]string) (Driver, error) {
	apiKey := ""
	if envKey, ok := ProviderEnvKeys[provider]; ok && secrets != nil {
		apiKey = secrets[envKey]
	}

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
	case "deepseek":
		return NewOpenAICompatible(apiKey, model, types.DeepSeekBaseURL), nil
	case "qwen":
		return NewOpenAICompatible(apiKey, model, types.QwenBaseURL), nil
	case "zhipu":
		return NewOpenAICompatible(apiKey, model, types.ZhipuBaseURL), nil
	case "moonshot":
		return NewOpenAICompatible(apiKey, model, types.MoonshotBaseURL), nil
	case "minimax":
		return NewOpenAICompatible(apiKey, model, types.MinimaxBaseURL), nil
	case "qianfan":
		return NewOpenAICompatible(apiKey, model, types.QianfanBaseURL), nil
	case "volcengine":
		return NewOpenAICompatible(apiKey, model, types.VolcEngineBaseURL), nil
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
	case "deepseek":
		apiKey := config.DeepSeek.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("DEEPSEEK_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.DeepSeekBaseURL), nil
	case "qwen":
		apiKey := config.Qwen.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("DASHSCOPE_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.QwenBaseURL), nil
	case "zhipu":
		apiKey := config.Zhipu.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("ZHIPU_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.ZhipuBaseURL), nil
	case "moonshot":
		apiKey := config.Moonshot.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("MOONSHOT_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.MoonshotBaseURL), nil
	case "minimax":
		apiKey := config.MiniMax.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("MINIMAX_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.MinimaxBaseURL), nil
	case "qianfan":
		apiKey := config.Qianfan.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("QIANFAN_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.QianfanBaseURL), nil
	case "volcengine":
		apiKey := config.VolcEngine.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("VOLCENGINE_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.VolcEngineBaseURL), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// SupportedProviders returns the list of supported providers.
func SupportedProviders() []string {
	return []string{
		"anthropic", "openai", "groq", "gemini", "openrouter",
		"deepseek", "qwen", "zhipu", "moonshot", "minimax", "qianfan", "volcengine",
	}
}
