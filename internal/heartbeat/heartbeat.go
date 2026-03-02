// Package heartbeat provides heartbeat monitoring for OpenFang.
package heartbeat

import (
	"context"
	"sync"
	"time"
)

// HeartbeatStatus represents the status of a heartbeat.
type HeartbeatStatus string

const (
	HeartbeatStatusHealthy HeartbeatStatus = "healthy"
	HeartbeatStatusWarning HeartbeatStatus = "warning"
	HeartbeatStatusError   HeartbeatStatus = "error"
	HeartbeatStatusStopped HeartbeatStatus = "stopped"
)

// Heartbeat represents a single heartbeat record.
type Heartbeat struct {
	ID        string
	Service   string
	Status    HeartbeatStatus
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// HeartbeatMonitor monitors service health via heartbeats.
type HeartbeatMonitor struct {
	mu          sync.RWMutex
	heartbeats  map[string]*Heartbeat
	listeners   []func(Heartbeat)
	interval    time.Duration
	timeout     time.Duration
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewHeartbeatMonitor creates a new heartbeat monitor.
func NewHeartbeatMonitor(interval, timeout time.Duration) *HeartbeatMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	if interval <= 0 {
		interval = 5 * time.Second
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	monitor := &HeartbeatMonitor{
		heartbeats: make(map[string]*Heartbeat),
		listeners:   make([]func(Heartbeat), 0),
		interval:    interval,
		timeout:     timeout,
		ctx:         ctx,
		cancel:      cancel,
	}

	go monitor.monitorLoop()

	return monitor
}

// Register registers a service for heartbeat monitoring.
func (h *HeartbeatMonitor) Register(serviceID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.heartbeats[serviceID] = &Heartbeat{
		ID:        serviceID,
		Service:   serviceID,
		Status:    HeartbeatStatusHealthy,
		Timestamp: time.Now(),
	}
}

// Unregister unregisters a service from heartbeat monitoring.
func (h *HeartbeatMonitor) Unregister(serviceID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if heartbeat, ok := h.heartbeats[serviceID]; ok {
		heartbeat.Status = HeartbeatStatusStopped
		h.notifyListeners(*heartbeat)
		delete(h.heartbeats, serviceID)
	}
}

// Beat sends a heartbeat for a service.
func (h *HeartbeatMonitor) Beat(serviceID string, metadata map[string]interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	heartbeat, ok := h.heartbeats[serviceID]
	if !ok {
		heartbeat = &Heartbeat{
			ID:      serviceID,
			Service: serviceID,
		}
		h.heartbeats[serviceID] = heartbeat
	}

	heartbeat.Status = HeartbeatStatusHealthy
	heartbeat.Timestamp = time.Now()
	heartbeat.Metadata = metadata

	h.notifyListeners(*heartbeat)
}

// GetStatus gets the status of a service.
func (h *HeartbeatMonitor) GetStatus(serviceID string) (HeartbeatStatus, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	heartbeat, ok := h.heartbeats[serviceID]
	if !ok {
		return "", false
	}
	return heartbeat.Status, true
}

// GetAllStatuses gets the status of all monitored services.
func (h *HeartbeatMonitor) GetAllStatuses() map[string]HeartbeatStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	statuses := make(map[string]HeartbeatStatus, len(h.heartbeats))
	for id, heartbeat := range h.heartbeats {
		statuses[id] = heartbeat.Status
	}
	return statuses
}

// GetHeartbeat gets a specific heartbeat.
func (h *HeartbeatMonitor) GetHeartbeat(serviceID string) (*Heartbeat, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	heartbeat, ok := h.heartbeats[serviceID]
	if !ok {
		return nil, false
	}

	clone := *heartbeat
	return &clone, true
}

// AddListener adds a listener for heartbeat changes.
func (h *HeartbeatMonitor) AddListener(listener func(Heartbeat)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.listeners = append(h.listeners, listener)
}

// Stop stops the heartbeat monitor.
func (h *HeartbeatMonitor) Stop() {
	h.cancel()
}

// monitorLoop checks for timed out heartbeats.
func (h *HeartbeatMonitor) monitorLoop() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.checkTimeouts()
		}
	}
}

// checkTimeouts checks for services that haven't sent a heartbeat recently.
func (h *HeartbeatMonitor) checkTimeouts() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for _, heartbeat := range h.heartbeats {
		if heartbeat.Status == HeartbeatStatusStopped {
			continue
		}

		elapsed := now.Sub(heartbeat.Timestamp)
		var newStatus HeartbeatStatus

		if elapsed > h.timeout {
			newStatus = HeartbeatStatusError
		} else if elapsed > h.timeout/2 {
			newStatus = HeartbeatStatusWarning
		} else {
			continue
		}

		if heartbeat.Status != newStatus {
			heartbeat.Status = newStatus
			h.notifyListeners(*heartbeat)
		}
	}
}

// notifyListeners notifies all listeners of a heartbeat change.
func (h *HeartbeatMonitor) notifyListeners(heartbeat Heartbeat) {
	for _, listener := range h.listeners {
		go listener(heartbeat)
	}
}

// IsHealthy checks if a service is healthy.
func (h *HeartbeatMonitor) IsHealthy(serviceID string) bool {
	status, ok := h.GetStatus(serviceID)
	if !ok {
		return false
	}
	return status == HeartbeatStatusHealthy
}

// GetUnhealthyServices returns a list of unhealthy services.
func (h *HeartbeatMonitor) GetUnhealthyServices() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	unhealthy := make([]string, 0)
	for id, heartbeat := range h.heartbeats {
		if heartbeat.Status != HeartbeatStatusHealthy && heartbeat.Status != HeartbeatStatusStopped {
			unhealthy = append(unhealthy, id)
		}
	}
	return unhealthy
}
