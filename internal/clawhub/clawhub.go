package clawhub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type ClawHubStats struct {
	Comments        uint64 `json:"comments"`
	Downloads       uint64 `json:"downloads"`
	InstallsAllTime uint64 `json:"installsAllTime"`
	InstallsCurrent uint64 `json:"installsCurrent"`
	Stars           uint64 `json:"stars"`
	Versions        uint64 `json:"versions"`
}

type ClawHubVersionInfo struct {
	Version   string `json:"version"`
	CreatedAt int64  `json:"createdAt"`
	Changelog string `json:"changelog"`
}

type ClawHubOwner struct {
	Handle      string `json:"handle"`
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Image       string `json:"image"`
}

type ClawHubBrowseEntry struct {
	Slug          string              `json:"slug"`
	DisplayName   string              `json:"displayName"`
	Summary       string              `json:"summary"`
	Tags          map[string]string   `json:"tags"`
	Stats         ClawHubStats        `json:"stats"`
	CreatedAt     int64               `json:"createdAt"`
	UpdatedAt     int64               `json:"updatedAt"`
	LatestVersion *ClawHubVersionInfo `json:"latestVersion,omitempty"`
}

type ClawHubBrowseResponse struct {
	Items      []ClawHubBrowseEntry `json:"items"`
	NextCursor *string              `json:"nextCursor"`
}

type ClawHubSearchEntry struct {
	Score       float64 `json:"score"`
	Slug        string  `json:"slug"`
	DisplayName string  `json:"displayName"`
	Summary     string  `json:"summary"`
	Version     string  `json:"version"`
	UpdatedAt   int64   `json:"updatedAt"`
}

type ClawHubSearchResponse struct {
	Results []ClawHubSearchEntry `json:"results"`
}

type ClawHubSkillInfo struct {
	Slug        string            `json:"slug"`
	DisplayName string            `json:"displayName"`
	Summary     string            `json:"summary"`
	Tags        map[string]string `json:"tags"`
	Stats       ClawHubStats      `json:"stats"`
	CreatedAt   int64             `json:"createdAt"`
	UpdatedAt   int64             `json:"updatedAt"`
}

type ClawHubSkillDetail struct {
	Skill         ClawHubSkillInfo    `json:"skill"`
	LatestVersion *ClawHubVersionInfo `json:"latestVersion,omitempty"`
	Owner         *ClawHubOwner       `json:"owner,omitempty"`
	Moderation    interface{}         `json:"moderation,omitempty"`
}

type ClawHubSort string

const (
	ClawHubSortTrending  ClawHubSort = "trending"
	ClawHubSortUpdated   ClawHubSort = "updated"
	ClawHubSortDownloads ClawHubSort = "downloads"
	ClawHubSortStars     ClawHubSort = "stars"
	ClawHubSortRating    ClawHubSort = "rating"
)

type ClawHubClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewClawHubClient() *ClawHubClient {
	return NewClawHubClientWithURL("https://clawhub.ai/api/v1")
}

func NewClawHubClientWithURL(baseURL string) *ClawHubClient {
	return &ClawHubClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *ClawHubClient) Search(query string, limit uint32) (*ClawHubSearchResponse, error) {
	if limit > 50 {
		limit = 50
	}

	u := fmt.Sprintf("%s/search?q=%s&limit=%d", c.baseURL, url.QueryEscape(query), limit)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "FangClawGo/0.1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ClawHub search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ClawHub API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result ClawHubSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ClawHub response: %w", err)
	}

	return &result, nil
}

func (c *ClawHubClient) Browse(sort ClawHubSort, limit uint32, cursor *string) (*ClawHubBrowseResponse, error) {
	if limit > 50 {
		limit = 50
	}

	u := fmt.Sprintf("%s/skills?limit=%d&sort=%s", c.baseURL, limit, sort)

	if cursor != nil {
		u = fmt.Sprintf("%s&cursor=%s", u, url.QueryEscape(*cursor))
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "FangClawGo/0.1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ClawHub browse failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ClawHub browse returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result ClawHubBrowseResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ClawHub browse: %w", err)
	}

	return &result, nil
}

func (c *ClawHubClient) GetSkill(slug string) (*ClawHubSkillDetail, error) {
	u := fmt.Sprintf("%s/skills/%s", c.baseURL, url.QueryEscape(slug))

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "FangClawGo/0.1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ClawHub detail failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ClawHub detail returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result ClawHubSkillDetail
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ClawHub detail: %w", err)
	}

	return &result, nil
}

func (c *ClawHubClient) GetFile(slug string, path string) (string, error) {
	u := fmt.Sprintf("%s/skills/%s/file?path=%s", c.baseURL, url.QueryEscape(slug), url.QueryEscape(path))

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "FangClawGo/0.1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ClawHub file fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ClawHub file returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

func EntryVersion(entry ClawHubBrowseEntry) string {
	if entry.LatestVersion != nil && entry.LatestVersion.Version != "" {
		return entry.LatestVersion.Version
	}
	if v, ok := entry.Tags["latest"]; ok {
		return v
	}
	return ""
}

type ClawHubInstallResult struct {
	SkillName        string
	Version          string
	Slug             string
	Warnings         []SkillWarning
	ToolTranslations []struct {
		From string
		To   string
	}
	IsPromptOnly bool
}

type WarningSeverity string

const (
	WarningSeverityInfo     WarningSeverity = "info"
	WarningSeverityWarning  WarningSeverity = "warning"
	WarningSeverityCritical WarningSeverity = "critical"
)

type SkillWarning struct {
	Severity WarningSeverity
	Message  string
}

func (c *ClawHubClient) Install(slug string, targetDir string) (*ClawHubInstallResult, error) {
	// fmt.Println("[DEBUG] ClawHub.Install called with slug:", slug)
	// fmt.Println("[DEBUG] Target directory:", targetDir)

	var bytes []byte
	var err error

	downloadURL := fmt.Sprintf("%s/download?slug=%s", c.baseURL, url.QueryEscape(slug))
	// fmt.Println("[DEBUG] Trying download URL:", downloadURL)

	var req *http.Request
	req, err = http.NewRequest("GET", downloadURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "FangClawGo/0.1")
		var resp *http.Response
		resp, err = c.httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			// fmt.Println("[DEBUG] Download response status:", resp.Status)
			if resp.StatusCode == http.StatusOK {
				bytes, err = io.ReadAll(resp.Body)
				if err == nil && len(bytes) > 0 {
					// fmt.Println("[DEBUG] Successfully downloaded via /download endpoint, read", len(bytes), "bytes")
				} else {
					// fmt.Println("[DEBUG] /download endpoint returned empty or failed to read, falling back")
					err = fmt.Errorf("empty response")
				}
			} else {
				// fmt.Println("[DEBUG] /download endpoint returned non-OK status, falling back")
				err = fmt.Errorf("status %d", resp.StatusCode)
			}
		} else {
			// fmt.Println("[DEBUG] /download request failed, falling back:", err)
		}
	}

	if err != nil {
		// fmt.Println("[DEBUG] Falling back to /file endpoint")
		fileContent, fileErr := c.GetFile(slug, "SKILL.md")
		if fileErr != nil {
			// fmt.Println("[DEBUG] /file endpoint also failed:", fileErr)
			return nil, fmt.Errorf("both /download and /file endpoints failed: %w", fileErr)
		}
		bytes = []byte(fileContent)
		// fmt.Println("[DEBUG] Successfully downloaded via /file endpoint, read", len(bytes), "bytes")
	}

	skillDir := filepath.Join(targetDir, slug)
	// fmt.Println("[DEBUG] Creating skill directory:", skillDir)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		// fmt.Println("[DEBUG] Failed to create skill directory:", err)
		return nil, fmt.Errorf("failed to create skill directory: %w", err)
	}

	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	// fmt.Println("[DEBUG] Writing SKILL.md to:", skillMDPath)
	if err := os.WriteFile(skillMDPath, bytes, 0644); err != nil {
		// fmt.Println("[DEBUG] Failed to write SKILL.md:", err)
		return nil, fmt.Errorf("failed to write SKILL.md: %w", err)
	}

	// fmt.Println("[DEBUG] Installation complete")
	return &ClawHubInstallResult{
		SkillName:        slug,
		Version:          "1.0.0",
		Slug:             slug,
		Warnings:         []SkillWarning{},
		ToolTranslations: []struct{ From, To string }{},
		IsPromptOnly:     true,
	}, nil
}

func (c *ClawHubClient) IsInstalled(slug string, skillsDir string) bool {
	skillDir := filepath.Join(skillsDir, slug)
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillMDPath); !os.IsNotExist(err) {
		return true
	}
	return false
}
