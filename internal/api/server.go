// Package api provides HTTP API server for the FangClaw daemon.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/penzhan8451/fangclaw-go/internal/a2a"
	"github.com/penzhan8451/fangclaw-go/internal/kernel"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/agent"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/llm"
)

func findStaticDir() string {
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		staticPath := filepath.Join(execDir, "internal", "api", "static")
		if _, err := os.Stat(staticPath); err == nil {
			return staticPath
		}
	}

	staticPath := filepath.Join("internal", "api", "static")
	if _, err := os.Stat(staticPath); err == nil {
		return staticPath
	}

	return staticPath
}

func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		fmt.Printf("[%s] %s %s\n", start.Format(time.RFC3339), r.Method, r.URL.Path)
		next(w, r)
		fmt.Printf("[%s] %s %s completed in %v\n", time.Now().Format(time.RFC3339), r.Method, r.URL.Path, time.Since(start))
	}
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

// Server is the OpenFang API server.
type Server struct {
	*http.Server
	Kernel *kernel.Kernel
	Config *ServerConfig
	router *Router
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	ListenAddr   string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DefaultServerConfig returns default configuration.
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		ListenAddr:   "127.0.0.1:4200",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second, // 5 minutes for workflow execution
		IdleTimeout:  60 * time.Second,
	}
}

// NewServer creates a new API server.
func NewServer(k *kernel.Kernel, cfg *ServerConfig) *Server {
	if cfg == nil {
		cfg = DefaultServerConfig()
	}

	mux := http.NewServeMux()
	server := &Server{
		Server: &http.Server{
			Addr:         cfg.ListenAddr,
			Handler:      mux, // Use regular HTTP handler instead of h2c
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		Kernel: k,
		Config: cfg,
	}

	// Register new router
	router := NewRouter(k)
	server.router = router
	router.RegisterRoutes(mux)

	// Register OpenAI-compatible routes
	oaiHandler := NewOpenAICompatibleHandler(k)
	oaiHandler.RegisterRoutes(mux)

	// Register stream routes
	RegisterStreamRoutes(mux, k)

	// Set up A2A task status change callback to broadcast via WebSocket
	a2aTaskStore := k.A2ATaskStore()
	a2aTaskStore.SetStatusCallback(func(task *a2a.A2aTask, oldStatus a2a.A2aTaskStatus, newStatus a2a.A2aTaskStatus) {
		// Prepare A2A event data
		type A2ATaskEventData struct {
			TaskID    string            `json:"task_id"`
			AgentID   string            `json:"agent_id"`
			OldStatus a2a.A2aTaskStatus `json:"old_status"`
			NewStatus a2a.A2aTaskStatus `json:"new_status"`
			Task      *a2a.A2aTask      `json:"task"`
		}
		eventData := A2ATaskEventData{
			TaskID:    task.ID,
			AgentID:   task.AgentID,
			OldStatus: oldStatus,
			NewStatus: newStatus,
			Task:      task,
		}
		eventDataBytes, _ := json.Marshal(eventData)

		// Create WebSocket message
		wsMsg := WSMessage{
			Type: "a2a.task_status_changed",
			Data: eventDataBytes,
		}
		wsMsgBytes, _ := json.Marshal(wsMsg)

		// Broadcast to subscribed clients
		wsManager.BroadcastA2AEvent(task.ID, task.AgentID, wsMsgBytes)
	})

	// Serve static files for Web dashboard
	staticDir := findStaticDir()
	fs := http.FileServer(http.Dir(staticDir))

	// Custom handler to serve index.html for root path
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			indexPath := filepath.Join(staticDir, "index.html")
			http.ServeFile(w, r, indexPath)
			return
		}
		fs.ServeHTTP(w, r)
	})

	return server
}

// Start starts the API server.
func (s *Server) Start() error {
	fmt.Printf("Starting API server on %s...\n", s.Config.ListenAddr)

	// Start server in goroutine
	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	return s.Shutdown(ctx)
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status  string          `json:"status"`
	Healthy bool            `json:"healthy"`
	Checks  map[string]bool `json:"checks,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// StatusResponse represents the daemon status.
type StatusResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	ListenAddr    string `json:"listen_addr"`
	AgentCount    int    `json:"agent_count"`
	ModelCount    int    `json:"model_count"`
	Uptime        string `json:"uptime"`
	UptimeSeconds int    `json:"uptime_seconds"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding response: %v\n", err)
	}
}

// WriteError writes an error response.
func WriteError(w http.ResponseWriter, status int, err error) {
	WriteJSON(w, status, ErrorResponse{Error: err.Error()})
}

// ParseJSON parses JSON from request body.
func ParseJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// SetDefaultAgent sets the default agent for A2A tasks.
func (s *Server) SetDefaultAgent(agentID string) {
	if s.router != nil {
		s.router.SetDefault(agentID)
	}
}

// RunServer runs the API server with signal handling.
func RunServer(k *kernel.Kernel, cfg *ServerConfig, defaultAgentID string) error {
	server := NewServer(k, cfg)

	if defaultAgentID != "" {
		server.SetDefaultAgent(defaultAgentID)
	}

	// Handle shutdown signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-sigChan:
			fmt.Println("\nShutting down...")
		case <-WaitForShutdown():
			fmt.Println("\nShutdown requested via API...")
		}
		cancel()
		server.Stop(context.Background())
	}()

	// Start server
	if err := server.Start(); err != nil {
		return err
	}

	// Wait for shutdown
	<-ctx.Done()
	return nil
}

// ============= SSE (Server-Sent Events) Support =============

// SSEClient represents a client subscribed to SSE events.
type SSEClient struct {
	ID      string
	AgentID string
	Writer  http.ResponseWriter
	Done    chan struct{}
}

// SSEManager manages SSE connections.
type SSEManager struct {
	clients map[string]map[string][]*SSEClient // agentID -> clients
	mu      sync.RWMutex
}

var sseManager = &SSEManager{
	clients: make(map[string]map[string][]*SSEClient),
}

// AddClient adds a new SSE client.
func (m *SSEManager) AddClient(agentID, clientID string, w http.ResponseWriter) *SSEClient {
	client := &SSEClient{
		ID:      clientID,
		AgentID: agentID,
		Writer:  w,
		Done:    make(chan struct{}),
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clients[agentID] == nil {
		m.clients[agentID] = make(map[string][]*SSEClient)
	}
	m.clients[agentID]["default"] = append(m.clients[agentID]["default"], client)

	return client
}

// RemoveClient removes an SSE client.
func (m *SSEManager) RemoveClient(agentID, clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clients[agentID] == nil {
		return
	}

	clients := m.clients[agentID]["default"]
	for i, c := range clients {
		if c.ID == clientID {
			m.clients[agentID]["default"] = append(clients[:i], clients[i+1:]...)
			return
		}
	}
}

// Broadcast sends a message to all clients for an agent.
func (m *SSEManager) Broadcast(agentID, event, data string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := m.clients[agentID]["default"]
	for _, client := range clients {
		select {
		case <-client.Done:
			continue
		default:
			fmt.Fprintf(client.Writer, "event: %s\ndata: %s\n\n", event, data)
			client.Writer.(http.Flusher).Flush()
		}
	}
}

// SSEHandler handles SSE connections for streaming responses.
func SSEHandler(k *kernel.Kernel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Get agent ID from query params
		agentID := r.URL.Query().Get("agent_id")
		if agentID == "" {
			agentID = "default"
		}

		// Create client
		clientID := fmt.Sprintf("sse-%d", time.Now().UnixNano())
		client := sseManager.AddClient(agentID, clientID, w)
		defer sseManager.RemoveClient(agentID, clientID)

		// Send initial connection event
		fmt.Fprintf(w, "event: connected\ndata: {\"client_id\":\"%s\"}\n\n", clientID)
		w.(http.Flusher).Flush()

		// Keep connection open
		<-r.Context().Done()
		close(client.Done)
	}
}

// SSEChatHandler handles SSE chat with streaming responses.
func SSEChatHandler(k *kernel.Kernel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}

		// Simple placeholder response
		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Send placeholder response
		fmt.Fprintf(w, "event: start\ndata: {\"agent_id\":\"default\"}\n\n")
		w.(http.Flusher).Flush()

		response := "This is a placeholder streaming response from FangClaw. The full LLM integration is coming soon!"
		chunkSize := 20
		for i := 0; i < len(response); i += chunkSize {
			end := i + chunkSize
			if end > len(response) {
				end = len(response)
			}
			chunk := response[i:end]
			fmt.Fprintf(w, "event: chunk\ndata: {\"content\":\"%s\"}\n\n", chunk)
			w.(http.Flusher).Flush()
			time.Sleep(50 * time.Millisecond)
		}

		fmt.Fprintf(w, "event: done\ndata: {\"response\":\"%s\"}\n\n", response)
		w.(http.Flusher).Flush()
	}
}

// ============= WebSocket Support =============

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

var shutdownChan = make(chan struct{}, 1)

func RequestShutdown() {
	select {
	case shutdownChan <- struct{}{}:
	default:
	}
}

func WaitForShutdown() <-chan struct{} {
	return shutdownChan
}

// WSMessage represents a WebSocket message.
type WSMessage struct {
	Type    string          `json:"type"`
	AgentID string          `json:"agent_id,omitempty"`
	Message string          `json:"message,omitempty"`
	Content string          `json:"content,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// WSClient represents a WebSocket client.
type WSClient struct {
	ID                    string
	AgentID               string
	Conn                  *websocket.Conn
	Send                  chan []byte
	Done                  chan struct{}
	SubscribedA2ATaskIDs  map[string]bool
	SubscribedA2AAgentIDs map[string]bool
}

// WSManager manages WebSocket connections.
type WSManager struct {
	clients map[string]map[string][]*WSClient
	mu      sync.RWMutex
}

var wsManager = &WSManager{
	clients: make(map[string]map[string][]*WSClient),
}

// AddClient adds a new WebSocket client.
func (m *WSManager) AddClient(agentID, clientID string, conn *websocket.Conn) *WSClient {
	client := &WSClient{
		ID:                    clientID,
		AgentID:               agentID,
		Conn:                  conn,
		Send:                  make(chan []byte, 256),
		Done:                  make(chan struct{}),
		SubscribedA2ATaskIDs:  make(map[string]bool),
		SubscribedA2AAgentIDs: make(map[string]bool),
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clients[agentID] == nil {
		m.clients[agentID] = make(map[string][]*WSClient)
	}
	m.clients[agentID]["default"] = append(m.clients[agentID]["default"], client)

	return client
}

// RemoveClient removes a WebSocket client.
func (m *WSManager) RemoveClient(agentID, clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clients[agentID] == nil {
		return
	}

	clients := m.clients[agentID]["default"]
	for i, c := range clients {
		if c.ID == clientID {
			m.clients[agentID]["default"] = append(clients[:i], clients[i+1:]...)
			return
		}
	}
}

// Broadcast sends a message to all clients for an agent.
func (m *WSManager) Broadcast(agentID string, message []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := m.clients[agentID]["default"]
	for _, client := range clients {
		select {
		case <-client.Done:
			continue
		case client.Send <- message:
		}
	}
}

// BroadcastToAll sends a message to all connected clients.
func (m *WSManager) BroadcastToAll(message []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for agentID := range m.clients {
		clients := m.clients[agentID]["default"]
		for _, client := range clients {
			select {
			case <-client.Done:
				continue
			case client.Send <- message:
			}
		}
	}
}

// SubscribeA2ATask subscribes a client to a specific A2A task.
func (m *WSManager) SubscribeA2ATask(clientID, taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for agentID := range m.clients {
		for _, clients := range m.clients[agentID] {
			for _, client := range clients {
				if client.ID == clientID {
					client.SubscribedA2ATaskIDs[taskID] = true
					return
				}
			}
		}
	}
}

// UnsubscribeA2ATask unsubscribes a client from a specific A2A task.
func (m *WSManager) UnsubscribeA2ATask(clientID, taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for agentID := range m.clients {
		for _, clients := range m.clients[agentID] {
			for _, client := range clients {
				if client.ID == clientID {
					delete(client.SubscribedA2ATaskIDs, taskID)
					return
				}
			}
		}
	}
}

// SubscribeA2AAgent subscribes a client to all A2A tasks from a specific agent.
func (m *WSManager) SubscribeA2AAgent(clientID, agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for aID := range m.clients {
		for _, clients := range m.clients[aID] {
			for _, client := range clients {
				if client.ID == clientID {
					client.SubscribedA2AAgentIDs[agentID] = true
					return
				}
			}
		}
	}
}

// UnsubscribeA2AAgent unsubscribes a client from all A2A tasks from a specific agent.
func (m *WSManager) UnsubscribeA2AAgent(clientID, agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for aID := range m.clients {
		for _, clients := range m.clients[aID] {
			for _, client := range clients {
				if client.ID == clientID {
					delete(client.SubscribedA2AAgentIDs, agentID)
					return
				}
			}
		}
	}
}

// BroadcastA2AEvent sends an A2A event to subscribed clients.
func (m *WSManager) BroadcastA2AEvent(taskID string, agentID string, message []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for aID := range m.clients {
		for _, clients := range m.clients[aID] {
			for _, client := range clients {
				shouldSend := false

				// Check if subscribed to the specific task
				if client.SubscribedA2ATaskIDs[taskID] {
					shouldSend = true
				}

				// Check if subscribed to the agent
				if !shouldSend && client.SubscribedA2AAgentIDs[agentID] {
					shouldSend = true
				}

				if shouldSend {
					select {
					case <-client.Done:
						continue
					case client.Send <- message:
					}
				}
			}
		}
	}
}

// A2AWSHandler handles A2A WebSocket connections for task status updates.
func A2AWSHandler(k *kernel.Kernel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get subscription parameters from query
		taskID := r.URL.Query().Get("taskId")
		agentID := r.URL.Query().Get("agentId")

		// Use a special agent ID for A2A connections
		a2aAgentID := "a2a"

		// Upgrade connection
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "A2A WebSocket upgrade error: %v\n", err)
			return
		}
		defer conn.Close()

		// Create client
		clientID := fmt.Sprintf("a2a-ws-%d", time.Now().UnixNano())
		client := wsManager.AddClient(a2aAgentID, clientID, conn)
		defer wsManager.RemoveClient(a2aAgentID, clientID)

		// Auto-subscribe based on query parameters
		if taskID != "" {
			wsManager.SubscribeA2ATask(clientID, taskID)
			fmt.Printf("[A2A WS] Client %s subscribed to task %s\n", clientID, taskID)
		}
		if agentID != "" {
			wsManager.SubscribeA2AAgent(clientID, agentID)
			fmt.Printf("[A2A WS] Client %s subscribed to agent %s\n", clientID, agentID)
		}

		// Send welcome message
		welcomeData := map[string]interface{}{
			"client_id":           clientID,
			"subscribed_task_id":  taskID,
			"subscribed_agent_id": agentID,
		}
		welcomeDataBytes, _ := json.Marshal(welcomeData)
		welcome := WSMessage{Type: "connected", Data: welcomeDataBytes}
		welcomeBytes, _ := json.Marshal(welcome)
		client.Send <- welcomeBytes

		// Handle messages in both directions
		_, cancel := context.WithCancel(r.Context())
		defer cancel()

		// Read loop
		go func() {
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						fmt.Fprintf(os.Stderr, "[A2A WS] Read error: %v\n", err)
					}
					select {
					case <-client.Done:
					default:
						close(client.Done)
					}
					return
				}

				// Parse message
				var msg WSMessage
				if err := json.Unmarshal(message, &msg); err != nil {
					fmt.Fprintf(os.Stderr, "[A2A WS] Parse error: %v\n", err)
					type ErrorData struct {
						Error string `json:"error"`
					}
					errorData := ErrorData{Error: err.Error()}
					errorDataBytes, _ := json.Marshal(errorData)
					errorResp, _ := json.Marshal(WSMessage{Type: "error", Data: errorDataBytes})
					client.Send <- errorResp
					continue
				}

				// Handle A2A subscription messages
				switch msg.Type {
				case "ping":
					pong, _ := json.Marshal(WSMessage{Type: "pong"})
					client.Send <- pong

				case "a2a.subscribe_task":
					type SubscribeTaskData struct {
						TaskID string `json:"task_id"`
					}
					var data SubscribeTaskData
					if err := json.Unmarshal(msg.Data, &data); err == nil && data.TaskID != "" {
						wsManager.SubscribeA2ATask(clientID, data.TaskID)
						ack, _ := json.Marshal(WSMessage{Type: "a2a.subscribed", Data: json.RawMessage(fmt.Sprintf(`{"task_id":"%s","type":"task"}`, data.TaskID))})
						client.Send <- ack
						fmt.Printf("[A2A WS] Client %s subscribed to task %s\n", clientID, data.TaskID)
					}

				case "a2a.unsubscribe_task":
					type UnsubscribeTaskData struct {
						TaskID string `json:"task_id"`
					}
					var data UnsubscribeTaskData
					if err := json.Unmarshal(msg.Data, &data); err == nil && data.TaskID != "" {
						wsManager.UnsubscribeA2ATask(clientID, data.TaskID)
						ack, _ := json.Marshal(WSMessage{Type: "a2a.unsubscribed", Data: json.RawMessage(fmt.Sprintf(`{"task_id":"%s","type":"task"}`, data.TaskID))})
						client.Send <- ack
						fmt.Printf("[A2A WS] Client %s unsubscribed from task %s\n", clientID, data.TaskID)
					}

				case "a2a.subscribe_agent":
					type SubscribeAgentData struct {
						AgentID string `json:"agent_id"`
					}
					var data SubscribeAgentData
					if err := json.Unmarshal(msg.Data, &data); err == nil && data.AgentID != "" {
						wsManager.SubscribeA2AAgent(clientID, data.AgentID)
						ack, _ := json.Marshal(WSMessage{Type: "a2a.subscribed", Data: json.RawMessage(fmt.Sprintf(`{"agent_id":"%s","type":"agent"}`, data.AgentID))})
						client.Send <- ack
						fmt.Printf("[A2A WS] Client %s subscribed to agent %s\n", clientID, data.AgentID)
					}

				case "a2a.unsubscribe_agent":
					type UnsubscribeAgentData struct {
						AgentID string `json:"agent_id"`
					}
					var data UnsubscribeAgentData
					if err := json.Unmarshal(msg.Data, &data); err == nil && data.AgentID != "" {
						wsManager.UnsubscribeA2AAgent(clientID, data.AgentID)
						ack, _ := json.Marshal(WSMessage{Type: "a2a.unsubscribed", Data: json.RawMessage(fmt.Sprintf(`{"agent_id":"%s","type":"agent"}`, data.AgentID))})
						client.Send <- ack
						fmt.Printf("[A2A WS] Client %s unsubscribed from agent %s\n", clientID, data.AgentID)
					}

				default:
					errorResp, _ := json.Marshal(WSMessage{Type: "error", Data: json.RawMessage(`{"content":"unknown message type for A2A WebSocket"}`)})
					client.Send <- errorResp
				}
			}
		}()

		// Write loop
		go func() {
			defer func() {
				select {
				case <-client.Done:
				default:
					close(client.Done)
				}
				cancel()
			}()

			for {
				select {
				case <-client.Done:
					return
				case message := <-client.Send:
					if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
						fmt.Fprintf(os.Stderr, "[A2A WS] Write error: %v\n", err)
						return
					}
				}
			}
		}()

		<-client.Done
	}
}

// WSHandler handles WebSocket connections.
func WSHandler(k *kernel.Kernel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get agent ID from query params
		agentID := r.URL.Query().Get("agent_id")
		if agentID == "" {
			agentID = "default"
		}

		// Upgrade connection
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WebSocket upgrade error: %v\n", err)
			return
		}
		defer conn.Close()

		// Create client
		clientID := fmt.Sprintf("ws-%d", time.Now().UnixNano())
		client := wsManager.AddClient(agentID, clientID, conn)
		defer wsManager.RemoveClient(agentID, clientID)

		// Send welcome message
		welcome := WSMessage{Type: "connected", AgentID: agentID, Data: json.RawMessage(fmt.Sprintf(`{"client_id":"%s"}`, clientID))}
		welcomeBytes, _ := json.Marshal(welcome)
		client.Send <- welcomeBytes

		// Handle messages in both directions
		_, cancel := context.WithCancel(r.Context())
		defer cancel()

		// Read loop
		go func() {
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						fmt.Fprintf(os.Stderr, "WebSocket read error: %v\n", err)
					}
					// Signal done, but don't close channel (write loop may still use it)
					select {
					case <-client.Done:
					default:
						close(client.Done)
					}
					return
				}

				// Parse message
				var msg WSMessage
				if err := json.Unmarshal(message, &msg); err != nil {
					fmt.Fprintf(os.Stderr, "[WebSocket] Parse error: %v\n", err)
					type ErrorData struct {
						Error string `json:"error"`
					}
					errorData := ErrorData{Error: err.Error()}
					errorDataBytes, _ := json.Marshal(errorData)
					errorResp, _ := json.Marshal(WSMessage{Type: "error", Data: errorDataBytes})
					client.Send <- errorResp
					continue
				}

				// Handle different message types
				switch msg.Type {
				case "chat":
					fallthrough
				case "message":
					// Process chat request
					go func() {
						// Send typing start
						typingStart, _ := json.Marshal(WSMessage{Type: "typing", Data: json.RawMessage(`{"state":"start"}`)})
						client.Send <- typingStart

						// Get message content
						text := msg.Content
						if text == "" {
							// Send error
							errorData := map[string]string{"content": "Error: No message content"}
							errorDataBytes, _ := json.Marshal(errorData)
							errorResp := WSMessage{Type: "error", Data: errorDataBytes}
							errorRespBytes, _ := json.Marshal(errorResp)
							client.Send <- errorRespBytes
							return
						}

						// Get agent runtime
						agentRuntime := k.AgentRuntime()
						if agentRuntime == nil {
							// Send error
							errorData := map[string]string{"content": "Error: Agent runtime not available"}
							errorDataBytes, _ := json.Marshal(errorData)
							errorResp := WSMessage{Type: "error", Data: errorDataBytes}
							errorRespBytes, _ := json.Marshal(errorResp)
							client.Send <- errorRespBytes
							return
						}

						// Agent lookup
						var actualAgentID string
						if _, ok := agentRuntime.GetAgent(agentID); ok {
							actualAgentID = agentID
						} else if agentCtx, ok := agentRuntime.FindAgentByName(agentID); ok {
							actualAgentID = agentCtx.ID
						} else {
							if agentEntry := k.AgentRegistry().FindByName(agentID); agentEntry != nil {
								actualAgentID = agentEntry.ID.String()
							} else {
								if agentCtx, ok := agentRuntime.GetFirstAgent(); ok {
									actualAgentID = agentCtx.ID
								} else {
									// Send error
									errorData := map[string]string{"content": "Error: No agents available"}
									errorDataBytes, _ := json.Marshal(errorData)
									errorResp := WSMessage{Type: "error", Data: errorDataBytes}
									errorRespBytes, _ := json.Marshal(errorResp)
									client.Send <- errorRespBytes
									return
								}
							}
						}

						// Stream callback to send text delta via WebSocket
						var fullResponse strings.Builder
						streamCb := func(event llm.StreamEvent) {
							switch event.Type {
							case llm.StreamEventTextDelta:
								if event.Text != "" {
									fullResponse.WriteString(event.Text)
									// Send streaming response using text_delta type
									deltaData := map[string]string{"content": event.Text}
									deltaDataBytes, _ := json.Marshal(deltaData)
									deltaMsg := WSMessage{Type: "text_delta", Data: deltaDataBytes}
									deltaMsgBytes, _ := json.Marshal(deltaMsg)
									client.Send <- deltaMsgBytes
								}
							}
						}

						// Phase callback
						onPhase := func(phase agent.LoopPhase) {
							switch phase {
							case agent.PhaseThinking:
								// Send thinking state
								thinkingData := map[string]interface{}{"phase": "thinking", "detail": "Thinking..."}
								thinkingDataBytes, _ := json.Marshal(thinkingData)
								thinkingMsg := WSMessage{Type: "phase", Data: thinkingDataBytes}
								thinkingMsgBytes, _ := json.Marshal(thinkingMsg)
								client.Send <- thinkingMsgBytes
							case agent.PhaseToolUse:
								// Send tool use state
								toolData := map[string]interface{}{"phase": "tool_use", "detail": "Using tools..."}
								toolDataBytes, _ := json.Marshal(toolData)
								toolMsg := WSMessage{Type: "phase", Data: toolDataBytes}
								toolMsgBytes, _ := json.Marshal(toolMsg)
								client.Send <- toolMsgBytes
							}
						}

						// Run agent with full loop
						runner := agent.NewAgentRunner(agentRuntime)
						ctx := context.Background()
						result, err := runner.RunAgent(ctx, actualAgentID, text, onPhase, streamCb)

						// Send typing stop
						typingStop, _ := json.Marshal(WSMessage{Type: "typing", Data: json.RawMessage(`{"state":"stop"}`)})
						client.Send <- typingStop

						if err != nil {
							fmt.Fprintf(os.Stderr, "[WebSocket] Agent error: %v\n", err)
							// Send error
							errorData := map[string]string{"content": "Error: " + err.Error()}
							errorDataBytes, _ := json.Marshal(errorData)
							errorResp := WSMessage{Type: "error", Data: errorDataBytes}
							errorRespBytes, _ := json.Marshal(errorResp)
							client.Send <- errorRespBytes
						} else {
							// Send final response
							type ResponseData struct {
								Content      string `json:"content"`
								InputTokens  int    `json:"input_tokens"`
								OutputTokens int    `json:"output_tokens"`
								Iterations   int    `json:"iterations"`
							}
							respData := ResponseData{
								Content:      result.Response,
								InputTokens:  result.TotalUsage.PromptTokens,
								OutputTokens: result.TotalUsage.CompletionTokens,
								Iterations:   int(result.Iterations),
							}
							respDataBytes, _ := json.Marshal(respData)
							respMsg := WSMessage{Type: "response", Data: respDataBytes}
							respBytes, _ := json.Marshal(respMsg)
							client.Send <- respBytes
						}
					}()

				case "ping":
					pong, _ := json.Marshal(WSMessage{Type: "pong"})
					client.Send <- pong

				case "a2a.subscribe_task":
					type SubscribeTaskData struct {
						TaskID string `json:"task_id"`
					}
					var data SubscribeTaskData
					if err := json.Unmarshal(msg.Data, &data); err == nil && data.TaskID != "" {
						wsManager.SubscribeA2ATask(clientID, data.TaskID)
						ack, _ := json.Marshal(WSMessage{Type: "a2a.subscribed", Data: json.RawMessage(fmt.Sprintf(`{"task_id":"%s","type":"task"}`, data.TaskID))})
						client.Send <- ack
					}

				case "a2a.unsubscribe_task":
					type UnsubscribeTaskData struct {
						TaskID string `json:"task_id"`
					}
					var data UnsubscribeTaskData
					if err := json.Unmarshal(msg.Data, &data); err == nil && data.TaskID != "" {
						wsManager.UnsubscribeA2ATask(clientID, data.TaskID)
						ack, _ := json.Marshal(WSMessage{Type: "a2a.unsubscribed", Data: json.RawMessage(fmt.Sprintf(`{"task_id":"%s","type":"task"}`, data.TaskID))})
						client.Send <- ack
					}

				case "a2a.subscribe_agent":
					type SubscribeAgentData struct {
						AgentID string `json:"agent_id"`
					}
					var data SubscribeAgentData
					if err := json.Unmarshal(msg.Data, &data); err == nil && data.AgentID != "" {
						wsManager.SubscribeA2AAgent(clientID, data.AgentID)
						ack, _ := json.Marshal(WSMessage{Type: "a2a.subscribed", Data: json.RawMessage(fmt.Sprintf(`{"agent_id":"%s","type":"agent"}`, data.AgentID))})
						client.Send <- ack
					}

				case "a2a.unsubscribe_agent":
					type UnsubscribeAgentData struct {
						AgentID string `json:"agent_id"`
					}
					var data UnsubscribeAgentData
					if err := json.Unmarshal(msg.Data, &data); err == nil && data.AgentID != "" {
						wsManager.UnsubscribeA2AAgent(clientID, data.AgentID)
						ack, _ := json.Marshal(WSMessage{Type: "a2a.unsubscribed", Data: json.RawMessage(fmt.Sprintf(`{"agent_id":"%s","type":"agent"}`, data.AgentID))})
						client.Send <- ack
					}

				default:
					errorResp, _ := json.Marshal(WSMessage{Type: "error", Data: json.RawMessage(`{"content":"unknown message type"}`)})
					client.Send <- errorResp
				}
			}
		}()

		// Write loop
		go func() {
			defer func() {
				// Only close channel if not already closed
				select {
				case <-client.Done:
				default:
					close(client.Done)
				}
				cancel()
			}()

			for {
				select {
				case <-client.Done:
					return
				case message := <-client.Send:
					if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
						fmt.Fprintf(os.Stderr, "[WebSocket Write] Error: %v\n", err)
						return
					}
				}
			}
		}()

		// Wait for disconnect
		<-client.Done
	}
}

// RegisterStreamRoutes registers SSE and WebSocket routes.
func RegisterStreamRoutes(mux *http.ServeMux, k *kernel.Kernel) {
	// SSE endpoints
	mux.HandleFunc("/api/stream/events", loggingMiddleware(corsMiddleware(SSEHandler(k))))
	mux.HandleFunc("/api/stream/chat", loggingMiddleware(corsMiddleware(SSEChatHandler(k))))

	// WebSocket endpoints
	mux.HandleFunc("/api/ws", loggingMiddleware(corsMiddleware(WSHandler(k))))
	mux.HandleFunc("/ws/a2a/tasks", loggingMiddleware(corsMiddleware(A2AWSHandler(k))))
}

// StreamEvent represents a streaming event.
type StreamEvent struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}
