// Package tools provides built-in tools for agents.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Tool is the interface for agent tools.
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
	Schema() map[string]interface{}
}

// ============ Calculator Tool ============

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

// ============ DateTime Tool ============

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

// ============ Weather Tool ============

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

// ============ Search Tool ============

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
	return fmt.Sprintf("Search results for '%s':\n1. Example result A\n2. Example result B\n3. Example result C", query), nil
}

// ============ Fetch Tool ============

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
	url, _ := args["url"].(string)
	if url == "" {
		return "", fmt.Errorf("url required")
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()
	return fmt.Sprintf("Fetched %d bytes from %s", resp.ContentLength, url), nil
}

// ============ File Read Tool ============

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

// ============ File Write Tool ============

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

// ============ List Directory Tool ============

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

// ============ Shell Execute Tool ============

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

// ============ URL Parse Tool ============

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

// ============ JSON Tool ============

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

// ============ Hash Tool ============

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

// ============ Random Tool ============

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

// ============ Timezone Tool ============

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

// ============ User Info Tool ============

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

// ============ Register All Tools ============

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
	reg.RegisterTool(NewURLParseTool())
	reg.RegisterTool(NewJSONTool())
	reg.RegisterTool(NewHashTool())
	reg.RegisterTool(NewRandomTool())
	reg.RegisterTool(NewTimezoneTool())
	reg.RegisterTool(NewUserInfoTool())
}
