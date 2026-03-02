package whatsappgateway

import (
	"context"
	_ "embed"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const (
	DefaultGatewayPort = 3009
	MaxRestarts        = 3
)

var restartDelays = []time.Duration{
	5 * time.Second,
	10 * time.Second,
	20 * time.Second,
}

var (
	//go:embed index.js
	embeddedIndexJS []byte

	//go:embed package.json
	embeddedPackageJSON []byte
)

type WhatsAppGatewayConfig struct {
	Enabled      bool
	Port         int
	OpenFangURL  string
	DefaultAgent string
	DataDir      string
}

type WhatsAppGateway struct {
	config     WhatsAppGatewayConfig
	dataDir    string
	mu         sync.RWMutex
	cmd        *exec.Cmd
	cancelFunc context.CancelFunc
	ctx        context.Context
	pid        int
	started    bool
}

func NewWhatsAppGateway(config WhatsAppGatewayConfig, dataDir string) *WhatsAppGateway {
	return &WhatsAppGateway{
		config:  config,
		dataDir: dataDir,
	}
}

func (wg *WhatsAppGateway) Start(ctx context.Context) error {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	if wg.started {
		return fmt.Errorf("gateway already started")
	}

	if !wg.config.Enabled {
		return nil
	}

	if !nodeAvailable() {
		return fmt.Errorf("whatsapp web gateway requires node.js >= 18 but node was not found")
	}

	if err := wg.ensureInstalled(); err != nil {
		return err
	}

	wg.ctx, wg.cancelFunc = context.WithCancel(ctx)
	wg.started = true

	go wg.runWithRestarts()

	return nil
}

func (wg *WhatsAppGateway) IsRunning() bool {
	wg.mu.RLock()
	defer wg.mu.RUnlock()
	return wg.started && wg.cmd != nil && wg.cmd.Process != nil
}

func (wg *WhatsAppGateway) GatewayURL() string {
	port := wg.config.Port
	if port == 0 {
		port = DefaultGatewayPort
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func (wg *WhatsAppGateway) runWithRestarts() {
	restarts := 0

	for {
		select {
		case <-wg.ctx.Done():
			return
		default:
		}

		cmd := wg.startGateway()
		if cmd == nil {
			return
		}

		wg.mu.Lock()
		wg.cmd = cmd
		wg.pid = cmd.Process.Pid
		wg.mu.Unlock()

		fmt.Printf("WhatsApp Web gateway started (PID %d)\n", cmd.Process.Pid)

		err := cmd.Wait()

		wg.mu.Lock()
		wg.cmd = nil
		wg.pid = 0
		wg.mu.Unlock()

		select {
		case <-wg.ctx.Done():
			fmt.Println("WhatsApp gateway stopped")
			return
		default:
		}

		if err == nil {
			fmt.Println("WhatsApp gateway exited cleanly")
			return
		}

		fmt.Printf("WhatsApp gateway crashed: %v, restart %d/%d\n", err, restarts+1, MaxRestarts)

		restarts++
		if restarts >= MaxRestarts {
			fmt.Printf("WhatsApp gateway exceeded max restarts (%d), giving up\n", MaxRestarts)
			return
		}

		delay := restartDelays[restarts-1]
		fmt.Printf("Restarting WhatsApp gateway in %v...\n", delay)

		select {
		case <-wg.ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func (wg *WhatsAppGateway) startGateway() *exec.Cmd {
	nodeCmd := "node"
	if runtime.GOOS == "windows" {
		nodeCmd = "node.exe"
	}

	dir := gatewayDir(wg.dataDir)
	port := wg.config.Port
	if port == 0 {
		port = DefaultGatewayPort
	}

	cmd := exec.CommandContext(wg.ctx, nodeCmd, "index.js")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("WHATSAPP_GATEWAY_PORT=%d", port),
		fmt.Sprintf("OPENFANG_URL=%s", wg.config.OpenFangURL),
		fmt.Sprintf("OPENFANG_DEFAULT_AGENT=%s", wg.config.DefaultAgent),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to spawn WhatsApp gateway: %v\n", err)
		return nil
	}

	return cmd
}

func (wg *WhatsAppGateway) Stop() error {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	if !wg.started {
		return nil
	}

	if wg.cancelFunc != nil {
		wg.cancelFunc()
	}

	if wg.cmd != nil && wg.cmd.Process != nil {
		if err := wg.cmd.Process.Kill(); err != nil {
			return err
		}
	}

	wg.started = false
	wg.cmd = nil
	wg.pid = 0
	return nil
}

func (wg *WhatsAppGateway) ensureInstalled() error {
	dir := gatewayDir(wg.dataDir)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	indexJSPath := filepath.Join(dir, "index.js")
	packageJSONPath := filepath.Join(dir, "package.json")
	hashPath := filepath.Join(dir, ".content_hash")

	currentHash := computeContentHash()

	var existingHash string
	if data, err := os.ReadFile(hashPath); err == nil {
		existingHash = string(data)
	}

	if existingHash == currentHash {
		if _, err := os.Stat(filepath.Join(dir, "node_modules")); err == nil {
			return nil
		}
	}

	fmt.Println("Extracting WhatsApp gateway package...")

	if err := os.WriteFile(indexJSPath, embeddedIndexJS, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(packageJSONPath, embeddedPackageJSON, 0644); err != nil {
		return err
	}

	if err := os.WriteFile(hashPath, []byte(currentHash), 0644); err != nil {
		return err
	}

	fmt.Println("Installing WhatsApp gateway dependencies (npm install)...")

	npmCmd := "npm"
	if runtime.GOOS == "windows" {
		npmCmd = "npm.cmd"
	}

	cmd := exec.Command(npmCmd, "install")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install failed: %w", err)
	}

	fmt.Println("WhatsApp gateway dependencies installed successfully")
	return nil
}

func computeContentHash() string {
	hasher := fnv.New64a()
	hasher.Write(embeddedIndexJS)
	hasher.Write(embeddedPackageJSON)
	return fmt.Sprintf("%x", hasher.Sum64())
}

func nodeAvailable() bool {
	nodeCmd := "node"
	if runtime.GOOS == "windows" {
		nodeCmd = "node.exe"
	}

	cmd := exec.Command(nodeCmd, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	return len(output) > 0
}

func gatewayDir(dataDir string) string {
	return filepath.Join(dataDir, "whatsapp_gateway")
}
