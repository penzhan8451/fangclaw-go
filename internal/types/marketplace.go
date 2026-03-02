package types

type MarketplaceConfig struct {
	RegistryURL string `json:"registry_url"`
	GitHubOrg   string `json:"github_org"`
}

func DefaultMarketplaceConfig() MarketplaceConfig {
	return MarketplaceConfig{
		RegistryURL: "https://api.github.com",
		GitHubOrg:   "openfang-skills",
	}
}

type SkillSearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Stars       uint64 `json:"stars"`
	URL         string `json:"url"`
}

const (
	SkillErrorTypeNetwork   = "network"
	SkillErrorTypeNotFound  = "not_found"
	SkillErrorTypeInvalid   = "invalid"
	SkillErrorTypeIO        = "io"
	SkillErrorTypeSignature = "signature"
)

type ClawHubConfig struct {
	BaseURL      string `json:"base_url"`
	APIToken     string `json:"api_token,omitempty"`
	CacheEnabled bool   `json:"cache_enabled"`
}

func DefaultClawHubConfig() ClawHubConfig {
	return ClawHubConfig{
		BaseURL:      "https://clawhub.io",
		CacheEnabled: true,
	}
}

type ClawHubSkill struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Author      string                 `json:"author"`
	Tags        []string               `json:"tags"`
	Downloads   uint64                 `json:"downloads"`
	Rating      float64                `json:"rating"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type ClawHubSearchResult struct {
	Skills   []ClawHubSkill `json:"skills"`
	Total    uint64         `json:"total"`
	Page     uint32         `json:"page"`
	PageSize uint32         `json:"page_size"`
}
