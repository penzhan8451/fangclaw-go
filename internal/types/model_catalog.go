package types

import (
	"encoding/json"
)

const (
	AnthropicBaseURL     = "https://api.anthropic.com"
	OpenAIBaseURL        = "https://api.openai.com/v1"
	GeminiBaseURL        = "https://generativelanguage.googleapis.com"
	DeepSeekBaseURL      = "https://api.deepseek.com/v1"
	GroqBaseURL          = "https://api.groq.com/openai/v1"
	OpenRouterBaseURL    = "https://openrouter.ai/api/v1"
	MistralBaseURL       = "https://api.mistral.ai/v1"
	TogetherBaseURL      = "https://api.together.xyz/v1"
	FireworksBaseURL     = "https://api.fireworks.ai/inference/v1"
	OllamaBaseURL        = "http://localhost:11434/v1"
	VLLMBaseURL          = "http://localhost:8000/v1"
	LMStudioBaseURL      = "http://localhost:1234/v1"
	PerplexityBaseURL    = "https://api.perplexity.ai"
	CohereBaseURL        = "https://api.cohere.com/v2"
	AI21BaseURL          = "https://api.ai21.com/studio/v1"
	CerebrasBaseURL      = "https://api.cerebras.ai/v1"
	SambaNovaBaseURL     = "https://api.sambanova.ai/v1"
	HuggingFaceBaseURL   = "https://api-inference.huggingface.co/v1"
	XAIBaseURL           = "https://api.x.ai/v1"
	ReplicateBaseURL     = "https://api.replicate.com/v1"
	GitHubCopilotBaseURL = "https://api.githubcopilot.com"
	QwenBaseURL          = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	MinimaxBaseURL       = "https://api.minimax.chat/v1"
	ZhipuBaseURL         = "https://open.bigmodel.cn/api/paas/v4"
	ZhipuCodingBaseURL   = "https://open.bigmodel.cn/api/paas/v4"
	MoonshotBaseURL      = "https://api.moonshot.cn/v1"
	QianfanBaseURL       = "https://qianfan.baidubce.com/v2"
	BedrockBaseURL       = "https://bedrock-runtime.us-east-1.amazonaws.com"
)

type ModelTier string

const (
	ModelTierFrontier ModelTier = "frontier"
	ModelTierSmart    ModelTier = "smart"
	ModelTierBalanced ModelTier = "balanced"
	ModelTierFast     ModelTier = "fast"
	ModelTierLocal    ModelTier = "local"
	ModelTierCustom   ModelTier = "custom"
)

type AuthStatus string

const (
	AuthStatusConfigured  AuthStatus = "configured"
	AuthStatusMissing     AuthStatus = "missing"
	AuthStatusNotRequired AuthStatus = "not_required"
)

type ModelCatalogEntry struct {
	ID                string    `json:"id"`
	ModelName         string    `json:"model_name"`
	DisplayName       string    `json:"display_name"`
	Provider          string    `json:"provider"`
	Tier              ModelTier `json:"tier"`
	ContextWindow     int64     `json:"context_window"`
	MaxOutputTokens   int64     `json:"max_output_tokens"`
	InputCostPerM     float64   `json:"input_cost_per_m"`
	OutputCostPerM    float64   `json:"output_cost_per_m"`
	SupportsTools     bool      `json:"supports_tools"`
	SupportsVision    bool      `json:"supports_vision"`
	SupportsStreaming bool      `json:"supports_streaming"`
	Aliases           []string  `json:"aliases"`
}

type ProviderInfo struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	APIKeyEnv   string     `json:"api_key_env"`
	BaseURL     string     `json:"base_url"`
	KeyRequired bool       `json:"key_required"`
	AuthStatus  AuthStatus `json:"auth_status"`
	ModelCount  int        `json:"model_count"`
}

func (t ModelTier) String() string {
	return string(t)
}

func (s AuthStatus) String() string {
	return string(s)
}

func (e *ModelCatalogEntry) UnmarshalJSON(data []byte) error {
	type Alias ModelCatalogEntry
	aux := &struct{ *Alias }{Alias: (*Alias)(e)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if e.Tier == "" {
		e.Tier = ModelTierBalanced
	}
	return nil
}

func (p *ProviderInfo) UnmarshalJSON(data []byte) error {
	type Alias ProviderInfo
	aux := &struct{ *Alias }{Alias: (*Alias)(p)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if p.AuthStatus == "" {
		p.AuthStatus = AuthStatusMissing
	}
	return nil
}

func BuiltinModels() []ModelCatalogEntry {
	return []ModelCatalogEntry{
		{
			ID:                "anthropic:claude-3-5-sonnet-20241022",
			ModelName:         "claude-3-5-sonnet-20241022",
			DisplayName:       "Claude 3.5 Sonnet",
			Provider:          "anthropic",
			Tier:              ModelTierSmart,
			ContextWindow:     200000,
			MaxOutputTokens:   8192,
			InputCostPerM:     3.0,
			OutputCostPerM:    15.0,
			SupportsTools:     true,
			SupportsVision:    true,
			SupportsStreaming: true,
			Aliases:           []string{"sonnet", "claude-sonnet"},
		},
		{
			ID:                "anthropic:claude-3-opus-20240229",
			ModelName:         "claude-3-opus-20240229",
			DisplayName:       "Claude 3 Opus",
			Provider:          "anthropic",
			Tier:              ModelTierFrontier,
			ContextWindow:     200000,
			MaxOutputTokens:   4096,
			InputCostPerM:     15.0,
			OutputCostPerM:    75.0,
			SupportsTools:     true,
			SupportsVision:    true,
			SupportsStreaming: true,
			Aliases:           []string{"opus", "claude-opus"},
		},
		{
			ID:                "openai:gpt-4o",
			ModelName:         "gpt-4o",
			DisplayName:       "GPT-4o",
			Provider:          "openai",
			Tier:              ModelTierSmart,
			ContextWindow:     128000,
			MaxOutputTokens:   4096,
			InputCostPerM:     5.0,
			OutputCostPerM:    15.0,
			SupportsTools:     true,
			SupportsVision:    true,
			SupportsStreaming: true,
			Aliases:           []string{"gpt-4o"},
		},
		{
			ID:                "openai:gpt-4o-mini",
			ModelName:         "gpt-4o-mini",
			DisplayName:       "GPT-4o Mini",
			Provider:          "openai",
			Tier:              ModelTierBalanced,
			ContextWindow:     128000,
			MaxOutputTokens:   16384,
			InputCostPerM:     0.15,
			OutputCostPerM:    0.6,
			SupportsTools:     true,
			SupportsVision:    true,
			SupportsStreaming: true,
			Aliases:           []string{"gpt-4o-mini"},
		},
		{
			ID:                "openai:gpt-4-turbo",
			ModelName:         "gpt-4-turbo",
			DisplayName:       "GPT-4 Turbo",
			Provider:          "openai",
			Tier:              ModelTierSmart,
			ContextWindow:     128000,
			MaxOutputTokens:   4096,
			InputCostPerM:     10.0,
			OutputCostPerM:    30.0,
			SupportsTools:     true,
			SupportsVision:    true,
			SupportsStreaming: true,
			Aliases:           []string{"gpt-4-turbo"},
		},
		{
			ID:                "openai:gpt-3.5-turbo",
			ModelName:         "gpt-3.5-turbo",
			DisplayName:       "GPT-3.5 Turbo",
			Provider:          "openai",
			Tier:              ModelTierFast,
			ContextWindow:     16384,
			MaxOutputTokens:   4096,
			InputCostPerM:     0.5,
			OutputCostPerM:    1.5,
			SupportsTools:     true,
			SupportsVision:    false,
			SupportsStreaming: true,
			Aliases:           []string{"gpt-3.5-turbo"},
		},
		{
			ID:                "gemini:gemini-2.0-flash",
			ModelName:         "gemini-2.0-flash",
			DisplayName:       "Gemini 2.0 Flash",
			Provider:          "gemini",
			Tier:              ModelTierBalanced,
			ContextWindow:     1048576,
			MaxOutputTokens:   8192,
			InputCostPerM:     0.10,
			OutputCostPerM:    0.40,
			SupportsTools:     true,
			SupportsVision:    true,
			SupportsStreaming: true,
			Aliases:           []string{"gemini-2.0-flash"},
		},
		{
			ID:                "gemini:gemini-1.5-pro",
			ModelName:         "gemini-1.5-pro",
			DisplayName:       "Gemini 1.5 Pro",
			Provider:          "gemini",
			Tier:              ModelTierSmart,
			ContextWindow:     2097152,
			MaxOutputTokens:   8192,
			InputCostPerM:     3.5,
			OutputCostPerM:    10.5,
			SupportsTools:     true,
			SupportsVision:    true,
			SupportsStreaming: true,
			Aliases:           []string{"gemini-1.5-pro"},
		},
		{
			ID:                "deepseek:deepseek-chat",
			ModelName:         "deepseek-chat",
			DisplayName:       "DeepSeek Chat",
			Provider:          "deepseek",
			Tier:              ModelTierSmart,
			ContextWindow:     64000,
			MaxOutputTokens:   4096,
			InputCostPerM:     0.55,
			OutputCostPerM:    2.19,
			SupportsTools:     true,
			SupportsVision:    false,
			SupportsStreaming: true,
			Aliases:           []string{"deepseek-chat"},
		},
		{
			ID:                "openrouter:anthropic/claude-3-5-sonnet",
			ModelName:         "anthropic/claude-3-5-sonnet",
			DisplayName:       "Claude 3.5 Sonnet (OpenRouter)",
			Provider:          "openrouter",
			Tier:              ModelTierSmart,
			ContextWindow:     200000,
			MaxOutputTokens:   8192,
			InputCostPerM:     3.0,
			OutputCostPerM:    15.0,
			SupportsTools:     true,
			SupportsVision:    true,
			SupportsStreaming: true,
			Aliases:           []string{"openrouter/claude-3-5-sonnet"},
		},
	}
}

func BuiltinAliases() map[string]string {
	return map[string]string{
		"sonnet":           "anthropic:claude-3-5-sonnet-20241022",
		"claude-sonnet":    "anthropic:claude-3-5-sonnet-20241022",
		"opus":             "anthropic:claude-3-opus-20240229",
		"claude-opus":      "anthropic:claude-3-opus-20240229",
		"gpt-4o":           "openai:gpt-4o",
		"gpt-4o-mini":      "openai:gpt-4o-mini",
		"gpt-4-turbo":      "openai:gpt-4-turbo",
		"gpt-3.5-turbo":    "openai:gpt-3.5-turbo",
		"gemini-2.0-flash": "gemini:gemini-2.0-flash",
		"gemini-1.5-pro":   "gemini:gemini-1.5-pro",
		"deepseek-chat":    "deepseek:deepseek-chat",
	}
}

// ModelCatalogFile represents the structure for model catalog JSON file.
type ModelCatalogFile struct {
	Version   string              `json:"version"`
	Providers []ProviderInfo      `json:"providers"`
	Models    []ModelCatalogEntry `json:"models"`
}

func BuiltinProviders() []ProviderInfo {
	return []ProviderInfo{
		{
			ID:          "anthropic",
			DisplayName: "Anthropic",
			APIKeyEnv:   "ANTHROPIC_API_KEY",
			BaseURL:     AnthropicBaseURL,
			KeyRequired: true,
			AuthStatus:  AuthStatusMissing,
			ModelCount:  0,
		},
		{
			ID:          "openai",
			DisplayName: "OpenAI",
			APIKeyEnv:   "OPENAI_API_KEY",
			BaseURL:     OpenAIBaseURL,
			KeyRequired: true,
			AuthStatus:  AuthStatusMissing,
			ModelCount:  0,
		},
		{
			ID:          "gemini",
			DisplayName: "Google Gemini",
			APIKeyEnv:   "GEMINI_API_KEY",
			BaseURL:     GeminiBaseURL,
			KeyRequired: true,
			AuthStatus:  AuthStatusMissing,
			ModelCount:  0,
		},
		{
			ID:          "deepseek",
			DisplayName: "DeepSeek",
			APIKeyEnv:   "DEEPSEEK_API_KEY",
			BaseURL:     DeepSeekBaseURL,
			KeyRequired: true,
			AuthStatus:  AuthStatusMissing,
			ModelCount:  0,
		},
		{
			ID:          "openrouter",
			DisplayName: "OpenRouter",
			APIKeyEnv:   "OPENROUTER_API_KEY",
			BaseURL:     OpenRouterBaseURL,
			KeyRequired: true,
			AuthStatus:  AuthStatusMissing,
			ModelCount:  0,
		},
		{
			ID:          "ollama",
			DisplayName: "Ollama",
			APIKeyEnv:   "",
			BaseURL:     OllamaBaseURL,
			KeyRequired: false,
			AuthStatus:  AuthStatusNotRequired,
			ModelCount:  0,
		},
		{
			ID:          "vllm",
			DisplayName: "vLLM",
			APIKeyEnv:   "",
			BaseURL:     VLLMBaseURL,
			KeyRequired: false,
			AuthStatus:  AuthStatusNotRequired,
			ModelCount:  0,
		},
		{
			ID:          "lmstudio",
			DisplayName: "LM Studio",
			APIKeyEnv:   "",
			BaseURL:     LMStudioBaseURL,
			KeyRequired: false,
			AuthStatus:  AuthStatusNotRequired,
			ModelCount:  0,
		},
	}
}
