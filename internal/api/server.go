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
	"github.com/penzhan8451/fangclaw-go/internal/hands"
	"github.com/penzhan8451/fangclaw-go/internal/kernel"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/llm"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
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
		WriteTimeout: 30 * time.Second,
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
			Handler:      h2c.NewHandler(mux, &http2.Server{}),
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		Kernel: k,
		Config: cfg,
	}

	// Register new router
	router := NewRouter(k)
	router.RegisterRoutes(mux)

	// Register OpenAI-compatible routes
	oaiHandler := NewOpenAICompatibleHandler(k)
	oaiHandler.RegisterRoutes(mux)

	// Register stream routes
	RegisterStreamRoutes(mux, k)

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
	Status     string `json:"status"`
	Version    string `json:"version"`
	ListenAddr string `json:"listen_addr"`
	AgentCount int    `json:"agent_count"`
	ModelCount int    `json:"model_count"`
	Uptime     string `json:"uptime"`
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

// RunServer runs the API server with signal handling.
func RunServer(k *kernel.Kernel, cfg *ServerConfig) error {
	server := NewServer(k, cfg)

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
	ID      string
	AgentID string
	Conn    *websocket.Conn
	Send    chan []byte
	Done    chan struct{}
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
		ID:      clientID,
		AgentID: agentID,
		Conn:    conn,
		Send:    make(chan []byte, 256),
		Done:    make(chan struct{}),
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
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
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
					errorResp, _ := json.Marshal(WSMessage{Type: "error", Data: json.RawMessage(fmt.Sprintf(`{"error":"%s"}`, err.Error()))})
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

						var response string
						var inputTokens, outputTokens int

						// Get message content
						text := msg.Content
						if text == "" {
							response = "Error: No message content"
						} else {
							// Try to get LLM driver
							driver, err := getLLMDriver()
							if err != nil {
								response = "👋 Hi! I'm FangClaw-go. To use the full chat capabilities, please set up an API key.\n\n**Supported providers:**\n- OpenRouter (recommended)\n- OpenAI\n- Anthropic\n- Groq\n\n**How to set up:**\n1. Go to Settings page\n2. Select your preferred provider\n3. Enter your API key\n\nOr set the API key via environment variables:\n- `OPENROUTER_API_KEY`\n- `OPENAI_API_KEY`\n- `ANTHROPIC_API_KEY`\n- `GROQ_API_KEY`"
							} else {
								// Build messages
								var messages []llm.Message

								// Get hand system prompt
								if hand, _ := hands.GetBundledHand(agentID); hand != nil {
									systemPrompt := getHandSystemPrompt(agentID)
									if systemPrompt != "" {
										messages = append(messages, llm.Message{
											Role:    "system",
											Content: systemPrompt,
										})
									}
								}

								// Add user message
								messages = append(messages, llm.Message{
									Role:    "user",
									Content: text,
								})

								// Call LLM
								llmReq := &llm.Request{
									Messages:    messages,
									Temperature: 0.7,
								}

								ctx := context.Background()
								resp, err := driver.Chat(ctx, llmReq)
								if err != nil {
									fmt.Fprintf(os.Stderr, "[WebSocket] LLM error: %v\n", err)
									response = "Error: " + err.Error()
								} else {
									response = resp.Content
								}
							}
						}

						// Send typing stop
						typingStop, _ := json.Marshal(WSMessage{Type: "typing", Data: json.RawMessage(`{"state":"stop"}`)})
						client.Send <- typingStop

						// Send final response
						respMsg := WSMessage{Type: "response", Data: json.RawMessage(fmt.Sprintf(`{"content":"%s","input_tokens":%d,"output_tokens":%d,"iterations":1}`, strings.ReplaceAll(response, `"`, `\"`), inputTokens, outputTokens))}
						respBytes, _ := json.Marshal(respMsg)
						client.Send <- respBytes
					}()

				case "ping":
					pong, _ := json.Marshal(WSMessage{Type: "pong"})
					client.Send <- pong

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

	// WebSocket endpoint
	mux.HandleFunc("/api/ws", loggingMiddleware(corsMiddleware(WSHandler(k))))
}

// StreamEvent represents a streaming event.
type StreamEvent struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}
