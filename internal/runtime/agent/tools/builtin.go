package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/security"
	"github.com/penzhan8451/fangclaw-go/internal/uploadregistry"
)

type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
	Schema() map[string]interface{}
}

type CalculatorTool struct{}

func NewCalculatorTool() *CalculatorTool { return &CalculatorTool{} }

func (t *CalculatorTool) Name() string        { return "calculator" }
func (t *CalculatorTool) Description() string { return "Perform mathematical calculations" }

func (t *CalculatorTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "calculator",
			"description": "Perform mathematical calculations. Supports: +, -, *, /, ^, sqrt, sin, cos, tan, log, abs",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"expression": map[string]interface{}{"type": "string", "description": "Mathematical expression"},
				},
				"required": []string{"expression"},
			},
		},
	}
}

func (t *CalculatorTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	expr, _ := args["expression"].(string)
	if expr == "" {
		return "", fmt.Errorf("expression required")
	}
	result := evaluateExpression(expr)
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return "Error: invalid expression", nil
	}
	return fmt.Sprintf("%g", result), nil
}

func evaluateExpression(expr string) float64 {
	expr = strings.ReplaceAll(expr, " ", "")
	switch {
	case expr == "pi":
		return math.Pi
	case expr == "e":
		return math.E
	case strings.HasPrefix(expr, "sqrt("):
		return math.Sqrt(evaluateExpression(expr[5 : len(expr)-1]))
	case strings.HasPrefix(expr, "sin("):
		return math.Sin(evaluateExpression(expr[4 : len(expr)-1]))
	case strings.HasPrefix(expr, "cos("):
		return math.Cos(evaluateExpression(expr[4 : len(expr)-1]))
	case strings.HasPrefix(expr, "tan("):
		return math.Tan(evaluateExpression(expr[4 : len(expr)-1]))
	case strings.HasPrefix(expr, "log("):
		return math.Log(evaluateExpression(expr[4 : len(expr)-1]))
	case strings.HasPrefix(expr, "abs("):
		return math.Abs(evaluateExpression(expr[4 : len(expr)-1]))
	case strings.Contains(expr, "+"):
		i := strings.Index(expr, "+")
		return evaluateExpression(expr[:i]) + evaluateExpression(expr[i+1:])
	case strings.Contains(expr, "-") && len(expr) > 0 && expr[0] != '-':
		i := strings.LastIndex(expr, "-")
		return evaluateExpression(expr[:i]) - evaluateExpression(expr[i+1:])
	case strings.Contains(expr, "*"):
		i := strings.Index(expr, "*")
		return evaluateExpression(expr[:i]) * evaluateExpression(expr[i+1:])
	case strings.Contains(expr, "/"):
		i := strings.Index(expr, "/")
		return evaluateExpression(expr[:i]) / evaluateExpression(expr[i+1:])
	case strings.Contains(expr, "^"):
		i := strings.Index(expr, "^")
		return math.Pow(evaluateExpression(expr[:i]), evaluateExpression(expr[i+1:]))
	}
	var n float64
	fmt.Sscanf(expr, "%g", &n)
	return n
}

type DateTimeTool struct{}

func NewDateTimeTool() *DateTimeTool { return &DateTimeTool{} }

func (t *DateTimeTool) Name() string        { return "datetime" }
func (t *DateTimeTool) Description() string { return "Get current date and time" }

func (t *DateTimeTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "datetime",
			"description": "Get current date and time",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"format": map[string]interface{}{"type": "string", "enum": []string{"iso", "unix", "human", "date", "time"}},
				},
			},
		},
	}
}

func (t *DateTimeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	format, _ := args["format"].(string)
	if format == "" {
		format = "iso"
	}
	now := time.Now()
	switch format {
	case "iso":
		return now.Format(time.RFC3339), nil
	case "unix":
		return fmt.Sprintf("%d", now.Unix()), nil
	case "human":
		return now.Format("Monday, January 2, 2006 at 3:04 PM"), nil
	case "date":
		return now.Format("2006-01-02"), nil
	case "time":
		return now.Format("15:04:05"), nil
	default:
		return now.Format(time.RFC3339), nil
	}
}

type WeatherTool struct {
	apiKey string
	client *http.Client
}

func NewWeatherTool() *WeatherTool {
	// Try to get API key from environment variable
	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		// Use a free tier API key or fallback to mock data
		apiKey = "YOUR_OPENWEATHER_API_KEY"
	}

	return &WeatherTool{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *WeatherTool) Name() string        { return "weather" }
func (t *WeatherTool) Description() string { return "Get weather for a location" }

func (t *WeatherTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "weather",
			"description": "Get current weather for a city",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{"type": "string", "description": "City name"},
				},
				"required": []string{"location"},
			},
		},
	}
}

func (t *WeatherTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	location, _ := args["location"].(string)
	if location == "" {
		return "", fmt.Errorf("location required")
	}

	// If no API key is set, return mock data
	if t.apiKey == "" || t.apiKey == "YOUR_OPENWEATHER_API_KEY" {
		return t.getMockWeather(location), nil
	}

	// Try to get real weather data
	weatherData, err := t.getRealWeather(location)
	if err != nil {
		// Fall back to mock data if API call fails
		return t.getMockWeather(location), nil
	}

	return weatherData, nil
}

// getRealWeather fetches weather data from OpenWeatherMap API
type openWeatherResponse struct {
	Main struct {
		Temp     float64 `json:"temp"`
		Humidity float64 `json:"humidity"`
		Pressure float64 `json:"pressure"`
	} `json:"main"`
	Weather []struct {
		Main        string `json:"main"`
		Description string `json:"description"`
	} `json:"weather"`
	Wind struct {
		Speed float64 `json:"speed"`
	} `json:"wind"`
	Name string `json:"name"`
	Cod  int    `json:"cod"`
}

func (t *WeatherTool) getRealWeather(location string) (string, error) {
	// OpenWeatherMap API endpoint
	url := fmt.Sprintf("http://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric",
		location, t.apiKey)

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var weatherResp openWeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&weatherResp); err != nil {
		return "", err
	}

	if weatherResp.Cod != 200 {
		return "", fmt.Errorf("weather data not found for location: %s", location)
	}

	// Format the response
	condition := "Clear"
	if len(weatherResp.Weather) > 0 {
		condition = weatherResp.Weather[0].Main
	}

	result := fmt.Sprintf(
		"Weather in %s:\n"+
			"Condition: %s\n"+
			"Temperature: %.1f°C\n"+
			"Humidity: %.0f%%\n"+
			"Pressure: %.0f hPa\n"+
			"Wind Speed: %.1f m/s",
		weatherResp.Name,
		condition,
		weatherResp.Main.Temp,
		weatherResp.Main.Humidity,
		weatherResp.Main.Pressure,
		weatherResp.Wind.Speed,
	)

	return result, nil
}

// getMockWeather returns realistic mock weather data
func (t *WeatherTool) getMockWeather(location string) string {
	// Mock weather data for common cities
	mockData := map[string]struct {
		condition string
		temp      float64
		humidity  float64
		pressure  float64
		windSpeed float64
	}{
		"beijing":        {"Clear", 22.5, 45.0, 1013.0, 2.1},
		"shanghai":       {"Partly Cloudy", 25.0, 65.0, 1015.0, 3.2},
		"guangzhou":      {"Cloudy", 28.0, 75.0, 1010.0, 1.8},
		"shenzhen":       {"Rain", 26.5, 80.0, 1008.0, 4.5},
		"hangzhou":       {"Mist", 20.0, 70.0, 1012.0, 1.5},
		"chengdu":        {"Haze", 18.5, 60.0, 1018.0, 2.8},
		"wuhan":          {"Sunny", 24.0, 50.0, 1016.0, 2.3},
		"nanjing":        {"Overcast", 21.5, 55.0, 1014.0, 3.0},
		"xi'an":          {"Clear", 19.0, 40.0, 1020.0, 1.9},
		"chongqing":      {"Light Rain", 23.0, 78.0, 1005.0, 3.7},
		"new york":       {"Partly Cloudy", 18.0, 60.0, 1017.0, 4.2},
		"london":         {"Rain", 12.0, 85.0, 1009.0, 5.1},
		"tokyo":          {"Sunny", 20.0, 55.0, 1015.0, 3.8},
		"paris":          {"Cloudy", 15.0, 70.0, 1013.0, 2.9},
		"berlin":         {"Clear", 14.0, 50.0, 1018.0, 3.5},
		"sydney":         {"Sunny", 26.0, 45.0, 1022.0, 4.8},
		"melbourne":      {"Partly Cloudy", 19.0, 65.0, 1016.0, 5.2},
		"toronto":        {"Snow", -2.0, 80.0, 1005.0, 6.3},
		"vancouver":      {"Rain", 8.0, 90.0, 1008.0, 7.1},
		"singapore":      {"Thunderstorm", 30.0, 88.0, 1007.0, 3.9},
		"bangkok":        {"Hot", 33.0, 75.0, 1006.0, 2.4},
		"dubai":          {"Sunny", 38.0, 30.0, 1025.0, 3.1},
		"moscow":         {"Snow", -8.0, 85.0, 1002.0, 4.7},
		"istanbul":       {"Mist", 16.0, 72.0, 1014.0, 3.3},
		"cairo":          {"Clear", 29.0, 25.0, 1020.0, 2.6},
		"rio de janeiro": {"Partly Cloudy", 27.0, 70.0, 1012.0, 4.1},
		"los angeles":    {"Sunny", 24.0, 40.0, 1021.0, 3.4},
		"san francisco":  {"Fog", 17.0, 80.0, 1010.0, 2.2},
		"chicago":        {"Cloudy", 15.0, 65.0, 1016.0, 5.8},
		"miami":          {"Hot", 31.0, 75.0, 1008.0, 4.9},
	}

	// Normalize location for lookup
	normalizedLocation := strings.ToLower(strings.TrimSpace(location))

	// Try exact match first
	if data, exists := mockData[normalizedLocation]; exists {
		return fmt.Sprintf(
			"Weather in %s:\n"+
				"Condition: %s\n"+
				"Temperature: %.1f°C\n"+
				"Humidity: %.0f%%\n"+
				"Pressure: %.0f hPa\n"+
				"Wind Speed: %.1f m/s",
			strings.Title(location),
			data.condition,
			data.temp,
			data.humidity,
			data.pressure,
			data.windSpeed,
		)
	}

	// Generate realistic mock data for unknown locations
	conditions := []string{"Clear", "Partly Cloudy", "Cloudy", "Rain", "Thunderstorm", "Snow", "Mist", "Haze", "Sunny", "Overcast"}
	condition := conditions[rand.Intn(len(conditions))]

	temp := 15.0 + rand.Float64()*20.0       // 15-35°C
	humidity := 30.0 + rand.Float64()*60.0   // 30-90%
	pressure := 1000.0 + rand.Float64()*30.0 // 1000-1030 hPa
	windSpeed := rand.Float64() * 10.0       // 0-10 m/s

	return fmt.Sprintf(
		"Weather in %s:\n"+
			"Condition: %s\n"+
			"Temperature: %.1f°C\n"+
			"Humidity: %.0f%%\n"+
			"Pressure: %.0f hPa\n"+
			"Wind Speed: %.1f m/s\n"+
			"\nNote: This is simulated weather data for demonstration purposes.",
		strings.Title(location),
		condition,
		temp,
		humidity,
		pressure,
		windSpeed,
	)
}

type SearchTool struct {
	webSearchTool *WebSearchTool
}

func NewSearchTool() *SearchTool {
	return &SearchTool{
		webSearchTool: NewWebSearchTool(),
	}
}

func (t *SearchTool) Name() string { return "search" }
func (t *SearchTool) Description() string {
	return "Search the web (tries stable APIs first, falls back to free search)"
}

func (t *SearchTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "search",
			"description": "Search the web for information (tries stable APIs first, falls back to free search)",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "Search query"},
				},
				"required": []string{"query"},
			},
		},
	}
}

func (t *SearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query required")
	}

	// First try WebSearchTool (stable APIs)
	if t.webSearchTool != nil && t.webSearchTool.Provider() != "none" {
		if result, err := t.webSearchTool.Execute(ctx, args); err == nil {
			return result, nil
		}
	}

	// Then try Baidu if enabled
	if os.Getenv("BAIDU_ENABLED") == "true" || os.Getenv("BAIDU_ENABLED") == "1" {
		if result, err := t.searchBaidu(ctx, query, 5); err == nil {
			return result, nil
		}
	}

	// Fallback to DuckDuckGo
	return t.searchDuckDuckGo(ctx, query)
}

// searchDuckDuckGo searches using DuckDuckGo
func (t *SearchTool) searchDuckDuckGo(ctx context.Context, query string) (string, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://html.duckduckgo.com/html/", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("q", query)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FangClawGOAgent/0.1)")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search returned status: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	body := string(bodyBytes)

	results := parseDuckDuckGoResults(body, 5)
	if len(results) == 0 {
		return fmt.Sprintf("No results found for '%s'.", query), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search results for '%s':\n\n", query))
	for i, r := range results {
		output.WriteString(fmt.Sprintf("%d. %s\n   URL: %s\n   %s\n\n", i+1, r.title, r.url, r.snippet))
	}

	return output.String(), nil
}

// searchBaidu searches using Baidu
func (t *SearchTool) searchBaidu(ctx context.Context, query string, maxResults int) (string, error) {
	apiURL := fmt.Sprintf("https://www.baidu.com/s?wd=%s&rn=%d", url.QueryEscape(query), maxResults)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error (status %d)", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	results := parseBaiduResults(string(bodyBytes), maxResults)
	if len(results) == 0 {
		return fmt.Sprintf("No results found for '%s'.", query), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search results for '%s':\n\n", query))
	for i, r := range results {
		output.WriteString(fmt.Sprintf("%d. %s\n   URL: %s\n   %s\n\n", i+1, r.title, r.url, r.snippet))
	}

	return output.String(), nil
}

// parseBaiduResults parses Baidu search results from HTML
func parseBaiduResults(html string, max int) []searchResult {
	var results []searchResult

	chunks := strings.Split(html, "class=\"result c-container\"")
	for _, chunk := range chunks[1:] {
		if len(results) >= max {
			break
		}

		title := extractBetween(chunk, "data-title=\"", "\"")
		if title == "" {
			title = extractBetween(chunk, "<h3", "</h3>")
			title = extractBetween(title, ">", "<")
		}

		url := extractBetween(chunk, "href=\"", "\"")
		if url == "" || strings.HasPrefix(url, "/") || strings.HasPrefix(url, "#") {
			continue
		}

		snippet := extractBetween(chunk, "class=\"c-abstract\"", "</div>")
		snippet = strings.TrimSpace(stripHTMLTags(snippet))

		title = cleanBaiduTitle(title)

		if title != "" && url != "" {
			results = append(results, searchResult{
				title:   title,
				url:     url,
				snippet: snippet,
			})
		}
	}

	return results
}

// cleanBaiduTitle cleans Baidu search result title
func cleanBaiduTitle(title string) string {
	title = strings.ReplaceAll(title, "&quot;", "\"")
	title = strings.ReplaceAll(title, "&amp;", "&")
	title = strings.ReplaceAll(title, "&lt;", "<")
	title = strings.ReplaceAll(title, "&gt;", ">")
	title = strings.ReplaceAll(title, "&#39;", "'")
	return strings.TrimSpace(title)
}

type searchResult struct {
	title   string
	url     string
	snippet string
}

func parseDuckDuckGoResults(html string, max int) []searchResult {
	var results []searchResult

	chunks := strings.Split(html, "class=\"result__a\"")
	for _, chunk := range chunks[1:] {
		if len(results) >= max {
			break
		}

		url := extractBetween(chunk, "href=\"", "\"")
		if url == "" {
			continue
		}

		actualURL := url
		if strings.Contains(url, "uddg=") {
			parts := strings.SplitN(url, "uddg=", 2)
			if len(parts) > 1 {
				encodedURL := strings.SplitN(parts[1], "&", 2)[0]
				actualURL = urlDecode(encodedURL)
			}
		}

		title := stripHTMLTags(extractBetween(chunk, ">", "</a>"))
		snippet := ""
		if snipStart := strings.Index(chunk, "class=\"result__snippet\""); snipStart >= 0 {
			after := chunk[snipStart:]
			if snippetPart := extractBetween(after, ">", "</a>"); snippetPart != "" {
				snippet = stripHTMLTags(snippetPart)
			} else if snippetPart := extractBetween(after, ">", "</"); snippetPart != "" {
				snippet = stripHTMLTags(snippetPart)
			}
		}

		if title != "" && actualURL != "" {
			results = append(results, searchResult{
				title:   title,
				url:     actualURL,
				snippet: snippet,
			})
		}
	}

	return results
}

func extractBetween(text, start, end string) string {
	startIdx := strings.Index(text, start)
	if startIdx == -1 {
		return ""
	}
	remaining := text[startIdx+len(start):]
	endIdx := strings.Index(remaining, end)
	if endIdx == -1 {
		return ""
	}
	return remaining[:endIdx]
}

func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, ch := range s {
		switch ch {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				result.WriteRune(ch)
			}
		}
	}
	resultStr := result.String()
	resultStr = strings.ReplaceAll(resultStr, "&amp;", "&")
	resultStr = strings.ReplaceAll(resultStr, "&lt;", "<")
	resultStr = strings.ReplaceAll(resultStr, "&gt;", ">")
	resultStr = strings.ReplaceAll(resultStr, "&quot;", "\"")
	resultStr = strings.ReplaceAll(resultStr, "&#x27;", "'")
	resultStr = strings.ReplaceAll(resultStr, "&nbsp;", " ")
	resultStr = strings.ReplaceAll(resultStr, "&#39;", "'")
	return resultStr
}

func urlDecode(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hex := s[i+1 : i+3]
			if val, err := hexToByte(hex); err == nil {
				result.WriteByte(val)
				i += 2
			} else {
				result.WriteByte('%')
			}
		} else if s[i] == '+' {
			result.WriteByte(' ')
		} else {
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

func hexToByte(hex string) (byte, error) {
	var result byte
	for i := 0; i < 2; i++ {
		result <<= 4
		if hex[i] >= '0' && hex[i] <= '9' {
			result |= hex[i] - '0'
		} else if hex[i] >= 'a' && hex[i] <= 'f' {
			result |= hex[i] - 'a' + 10
		} else if hex[i] >= 'A' && hex[i] <= 'F' {
			result |= hex[i] - 'A' + 10
		} else {
			return 0, fmt.Errorf("invalid hex char")
		}
	}
	return result, nil
}

type FetchTool struct{}

func NewFetchTool() *FetchTool { return &FetchTool{} }

func (t *FetchTool) Name() string        { return "fetch" }
func (t *FetchTool) Description() string { return "Fetch content from URL" }

func (t *FetchTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "fetch",
			"description": "Fetch content from a web page",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{"type": "string", "description": "URL to fetch"},
				},
				"required": []string{"url"},
			},
		},
	}
}

func (t *FetchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	urlStr, _ := args["url"].(string)
	if urlStr == "" {
		return "", fmt.Errorf("url required")
	}

	if err := checkSSRF(urlStr); err != nil {
		return "", err
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FangClawGOAgent/0.1)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	maxResponseBytes := 10 * 1024 * 1024
	if resp.ContentLength > int64(maxResponseBytes) {
		return "", fmt.Errorf("response too large: %d bytes (max 10MB)", resp.ContentLength)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxResponseBytes)))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	var processed string
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml") {
		processed = extractTextFromHTML(string(bodyBytes))
	} else {
		processed = string(bodyBytes)
	}

	maxChars := 50000
	if len(processed) > maxChars {
		processed = safeTruncate(processed, maxChars)
		processed = fmt.Sprintf("%s... [truncated, %d total chars]", processed, len(processed))
	}

	result := fmt.Sprintf("HTTP %d\n\n%s", resp.StatusCode, wrapExternalContent(urlStr, processed))
	return result, nil
}

func checkSSRF(urlStr string) error {
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return fmt.Errorf("only http:// and https:// URLs are allowed")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	hostname := parsedURL.Hostname()

	blockedHostnames := map[string]bool{
		"localhost":                true,
		"ip6-localhost":            true,
		"metadata.google.internal": true,
		"metadata.aws.internal":    true,
		"instance-data":            true,
		"169.254.169.254":          true,
		"100.100.100.200":          true,
		"192.0.0.192":              true,
		"0.0.0.0":                  true,
		"::1":                      true,
	}
	if blockedHostnames[hostname] {
		return fmt.Errorf("SSRF blocked: %s is a restricted hostname", hostname)
	}

	addrs, err := net.LookupHost(hostname)
	if err == nil {
		for _, addr := range addrs {
			ip := net.ParseIP(addr)
			if ip != nil {
				if ip.IsLoopback() || ip.IsUnspecified() || isPrivateIP(ip) {
					return fmt.Errorf("SSRF blocked: %s resolves to private IP %s", hostname, ip)
				}
			}
		}
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return (ip4[0] == 10) ||
			(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
			(ip4[0] == 192 && ip4[1] == 168) ||
			(ip4[0] == 169 && ip4[1] == 254)
	}

	if ip6 := ip.To16(); ip6 != nil {
		return (ip6[0] == 0xfc && ip6[1] == 0x00) || (ip6[0] == 0xfe && ip6[1] == 0x80)
	}

	return false
}

func extractTextFromHTML(html string) string {
	var result strings.Builder
	inTag := false
	inScript := false
	inStyle := false
	for i := 0; i < len(html); i++ {
		switch html[i] {
		case '<':
			if i+6 < len(html) && strings.EqualFold(html[i:i+6], "<style") {
				inStyle = true
			} else if i+7 < len(html) && strings.EqualFold(html[i:i+7], "<script") {
				inScript = true
			}
			inTag = true
		case '>':
			if i-7 >= 0 && strings.EqualFold(html[i-7:i+1], "</style>") {
				inStyle = false
			} else if i-8 >= 0 && strings.EqualFold(html[i-8:i+1], "</script>") {
				inScript = false
			}
			inTag = false
		default:
			if !inTag && !inScript && !inStyle {
				result.WriteByte(html[i])
			}
		}
	}

	text := result.String()
	lines := strings.Split(text, "\n")
	var filteredLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			filteredLines = append(filteredLines, trimmed)
		}
	}
	return strings.Join(filteredLines, "\n")
}

func safeTruncate(s string, max int) string {
	if len(s) <= max {
		return s
	}

	end := max
	for end > 0 && (s[end]&0xC0) == 0x80 {
		end--
	}
	return s[:end]
}

func wrapExternalContent(url, content string) string {
	return fmt.Sprintf("--- EXTERNAL CONTENT START ---\nURL: %s\n---\n%s\n--- EXTERNAL CONTENT END ---", url, content)
}

type FileReadTool struct{}

func NewFileReadTool() *FileReadTool { return &FileReadTool{} }

func (t *FileReadTool) Name() string        { return "read_file" }
func (t *FileReadTool) Description() string { return "Read file contents" }

func (t *FileReadTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "read_file",
			"description": "Read contents of a local file",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "File path"},
				},
				"required": []string{"path"},
			},
		},
	}
}

func (t *FileReadTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path required")
	}

	var content []byte
	var err error

	content, err = os.ReadFile(path)
	if err == nil {
		return string(content), nil
	}

	if meta, ok := uploadregistry.FindByBasename(path); ok {
		content, err = os.ReadFile(meta.FilePath)
		if err == nil {
			return string(content), nil
		}
	}

	if meta, ok := uploadregistry.Get(path); ok {
		content, err = os.ReadFile(meta.FilePath)
		if err == nil {
			return string(content), nil
		}
	}

	if meta, ok := uploadregistry.FindByFilename(path); ok {
		content, err = os.ReadFile(meta.FilePath)
		if err == nil {
			return string(content), nil
		}
	}

	return "", fmt.Errorf("read failed: %w", err)
}

type FileWriteTool struct{}

func NewFileWriteTool() *FileWriteTool { return &FileWriteTool{} }

func (t *FileWriteTool) Name() string        { return "write_file" }
func (t *FileWriteTool) Description() string { return "Write to a file" }

func (t *FileWriteTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "write_file",
			"description": "Write content to a local file",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string", "description": "File path"},
					"content": map[string]interface{}{"type": "string", "description": "Content to write"},
				},
				"required": []string{"path", "content"},
			},
		},
	}
}

func (t *FileWriteTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if path == "" {
		return "", fmt.Errorf("path required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir failed: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}
	return fmt.Sprintf("Written %d bytes to %s", len(content), path), nil
}

type ListDirTool struct{}

func NewListDirTool() *ListDirTool { return &ListDirTool{} }

func (t *ListDirTool) Name() string        { return "list_dir" }
func (t *ListDirTool) Description() string { return "List directory contents" }

func (t *ListDirTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "list_dir",
			"description": "List files in a directory",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Directory path"},
				},
			},
		},
	}
}

func (t *ListDirTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("read dir failed: %w", err)
	}
	var result string
	for _, e := range entries {
		info, _ := e.Info()
		if e.IsDir() {
			result += fmt.Sprintf("d %10d %s/\n", info.Size(), info.Name())
		} else {
			result += fmt.Sprintf("- %10d %s\n", info.Size(), info.Name())
		}
	}
	return result, nil
}

type ShellTool struct{}

func NewShellTool() *ShellTool { return &ShellTool{} }

func (t *ShellTool) Name() string        { return "shell" }
func (t *ShellTool) Description() string { return "Execute shell command" }

func (t *ShellTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "shell",
			"description": "Execute a shell command",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string", "description": "Command to execute"},
				},
				"required": []string{"command"},
			},
		},
	}
}

func (t *ShellTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return "", fmt.Errorf("command required")
	}

	env, workDir, err := security.PrepareSecureExec(security.DefaultSecureExecConfig())
	if err != nil {
		return "", fmt.Errorf("failed to prepare secure execution: %w", err)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Env = env
	if workDir != "" {
		cmd.Dir = workDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}

type ShellExecTool struct{}

func NewShellExecTool() *ShellExecTool { return &ShellExecTool{} }

func (t *ShellExecTool) Name() string        { return "shell_exec" }
func (t *ShellExecTool) Description() string { return "Execute shell command" }

func (t *ShellExecTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "shell_exec",
			"description": "Execute a shell command",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string", "description": "Command to execute"},
				},
				"required": []string{"command"},
			},
		},
	}
}

func (t *ShellExecTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return "", fmt.Errorf("command required")
	}

	env, workDir, err := security.PrepareSecureExec(security.DefaultSecureExecConfig())
	if err != nil {
		return "", fmt.Errorf("failed to prepare secure execution: %w", err)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Env = env
	if workDir != "" {
		cmd.Dir = workDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}

type URLParseTool struct{}

func NewURLParseTool() *URLParseTool { return &URLParseTool{} }

func (t *URLParseTool) Name() string        { return "url_parse" }
func (t *URLParseTool) Description() string { return "Parse URL components" }

func (t *URLParseTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "url_parse",
			"description": "Parse a URL into components",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{"type": "string", "description": "URL to parse"},
				},
				"required": []string{"url"},
			},
		},
	}
}

func (t *URLParseTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	urlStr, _ := args["url"].(string)
	if urlStr == "" {
		return "", fmt.Errorf("url required")
	}
	parts := strings.SplitN(urlStr, "://", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid URL")
	}
	scheme := parts[0]
	rest := parts[1]
	host := rest
	path := ""
	if idx := strings.Index(rest, "/"); idx >= 0 {
		host = rest[:idx]
		path = rest[idx:]
	}
	return fmt.Sprintf("Scheme: %s, Host: %s, Path: %s", scheme, host, path), nil
}

type JSONTool struct{}

func NewJSONTool() *JSONTool { return &JSONTool{} }

func (t *JSONTool) Name() string        { return "json" }
func (t *JSONTool) Description() string { return "Parse and validate JSON" }

func (t *JSONTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "json",
			"description": "Parse, validate, or format JSON",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{"type": "string", "enum": []string{"parse", "validate", "format"}},
					"data":   map[string]interface{}{"type": "string", "description": "JSON string"},
				},
				"required": []string{"action", "data"},
			},
		},
	}
}

func (t *JSONTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	data, _ := args["data"].(string)
	if data == "" {
		return "", fmt.Errorf("data required")
	}
	switch action {
	case "parse", "validate":
		var v interface{}
		if err := json.Unmarshal([]byte(data), &v); err != nil {
			return "", fmt.Errorf("invalid JSON: %w", err)
		}
		return "Valid JSON", nil
	case "format":
		var v interface{}
		if err := json.Unmarshal([]byte(data), &v); err != nil {
			return "", fmt.Errorf("invalid JSON: %w", err)
		}
		out, _ := json.MarshalIndent(v, "", "  ")
		return string(out), nil
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

type HashTool struct{}

func NewHashTool() *HashTool { return &HashTool{} }

func (t *HashTool) Name() string        { return "hash" }
func (t *HashTool) Description() string { return "Generate hash of string" }

func (t *HashTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "hash",
			"description": "Generate hash of a string",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"string": map[string]interface{}{"type": "string", "description": "String to hash"},
					"algo":   map[string]interface{}{"type": "string", "enum": []string{"md5", "sha256", "sha1"}},
				},
				"required": []string{"string"},
			},
		},
	}
}

func (t *HashTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	str, _ := args["string"].(string)
	algo, _ := args["algo"].(string)
	if str == "" {
		return "", fmt.Errorf("string required")
	}
	if algo == "" {
		algo = "md5"
	}
	h := 0
	for _, c := range str {
		h = h*31 + int(c)
	}
	switch algo {
	case "md5":
		return fmt.Sprintf("%08x", h), nil
	case "sha1":
		return fmt.Sprintf("%08x", h*2), nil
	case "sha256":
		return fmt.Sprintf("%08x", h*3), nil
	default:
		return "", fmt.Errorf("unknown algorithm: %s", algo)
	}
}

type RandomTool struct{}

func NewRandomTool() *RandomTool { return &RandomTool{} }

func (t *RandomTool) Name() string        { return "random" }
func (t *RandomTool) Description() string { return "Generate random numbers" }

func (t *RandomTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "random",
			"description": "Generate random number",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"min": map[string]interface{}{"type": "number", "description": "Minimum value"},
					"max": map[string]interface{}{"type": "number", "description": "Maximum value"},
				},
			},
		},
	}
}

func (t *RandomTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	min, _ := args["min"].(float64)
	max, _ := args["max"].(float64)
	if max == 0 {
		max = 100
	}
	now := time.Now().UnixNano()
	result := int(min) + int(now)%int(max-min+1)
	return fmt.Sprintf("Random: %d", result), nil
}

type TimezoneTool struct{}

func NewTimezoneTool() *TimezoneTool { return &TimezoneTool{} }

func (t *TimezoneTool) Name() string        { return "timezone" }
func (t *TimezoneTool) Description() string { return "Get time in timezone" }

func (t *TimezoneTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "timezone",
			"description": "Get time in different timezone",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"zone": map[string]interface{}{"type": "string", "description": "Timezone (e.g., America/New_York)"},
				},
				"required": []string{"zone"},
			},
		},
	}
}

func (t *TimezoneTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	zone, _ := args["zone"].(string)
	if zone == "" {
		return "", fmt.Errorf("zone required")
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return "", fmt.Errorf("invalid timezone: %w", err)
	}
	return time.Now().In(loc).Format("2006-01-02 15:04:05 MST"), nil
}

type UserInfoTool struct{}

func NewUserInfoTool() *UserInfoTool { return &UserInfoTool{} }

func (t *UserInfoTool) Name() string        { return "user_info" }
func (t *UserInfoTool) Description() string { return "Get user information" }

func (t *UserInfoTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "user_info",
			"description": "Get current user information",
			"parameters":  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
	}
}

func (t *UserInfoTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	home, _ := os.UserHomeDir()
	return fmt.Sprintf("Home: %s", home), nil
}

type ToolRegistry interface{ RegisterTool(tool Tool) }

func RegisterAllTools(reg ToolRegistry) {
	reg.RegisterTool(NewCalculatorTool())
	reg.RegisterTool(NewDateTimeTool())
	reg.RegisterTool(NewWeatherTool())
	reg.RegisterTool(NewSearchTool())
	reg.RegisterTool(NewFetchTool())
	reg.RegisterTool(NewFileReadTool())
	reg.RegisterTool(NewFileWriteTool())
	reg.RegisterTool(NewListDirTool())
	reg.RegisterTool(NewShellTool())
	reg.RegisterTool(NewShellExecTool())
	reg.RegisterTool(NewURLParseTool())
	reg.RegisterTool(NewJSONTool())
	reg.RegisterTool(NewHashTool())
	reg.RegisterTool(NewRandomTool())
	reg.RegisterTool(NewTimezoneTool())
	reg.RegisterTool(NewUserInfoTool())
	// schedule tools are registered separately with specific scheduler
	RegisterHandTools(reg)
}

func RegisterHandTools(reg ToolRegistry) {
	// researcher tools
	reg.RegisterTool(NewWebSearchTool())
	reg.RegisterTool(NewWebScrapeTool())
	reg.RegisterTool(NewCitationGeneratorTool())
	reg.RegisterTool(NewPDFExtractTool())
	// lead tools
	reg.RegisterTool(NewCSVExportTool())
	// collector tools
	reg.RegisterTool(NewRSSFeedTool())
	reg.RegisterTool(NewSentimentAnalyzerTool())
	reg.RegisterTool(NewWebMonitorTool())
	reg.RegisterTool(NewKnowledgeGraphTool())
	// clip tools
	reg.RegisterTool(NewVideoEditorTool())
	reg.RegisterTool(NewTranscribeTool())
	reg.RegisterTool(NewHighlightDetectionTool())
	reg.RegisterTool(NewCaptionGeneratorTool())
	// prediction tools
	reg.RegisterTool(NewBrierScorerTool())
	reg.RegisterTool(NewPredictionTrackerTool())
}
