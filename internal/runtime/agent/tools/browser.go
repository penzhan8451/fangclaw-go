package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type BrowserManager interface {
	Navigate(agentID, url string) (string, error)
	Click(agentID, selector string) (string, error)
	Type(agentID, selector, text string) (string, error)
	Screenshot(agentID string) (map[string]interface{}, error)
	ReadPage(agentID string) (string, error)
	Scroll(agentID, direction string, amount int) (string, error)
	Wait(agentID, selector string, timeoutMs int) (string, error)
	WaitForUser(agentID, prompt string) (string, error)
	RunJs(agentID, expression string) (interface{}, error)
	Back(agentID string) (string, error)
	Close(agentID string)
	HasSession(agentID string) bool
}

type BrowserToolContext struct {
	BrowserMgr BrowserManager
	AgentID    string
}

func checkBrowserSSRF(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("missing 'url' parameter")
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("invalid URL: missing host")
	}

	blockedHosts := []string{
		"localhost", "127.0.0.1", "0.0.0.0", "::1",
		"169.254.169.254",
		"10.", "172.16.", "172.17.", "172.18.", "172.19.",
		"172.20.", "172.21.", "172.22.", "172.23.",
		"172.24.", "172.25.", "172.26.", "172.27.",
		"172.28.", "172.29.", "172.30.", "172.31.",
		"192.168.",
	}

	for _, blocked := range blockedHosts {
		if host == blocked || strings.HasPrefix(host, blocked) {
			return fmt.Errorf("SSRF blocked: cannot access internal address %s", host)
		}
	}

	return nil
}

type BrowserNavigateTool struct {
	ctx *BrowserToolContext
}

func NewBrowserNavigateTool(ctx *BrowserToolContext) *BrowserNavigateTool {
	return &BrowserNavigateTool{ctx: ctx}
}

func (t *BrowserNavigateTool) Name() string { return "browser_navigate" }
func (t *BrowserNavigateTool) Description() string {
	return "Navigate the browser to a URL. Use this to open web pages for automation."
}

func (t *BrowserNavigateTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "browser_navigate",
			"description": "Navigate the browser to a URL. Use this to open web pages for automation.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL to navigate to",
					},
				},
				"required": []string{"url"},
			},
		},
	}
}

func (t *BrowserNavigateTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx.BrowserMgr == nil {
		return "", fmt.Errorf("browser tools not available. Ensure Chrome/Chromium is installed")
	}

	urlStr, _ := args["url"].(string)
	urlStr = strings.TrimSpace(urlStr)
	urlStr = strings.Trim(urlStr, "`'\"")

	if err := checkBrowserSSRF(urlStr); err != nil {
		return "", err
	}

	return t.ctx.BrowserMgr.Navigate(t.ctx.AgentID, urlStr)
}

type BrowserClickTool struct {
	ctx *BrowserToolContext
}

func NewBrowserClickTool(ctx *BrowserToolContext) *BrowserClickTool {
	return &BrowserClickTool{ctx: ctx}
}

func (t *BrowserClickTool) Name() string { return "browser_click" }
func (t *BrowserClickTool) Description() string {
	return "Click an element on the page by CSS selector or visible text."
}

func (t *BrowserClickTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "browser_click",
			"description": "Click an element on the page by CSS selector or visible text.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector or visible text of the element to click",
					},
				},
				"required": []string{"selector"},
			},
		},
	}
}

func (t *BrowserClickTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx.BrowserMgr == nil {
		return "", fmt.Errorf("browser tools not available. Ensure Chrome/Chromium is installed")
	}

	selector, _ := args["selector"].(string)
	if selector == "" {
		return "", fmt.Errorf("missing 'selector' parameter")
	}

	return t.ctx.BrowserMgr.Click(t.ctx.AgentID, selector)
}

type BrowserTypeTool struct {
	ctx *BrowserToolContext
}

func NewBrowserTypeTool(ctx *BrowserToolContext) *BrowserTypeTool {
	return &BrowserTypeTool{ctx: ctx}
}

func (t *BrowserTypeTool) Name() string { return "browser_type" }
func (t *BrowserTypeTool) Description() string {
	return "Type text into an input field on the page."
}

func (t *BrowserTypeTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "browser_type",
			"description": "Type text into an input field on the page.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector for the input field",
					},
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Text to type into the field",
					},
				},
				"required": []string{"selector", "text"},
			},
		},
	}
}

func (t *BrowserTypeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx.BrowserMgr == nil {
		return "", fmt.Errorf("browser tools not available. Ensure Chrome/Chromium is installed")
	}

	selector, _ := args["selector"].(string)
	text, _ := args["text"].(string)

	if selector == "" {
		return "", fmt.Errorf("missing 'selector' parameter")
	}
	if text == "" {
		return "", fmt.Errorf("missing 'text' parameter")
	}

	return t.ctx.BrowserMgr.Type(t.ctx.AgentID, selector, text)
}

type BrowserScreenshotTool struct {
	ctx *BrowserToolContext
}

func NewBrowserScreenshotTool(ctx *BrowserToolContext) *BrowserScreenshotTool {
	return &BrowserScreenshotTool{ctx: ctx}
}

func (t *BrowserScreenshotTool) Name() string { return "browser_screenshot" }
func (t *BrowserScreenshotTool) Description() string {
	return "Take a screenshot of the current page."
}

func (t *BrowserScreenshotTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "browser_screenshot",
			"description": "Take a screenshot of the current page.",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

func (t *BrowserScreenshotTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx.BrowserMgr == nil {
		return "", fmt.Errorf("browser tools not available. Ensure Chrome/Chromium is installed")
	}

	result, err := t.ctx.BrowserMgr.Screenshot(t.ctx.AgentID)
	if err != nil {
		return "", err
	}

	if imgURL, ok := result["image_url"].(string); ok {
		return fmt.Sprintf("Screenshot saved. View at: %s", imgURL), nil
	}
	return "Screenshot taken but not saved", nil
}

type BrowserReadPageTool struct {
	ctx *BrowserToolContext
}

func NewBrowserReadPageTool(ctx *BrowserToolContext) *BrowserReadPageTool {
	return &BrowserReadPageTool{ctx: ctx}
}

func (t *BrowserReadPageTool) Name() string { return "browser_read_page" }
func (t *BrowserReadPageTool) Description() string {
	return "Read the content of the current page."
}

func (t *BrowserReadPageTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "browser_read_page",
			"description": "Read the content of the current page.",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

func (t *BrowserReadPageTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx.BrowserMgr == nil {
		return "", fmt.Errorf("browser tools not available. Ensure Chrome/Chromium is installed")
	}

	return t.ctx.BrowserMgr.ReadPage(t.ctx.AgentID)
}

type BrowserScrollTool struct {
	ctx *BrowserToolContext
}

func NewBrowserScrollTool(ctx *BrowserToolContext) *BrowserScrollTool {
	return &BrowserScrollTool{ctx: ctx}
}

func (t *BrowserScrollTool) Name() string { return "browser_scroll" }
func (t *BrowserScrollTool) Description() string {
	return "Scroll the page in a direction (up, down, left, right)."
}

func (t *BrowserScrollTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "browser_scroll",
			"description": "Scroll the page in a direction (up, down, left, right).",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"direction": map[string]interface{}{
						"type":        "string",
						"description": "Direction to scroll: up, down, left, or right",
						"enum":        []string{"up", "down", "left", "right"},
					},
					"amount": map[string]interface{}{
						"type":        "integer",
						"description": "Number of pixels to scroll (default: 300)",
					},
				},
				"required": []string{"direction"},
			},
		},
	}
}

func (t *BrowserScrollTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx.BrowserMgr == nil {
		return "", fmt.Errorf("browser tools not available. Ensure Chrome/Chromium is installed")
	}

	direction, _ := args["direction"].(string)
	amount := 300
	if a, ok := args["amount"].(float64); ok {
		amount = int(a)
	}

	if direction == "" {
		direction = "down"
	}

	return t.ctx.BrowserMgr.Scroll(t.ctx.AgentID, direction, amount)
}

type BrowserWaitTool struct {
	ctx *BrowserToolContext
}

func NewBrowserWaitTool(ctx *BrowserToolContext) *BrowserWaitTool {
	return &BrowserWaitTool{ctx: ctx}
}

func (t *BrowserWaitTool) Name() string { return "browser_wait" }
func (t *BrowserWaitTool) Description() string {
	return "Wait for an element to appear on the page."
}

func (t *BrowserWaitTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "browser_wait",
			"description": "Wait for an element to appear on the page.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector for the element to wait for",
					},
					"timeout_ms": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum time to wait in milliseconds (default: 5000)",
					},
				},
				"required": []string{"selector"},
			},
		},
	}
}

func (t *BrowserWaitTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx.BrowserMgr == nil {
		return "", fmt.Errorf("browser tools not available. Ensure Chrome/Chromium is installed")
	}

	selector, _ := args["selector"].(string)
	timeoutMs := 5000
	if t, ok := args["timeout_ms"].(float64); ok {
		timeoutMs = int(t)
	}

	if selector == "" {
		return "", fmt.Errorf("missing 'selector' parameter")
	}

	return t.ctx.BrowserMgr.Wait(t.ctx.AgentID, selector, timeoutMs)
}

type BrowserRunJsTool struct {
	ctx *BrowserToolContext
}

func NewBrowserRunJsTool(ctx *BrowserToolContext) *BrowserRunJsTool {
	return &BrowserRunJsTool{ctx: ctx}
}

func (t *BrowserRunJsTool) Name() string { return "browser_run_js" }
func (t *BrowserRunJsTool) Description() string {
	return "Execute JavaScript in the browser and return the result."
}

func (t *BrowserRunJsTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "browser_run_js",
			"description": "Execute JavaScript in the browser and return the result.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"expression": map[string]interface{}{
						"type":        "string",
						"description": "JavaScript expression to execute",
					},
				},
				"required": []string{"expression"},
			},
		},
	}
}

func (t *BrowserRunJsTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx.BrowserMgr == nil {
		return "", fmt.Errorf("browser tools not available. Ensure Chrome/Chromium is installed")
	}

	expression, _ := args["expression"].(string)
	if expression == "" {
		return "", fmt.Errorf("missing 'expression' parameter")
	}

	result, err := t.ctx.BrowserMgr.RunJs(t.ctx.AgentID, expression)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Result: %v", result), nil
}

type BrowserBackTool struct {
	ctx *BrowserToolContext
}

func NewBrowserBackTool(ctx *BrowserToolContext) *BrowserBackTool {
	return &BrowserBackTool{ctx: ctx}
}

func (t *BrowserBackTool) Name() string { return "browser_back" }
func (t *BrowserBackTool) Description() string {
	return "Navigate back in browser history."
}

func (t *BrowserBackTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "browser_back",
			"description": "Navigate back in browser history.",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

func (t *BrowserBackTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx.BrowserMgr == nil {
		return "", fmt.Errorf("browser tools not available. Ensure Chrome/Chromium is installed")
	}

	return t.ctx.BrowserMgr.Back(t.ctx.AgentID)
}

type BrowserCloseTool struct {
	ctx *BrowserToolContext
}

func NewBrowserCloseTool(ctx *BrowserToolContext) *BrowserCloseTool {
	return &BrowserCloseTool{ctx: ctx}
}

func (t *BrowserCloseTool) Name() string { return "browser_close" }
func (t *BrowserCloseTool) Description() string {
	return "Close the browser session."
}

func (t *BrowserCloseTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "browser_close",
			"description": "Close the browser session.",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

func (t *BrowserCloseTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx.BrowserMgr == nil {
		return "", fmt.Errorf("browser tools not available. Ensure Chrome/Chromium is installed")
	}

	t.ctx.BrowserMgr.Close(t.ctx.AgentID)
	return "Browser session closed", nil
}

type BrowserWaitForUserTool struct {
	ctx *BrowserToolContext
}

func NewBrowserWaitForUserTool(ctx *BrowserToolContext) *BrowserWaitForUserTool {
	return &BrowserWaitForUserTool{ctx: ctx}
}

func (t *BrowserWaitForUserTool) Name() string { return "browser_wait_for_user" }
func (t *BrowserWaitForUserTool) Description() string {
	return "CRITICAL: Call this tool IMMEDIATELY when browser_navigate returns 'LOGIN PAGE DETECTED'. Pauses execution and waits for user to complete login, captcha, or other manual actions in the browser window. The browser stays open during this time."
}

func (t *BrowserWaitForUserTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "browser_wait_for_user",
			"description": "CRITICAL: Call this tool IMMEDIATELY when browser_navigate returns 'LOGIN PAGE DETECTED'. Pauses execution and waits for user to complete login, captcha, or other manual actions in the browser window. The browser stays open during this time.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "Message to tell user what action to complete (e.g., '请在浏览器中完成手机号和短信验证码登录')",
					},
				},
				"required": []string{"prompt"},
			},
		},
	}
}

func (t *BrowserWaitForUserTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx.BrowserMgr == nil {
		return "", fmt.Errorf("browser tools not available. Ensure Chrome/Chromium is installed")
	}

	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		prompt = "Please complete the required action in the browser window"
	}

	return t.ctx.BrowserMgr.WaitForUser(t.ctx.AgentID, prompt)
}

func GetAllBrowserTools(ctx *BrowserToolContext) []Tool {
	return []Tool{
		NewBrowserNavigateTool(ctx),
		NewBrowserClickTool(ctx),
		NewBrowserTypeTool(ctx),
		NewBrowserScreenshotTool(ctx),
		NewBrowserReadPageTool(ctx),
		NewBrowserScrollTool(ctx),
		NewBrowserWaitTool(ctx),
		NewBrowserWaitForUserTool(ctx),
		NewBrowserRunJsTool(ctx),
		NewBrowserBackTool(ctx),
		NewBrowserCloseTool(ctx),
	}
}

func FindBrowserTool(ctx *BrowserToolContext, name string) Tool {
	for _, tool := range GetAllBrowserTools(ctx) {
		if tool.Name() == name {
			return tool
		}
	}
	return nil
}
