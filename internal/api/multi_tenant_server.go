package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/auth"
	"github.com/penzhan8451/fangclaw-go/internal/channels"
	"github.com/penzhan8451/fangclaw-go/internal/kernel"
	"github.com/penzhan8451/fangclaw-go/internal/types"
	"github.com/rs/zerolog/log"
)

type MultiTenantServer struct {
	*http.Server
	baseConfig    types.KernelConfig
	authManager   *auth.AuthManager
	kernelManager *kernel.UserKernelManager
	globalKernel  *kernel.Kernel
	router        *Router
	rateLimiter   *RateLimiter
	config        *ServerConfig
	bridgeManager *channels.BridgeManager
	mu            sync.RWMutex
}

func NewMultiTenantServer(baseConfig types.KernelConfig, authManager *auth.AuthManager, cfg *ServerConfig, globalKernel *kernel.Kernel) (*MultiTenantServer, error) {
	if cfg == nil {
		cfg = DefaultServerConfig()
	}

	rateLimiter := NewRateLimiter(DefaultRequestsPerMinute)

	mux := http.NewServeMux()

	if globalKernel == nil {
		globalConfig := baseConfig
		globalConfig.Auth.Enabled = false
		var err error
		globalKernel, err = kernel.NewKernel(globalConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create global kernel: %w", err)
		}

		if err := globalKernel.ReloadSecrets(); err != nil {
			log.Warn().Err(err).Msg("Failed to load global secrets")
		}
	}

	kernelManager, err := kernel.NewUserKernelManager(baseConfig, authManager, globalKernel)
	if err != nil {
		return nil, fmt.Errorf("failed to create kernel manager: %w", err)
	}

	server := &MultiTenantServer{
		Server: &http.Server{
			Addr:         cfg.ListenAddr,
			Handler:      mux,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		baseConfig:    baseConfig,
		authManager:   authManager,
		kernelManager: kernelManager,
		globalKernel:  globalKernel,
		rateLimiter:   rateLimiter,
		config:        cfg,
	}

	router := NewRouter(globalKernel)
	server.router = router

	authHandler := NewAuthHandler(authManager)
	router.SetAuthHandler(authHandler)

	wrappedMux := server.createHandler(mux)
	server.Handler = wrappedMux

	router.RegisterRoutes(mux)

	RegisterUserProviderRoutes(mux, router)

	RegisterUserConfigRoutes(mux, router)

	oaiHandler := NewOpenAICompatibleHandler(globalKernel)
	oaiHandler.RegisterRoutes(mux)

	RegisterStreamRoutes(mux, globalKernel)

	server.setupStaticFiles(mux)

	return server, nil
}

func (s *MultiTenantServer) SetBridgeManager(bm *channels.BridgeManager) {
	s.bridgeManager = bm
	if bm != nil {
		bm.SetKernelResolver(s.kernelManager)
		// Also set bridge manager on router
		s.router.SetBridgeManager(bm)
	}
}

func (s *MultiTenantServer) GetKernelManager() *kernel.UserKernelManager {
	return s.kernelManager
}

func (s *MultiTenantServer) createHandler(mux *http.ServeMux) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if !s.rateLimiter.Allow(key) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			WriteJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "Rate limit exceeded. Please try again later.",
			})
			return
		}

		s.setSecurityHeaders(w)

		log.Debug().Str("path", r.URL.Path).Str("method", r.Method).Msg("Processing request")

		if s.isAuthRoute(r.URL.Path) {
			log.Debug().Str("path", r.URL.Path).Msg("Auth route, passing through")
			mux.ServeHTTP(w, r)
			return
		}

		if s.isStaticRoute(r.URL.Path) {
			mux.ServeHTTP(w, r)
			return
		}

		isWebSocket := strings.ToLower(r.Header.Get("Upgrade")) == "websocket"

		isCLILocalRequest := s.isCLILocalRequest(r)
		if isCLILocalRequest {
			log.Debug().Str("path", r.URL.Path).Msg("CLI local request, passing through without auth")
			mux.ServeHTTP(w, r)
			return
		}

		token := extractTokenFromRequest(r)
		log.Debug().Str("token", token).Bool("has_token", token != "").Msg("Extracted token")

		if token != "" {
			user, err := s.authManager.ValidateToken(token)
			if err != nil {
				log.Error().Err(err).Msg("Failed to validate token")
			} else {
				log.Debug().Str("user_id", user.ID).Str("username", user.Username).Msg("Token validated successfully")
				// 首先设置基本的认证 context（这些是 /api/auth/me 等路由需要的）
				ctx := r.Context()
				ctx = context.WithValue(ctx, "user_id", user.ID)
				ctx = context.WithValue(ctx, "username", user.Username)
				ctx = context.WithValue(ctx, UserKey, user)

				// 尝试获取或创建 userKernel（对于需要 kernel 的路由）
				userKernel, err := s.kernelManager.GetOrCreateKernel(user.ID, user.Username, user.Role)
				if err != nil {
					log.Error().Err(err).Str("user", user.Username).Msg("Failed to get/create user kernel")
				} else {
					ctx = context.WithValue(ctx, "user_kernel", userKernel)
				}

				// 无论是否成功获取 userKernel，都继续处理请求
				mux.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		if s.isPublicRoute(r.URL.Path) {
			log.Debug().Str("path", r.URL.Path).Msg("Public route, passing through")
			mux.ServeHTTP(w, r)
			return
		}

		if isWebSocket {
			log.Debug().Str("path", r.URL.Path).Msg("WebSocket route without valid token, passing through")
			mux.ServeHTTP(w, r)
			return
		}

		log.Warn().Str("path", r.URL.Path).Msg("No token provided")
		WriteJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "authentication required",
		})
	}
}

func (s *MultiTenantServer) isCLILocalRequest(r *http.Request) bool {
	if r.Header.Get("X-Client-Type") != "cli" {
		return false
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func (s *MultiTenantServer) setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' cdn.jsdelivr.net fonts.googleapis.com; img-src 'self' data:; connect-src 'self' ws: wss:; font-src 'self' cdn.jsdelivr.net fonts.gstatic.com; object-src 'none'; base-uri 'self'; form-action 'self'")
}

func (s *MultiTenantServer) isAuthRoute(path string) bool {
	authRoutes := []string{
		"/api/auth/login",
		"/api/auth/logout",
		"/api/auth/register",
		"/api/auth/github",
		"/api/auth/github/callback",
	}
	for _, route := range authRoutes {
		if path == route {
			return true
		}
	}
	return false
}

func (s *MultiTenantServer) isPublicRoute(path string) bool {
	publicRoutes := []string{
		"/api/health",
		"/api/status",
		"/api/version",
		"/.well-known/",
		"/home",
		"/index",
		"/index.html",
		"/markdown",
	}
	for _, route := range publicRoutes {
		if path == route {
			return true
		}
		if strings.HasSuffix(route, "/") && strings.HasPrefix(path, route) {
			return true
		}
	}
	if path == "/" {
		return true
	}
	// Allow markdown files and code files to be accessed publicly
	if strings.HasSuffix(path, ".md") ||
		strings.HasSuffix(path, ".go") ||
		strings.HasSuffix(path, ".json") ||
		strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".ts") ||
		strings.HasSuffix(path, ".py") ||
		strings.HasSuffix(path, ".yaml") ||
		strings.HasSuffix(path, ".yml") ||
		strings.HasSuffix(path, ".toml") ||
		strings.HasSuffix(path, ".sh") ||
		strings.HasSuffix(path, ".bash") ||
		strings.HasSuffix(path, ".html") ||
		strings.HasSuffix(path, ".css") {
		return true
	}
	return false
}

func (s *MultiTenantServer) isStaticRoute(path string) bool {
	staticPrefixes := []string{
		"/css/",
		"/js/",
		"/img/",
		"/vendor/",
		"/fonts/",
		"/static/",
		"/locales/",
		"/docs/",
		"/examples/",
	}
	for _, prefix := range staticPrefixes {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}

	staticExtensions := []string{
		".css",
		".js",
		".png",
		".jpg",
		".jpeg",
		".gif",
		".svg",
		".ico",
		".woff",
		".woff2",
		".ttf",
		".eot",
		".json",
	}
	for _, ext := range staticExtensions {
		if len(path) >= len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}

	return false
}

func (s *MultiTenantServer) setupStaticFiles(mux *http.ServeMux) {
	staticDir := findStaticDir()
	fs := http.FileServer(http.Dir(staticDir))

	// Markdown viewer route
	mux.HandleFunc("/markdown", func(w http.ResponseWriter, r *http.Request) {
		viewerPath := staticDir + "/markdown-viewer.html"
		http.ServeFile(w, r, viewerPath)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.html":
			http.Redirect(w, r, "/index", http.StatusFound)
			return
		case "/", "/home":
			redirectPath := "/home"
			if r.URL.RawQuery != "" {
				redirectPath += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, redirectPath, http.StatusFound)
			return
		}
		fs.ServeHTTP(w, r)
	})

	mux.HandleFunc("/index", func(w http.ResponseWriter, r *http.Request) {
		indexPath := staticDir + "/index.html"
		http.ServeFile(w, r, indexPath)
	})

	mux.HandleFunc("/home", func(w http.ResponseWriter, r *http.Request) {
		homePath := staticDir + "/home.html"
		http.ServeFile(w, r, homePath)
	})
}

func (s *MultiTenantServer) Start() error {
	fmt.Printf("Starting multi-tenant API server on %s...\n", s.config.ListenAddr)
	fmt.Printf("BridgeManager is nil: %v\n", s.bridgeManager == nil)

	if s.bridgeManager != nil {
		fmt.Println("Loading all user channels...")
		s.loadAllUserChannels()
	} else {
		fmt.Println("WARNING: BridgeManager is nil, skipping user channel loading")
	}

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	return nil
}

func (s *MultiTenantServer) loadAllUserChannels() {
	users, err := s.authManager.ListUsers()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list users for channel loading")
		return
	}

	for _, user := range users {
		if user.Role == "owner" {
			continue
		}

		userKernel, err := s.kernelManager.GetOrCreateKernel(user.ID, user.Username, auth.Role(user.Role))
		if err != nil {
			log.Warn().Err(err).Str("user", user.Username).Msg("Failed to create kernel for user")
			continue
		}

		adapters := userKernel.Registry().ListAdapters()
		for id, adapter := range adapters {
			if err := s.bridgeManager.RegisterAdapter(id, adapter); err != nil {
				log.Warn().Err(err).Str("adapter", id).Str("user", user.Username).Msg("Failed to register adapter to bridge manager")
			} else {
				fmt.Printf("Registered adapter %s for user %s to bridge manager\n", id, user.Username)
			}
		}

		fmt.Printf("Loaded channels for user: %s\n", user.Username)
	}
}

func (s *MultiTenantServer) Stop(ctx context.Context) error {
	if err := s.kernelManager.StopAll(); err != nil {
		log.Warn().Err(err).Msg("Error stopping user kernels")
	}

	if s.globalKernel != nil {
		if err := s.globalKernel.Stop(ctx); err != nil {
			log.Warn().Err(err).Msg("Error stopping global kernel")
		}
	}

	return s.Shutdown(ctx)
}

func (s *MultiTenantServer) WaitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Stop(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error during shutdown: %v\n", err)
	}

	fmt.Println("Server stopped")
}

func RunMultiTenantServer(baseKernel *kernel.Kernel, authManager *auth.AuthManager, cfg *ServerConfig, defaultAgentID string, bridgeManager *channels.BridgeManager) error {
	baseConfig := baseKernel.Config()

	server, err := NewMultiTenantServer(baseConfig, authManager, cfg, baseKernel)
	if err != nil {
		return fmt.Errorf("failed to create multi-tenant server: %w", err)
	}

	server.SetBridgeManager(bridgeManager)

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

	if err := server.Start(); err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

func GetUserKernelFromContext(ctx context.Context) *kernel.Kernel {
	if k, ok := ctx.Value("user_kernel").(*kernel.Kernel); ok {
		return k
	}
	return nil
}
