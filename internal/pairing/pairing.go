package pairing

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	maxPendingRequests = 5
	tokenLength        = 32
)

type PairedDevice struct {
	DeviceID    string
	DisplayName string
	Platform    string
	PairedAt    time.Time
	LastSeen    time.Time
	PushToken   *string
}

type PairingRequest struct {
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type PairingConfig struct {
	Enabled         bool
	MaxDevices      int
	TokenExpirySecs int
	PushProvider    string
	NtfyURL         *string
	NtfyTopic       *string
}

type PersistOp int

const (
	PersistSave PersistOp = iota
	PersistRemove
)

type PersistFn func(device *PairedDevice, op PersistOp)

type NotifyResult struct {
	DeviceID string
	Error    error
}

type PairingManager struct {
	config  PairingConfig
	pending map[string]*PairingRequest
	devices map[string]*PairedDevice
	persist PersistFn
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.RWMutex
}

func NewPairingManager(config PairingConfig) *PairingManager {
	ctx, cancel := context.WithCancel(context.Background())
	pm := &PairingManager{
		config:  config,
		pending: make(map[string]*PairingRequest),
		devices: make(map[string]*PairedDevice),
		ctx:     ctx,
		cancel:  cancel,
	}

	go pm.cleanupLoop()
	return pm
}

func (pm *PairingManager) Stop() {
	pm.cancel()
}

func (pm *PairingManager) SetPersist(fn PersistFn) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.persist = fn
}

func (pm *PairingManager) LoadDevices(devices []*PairedDevice) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, d := range devices {
		pm.devices[d.DeviceID] = d
	}
}

func (pm *PairingManager) CreatePairingRequest() (*PairingRequest, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.config.Enabled {
		return nil, fmt.Errorf("device pairing is disabled")
	}

	if len(pm.pending) >= maxPendingRequests {
		pm.cleanExpiredLocked()
		if len(pm.pending) >= maxPendingRequests {
			return nil, fmt.Errorf("too many pending pairing requests. try again later")
		}
	}

	tokenBytes := make([]byte, tokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)

	expirySecs := pm.config.TokenExpirySecs
	if expirySecs == 0 {
		expirySecs = 300
	}

	req := &PairingRequest{
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(expirySecs) * time.Second),
	}

	pm.pending[token] = req
	return req, nil
}

func (pm *PairingManager) CompletePairing(token string, device PairedDevice) (*PairedDevice, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var foundReq *PairingRequest
	var foundToken string
	for t, req := range pm.pending {
		if subtle.ConstantTimeCompare([]byte(t), []byte(token)) == 1 {
			foundReq = req
			foundToken = t
			break
		}
	}

	if foundReq == nil {
		return nil, fmt.Errorf("invalid or expired pairing token")
	}

	if time.Now().After(foundReq.ExpiresAt) {
		delete(pm.pending, foundToken)
		return nil, fmt.Errorf("pairing token has expired")
	}

	if len(pm.devices) >= pm.config.MaxDevices {
		if pm.config.MaxDevices == 0 {
			pm.config.MaxDevices = 10
		}
		return nil, fmt.Errorf("maximum paired devices (%d) reached. remove a device first", pm.config.MaxDevices)
	}

	delete(pm.pending, foundToken)

	newDevice := &PairedDevice{
		DeviceID:    device.DeviceID,
		DisplayName: device.DisplayName,
		Platform:    device.Platform,
		PairedAt:    time.Now(),
		LastSeen:    time.Now(),
		PushToken:   device.PushToken,
	}

	pm.devices[newDevice.DeviceID] = newDevice

	if pm.persist != nil {
		pm.persist(newDevice, PersistSave)
	}

	return newDevice, nil
}

func (pm *PairingManager) RemoveDevice(deviceID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	device, exists := pm.devices[deviceID]
	if !exists {
		return fmt.Errorf("device '%s' not found", deviceID)
	}

	delete(pm.devices, deviceID)

	if pm.persist != nil {
		pm.persist(device, PersistRemove)
	}

	return nil
}

func (pm *PairingManager) ListDevices() []*PairedDevice {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	devices := make([]*PairedDevice, 0, len(pm.devices))
	for _, d := range pm.devices {
		devices = append(devices, d)
	}
	return devices
}

func (pm *PairingManager) Config() PairingConfig {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.config
}

func (pm *PairingManager) GetDevice(deviceID string) (*PairedDevice, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	device, exists := pm.devices[deviceID]
	return device, exists
}

func (pm *PairingManager) UpdateLastSeen(deviceID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if device, exists := pm.devices[deviceID]; exists {
		device.LastSeen = time.Now()
	}
}

func (pm *PairingManager) CleanExpired() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.cleanExpiredLocked()
}

func (pm *PairingManager) cleanExpiredLocked() {
	now := time.Now()
	for token, req := range pm.pending {
		if now.After(req.ExpiresAt) {
			delete(pm.pending, token)
		}
	}
}

func (pm *PairingManager) NotifyDevices(ctx context.Context, title, message string) []NotifyResult {
	results := make([]NotifyResult, 0)

	switch pm.config.PushProvider {
	case "ntfy":
		results = pm.notifyNtfy(ctx, title, message)
	case "gotify":
		results = pm.notifyGotify(ctx, title, message)
	case "none", "":
	default:
	}

	return results
}

func (pm *PairingManager) notifyNtfy(ctx context.Context, title, message string) []NotifyResult {
	results := make([]NotifyResult, 0)

	url := "https://ntfy.sh"
	if pm.config.NtfyURL != nil && *pm.config.NtfyURL != "" {
		url = *pm.config.NtfyURL
	}

	var topic string
	if pm.config.NtfyTopic == nil || *pm.config.NtfyTopic == "" {
		results = append(results, NotifyResult{
			DeviceID: "ntfy",
			Error:    fmt.Errorf("ntfy_topic not configured"),
		})
		return results
	}
	topic = *pm.config.NtfyTopic

	fullURL := fmt.Sprintf("%s/%s", trimTrailingSlash(url), topic)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, bytes.NewBufferString(message))
	if err != nil {
		results = append(results, NotifyResult{
			DeviceID: "ntfy",
			Error:    fmt.Errorf("ntfy request failed: %w", err),
		})
		return results
	}
	req.Header.Set("Title", title)

	resp, err := client.Do(req)
	if err != nil {
		results = append(results, NotifyResult{
			DeviceID: "ntfy",
			Error:    fmt.Errorf("ntfy request failed: %w", err),
		})
		return results
	}
	defer resp.Body.Close()

	pm.mu.RLock()
	devices := make([]*PairedDevice, 0, len(pm.devices))
	for _, d := range pm.devices {
		devices = append(devices, d)
	}
	pm.mu.RUnlock()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		for _, device := range devices {
			results = append(results, NotifyResult{
				DeviceID: device.DeviceID,
				Error:    nil,
			})
		}
	} else {
		results = append(results, NotifyResult{
			DeviceID: "ntfy",
			Error:    fmt.Errorf("ntfy returned HTTP %d", resp.StatusCode),
		})
	}

	return results
}

func (pm *PairingManager) notifyGotify(ctx context.Context, title, message string) []NotifyResult {
	results := make([]NotifyResult, 0)

	appToken := os.Getenv("GOTIFY_APP_TOKEN")
	if appToken == "" {
		results = append(results, NotifyResult{
			DeviceID: "gotify",
			Error:    fmt.Errorf("GOTIFY_APP_TOKEN not set"),
		})
		return results
	}

	serverURL := os.Getenv("GOTIFY_SERVER_URL")
	if serverURL == "" {
		results = append(results, NotifyResult{
			DeviceID: "gotify",
			Error:    fmt.Errorf("GOTIFY_SERVER_URL not set"),
		})
		return results
	}

	url := fmt.Sprintf("%s/message", trimTrailingSlash(serverURL))

	body := map[string]interface{}{
		"title":    title,
		"message":  message,
		"priority": 5,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		results = append(results, NotifyResult{
			DeviceID: "gotify",
			Error:    fmt.Errorf("failed to marshal gotify request: %w", err),
		})
		return results
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		results = append(results, NotifyResult{
			DeviceID: "gotify",
			Error:    fmt.Errorf("gotify request failed: %w", err),
		})
		return results
	}
	req.Header.Set("X-Gotify-Key", appToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		results = append(results, NotifyResult{
			DeviceID: "gotify",
			Error:    fmt.Errorf("gotify request failed: %w", err),
		})
		return results
	}
	defer resp.Body.Close()

	pm.mu.RLock()
	devices := make([]*PairedDevice, 0, len(pm.devices))
	for _, d := range pm.devices {
		devices = append(devices, d)
	}
	pm.mu.RUnlock()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		for _, device := range devices {
			results = append(results, NotifyResult{
				DeviceID: device.DeviceID,
				Error:    nil,
			})
		}
	} else {
		results = append(results, NotifyResult{
			DeviceID: "gotify",
			Error:    fmt.Errorf("gotify returned HTTP %d", resp.StatusCode),
		})
	}

	return results
}

func (pm *PairingManager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.CleanExpired()
		}
	}
}

func trimTrailingSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func (pm *PairingManager) ValidatePairingToken(token string) (*PairingRequest, bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	req, exists := pm.pending[token]
	if !exists {
		return nil, false
	}

	if time.Now().After(req.ExpiresAt) {
		delete(pm.pending, token)
		return nil, false
	}

	return req, true
}

func (pm *PairingManager) PairDevice(token string, deviceID, displayName, platform string) (*PairedDevice, error) {
	device := PairedDevice{
		DeviceID:    deviceID,
		DisplayName: displayName,
		Platform:    platform,
	}
	return pm.CompletePairing(token, device)
}
