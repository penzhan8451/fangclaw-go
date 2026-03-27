package browser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	cdpConnectTimeoutSecs  = 15
	cdpCommandTimeoutSecs  = 30
	pageLoadPollIntervalMs = 200
	pageLoadMaxPolls       = 150
	maxContentChars        = 50000
)

type CDPBrowserConfig struct {
	ChromiumPath   string `mapstructure:"chromium_path" json:"chromium_path"`
	Headless       bool   `mapstructure:"headless" json:"headless"`
	ViewportWidth  int    `mapstructure:"viewport_width" json:"viewport_width"`
	ViewportHeight int    `mapstructure:"viewport_height" json:"viewport_height"`
	MaxSessions    int    `mapstructure:"max_sessions" json:"max_sessions"`
}

type BrowserCommand struct {
	Action     string `json:"action"`
	URL        string `json:"url,omitempty"`
	Selector   string `json:"selector,omitempty"`
	Text       string `json:"text,omitempty"`
	Direction  string `json:"direction,omitempty"`
	Amount     int    `json:"amount,omitempty"`
	TimeoutMs  int    `json:"timeout_ms,omitempty"`
	Expression string `json:"expression,omitempty"`
}

// BrowserResponse represents response from Python bridge
type BrowserResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func DefaultCDPBrowserConfig() CDPBrowserConfig {
	return CDPBrowserConfig{
		Headless:       true,
		ViewportWidth:  1280,
		ViewportHeight: 720,
		MaxSessions:    5,
	}
}

type CDPCommand struct {
	ID     int                    `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

type CDPResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *CDPError       `json:"error,omitempty"`
}

type CDPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type CDPConnection struct {
	conn      *websocket.Conn
	writeMu   sync.Mutex
	pending   map[int]chan *CDPResponse
	pendingMu sync.RWMutex
	nextID    int32
	done      chan struct{}
}

func NewCDPConnection(wsURL string) (*CDPConnection, error) {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = time.Duration(cdpConnectTimeoutSecs) * time.Second

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CDP WebSocket: %w", err)
	}

	cdp := &CDPConnection{
		conn:    conn,
		pending: make(map[int]chan *CDPResponse),
		done:    make(chan struct{}),
	}

	go cdp.readLoop()

	return cdp, nil
}

func (c *CDPConnection) readLoop() {
	defer close(c.done)

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var resp CDPResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			continue
		}

		if resp.ID != 0 {
			c.pendingMu.RLock()
			ch, ok := c.pending[resp.ID]
			c.pendingMu.RUnlock()

			if ok {
				ch <- &resp
				c.pendingMu.Lock()
				delete(c.pending, resp.ID)
				c.pendingMu.Unlock()
			}
		}
	}
}

func (c *CDPConnection) Send(method string, params map[string]interface{}) (json.RawMessage, error) {
	id := int(atomic.AddInt32(&c.nextID, 1))

	cmd := CDPCommand{
		ID:     id,
		Method: method,
		Params: params,
	}

	respCh := make(chan *CDPResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	c.writeMu.Lock()
	err := c.conn.WriteJSON(cmd)
	c.writeMu.Unlock()

	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("failed to send CDP command: %w", err)
	}

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("CDP error: %s", resp.Error.Message)
		}
		return resp.Result, nil
	case <-time.After(time.Duration(cdpCommandTimeoutSecs) * time.Second):
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("CDP command timed out")
	case <-c.done:
		return nil, fmt.Errorf("CDP connection closed")
	}
}

func (c *CDPConnection) RunJS(expression string) (interface{}, error) {
	result, err := c.Send("Runtime.evaluate", map[string]interface{}{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			Value interface{} `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}

	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}

	if resp.ExceptionDetails != nil {
		return nil, fmt.Errorf("JS error: %s", resp.ExceptionDetails.Text)
	}

	return resp.Result.Value, nil
}

func (c *CDPConnection) Close() {
	c.conn.Close()
	<-c.done
}

type CDPSession struct {
	process    *exec.Cmd
	cdp        *CDPConnection
	lastActive time.Time
	mu         sync.Mutex
}

func LaunchChromium(config CDPBrowserConfig) (*CDPSession, error) {
	chromePath, err := findChromium(config.ChromiumPath)
	if err != nil {
		return nil, err
	}

	port := 9222 + int(atomic.AddInt32(&nextPort, 1))%1000

	userDataDir, err := os.MkdirTemp("", "fangclaw-chrome-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--disable-background-networking",
		"--disable-sync",
		"--disable-translate",
		"--disable-features=TranslateUI",
		"--metrics-recording-only",
		fmt.Sprintf("--window-size=%d,%d", config.ViewportWidth, config.ViewportHeight),
		"--user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"about:blank",
	}

	if config.Headless {
		args = append([]string{"--headless=new"}, args...)
		args = append(args, "--disable-gpu")
	}

	if isRunningAsRoot() {
		args = append(args, "--no-sandbox")
	}

	cmd := exec.Command(chromePath, args...)

	cmd.Env = []string{}
	for _, key := range []string{
		"PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "TEMP", "TMP",
		"TMPDIR", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME",
		"XDG_CACHE_HOME", "DISPLAY", "WAYLAND_DISPLAY",
	} {
		if val, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to launch Chromium: %w", err)
	}

	_, err = readDevToolsURL(stderr, time.Duration(cdpConnectTimeoutSecs)*time.Second)
	if err != nil {
		cmd.Process.Kill()
		return nil, err
	}

	pageWS, err := findPageWebSocket(port)
	if err != nil {
		cmd.Process.Kill()
		return nil, err
	}

	cdp, err := NewCDPConnection(pageWS)
	if err != nil {
		cmd.Process.Kill()
		return nil, err
	}

	cdp.Send("Page.enable", nil)
	cdp.Send("Runtime.enable", nil)

	return &CDPSession{
		process:    cmd,
		cdp:        cdp,
		lastActive: time.Now(),
	}, nil
}

func (s *CDPSession) Execute(cmd BrowserCommand) (*BrowserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActive = time.Now()

	switch cmd.Action {
	case "Navigate":
		return s.cmdNavigate(cmd.URL)
	case "Click":
		return s.cmdClick(cmd.Selector)
	case "Type":
		return s.cmdType(cmd.Selector, cmd.Text)
	case "Screenshot":
		return s.cmdScreenshot()
	case "ReadPage":
		return s.cmdReadPage()
	case "Scroll":
		return s.cmdScroll(cmd.Direction, cmd.Amount)
	case "Wait":
		return s.cmdWait(cmd.Selector, cmd.TimeoutMs)
	case "RunJs":
		return s.cmdRunJs(cmd.Expression)
	case "Back":
		return s.cmdBack()
	case "Close":
		return &BrowserResponse{Success: true, Data: json.RawMessage(`{"closed":true}`)}, nil
	default:
		return nil, fmt.Errorf("unknown action: %s", cmd.Action)
	}
}

func (s *CDPSession) cmdNavigate(url string) (*BrowserResponse, error) {
	_, err := s.cdp.Send("Page.navigate", map[string]interface{}{"url": url})
	if err != nil {
		return nil, err
	}

	s.waitForLoad()

	info, err := s.pageInfo()
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(info)
	return &BrowserResponse{Success: true, Data: data}, nil
}

func (s *CDPSession) cmdClick(selector string) (*BrowserResponse, error) {
	selJSON, _ := json.Marshal(selector)
	js := fmt.Sprintf(`(() => {
		let sel = %s;
		let el = document.querySelector(sel);
		if (!el) {
			const all = document.querySelectorAll('a, button, [role="button"], input[type="submit"], [onclick]');
			const lower = sel.toLowerCase();
			for (const e of all) {
				if (e.textContent.trim().toLowerCase().includes(lower)) { el = e; break; }
			}
		}
		if (!el) return JSON.stringify({success: false, error: 'Element not found: ' + sel});
		el.scrollIntoView({block: 'center'});
		el.click();
		return JSON.stringify({success: true, tag: el.tagName, text: el.textContent.substring(0, 100).trim()});
	})()`, selJSON)

	result, err := s.cdp.RunJS(js)
	if err != nil {
		return nil, err
	}

	var parsed map[string]interface{}
	switch v := result.(type) {
	case string:
		json.Unmarshal([]byte(v), &parsed)
	case map[string]interface{}:
		parsed = v
	}

	if success, ok := parsed["success"].(bool); !ok || !success {
		errMsg := "Click failed"
		if e, ok := parsed["error"].(string); ok {
			errMsg = e
		}
		return nil, fmt.Errorf(errMsg)
	}

	time.Sleep(500 * time.Millisecond)
	s.waitForLoad()

	info, err := s.pageInfo()
	if err != nil {
		data, _ := json.Marshal(parsed)
		return &BrowserResponse{Success: true, Data: data}, nil
	}

	data, _ := json.Marshal(info)
	return &BrowserResponse{Success: true, Data: data}, nil
}

func (s *CDPSession) cmdType(selector, text string) (*BrowserResponse, error) {
	selJSON, _ := json.Marshal(selector)
	textJSON, _ := json.Marshal(text)
	js := fmt.Sprintf(`(() => {
		let sel = %s;
		let txt = %s;
		let el = document.querySelector(sel);
		if (!el) return JSON.stringify({success: false, error: 'Input not found: ' + sel});
		el.focus();
		el.value = txt;
		el.dispatchEvent(new Event('input', {bubbles: true}));
		el.dispatchEvent(new Event('change', {bubbles: true}));
		return JSON.stringify({success: true, selector: sel, typed: txt.length + ' chars'});
	})()`, selJSON, textJSON)

	result, err := s.cdp.RunJS(js)
	if err != nil {
		return nil, err
	}

	var parsed map[string]interface{}
	switch v := result.(type) {
	case string:
		json.Unmarshal([]byte(v), &parsed)
	case map[string]interface{}:
		parsed = v
	}

	if success, ok := parsed["success"].(bool); !ok || !success {
		errMsg := "Type failed"
		if e, ok := parsed["error"].(string); ok {
			errMsg = e
		}
		return nil, fmt.Errorf(errMsg)
	}

	data, _ := json.Marshal(parsed)
	return &BrowserResponse{Success: true, Data: data}, nil
}

func (s *CDPSession) cmdScreenshot() (*BrowserResponse, error) {
	result, err := s.cdp.Send("Page.captureScreenshot", map[string]interface{}{"format": "png"})
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}

	url, _ := s.cdp.RunJS("location.href")
	urlStr, _ := url.(string)

	data, _ := json.Marshal(map[string]interface{}{
		"image_base64": resp.Data,
		"url":          urlStr,
		"format":       "png",
	})
	return &BrowserResponse{Success: true, Data: data}, nil
}

func (s *CDPSession) cmdReadPage() (*BrowserResponse, error) {
	info, err := s.pageInfo()
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(info)
	return &BrowserResponse{Success: true, Data: data}, nil
}

func (s *CDPSession) cmdScroll(direction string, amount int) (*BrowserResponse, error) {
	dx, dy := 0, 0
	switch direction {
	case "up":
		dy = -amount
	case "down":
		dy = amount
	case "left":
		dx = -amount
	case "right":
		dx = amount
	}

	js := fmt.Sprintf("window.scrollBy(%d, %d); JSON.stringify({scrollX: window.scrollX, scrollY: window.scrollY})", dx, dy)
	result, err := s.cdp.RunJS(js)
	if err != nil {
		return nil, err
	}

	var parsed map[string]interface{}
	switch v := result.(type) {
	case string:
		json.Unmarshal([]byte(v), &parsed)
	case map[string]interface{}:
		parsed = v
	}

	data, _ := json.Marshal(parsed)
	return &BrowserResponse{Success: true, Data: data}, nil
}

func (s *CDPSession) cmdWait(selector string, timeoutMs int) (*BrowserResponse, error) {
	selJSON, _ := json.Marshal(selector)
	maxMs := timeoutMs
	if maxMs <= 0 {
		maxMs = 30000
	}
	if maxMs > 30000 {
		maxMs = 30000
	}

	polls := maxMs / pageLoadPollIntervalMs
	if polls < 1 {
		polls = 1
	}

	for i := 0; i < polls; i++ {
		js := fmt.Sprintf("document.querySelector(%s) ? 'found' : null", selJSON)
		result, err := s.cdp.RunJS(js)
		if err == nil {
			if str, ok := result.(string); ok && str == "found" {
				data, _ := json.Marshal(map[string]interface{}{
					"found":    true,
					"selector": selector,
				})
				return &BrowserResponse{Success: true, Data: data}, nil
			}
		}
		time.Sleep(pageLoadPollIntervalMs * time.Millisecond)
	}

	return nil, fmt.Errorf("timed out waiting for selector: %s (%dms)", selector, maxMs)
}

func (s *CDPSession) cmdRunJs(expression string) (*BrowserResponse, error) {
	result, err := s.cdp.RunJS(expression)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(map[string]interface{}{"result": result})
	return &BrowserResponse{Success: true, Data: data}, nil
}

func (s *CDPSession) cmdBack() (*BrowserResponse, error) {
	_, err := s.cdp.RunJS("history.back(); 'ok'")
	if err != nil {
		return nil, err
	}

	time.Sleep(500 * time.Millisecond)
	s.waitForLoad()

	info, err := s.pageInfo()
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(info)
	return &BrowserResponse{Success: true, Data: data}, nil
}

func (s *CDPSession) waitForLoad() {
	for i := 0; i < pageLoadMaxPolls; i++ {
		result, err := s.cdp.RunJS("document.readyState")
		if err == nil {
			if state, ok := result.(string); ok && (state == "complete" || state == "interactive") {
				return
			}
		}
		time.Sleep(pageLoadPollIntervalMs * time.Millisecond)
	}
}

func (s *CDPSession) pageInfo() (map[string]interface{}, error) {
	result, err := s.cdp.RunJS("JSON.stringify({title: document.title, url: location.href})")
	if err != nil {
		return nil, err
	}

	var info map[string]interface{}
	switch v := result.(type) {
	case string:
		json.Unmarshal([]byte(v), &info)
	case map[string]interface{}:
		info = v
	}

	contentResult, err := s.cdp.RunJS(extractContentJS)
	if err == nil {
		var contentObj map[string]interface{}
		switch v := contentResult.(type) {
		case string:
			json.Unmarshal([]byte(v), &contentObj)
		case map[string]interface{}:
			contentObj = v
		}
		if content, ok := contentObj["content"].(string); ok {
			info["content"] = content
		}
	}

	loginInfo := s.detectLoginPage()
	if loginInfo["requires_login"].(bool) {
		info["requires_login"] = true
		info["login_hint"] = loginInfo["hint"]
	}

	return info, nil
}

func (s *CDPSession) detectLoginPage() map[string]interface{} {
	js := `JSON.stringify((() => {
		const indicators = {
			loginKeywords: ['登录', 'login', 'sign in', 'signin', 'log in', '登录账号', '账号登录', '用户登录'],
			phoneKeywords: ['手机号', '手机', 'phone', 'mobile', '电话'],
			codeKeywords: ['验证码', 'code', 'captcha', '短信', 'sms', 'verification'],
			passwordKeywords: ['密码', 'password', 'pwd']
		};
		
		const text = document.body.innerText.toLowerCase();
		const html = document.body.innerHTML.toLowerCase();
		
		let hasLoginForm = false;
		let hasPhoneInput = false;
		let hasCodeInput = false;
		let hasPasswordInput = false;
		
		for (const kw of indicators.loginKeywords) {
			if (text.includes(kw.toLowerCase())) {
				hasLoginForm = true;
				break;
			}
		}
		
		for (const kw of indicators.phoneKeywords) {
			if (text.includes(kw.toLowerCase()) || html.includes(kw.toLowerCase())) {
				hasPhoneInput = true;
				break;
			}
		}
		
		for (const kw of indicators.codeKeywords) {
			if (text.includes(kw.toLowerCase()) || html.includes(kw.toLowerCase())) {
				hasCodeInput = true;
				break;
			}
		}
		
		for (const kw of indicators.passwordKeywords) {
			if (text.includes(kw.toLowerCase()) || html.includes(kw.toLowerCase())) {
				hasPasswordInput = true;
				break;
			}
		}
		
		const hasPasswordFields = document.querySelectorAll('input[type="password"]').length > 0;
		const hasPhoneFields = document.querySelectorAll('input[type="tel"], input[placeholder*="手机"], input[placeholder*="phone"]').length > 0;
		
		let hint = '';
		if (hasLoginForm || hasPasswordFields) {
			if (hasPhoneInput || hasPhoneFields) {
				if (hasCodeInput) {
					hint = '需要手机号和短信验证码登录';
				} else if (hasPasswordInput || hasPasswordFields) {
					hint = '需要手机号和密码登录';
				} else {
					hint = '需要登录';
				}
			} else if (hasPasswordFields) {
				hint = '需要账号密码登录';
			} else {
				hint = '可能需要登录';
			}
		}
		
		return {
			requires_login: hasLoginForm || hasPasswordFields,
			has_phone: hasPhoneInput || hasPhoneFields,
			has_code: hasCodeInput,
			has_password: hasPasswordInput || hasPasswordFields,
			hint: hint
		};
	})())`

	result, err := s.cdp.RunJS(js)
	if err != nil {
		return map[string]interface{}{"requires_login": false}
	}

	var info map[string]interface{}
	switch v := result.(type) {
	case string:
		json.Unmarshal([]byte(v), &info)
	case map[string]interface{}:
		info = v
	}

	if info == nil {
		return map[string]interface{}{"requires_login": false}
	}

	return info
}

func (s *CDPSession) Close() {
	if s.cdp != nil {
		s.cdp.Close()
	}
	if s.process != nil && s.process.Process != nil {
		s.process.Process.Kill()
		s.process.Wait()
	}
}

var nextPort int32

func readDevToolsURL(stderr io.Reader, timeout time.Duration) (string, error) {
	scanner := bufio.NewScanner(stderr)
	deadline := time.After(timeout)

	for {
		select {
		case <-deadline:
			return "", fmt.Errorf("timed out waiting for DevTools URL")
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return "", fmt.Errorf("failed to read stderr: %w", err)
				}
				return "", fmt.Errorf("Chromium exited before printing DevTools URL")
			}

			line := scanner.Text()
			if strings.Contains(line, "DevTools listening on") {
				re := regexp.MustCompile(`DevTools listening on (ws://[^\s]+)`)
				matches := re.FindStringSubmatch(line)
				if len(matches) > 1 {
					return matches[1], nil
				}
			}
		}
	}
}

func findPageWebSocket(port int) (string, error) {
	listURL := fmt.Sprintf("http://127.0.0.1:%d/json/list", port)

	for i := 0; i < 10; i++ {
		if i > 0 {
			time.Sleep(300 * time.Millisecond)
		}

		resp, err := http.Get(listURL)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		var targets []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
			continue
		}

		for _, target := range targets {
			if typ, ok := target["type"].(string); ok && typ == "page" {
				if ws, ok := target["webSocketDebuggerUrl"].(string); ok && ws != "" {
					return ws, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no page target found in Chromium")
}

func findChromium(configPath string) (string, error) {
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
		return "", fmt.Errorf("configured chromium_path not found: %s", configPath)
	}

	if path := os.Getenv("CHROME_PATH"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	candidates := chromiumCandidates()
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("Chromium/Chrome not found. Install Chrome or set CHROME_PATH")
}

func chromiumCandidates() []string {
	var paths []string

	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
	case "linux":
		paths = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
			"/usr/bin/microsoft-edge",
			"/usr/bin/brave-browser",
		}
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		localApp := os.Getenv("LOCALAPPDATA")

		if programFiles != "" {
			paths = append(paths,
				filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
			)
		}
		if programFilesX86 != "" {
			paths = append(paths,
				filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
			)
		}
		if localApp != "" {
			paths = append(paths,
				filepath.Join(localApp, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(localApp, "Microsoft", "Edge", "Application", "msedge.exe"),
			)
		}
	}

	return paths
}

func isRunningAsRoot() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	return os.Geteuid() == 0
}

const extractContentJS = `(() => {
	const clone = document.body.cloneNode(true);
	const remove = ['script', 'style', 'nav', 'footer', 'header', 'aside', 'iframe', 'noscript', 'svg', 'canvas'];
	remove.forEach(tag => {
		clone.querySelectorAll(tag).forEach(el => el.remove());
	});
	const main = clone.querySelector('main, article, [role="main"], .content, #content');
	const source = main || clone;
	const lines = [];
	const walk = (node) => {
		if (node.nodeType === 3) {
			const text = node.textContent.trim();
			if (text) lines.push(text);
		} else if (node.nodeType === 1) {
			const tag = node.tagName.toLowerCase();
			if (['h1','h2','h3','h4','h5','h6'].includes(tag)) {
				lines.push('\\n## ' + node.textContent.trim());
			} else if (tag === 'li') {
				lines.push('- ' + node.textContent.trim());
			} else if (tag === 'a' && node.href) {
				lines.push('[' + node.textContent.trim() + '](' + node.href + ')');
			} else if (['p', 'div', 'section', 'td', 'th'].includes(tag)) {
				for (const child of node.childNodes) walk(child);
				lines.push('');
			} else {
				for (const child of node.childNodes) walk(child);
			}
		}
	};
	walk(source);
	let content = lines.join('\\n').replace(/\\n{3,}/g, '\\n\\n').trim();
	if (content.length > 50000) {
		content = content.substring(0, 50000) + '\\n\\n[Truncated]';
	}
	return JSON.stringify({content: content});
})()`

type CDPBrowserManager struct {
	config   CDPBrowserConfig
	sessions map[string]*CDPSession
	mu       sync.RWMutex
}

func NewCDPBrowserManager(config CDPBrowserConfig) *CDPBrowserManager {
	return &CDPBrowserManager{
		config:   config,
		sessions: make(map[string]*CDPSession),
	}
}

func (m *CDPBrowserManager) HasSession(agentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.sessions[agentID]
	return ok
}

func (m *CDPBrowserManager) getOrCreate(agentID string) (*CDPSession, error) {
	m.mu.RLock()
	session, ok := m.sessions[agentID]
	m.mu.RUnlock()

	if ok {
		return session, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[agentID]; ok {
		return session, nil
	}

	if len(m.sessions) >= m.config.MaxSessions {
		return nil, fmt.Errorf("maximum browser sessions reached (%d)", m.config.MaxSessions)
	}

	session, err := LaunchChromium(m.config)
	if err != nil {
		return nil, err
	}

	m.sessions[agentID] = session
	log.Info().Str("agent_id", agentID).Msg("CDP browser session created")

	return session, nil
}

func (m *CDPBrowserManager) Navigate(agentID, url string) (string, error) {
	session, err := m.getOrCreate(agentID)
	if err != nil {
		return "", err
	}

	resp, err := session.Execute(BrowserCommand{Action: "Navigate", URL: url})
	if err != nil {
		return "", err
	}

	var data struct {
		Title         string `json:"title"`
		URL           string `json:"url"`
		Content       string `json:"content"`
		RequiresLogin bool   `json:"requires_login"`
		LoginHint     string `json:"login_hint"`
	}
	json.Unmarshal(resp.Data, &data)

	result := fmt.Sprintf("Navigated to: %s\nTitle: %s", data.URL, data.Title)

	if data.RequiresLogin {
		result += ("\n\n========================================")
		result += ("\n⛔ ACTION REQUIRED: LOGIN PAGE DETECTED")
		result += ("\n========================================")
		result += fmt.Sprintf("\n\nDetected: %s", data.LoginHint)
		result += "\n\nIMPORTANT: You MUST call browser_wait_for_user tool NOW to let the user complete login."
		result += "\nDO NOT attempt to click any elements until login is complete."
		result += "\n\nExample: browser_wait_for_user(prompt=\"请在浏览器中完成登录操作\")"
		result += "\n\n========================================"
	}

	if data.Content != "" {
		if data.RequiresLogin {
			maxLen := 500
			if len(data.Content) > maxLen {
				result += fmt.Sprintf("\n\n[Page content truncated - login required first]\n%s...", data.Content[:maxLen])
			} else {
				result += fmt.Sprintf("\n\n%s", data.Content)
			}
		} else {
			result += fmt.Sprintf("\n\n%s", data.Content)
		}
	}

	return result, nil
}

func (m *CDPBrowserManager) Click(agentID, selector string) (string, error) {
	session, err := m.getOrCreate(agentID)
	if err != nil {
		return "", err
	}

	resp, err := session.Execute(BrowserCommand{Action: "Click", Selector: selector})
	if err != nil {
		return "", err
	}

	var data struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	}
	json.Unmarshal(resp.Data, &data)

	return fmt.Sprintf("Clicked: %s\nPage: %s\nURL: %s", selector, data.Title, data.URL), nil
}

func (m *CDPBrowserManager) Type(agentID, selector, text string) (string, error) {
	session, err := m.getOrCreate(agentID)
	if err != nil {
		return "", err
	}

	_, err = session.Execute(BrowserCommand{Action: "Type", Selector: selector, Text: text})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Typed into %s: %s", selector, text), nil
}

func (m *CDPBrowserManager) Screenshot(agentID string) (map[string]interface{}, error) {
	session, err := m.getOrCreate(agentID)
	if err != nil {
		return nil, err
	}

	resp, err := session.Execute(BrowserCommand{Action: "Screenshot"})
	if err != nil {
		return nil, err
	}

	var data struct {
		ImageBase64 string `json:"image_base64"`
		URL         string `json:"url"`
	}
	json.Unmarshal(resp.Data, &data)

	result := map[string]interface{}{
		"screenshot": true,
		"url":        data.URL,
	}

	if data.ImageBase64 != "" {
		uploadDir := filepath.Join(os.TempDir(), "fangclaw_uploads")
		os.MkdirAll(uploadDir, 0755)

		fileID := fmt.Sprintf("%d", time.Now().UnixNano())
		path := filepath.Join(uploadDir, fileID)

		if err := os.WriteFile(path, []byte(data.ImageBase64), 0644); err == nil {
			result["image_url"] = fmt.Sprintf("/api/uploads/%s", fileID)
		}
	}

	return result, nil
}

func (m *CDPBrowserManager) ReadPage(agentID string) (string, error) {
	session, err := m.getOrCreate(agentID)
	if err != nil {
		return "", err
	}

	resp, err := session.Execute(BrowserCommand{Action: "ReadPage"})
	if err != nil {
		return "", err
	}

	var data struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	}
	json.Unmarshal(resp.Data, &data)

	return fmt.Sprintf("Page: %s\nURL: %s\n\n%s", data.Title, data.URL, data.Content), nil
}

func (m *CDPBrowserManager) Scroll(agentID, direction string, amount int) (string, error) {
	session, err := m.getOrCreate(agentID)
	if err != nil {
		return "", err
	}

	resp, err := session.Execute(BrowserCommand{Action: "Scroll", Direction: direction, Amount: amount})
	if err != nil {
		return "", err
	}

	var data struct {
		ScrollX float64 `json:"scrollX"`
		ScrollY float64 `json:"scrollY"`
	}
	json.Unmarshal(resp.Data, &data)

	return fmt.Sprintf("Scrolled %s by %d pixels. Position: (%.0f, %.0f)", direction, amount, data.ScrollX, data.ScrollY), nil
}

func (m *CDPBrowserManager) Wait(agentID, selector string, timeoutMs int) (string, error) {
	session, err := m.getOrCreate(agentID)
	if err != nil {
		return "", err
	}

	_, err = session.Execute(BrowserCommand{Action: "Wait", Selector: selector, TimeoutMs: timeoutMs})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Element found: %s", selector), nil
}

func (m *CDPBrowserManager) WaitForUser(agentID, prompt string) (string, error) {
	if !m.HasSession(agentID) {
		return "", fmt.Errorf("no browser session found for agent")
	}

	return fmt.Sprintf("USER_ACTION_REQUIRED: %s\n\nPlease complete the action in the browser window, then reply 'done' or 'completed' to continue.", prompt), nil
}

func (m *CDPBrowserManager) RunJs(agentID, expression string) (interface{}, error) {
	session, err := m.getOrCreate(agentID)
	if err != nil {
		return nil, err
	}

	resp, err := session.Execute(BrowserCommand{Action: "RunJs", Expression: expression})
	if err != nil {
		return nil, err
	}

	var data struct {
		Result interface{} `json:"result"`
	}
	json.Unmarshal(resp.Data, &data)

	return data.Result, nil
}

func (m *CDPBrowserManager) Back(agentID string) (string, error) {
	session, err := m.getOrCreate(agentID)
	if err != nil {
		return "", err
	}

	resp, err := session.Execute(BrowserCommand{Action: "Back"})
	if err != nil {
		return "", err
	}

	var data struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	}
	json.Unmarshal(resp.Data, &data)

	return fmt.Sprintf("Navigated back to: %s\nTitle: %s", data.URL, data.Title), nil
}

func (m *CDPBrowserManager) Close(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[agentID]; ok {
		session.Close()
		delete(m.sessions, agentID)
		log.Info().Str("agent_id", agentID).Msg("CDP browser session closed")
	}
}

func (m *CDPBrowserManager) CloseSession(agentID string) {
	m.Close(agentID)
}

func (m *CDPBrowserManager) CleanupAgent(agentID string) {
	m.Close(agentID)
}
