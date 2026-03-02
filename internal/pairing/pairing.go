package pairing

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const (
	maxPendingRequests = 5
	tokenLength        = 32
	tokenExpiryMinutes = 5
)

type PairedDevice struct {
	DeviceID     string
	DisplayName  string
	Platform     string
	PairedAt     time.Time
	LastSeen     time.Time
	PushToken    *string
}

type PairingRequest struct {
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type PairingConfig struct {
	Enabled        bool
	MaxDevices     int
	TokenExpiry    time.Duration
}

type PersistOp int

const (
	PersistSave PersistOp = iota
	PersistRemove
)

type PersistFn func(device *PairedDevice, op PersistOp)

type PairingManager struct {
	config   PairingConfig
	pending  map[string]*PairingRequest
	devices  map[string]*PairedDevice
	persist  PersistFn
	mu       sync.RWMutex
}

func NewPairingManager(config PairingConfig) *PairingManager {
	return &PairingManager{
		config:   config,
		pending:  make(map[string]*PairingRequest),
		devices:  make(map[string]*PairedDevice),
	}
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

	if len(pm.pending) >= maxPendingRequests {
		return nil, nil
	}

	tokenBytes := make([]byte, tokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)

	expiry := pm.config.TokenExpiry
	if expiry == 0 {
		expiry = tokenExpiryMinutes * time.Minute
	}

	req := &PairingRequest{
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(expiry),
	}

	pm.pending[token] = req
	return req, nil
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
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.pending[token]; !exists {
		return nil, nil
	}

	delete(pm.pending, token)

	device := &PairedDevice{
		DeviceID:    deviceID,
		DisplayName: displayName,
		Platform:    platform,
		PairedAt:    time.Now(),
		LastSeen:    time.Now(),
	}

	pm.devices[deviceID] = device

	if pm.persist != nil {
		pm.persist(device, PersistSave)
	}

	return device, nil
}

func (pm *PairingManager) RemoveDevice(deviceID string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	device, exists := pm.devices[deviceID]
	if !exists {
		return false
	}

	delete(pm.devices, deviceID)

	if pm.persist != nil {
		pm.persist(device, PersistRemove)
	}

	return true
}

func (pm *PairingManager) GetDevice(deviceID string) (*PairedDevice, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	device, exists := pm.devices[deviceID]
	return device, exists
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

func (pm *PairingManager) UpdateLastSeen(deviceID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if device, exists := pm.devices[deviceID]; exists {
		device.LastSeen = time.Now()
	}
}
