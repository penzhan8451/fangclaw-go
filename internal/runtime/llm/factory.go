// Package llm provides LLM provider integrations.
package llm

import (
	"fmt"
	"os"

	"github.com/penzhan8451/fangclaw-go/internal/types"
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
	case "deepseek":
		if apiKey == "" {
			apiKey = os.Getenv("DEEPSEEK_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.DeepSeekBaseURL), nil
	case "qwen":
		if apiKey == "" {
			apiKey = os.Getenv("DASHSCOPE_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.QwenBaseURL), nil
	case "zhipu":
		if apiKey == "" {
			apiKey = os.Getenv("ZHIPU_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.ZhipuBaseURL), nil
	case "moonshot":
		if apiKey == "" {
			apiKey = os.Getenv("MOONSHOT_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.MoonshotBaseURL), nil
	case "minimax":
		if apiKey == "" {
			apiKey = os.Getenv("MINIMAX_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.MinimaxBaseURL), nil
	case "qianfan":
		if apiKey == "" {
			apiKey = os.Getenv("QIANFAN_API_KEY")
		}
		return NewOpenAICompatible(apiKey, model, types.QianfanBaseURL), nil
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
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// SupportedProviders returns the list of supported providers.
func SupportedProviders() []string {
	return []string{
		"anthropic", "openai", "groq", "gemini", "openrouter",
		"deepseek", "qwen", "zhipu", "moonshot", "minimax", "qianfan",
	}
}
