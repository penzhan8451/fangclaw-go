// Package oauth provides OAuth2 authentication flows for OpenFang.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OAuthProvider represents an OAuth2 provider.
type OAuthProvider string

const (
	OAuthProviderGoogle    OAuthProvider = "google"
	OAuthProviderGitHub    OAuthProvider = "github"
	OAuthProviderMicrosoft OAuthProvider = "microsoft"
	OAuthProviderSlack     OAuthProvider = "slack"
	OAuthProviderDiscord   OAuthProvider = "discord"
	OAuthProviderNotion    OAuthProvider = "notion"
	OAuthProviderDropbox   OAuthProvider = "dropbox"
	OAuthProviderSpotify   OAuthProvider = "spotify"
)

// OAuthConfig represents the configuration for an OAuth2 provider.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	AuthURL      string
	TokenURL     string
}

// OAuthToken represents an OAuth2 token response.
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

// PKCEVerifier represents a PKCE verifier.
type PKCEVerifier struct {
	Verifier  string
	Challenge string
	Method    string
}

// OAuthFlow represents an ongoing OAuth2 flow.
type OAuthFlow struct {
	ID          string
	Provider    OAuthProvider
	State       string
	PKCE        *PKCEVerifier
	RedirectURL string
	StartedAt   time.Time
	Completed   bool
	Token       *OAuthToken
	Error       error
	resultChan  chan *OAuthToken
	errorChan   chan error
}

// OAuthManager manages OAuth2 flows.
type OAuthManager struct {
	mu         sync.RWMutex
	configs    map[OAuthProvider]*OAuthConfig
	flows      map[string]*OAuthFlow
	httpServer *http.Server
	listenAddr string
}

// NewOAuthManager creates a new OAuth manager.
func NewOAuthManager(listenAddr string) *OAuthManager {
	if listenAddr == "" {
		listenAddr = "127.0.0.1:9876"
	}

	return &OAuthManager{
		configs:    make(map[OAuthProvider]*OAuthConfig),
		flows:      make(map[string]*OAuthFlow),
		listenAddr: listenAddr,
	}
}

// RegisterProvider registers an OAuth provider configuration.
func (m *OAuthManager) RegisterProvider(provider OAuthProvider, config *OAuthConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[provider] = config
}

// GetProviderConfig gets the configuration for an OAuth provider.
func (m *OAuthManager) GetProviderConfig(provider OAuthProvider) (*OAuthConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.configs[provider]
	return cfg, ok
}

// StartFlow starts a new OAuth2 flow.
func (m *OAuthManager) StartFlow(ctx context.Context, provider OAuthProvider) (*OAuthFlow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.configs[provider]
	if !ok {
		return nil, fmt.Errorf("provider not configured: %s", provider)
	}

	flowID := generateRandomID()
	state := generateRandomID()
	pkce, err := generatePKCE()
	if err != nil {
		return nil, err
	}

	redirectURL := fmt.Sprintf("http://%s/callback/%s", m.listenAddr, provider)

	flow := &OAuthFlow{
		ID:          flowID,
		Provider:    provider,
		State:       state,
		PKCE:        pkce,
		RedirectURL: redirectURL,
		StartedAt:   time.Now(),
		resultChan:  make(chan *OAuthToken, 1),
		errorChan:   make(chan error, 1),
	}

	m.flows[flowID] = flow

	return flow, nil
}

// GetAuthURL gets the authorization URL for a flow.
func (m *OAuthManager) GetAuthURL(flowID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	flow, ok := m.flows[flowID]
	if !ok {
		return "", fmt.Errorf("flow not found: %s", flowID)
	}

	cfg, ok := m.configs[flow.Provider]
	if !ok {
		return "", fmt.Errorf("provider not configured: %s", flow.Provider)
	}

	params := url.Values{}
	params.Set("client_id", cfg.ClientID)
	params.Set("redirect_uri", flow.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(cfg.Scopes, " "))
	params.Set("state", flow.State)

	if flow.PKCE != nil {
		params.Set("code_challenge", flow.PKCE.Challenge)
		params.Set("code_challenge_method", flow.PKCE.Method)
	}

	return fmt.Sprintf("%s?%s", cfg.AuthURL, params.Encode()), nil
}

// ExchangeCode exchanges an authorization code for a token.
func (m *OAuthManager) ExchangeCode(ctx context.Context, flowID, code string) (*OAuthToken, error) {
	m.mu.Lock()
	flow, ok := m.flows[flowID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("flow not found: %s", flowID)
	}
	m.mu.Unlock()

	cfg, ok := m.configs[flow.Provider]
	if !ok {
		return nil, fmt.Errorf("provider not configured: %s", flow.Provider)
	}

	data := url.Values{}
	data.Set("client_id", cfg.ClientID)
	data.Set("client_secret", cfg.ClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", flow.RedirectURL)

	if flow.PKCE != nil {
		data.Set("code_verifier", flow.PKCE.Verifier)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %d", resp.StatusCode)
	}

	var token OAuthToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	if token.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	m.mu.Lock()
	flow.Completed = true
	flow.Token = &token
	m.mu.Unlock()

	select {
	case flow.resultChan <- &token:
	default:
	}

	return &token, nil
}

// WaitForToken waits for a flow to complete and returns the token.
func (m *OAuthManager) WaitForToken(ctx context.Context, flowID string) (*OAuthToken, error) {
	m.mu.RLock()
	flow, ok := m.flows[flowID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("flow not found: %s", flowID)
	}
	m.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case token := <-flow.resultChan:
		return token, nil
	case err := <-flow.errorChan:
		return nil, err
	}
}

// StartServer starts the OAuth callback server.
func (m *OAuthManager) StartServer() error {
	mux := http.NewServeMux()

	for provider := range m.configs {
		path := fmt.Sprintf("/callback/%s", provider)
		mux.HandleFunc(path, m.callbackHandler(provider))
	}

	m.httpServer = &http.Server{
		Addr:    m.listenAddr,
		Handler: mux,
	}

	go func() {
		if err := m.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("OAuth server error: %v\n", err)
		}
	}()

	return nil
}

// StopServer stops the OAuth callback server.
func (m *OAuthManager) StopServer(ctx context.Context) error {
	if m.httpServer != nil {
		return m.httpServer.Shutdown(ctx)
	}
	return nil
}

// callbackHandler handles OAuth callbacks.
func (m *OAuthManager) callbackHandler(provider OAuthProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		errorMsg := r.URL.Query().Get("error")

		if errorMsg != "" {
			http.Error(w, fmt.Sprintf("OAuth error: %s", errorMsg), http.StatusBadRequest)
			return
		}

		if code == "" || state == "" {
			http.Error(w, "Missing code or state", http.StatusBadRequest)
			return
		}

		m.mu.RLock()
		var targetFlow *OAuthFlow
		for _, flow := range m.flows {
			if flow.State == state && flow.Provider == provider && !flow.Completed {
				targetFlow = flow
				break
			}
		}
		m.mu.RUnlock()

		if targetFlow == nil {
			http.Error(w, "Invalid state", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := m.ExchangeCode(ctx, targetFlow.ID, code)
		if err != nil {
			http.Error(w, fmt.Sprintf("Token exchange failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<!DOCTYPE html>
			<html>
			<body>
				<h1>Authentication Successful!</h1>
				<p>You can close this window now.</p>
			</body>
			</html>
		`)

		m.mu.Lock()
		delete(m.flows, targetFlow.ID)
		m.mu.Unlock()
	}
}

// generateRandomID generates a random ID.
func generateRandomID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// generatePKCE generates a PKCE verifier and challenge.
func generatePKCE() (*PKCEVerifier, error) {
	verifierBytes := make([]byte, 64)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, err
	}

	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	challenge := sha256String(verifier)
	challenge = base64.RawURLEncoding.EncodeToString([]byte(challenge))

	return &PKCEVerifier{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}, nil
}

// sha256String computes the SHA256 hash of a string.
func sha256String(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return string(h.Sum(nil))
}

// GetDefaultProviderConfig gets the default configuration for a provider.
func GetDefaultProviderConfig(provider OAuthProvider, redirectURL string) (*OAuthConfig, error) {
	switch provider {
	case OAuthProviderGitHub:
		return &OAuthConfig{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
			Scopes:   []string{"repo", "user"},
		}, nil
	case OAuthProviderGoogle:
		return &OAuthConfig{
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
			Scopes:   []string{"openid", "email", "profile"},
		}, nil
	case OAuthProviderSlack:
		return &OAuthConfig{
			AuthURL:  "https://slack.com/oauth/v2/authorize",
			TokenURL: "https://slack.com/api/oauth.v2.access",
			Scopes:   []string{"chat:write", "channels:read"},
		}, nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}
