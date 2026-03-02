package browser

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// BrowserConfig holds browser automation configuration
type BrowserConfig struct {
	PythonPath      string `mapstructure:"python_path" json:"python_path"`
	Headless        bool   `mapstructure:"headless" json:"headless"`
	ViewportWidth   int    `mapstructure:"viewport_width" json:"viewport_width"`
	ViewportHeight  int    `mapstructure:"viewport_height" json:"viewport_height"`
	TimeoutSecs     int    `mapstructure:"timeout_secs" json:"timeout_secs"`
	IdleTimeoutSecs int    `mapstructure:"idle_timeout_secs" json:"idle_timeout_secs"`
	MaxSessions     int    `mapstructure:"max_sessions" json:"max_sessions"`
}

// DefaultBrowserConfig returns default browser configuration
func DefaultBrowserConfig() BrowserConfig {
	return BrowserConfig{
		PythonPath:      "python3",
		Headless:        true,
		ViewportWidth:   1280,
		ViewportHeight:  720,
		TimeoutSecs:     30,
		IdleTimeoutSecs: 300,
		MaxSessions:     5,
	}
}

// BrowserCommand represents a command sent from Go to Python bridge
type BrowserCommand struct {
	Action   string `json:"action"`
	URL      string `json:"url,omitempty"`
	Selector string `json:"selector,omitempty"`
	Text     string `json:"text,omitempty"`
}

// BrowserResponse represents response from Python bridge
type BrowserResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// BrowserData contains parsed response data
type BrowserData struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Content     string `json:"content"`
	Clicked     string `json:"clicked"`
	Typed       string `json:"typed"`
	TypedInto   string `json:"typed_into"`
	ImageBase64 string `json:"image_base64"`
	Status      string `json:"status"`
}

// browserSession represents a live browser session
type browserSession struct {
	cmd        *exec.Cmd
	stdin      *bufio.Writer
	stdout     *bufio.Reader
	lastActive time.Time
	mu         sync.Mutex
}

// BrowserManager manages browser sessions for agents
type BrowserManager struct {
	config     BrowserConfig
	sessions   map[string]*browserSession
	sessionsMu sync.RWMutex
	bridgePath string
}

// NewBrowserManager creates a new browser manager
func NewBrowserManager(config BrowserConfig) *BrowserManager {
	if config.PythonPath == "" {
		config.PythonPath = "python3"
	}
	if config.TimeoutSecs == 0 {
		config.TimeoutSecs = 30
	}
	if config.IdleTimeoutSecs == 0 {
		config.IdleTimeoutSecs = 300
	}
	if config.MaxSessions == 0 {
		config.MaxSessions = 5
	}
	if config.ViewportWidth == 0 {
		config.ViewportWidth = 1280
	}
	if config.ViewportHeight == 0 {
		config.ViewportHeight = 720
	}

	return &BrowserManager{
		config:   config,
		sessions: make(map[string]*browserSession),
	}
}

// ensureBridgeScript writes the embedded Python bridge script to a temp file
func (m *BrowserManager) ensureBridgeScript() (string, error) {
	if m.bridgePath != "" {
		return m.bridgePath, nil
	}

	dir := filepath.Join(os.TempDir(), "fangclaw")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	path := filepath.Join(dir, "browser_bridge.py")
	if err := os.WriteFile(path, []byte(bridgeScript), 0644); err != nil {
		return "", fmt.Errorf("failed to write bridge script: %w", err)
	}

	m.bridgePath = path
	log.Debug().Str("path", path).Msg("Wrote browser bridge script")
	return path, nil
}

// getOrCreateSession gets or creates a browser session for an agent
func (m *BrowserManager) getOrCreateSession(agentID string) (*browserSession, error) {
	m.sessionsMu.RLock()
	session, exists := m.sessions[agentID]
	m.sessionsMu.RUnlock()

	if exists {
		return session, nil
	}

	m.sessionsMu.Lock()
	defer m.sessionsMu.Unlock()

	// Double-check after acquiring write lock
	if session, exists := m.sessions[agentID]; exists {
		return session, nil
	}

	// Enforce session limit
	if len(m.sessions) >= m.config.MaxSessions {
		return nil, fmt.Errorf("maximum browser sessions reached (%d). Close an existing session first", m.config.MaxSessions)
	}

	bridgePath, err := m.ensureBridgeScript()
	if err != nil {
		return nil, err
	}

	// Build command
	cmd := exec.Command(m.config.PythonPath, bridgePath)
	if m.config.Headless {
		cmd.Args = append(cmd.Args, "--headless")
	} else {
		cmd.Args = append(cmd.Args, "--no-headless")
	}
	cmd.Args = append(cmd.Args, "--width", fmt.Sprintf("%d", m.config.ViewportWidth))
	cmd.Args = append(cmd.Args, "--height", fmt.Sprintf("%d", m.config.ViewportHeight))
	cmd.Args = append(cmd.Args, "--timeout", fmt.Sprintf("%d", m.config.TimeoutSecs))

	// Set up environment - clear everything, pass through essentials
	cmd.Env = os.Environ()
	// Note: In production, would use env_clear() and selectively pass env vars

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// For subprocess mode, we need to capture stdin/stdout
	cmd = exec.Command(m.config.PythonPath, bridgePath)
	if m.config.Headless {
		cmd.Args = append(cmd.Args, "--headless")
	} else {
		cmd.Args = append(cmd.Args, "--no-headless")
	}
	cmd.Args = append(cmd.Args, "--width", fmt.Sprintf("%d", m.config.ViewportWidth))
	cmd.Args = append(cmd.Args, "--height", fmt.Sprintf("%d", m.config.ViewportHeight))
	cmd.Args = append(cmd.Args, "--timeout", fmt.Sprintf("%d", m.config.TimeoutSecs))

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start browser bridge: %w. Ensure Python and playwright are installed", err)
	}

	// Wait for ready signal
	readyReader := bufio.NewReader(stdout)
	readyLine, err := readyReader.ReadString('\n')
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("bridge failed to start: %w", err)
	}

	var readyResp BrowserResponse
	if err := json.Unmarshal([]byte(readyLine), &readyResp); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("bridge startup failed: %w. Output: %s", err, readyLine)
	}

	if !readyResp.Success {
		cmd.Process.Kill()
		return nil, fmt.Errorf("browser bridge failed to start: %s", readyResp.Error)
	}

	session = &browserSession{
		cmd:        cmd,
		stdin:      bufio.NewWriter(stdin),
		stdout:     readyReader,
		lastActive: time.Now(),
	}

	m.sessions[agentID] = session
	log.Info().Str("agent_id", agentID).Msg("Browser session created")

	return session, nil
}

// sendCommand sends a command to the browser session
func (m *BrowserManager) sendCommand(agentID string, cmd BrowserCommand) (*BrowserResponse, error) {
	session, err := m.getOrCreateSession(agentID)
	if err != nil {
		return nil, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	jsonCmd, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("serialize error: %w", err)
	}

	if _, err := session.stdin.Write(jsonCmd); err != nil {
		return nil, fmt.Errorf("failed to write to bridge stdin: %w", err)
	}
	if _, err := session.stdin.Write([]byte("\n")); err != nil {
		return nil, fmt.Errorf("failed to write newline: %w", err)
	}
	if err := session.stdin.Flush(); err != nil {
		return nil, fmt.Errorf("failed to flush bridge stdin: %w", err)
	}

	line, err := session.stdout.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read bridge stdout: %w", err)
	}

	session.lastActive = time.Now()

	var resp BrowserResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse bridge response: %w", err)
	}

	if !resp.Success && resp.Error != "" {
		log.Warn().Str("agent_id", agentID).Str("error", resp.Error).Msg("Browser command failed")
	}

	return &resp, nil
}

// HasSession checks if an agent has an active browser session
func (m *BrowserManager) HasSession(agentID string) bool {
	m.sessionsMu.RLock()
	defer m.sessionsMu.RUnlock()
	_, exists := m.sessions[agentID]
	return exists
}

// CloseSession closes an agent's browser session
func (m *BrowserManager) CloseSession(agentID string) {
	m.sessionsMu.Lock()
	defer m.sessionsMu.Unlock()

	session, exists := m.sessions[agentID]
	if !exists {
		return
	}

	// Try graceful close
	session.mu.Lock()
	session.stdin.WriteString(`{"action":"Close"}` + "\n")
	session.stdin.Flush()
	session.mu.Unlock()

	session.cmd.Process.Kill()
	session.cmd.Wait()

	delete(m.sessions, agentID)
	log.Info().Str("agent_id", agentID).Msg("Browser session closed")
}

// CleanupAgent cleans up an agent's browser session
func (m *BrowserManager) CleanupAgent(agentID string) {
	m.CloseSession(agentID)
}

// Tool functions

// Navigate navigates to a URL
func (m *BrowserManager) Navigate(agentID, url string) (string, error) {
	// Security: SSRF check should be done in the caller before calling this

	resp, err := m.sendCommand(agentID, BrowserCommand{
		Action: "Navigate",
		URL:    url,
	})
	if err != nil {
		return "", err
	}

	if !resp.Success {
		return "", fmt.Errorf(resp.Error)
	}

	var data BrowserData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return fmt.Sprintf("Navigated to: %s\nTitle: %s\n\n%s", data.URL, data.Title, data.Content), nil
}

// Click clicks an element by selector
func (m *BrowserManager) Click(agentID, selector string) (string, error) {
	resp, err := m.sendCommand(agentID, BrowserCommand{
		Action:   "Click",
		Selector: selector,
	})
	if err != nil {
		return "", err
	}

	if !resp.Success {
		return "", fmt.Errorf(resp.Error)
	}

	var data BrowserData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return fmt.Sprintf("Clicked: %s\nPage: %s\nURL: %s", selector, data.Title, data.URL), nil
}

// Type types text into an input field
func (m *BrowserManager) Type(agentID, selector, text string) (string, error) {
	resp, err := m.sendCommand(agentID, BrowserCommand{
		Action:   "Type",
		Selector: selector,
		Text:     text,
	})
	if err != nil {
		return "", err
	}

	if !resp.Success {
		return "", fmt.Errorf(resp.Error)
	}

	return fmt.Sprintf("Typed into %s: %s", selector, text), nil
}

// Screenshot takes a screenshot of the current page
func (m *BrowserManager) Screenshot(agentID string) (map[string]interface{}, error) {
	resp, err := m.sendCommand(agentID, BrowserCommand{
		Action: "Screenshot",
	})
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf(resp.Error)
	}

	var data BrowserData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := map[string]interface{}{
		"screenshot": true,
		"url":        data.URL,
	}

	if data.ImageBase64 != "" {
		// Save to uploads temp dir
		uploadDir := filepath.Join(os.TempDir(), "fangclaw_uploads")
		os.MkdirAll(uploadDir, 0755)

		decoded, err := base64.StdEncoding.DecodeString(data.ImageBase64)
		if err == nil {
			fileID := fmt.Sprintf("%d", time.Now().UnixNano())
			path := filepath.Join(uploadDir, fileID)
			if err := os.WriteFile(path, decoded, 0644); err == nil {
				result["image_url"] = fmt.Sprintf("/api/uploads/%s", fileID)
			}
		}
	}

	return result, nil
}

// ReadPage reads the current page content
func (m *BrowserManager) ReadPage(agentID string) (string, error) {
	resp, err := m.sendCommand(agentID, BrowserCommand{
		Action: "ReadPage",
	})
	if err != nil {
		return "", err
	}

	if !resp.Success {
		return "", fmt.Errorf(resp.Error)
	}

	var data BrowserData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return fmt.Sprintf("Page: %s\nURL: %s\n\n%s", data.Title, data.URL, data.Content), nil
}

// Close closes the browser session
func (m *BrowserManager) Close(agentID string) {
	m.CloseSession(agentID)
}

// bridgeScript is the embedded Python Playwright bridge
const bridgeScript = `#!/usr/bin/env python3
"""OpenFang Browser Bridge — Playwright automation over JSON-line stdio protocol.

Reads JSON commands from stdin (one per line), executes browser actions via
Playwright, and writes JSON responses to stdout (one per line).

Usage:
    python browser_bridge.py [--headless] [--width 1280] [--height 720] [--timeout 30]
"""

import argparse
import base64
import json
import sys
import traceback


def main():
    parser = argparse.ArgumentParser(description="OpenFang Browser Bridge")
    parser.add_argument("--headless", action="store_true", default=True)
    parser.add_argument("--no-headless", dest="headless", action="store_false")
    parser.add_argument("--width", type=int, default=1280)
    parser.add_argument("--height", type=int, default=720)
    parser.add_argument("--timeout", type=int, default=30)
    args = parser.parse_args()

    timeout_ms = args.timeout * 1000

    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        respond({"success": False, "error": "playwright not installed. Run: pip install playwright && playwright install chromium"})
        return

    pw = sync_playwright().start()
    browser = pw.chromium.launch(headless=args.headless)
    context = browser.new_context(
        viewport={"width": args.width, "height": args.height},
        user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
    )
    page = context.new_page()
    page.set_default_timeout(timeout_ms)
    page.set_default_navigation_timeout(timeout_ms)

    # Signal ready
    respond({"success": True, "data": {"status": "ready"}})

    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        action = None
        try:
            cmd = json.loads(line)
            action = cmd.get("action", "")
            result = handle_command(page, context, action, cmd, timeout_ms)
            respond(result)
        except Exception as e:
            respond({"success": False, "error": f"{type(e).__name__}: {e}"})

        if action == "Close":
            break

    # Cleanup
    try:
        context.close()
        browser.close()
        pw.stop()
    except Exception:
        pass


def handle_command(page, context, action, cmd, timeout_ms):
    if action == "Navigate":
        url = cmd.get("url", "")
        if not url:
            return {"success": False, "error": "Missing 'url' parameter"}
        page.goto(url, wait_until="domcontentloaded", timeout=timeout_ms)
        title = page.title()
        content = extract_readable(page)
        return {"success": True, "data": {"title": title, "url": page.url, "content": content}}

    elif action == "Click":
        selector = cmd.get("selector", "")
        if not selector:
            return {"success": False, "error": "Missing 'selector' parameter"}
        try:
            page.click(selector, timeout=timeout_ms)
        except Exception:
            page.get_by_text(selector, exact=False).first.click(timeout=timeout_ms)
        page.wait_for_load_state("domcontentloaded", timeout=timeout_ms)
        title = page.title()
        return {"success": True, "data": {"clicked": selector, "title": title, "url": page.url}}

    elif action == "Type":
        selector = cmd.get("selector", "")
        text = cmd.get("text", "")
        if not selector:
            return {"success": False, "error": "Missing 'selector' parameter"}
        if not text:
            return {"success": False, "error": "Missing 'text' parameter"}
        page.fill(selector, text, timeout=timeout_ms)
        return {"success": True, "data": {"typed": text, "selector": selector}}

    elif action == "Screenshot":
        screenshot_bytes = page.screenshot(full_page=False)
        b64 = base64.b64encode(screenshot_bytes).decode("utf-8")
        return {"success": True, "data": {"image_base64": b64, "format": "png", "url": page.url}}

    elif action == "ReadPage":
        title = page.title()
        content = extract_readable(page)
        return {"success": True, "data": {"title": title, "url": page.url, "content": content}}

    elif action == "Close":
        return {"success": True, "data": {"status": "closed"}}

    else:
        return {"success": False, "error": f"Unknown action: {action}"}


def extract_readable(page):
    """Extract readable text content from the page, stripping nav/footer/script noise."""
    try:
        content = page.evaluate("""() => {
            const clone = document.body.cloneNode(true);
            const remove = ['script', 'style', 'nav', 'footer', 'header', 'aside',
                           'iframe', 'noscript', 'svg', 'canvas'];
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
            return lines.join('\\n').replace(/\\n{3,}/g, '\\n\\n').trim();
        }""")
        max_chars = 50000
        if len(content) > max_chars:
            content = content[:max_chars] + f"\\n\\n[Truncated — {len(content)} total chars]"
        return content
    except Exception:
        try:
            text = page.inner_text("body")
            if len(text) > 50000:
                text = text[:50000] + f"\\n\\n[Truncated — {len(text)} total chars]"
            return text
        except Exception:
            return "(could not extract page content)"


def respond(data):
    """Write a JSON response line to stdout."""
    sys.stdout.write(json.dumps(data) + "\\n")
    sys.stdout.flush()


if __name__ == "__main__":
    main()
`

// Need to fix the string escaping
func init() {
	// The bridgeScript constant above has escaped newlines - fix them
	// This is handled at runtime when writing the file
}
