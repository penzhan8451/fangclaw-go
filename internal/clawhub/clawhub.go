package clawhub

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
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

type cacheEntry struct {
	data      interface{}
	timestamp time.Time
}

type ClawHubClient struct {
	baseURL    string
	httpClient *http.Client
	cache      map[string]cacheEntry
	cacheMu    sync.RWMutex
	cacheTTL   time.Duration
}

var defaultClient *ClawHubClient
var defaultClientOnce sync.Once

func NewClawHubClient() *ClawHubClient {
	defaultClientOnce.Do(func() {
		defaultClient = NewClawHubClientWithURL("https://clawhub.ai/api/v1")
	})
	return defaultClient
}

func NewClawHubClientWithURL(baseURL string) *ClawHubClient {
	return &ClawHubClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache:    make(map[string]cacheEntry),
		cacheTTL: 5 * time.Minute,
	}
}

func (c *ClawHubClient) getFromCache(key string) interface{} {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	entry, ok := c.cache[key]
	if !ok {
		return nil
	}
	if time.Since(entry.timestamp) > c.cacheTTL {
		return nil
	}
	return entry.data
}

func (c *ClawHubClient) setCache(key string, data interface{}) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.cache[key] = cacheEntry{
		data:      data,
		timestamp: time.Now(),
	}
}

func (c *ClawHubClient) Search(query string, limit uint32) (*ClawHubSearchResponse, error) {
	if limit > 50 {
		limit = 50
	}

	cacheKey := fmt.Sprintf("search:%s:%d", query, limit)
	if cached := c.getFromCache(cacheKey); cached != nil {
		return cached.(*ClawHubSearchResponse), nil
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

	c.setCache(cacheKey, &result)
	return &result, nil
}

func (c *ClawHubClient) Browse(sort ClawHubSort, limit uint32, cursor *string) (*ClawHubBrowseResponse, error) {
	if limit > 50 {
		limit = 50
	}

	cacheKey := fmt.Sprintf("browse:%s:%d:%v", sort, limit, cursor)
	if cached := c.getFromCache(cacheKey); cached != nil {
		return cached.(*ClawHubBrowseResponse), nil
	}

	trySort := func(s ClawHubSort) (*ClawHubBrowseResponse, error) {
		u := fmt.Sprintf("%s/skills?limit=%d&sort=%s", c.baseURL, limit, s)
		if cursor != nil {
			u = fmt.Sprintf("%s&cursor=%s", u, url.QueryEscape(*cursor))
		}

		log.Debug().Str("url", u).Msg("ClawHub requesting URL")
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "FangClawGo/0.1")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			log.Warn().Err(err).Msg("ClawHub request failed")
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var result ClawHubBrowseResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}
		return &result, nil
	}

	result, err := trySort(sort)
	if err != nil && sort != ClawHubSortTrending {
		result, err = trySort(ClawHubSortTrending)
	}

	if err != nil {
		return &ClawHubBrowseResponse{
			Items: []ClawHubBrowseEntry{},
		}, nil
	}

	c.setCache(cacheKey, result)
	return result, nil
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

	var data []byte
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
				data, err = io.ReadAll(resp.Body)
				if err == nil && len(data) > 0 {
					// fmt.Println("[DEBUG] Successfully downloaded via /download endpoint, read", len(data), "bytes")
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
		data = []byte(fileContent)
		// fmt.Println("[DEBUG] Successfully downloaded via /file endpoint, read", len(data), "bytes")
	}

	skillDir := filepath.Join(targetDir, slug)
	// fmt.Println("[DEBUG] Creating skill directory:", skillDir)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		// fmt.Println("[DEBUG] Failed to create skill directory:", err)
		return nil, fmt.Errorf("failed to create skill directory: %w", err)
	}

	// Check if data are a zip archive
	zipReader, zipErr := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if zipErr == nil {
		// It's a zip file! Extract all contents
		// fmt.Println("[DEBUG] Detected zip archive, extracting contents")
		for _, file := range zipReader.File {
			destPath := filepath.Join(skillDir, file.Name)
			// Ensure dest path is within skill dir to prevent path traversal
			destPath = filepath.Clean(destPath)
			if !strings.HasPrefix(destPath, filepath.Clean(skillDir)+string(os.PathSeparator)) && destPath != filepath.Clean(skillDir) {
				return nil, fmt.Errorf("invalid file path in zip: %s", file.Name)
			}

			if file.FileInfo().IsDir() {
				// Create directory
				if err := os.MkdirAll(destPath, file.Mode()); err != nil {
					return nil, fmt.Errorf("failed to create directory from zip: %w", err)
				}
				continue
			}

			// Create parent directory
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return nil, fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Open file inside zip
			srcFile, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open zip entry: %w", err)
			}

			// Create dest file
			destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				srcFile.Close()
				return nil, fmt.Errorf("failed to create dest file: %w", err)
			}

			// Copy content
			if _, err := io.Copy(destFile, srcFile); err != nil {
				srcFile.Close()
				destFile.Close()
				return nil, fmt.Errorf("failed to copy zip file content: %w", err)
			}

			srcFile.Close()
			destFile.Close()
		}
	} else {
		// Not a zip, just write as SKILL.md
		skillMDPath := filepath.Join(skillDir, "SKILL.md")
		// fmt.Println("[DEBUG] Writing SKILL.md to:", skillMDPath)
		if err := os.WriteFile(skillMDPath, data, 0644); err != nil {
			// fmt.Println("[DEBUG] Failed to write SKILL.md:", err)
			return nil, fmt.Errorf("failed to write SKILL.md: %w", err)
		}
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
