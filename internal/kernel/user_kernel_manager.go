package kernel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/auth"
	"github.com/penzhan8451/fangclaw-go/internal/channels"
	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/penzhan8451/fangclaw-go/internal/types"
	"github.com/penzhan8451/fangclaw-go/internal/userdir"
	"github.com/rs/zerolog/log"
)

type UserKernelManager struct {
	authManager         *auth.AuthManager
	userDirMgr          *userdir.Manager
	baseConfig          types.KernelConfig
	globalKernel        *Kernel
	kernels             map[string]*Kernel
	mu                  sync.RWMutex
	cleanupTicker       *time.Ticker
	cleanupStopChan     chan struct{}
	inactivityThreshold time.Duration
}

func NewUserKernelManager(baseConfig types.KernelConfig, authManager *auth.AuthManager, globalKernel *Kernel) (*UserKernelManager, error) {
	userDirMgr, err := userdir.GetDefaultManager()
	if err != nil {
		return nil, fmt.Errorf("failed to get userdir manager: %w", err)
	}

	mgr := &UserKernelManager{
		authManager:         authManager,
		userDirMgr:          userDirMgr,
		baseConfig:          baseConfig,
		globalKernel:        globalKernel,
		kernels:             make(map[string]*Kernel),
		inactivityThreshold: 30 * time.Minute,
		cleanupStopChan:     make(chan struct{}),
	}

	mgr.startCleanupTicker()

	return mgr, nil
}

func (m *UserKernelManager) loadUserEnvAndConfig(userID, username string) (types.KernelConfig, map[string]string, error) {
	userConfig, err := config.LoadUserConfig(username)
	if err != nil {
		log.Warn().Err(err).Str("user", username).Msg("Failed to load user config, using defaults")
		userConfig = config.DefaultConfig()
	}

	userSecrets, err := userdir.LoadUserSecrets(username)
	if err != nil {
		log.Warn().Err(err).Str("user", username).Msg("Failed to load user secrets")
		userSecrets = make(map[string]string)
	}

	userKernelConfig := m.baseConfig
	userKernelConfig.UserID = userID
	userKernelConfig.Username = username
	userKernelConfig.Auth.Enabled = true

	userDataDir := m.userDirMgr.UserDir(username)
	userKernelConfig.DataDir = userDataDir
	log.Info().Str("user", username).Str("userDataDir", userDataDir).Msg("Setting user kernel DataDir")

	if userConfig.DefaultModel.Provider != "" {
		userKernelConfig.Models.DefaultProvider = userConfig.DefaultModel.Provider
	}
	if userConfig.DefaultModel.Model != "" {
		userKernelConfig.Models.DefaultModel = userConfig.DefaultModel.Model
	}

	if len(userConfig.McpServers) > 0 {
		userKernelConfig.McpServers = userConfig.McpServers
	} else {
		userKernelConfig.McpServers = []types.McpServerConfig{}
	}

	userKernelConfig.Browser = types.BrowserConfig{
		Enabled:        userConfig.Browser.Enabled,
		ChromiumPath:   userConfig.Browser.ChromiumPath,
		Headless:       userConfig.Browser.Headless,
		ViewportWidth:  userConfig.Browser.ViewportWidth,
		ViewportHeight: userConfig.Browser.ViewportHeight,
		MaxSessions:    userConfig.Browser.MaxSessions,
	}

	userKernelConfig.A2a = userConfig.A2a

	return userKernelConfig, userSecrets, nil
}

func (m *UserKernelManager) GetOrCreateKernel(userID, username string, role auth.Role) (*Kernel, error) {
	if role == auth.RoleOwner {

		ownerSecrets, err := userdir.LoadUserSecrets(username)
		if err != nil {
			log.Warn().Err(err).Str("user", username).Msg("Failed to load owner secrets")
		} else if len(ownerSecrets) > 0 {
			existingSecrets := m.globalKernel.GetSecrets()
			mergedSecrets := make(map[string]string)
			for k, v := range existingSecrets {
				mergedSecrets[k] = v
			}
			for k, v := range ownerSecrets {
				mergedSecrets[k] = v
			}
			m.globalKernel.SetSecrets(mergedSecrets)
		}
		return m.globalKernel, nil
	}

	m.mu.RLock()
	if kernel, ok := m.kernels[userID]; ok {
		m.mu.RUnlock()
		if err := m.authManager.UpdateLastActivity(userID); err != nil {
			log.Warn().Err(err).Str("userID", userID).Msg("Failed to update last activity")
		}
		return kernel, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if kernel, ok := m.kernels[userID]; ok {
		if err := m.authManager.UpdateLastActivity(userID); err != nil {
			log.Warn().Err(err).Str("userID", userID).Msg("Failed to update last activity")
		}
		return kernel, nil
	}

	if err := m.authManager.UpdateLastActivity(userID); err != nil {
		log.Warn().Err(err).Str("userID", userID).Msg("Failed to update last activity")
	}

	userConfig, userSecrets, err := m.loadUserEnvAndConfig(userID, username)
	if err != nil {
		return nil, fmt.Errorf("failed to load user config: %w", err)
	}

	kernel, err := NewKernelWithShared(userConfig, m.globalKernel.modelCatalog, m.globalKernel.agentTemplates)
	if err != nil {
		return nil, fmt.Errorf("failed to create kernel for user %s: %w", username, err)
	}

	kernel.SetSecrets(userSecrets)
	// install embedded skills for user kernel
	if err := kernel.SkillLoader().InstallAllEmbeddedSkills(); err != nil {
		log.Warn().Err(err).Str("user", username).Msg("Failed to install embedded skills for user")
	}

	// set shared channel registry
	kernel.SetSharedRegistry(m.globalKernel.Registry())

	getSecret := func(key string) string {
		return kernel.GetSecret(key)
	}
	userCfg, err := config.LoadUserConfig(username)
	if err != nil {
		log.Warn().Err(err).Str("user", username).Msg("Failed to load user config for channels")
	} else {
		if userCfg.Channels.QQ != nil {
			log.Info().Str("user", username).Str("app_id", userCfg.Channels.QQ.AppID).Bool("has_secret_env", userCfg.Channels.QQ.AppSecretEnv != "").Msg("User has QQ channel config")
		} else {
			log.Info().Str("user", username).Msg("User has no QQ channel config")
		}
		started, err := channels.LoadConfiguredChannelsWithOwner(kernel.Registry(), userCfg, getSecret, username)
		if err != nil {
			log.Warn().Err(err).Str("user", username).Msg("Failed to load user channels")
		} else if len(started) > 0 {
			log.Info().Str("user", username).Strs("channels", started).Msg("Started user channels")
		} else {
			log.Info().Str("user", username).Msg("No channels started for user")
		}
	}

	m.kernels[userID] = kernel
	log.Info().Str("user", username).Str("userID", userID).Msg("Created new kernel for user")

	log.Info().Str("user", username).Msg("Starting user kernel...")
	if err := kernel.Start(context.Background()); err != nil {
		log.Warn().Err(err).Str("user", username).Msg("Failed to start user kernel, but continuing")
	} else {
		log.Info().Str("user", username).Msg("User kernel started successfully")
	}

	return kernel, nil
}

func (m *UserKernelManager) GetKernel(userID string) (*Kernel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	kernel, exists := m.kernels[userID]
	return kernel, exists
}

func (m *UserKernelManager) RemoveKernel(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	kernel, exists := m.kernels[userID]
	if !exists {
		return nil
	}

	if err := kernel.Stop(nil); err != nil {
		log.Warn().Err(err).Str("userID", userID).Msg("Error stopping kernel during removal")
	}

	delete(m.kernels, userID)
	log.Info().Str("userID", userID).Msg("Removed kernel for user")

	return nil
}

func (m *UserKernelManager) ListKernels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userIDs := make([]string, 0, len(m.kernels))
	for userID := range m.kernels {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

func (m *UserKernelManager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for userID, kernel := range m.kernels {
		if err := kernel.Stop(nil); err != nil {
			log.Warn().Err(err).Str("userID", userID).Msg("Error stopping kernel")
			lastErr = err
		}
	}

	m.kernels = make(map[string]*Kernel)
	return lastErr
}

func (m *UserKernelManager) StartAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastErr error
	for userID, kernel := range m.kernels {
		if err := kernel.Start(nil); err != nil {
			log.Warn().Err(err).Str("userID", userID).Msg("Error starting kernel")
			lastErr = err
		}
	}

	return lastErr
}

func (m *UserKernelManager) GetKernelByOwner(owner string) (channels.ChannelBridgeHandle, bool) {
	if owner == "owner" || owner == "" {
		return m.globalKernel, true
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, kernel := range m.kernels {
		if kernel.config.Username == owner {
			return kernel, true
		}
	}

	return nil, false
}

func (m *UserKernelManager) startCleanupTicker() {
	m.cleanupTicker = time.NewTicker(5 * time.Minute)

	go func() {
		for {
			select {
			case <-m.cleanupTicker.C:
				m.cleanupInactiveKernels()
			case <-m.cleanupStopChan:
				m.cleanupTicker.Stop()
				return
			}
		}
	}()
}

func (m *UserKernelManager) cleanupInactiveKernels() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	inactiveUserIDs := []string{}

	for userID := range m.kernels {
		user, err := m.authManager.GetUserByID(userID)
		if err != nil {
			log.Warn().Err(err).Str("userID", userID).Msg("Failed to get user for cleanup check")
			continue
		}

		lastActivity := user.CreatedAt
		if user.LastActivityAt != nil {
			lastActivity = *user.LastActivityAt
		} else if user.LastLogin != nil {
			lastActivity = *user.LastLogin
		}

		if now.Sub(lastActivity) > m.inactivityThreshold {
			inactiveUserIDs = append(inactiveUserIDs, userID)
		}
	}

	for _, userID := range inactiveUserIDs {
		kernel := m.kernels[userID]
		if err := kernel.Stop(nil); err != nil {
			log.Warn().Err(err).Str("userID", userID).Msg("Error stopping kernel during cleanup")
		}
		delete(m.kernels, userID)
		log.Info().Str("userID", userID).Msg("Removed inactive user's kernel")
	}
}

func (m *UserKernelManager) Close() error {
	close(m.cleanupStopChan)
	return m.StopAll()
}
