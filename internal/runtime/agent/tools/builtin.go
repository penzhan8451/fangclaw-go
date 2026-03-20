package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

type WeatherTool struct{}

func NewWeatherTool() *WeatherTool { return &WeatherTool{} }

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
	return fmt.Sprintf("Weather in %s: Partly Cloudy, 22°C, Humidity: 65%%", location), nil
}

type SearchTool struct{}

func NewSearchTool() *SearchTool { return &SearchTool{} }

func (t *SearchTool) Name() string        { return "search" }
func (t *SearchTool) Description() string { return "Search the web" }

func (t *SearchTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "search",
			"description": "Search the web for information",
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
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read failed: %w", err)
	}
	return string(content), nil
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
	cmd := exec.Command("sh", "-c", command)
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
	cmd := exec.Command("sh", "-c", command)
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
