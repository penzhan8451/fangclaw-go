package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/auth"
)

type AuthHandler struct {
	authManager *auth.AuthManager
}

func NewAuthHandler(authManager *auth.AuthManager) *AuthHandler {
	return &AuthHandler{
		authManager: authManager,
	}
}

type AuthLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthLoginResponse struct {
	Token     string     `json:"token"`
	ExpiresAt time.Time  `json:"expires_at"`
	User      *auth.User `json:"user"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID              string                 `json:"id"`
	Username        string                 `json:"username"`
	Email           string                 `json:"email,omitempty"`
	Role            string                 `json:"role"`
	ChannelBindings map[string]string      `json:"channel_bindings,omitempty"`
	Settings        map[string]interface{} `json:"settings,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	LastLogin       *time.Time             `json:"last_login,omitempty"`
	Disabled        bool                   `json:"disabled"`
	IsVIP           bool                   `json:"is_vip"`
}

type UpdateUserRequest struct {
	Email           string                 `json:"email,omitempty"`
	Password        string                 `json:"password,omitempty"`
	Role            string                 `json:"role,omitempty"`
	Disabled        *bool                  `json:"disabled,omitempty"`
	IsVIP           *bool                  `json:"is_vip,omitempty"`
	Settings        map[string]interface{} `json:"settings,omitempty"`
	ChannelBindings map[string]string      `json:"channel_bindings,omitempty"`
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req AuthLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "username and password required")
		return
	}

	session, err := h.authManager.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		respondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	user, err := h.authManager.GetUserByID(session.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	respondJSON(w, http.StatusOK, AuthLoginResponse{
		Token:     session.Token,
		ExpiresAt: session.ExpiresAt,
		User:      user,
	})
}

func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		respondError(w, http.StatusBadRequest, "no token provided")
		return
	}

	if err := h.authManager.Logout(token); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "username and password required")
		return
	}

	if len(req.Password) < 6 {
		respondError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	user, err := h.authManager.CreateUser(req.Username, req.Email, req.Password, auth.RoleUser)
	if err != nil {
		respondError(w, http.StatusConflict, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, userToResponse(user))
}

func (h *AuthHandler) HandleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	user, err := h.authManager.GetUserByID(userID.(string))
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	respondJSON(w, http.StatusOK, userToResponse(user))
}

func (h *AuthHandler) HandleUpdateCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := make(map[string]interface{})
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Password != "" {
		if len(req.Password) < 6 {
			respondError(w, http.StatusBadRequest, "password must be at least 6 characters")
			return
		}
		updates["password"] = req.Password
	}
	if req.Settings != nil {
		updates["settings"] = req.Settings
	}

	if len(updates) > 0 {
		if err := h.authManager.UpdateUser(userID.(string), updates); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	user, err := h.authManager.GetUserByID(userID.(string))
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	respondJSON(w, http.StatusOK, userToResponse(user))
}

func (h *AuthHandler) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	actor := GetUserFromContext(r.Context())
	if actor == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	if !IsAdmin(actor) {
		respondError(w, http.StatusForbidden, "admin access required")
		return
	}

	users, err := h.authManager.ListUsers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	responses := make([]UserResponse, len(users))
	for i, u := range users {
		responses[i] = *userToResponse(u)
	}

	respondJSON(w, http.StatusOK, responses)
}

func (h *AuthHandler) HandleGetUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user id required")
		return
	}

	user, err := h.authManager.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	respondJSON(w, http.StatusOK, userToResponse(user))
}

func (h *AuthHandler) HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
	actor := GetUserFromContext(r.Context())
	if actor == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	userID := r.PathValue("id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user id required")
		return
	}

	targetUser, err := h.authManager.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	if !CanManageUser(actor, targetUser) {
		respondError(w, http.StatusForbidden, "insufficient permissions to manage this user")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role != "" && !CanModifyRole(actor, auth.Role(req.Role)) {
		respondError(w, http.StatusForbidden, "insufficient permissions to assign this role")
		return
	}

	updates := make(map[string]interface{})
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Password != "" {
		if len(req.Password) < 6 {
			respondError(w, http.StatusBadRequest, "password must be at least 6 characters")
			return
		}
		updates["password"] = req.Password
	}
	if req.Role != "" {
		updates["role"] = req.Role
	}
	if req.Disabled != nil {
		updates["disabled"] = *req.Disabled
	}
	if req.IsVIP != nil {
		updates["is_vip"] = *req.IsVIP
	}
	if req.Settings != nil {
		updates["settings"] = req.Settings
	}
	if req.ChannelBindings != nil {
		updates["channel_bindings"] = req.ChannelBindings
	}

	if len(updates) > 0 {
		if err := h.authManager.UpdateUser(userID, updates); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	user, err := h.authManager.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	respondJSON(w, http.StatusOK, userToResponse(user))
}

func (h *AuthHandler) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	actor := GetUserFromContext(r.Context())
	if actor == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	userID := r.PathValue("id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user id required")
		return
	}

	targetUser, err := h.authManager.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	if !CanManageUser(actor, targetUser) {
		respondError(w, http.StatusForbidden, "insufficient permissions to delete this user")
		return
	}

	if actor.ID == userID {
		respondError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	if err := h.authManager.DeleteUser(userID); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "user deleted"})
}

func (h *AuthHandler) HandleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	apiKey := auth.GenerateSecureToken()

	if err := h.authManager.AddAPIKey(userID.(string), apiKey); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"api_key": apiKey})
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", h.HandleLogin)
	mux.HandleFunc("POST /api/auth/logout", h.HandleLogout)
	mux.HandleFunc("POST /api/auth/register", h.HandleRegister)
	mux.HandleFunc("GET /api/auth/me", h.HandleGetCurrentUser)
	mux.HandleFunc("PUT /api/auth/me", h.HandleUpdateCurrentUser)
	mux.HandleFunc("POST /api/auth/api-keys", h.HandleCreateAPIKey)
	mux.HandleFunc("GET /api/users", h.HandleListUsers)
	mux.HandleFunc("GET /api/users/{id}", h.HandleGetUser)
	mux.HandleFunc("PUT /api/users/{id}", h.HandleUpdateUser)
	mux.HandleFunc("DELETE /api/users/{id}", h.HandleDeleteUser)
}

func userToResponse(u *auth.User) *UserResponse {
	return &UserResponse{
		ID:              u.ID,
		Username:        u.Username,
		Email:           u.Email,
		Role:            string(u.Role),
		ChannelBindings: u.ChannelBindings,
		Settings:        u.Settings,
		CreatedAt:       u.CreatedAt,
		LastLogin:       u.LastLogin,
		Disabled:        u.Disabled,
		IsVIP:           u.IsVIP,
	}
}

func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	cookie, err := r.Cookie("session")
	if err == nil {
		return cookie.Value
	}

	token := r.URL.Query().Get("token")
	if token != "" {
		return token
	}

	return ""
}

func GenerateSecureToken() string {
	return auth.GenerateSecureToken()
}

type GitHubUserInfo struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type GitHubOAuthState struct {
	State        string `json:"state"`
	PKCEVerifier string `json:"pkce_verifier"`
	RedirectURL  string `json:"redirect_url"`
}

func (h *AuthHandler) HandleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	if h.authManager == nil {
		respondError(w, http.StatusNotImplemented, "authentication not enabled")
		return
	}

	cfg := h.authManager.GetGitHubOAuthConfig()
	if cfg == nil || !cfg.Enabled {
		respondError(w, http.StatusBadRequest, "GitHub OAuth not configured")
		return
	}

	if cfg.ClientID == "" {
		respondError(w, http.StatusBadRequest, "GitHub OAuth client_id not set")
		return
	}

	state := auth.GenerateSecureToken()
	pkceVerifier := auth.GeneratePKCEVerifier()
	pkceChallenge := auth.GeneratePKCEChallenge(pkceVerifier)

	redirectURL := cfg.RedirectURL
	if redirectURL == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
			scheme = forwardedProto
		}
		redirectURL = scheme + "://" + r.Host + "/api/auth/github/callback"
	}

	h.authManager.StoreGitHubOAuthState(state, &auth.GitHubOAuthStateData{
		State:        state,
		PKCEVerifier: pkceVerifier,
		RedirectURL:  redirectURL,
	})

	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=read:user&state=%s&code_challenge=%s&code_challenge_method=S256",
		cfg.ClientID,
		url.QueryEscape(redirectURL),
		state,
		pkceChallenge,
	)

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) HandleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	if h.authManager == nil {
		respondError(w, http.StatusInternalServerError, "authentication not enabled")
		return
	}

	cfg := h.authManager.GetGitHubOAuthConfig()
	if cfg == nil || !cfg.Enabled {
		respondError(w, http.StatusBadRequest, "GitHub OAuth not configured")
		return
	}

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		respondError(w, http.StatusBadRequest, "missing state or code")
		return
	}

	oauthState, ok := h.authManager.GetGitHubOAuthState(state)
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid or expired state")
		return
	}
	h.authManager.RemoveGitHubOAuthState(state)

	token, err := exchangeGitHubCode(cfg.ClientID, cfg.ClientSecret, code, oauthState.RedirectURL, oauthState.PKCEVerifier)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to exchange code: "+err.Error())
		return
	}

	githubUser, err := getGitHubUserInfo(token)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get GitHub user: "+err.Error())
		return
	}

	var user *auth.User
	existingUser, err := h.authManager.GetUserByUsername(githubUser.Login)
	if err == nil && existingUser != nil {
		user = existingUser
	} else {
		user, err = h.authManager.CreateUser(githubUser.Login, githubUser.Email, "", auth.RoleUser)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to create user: "+err.Error())
			return
		}
		if githubUser.Name != "" {
			h.authManager.UpdateUser(user.ID, map[string]interface{}{
				"settings": map[string]interface{}{
					"github_name": githubUser.Name,
					"github_id":   githubUser.ID,
				},
			})
		}
	}

	session, err := h.authManager.CreateSession(user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.Redirect(w, r, "/?github_login=success&token="+session.Token, http.StatusTemporaryRedirect)
}

func exchangeGitHubCode(clientID, clientSecret, code, redirectURI, pkceVerifier string) (string, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("code_verifier", pkceVerifier)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err == nil && result.AccessToken != "" {
		if result.Error != "" {
			return "", fmt.Errorf("github error: %s", result.Error)
		}
		return result.AccessToken, nil
	}

	bodyStr := string(bodyBytes)
	values, err := url.ParseQuery(bodyStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse response: %s", bodyStr)
	}

	if errorStr := values.Get("error"); errorStr != "" {
		return "", fmt.Errorf("github error: %s", errorStr)
	}

	accessToken := values.Get("access_token")
	if accessToken == "" {
		return "", fmt.Errorf("no access token in response: %s", bodyStr)
	}

	return accessToken, nil
}

func getGitHubUserInfo(accessToken string) (*GitHubUserInfo, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user GitHubUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}
