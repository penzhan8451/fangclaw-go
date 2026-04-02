package userdir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	DefaultBaseDir = ".fangclaw-go"
	UsersSubDir    = "users"
	ConfigFileName = "config.toml"
	DBFileName     = "fangclaw.db"
	SkillsSubDir   = "skills"
	SessionsSubDir = "sessions"
	AgentsSubDir   = "agents"
	SecretsEnvName = "secrets.env"
)

type Manager struct {
	baseDir    string
	usersDir   string
	globalFile string
	mu         sync.RWMutex
}

var (
	defaultManager *Manager
	once           sync.Once
)

func GetDefaultManager() (*Manager, error) {
	var err error
	once.Do(func() {
		homeDir, e := os.UserHomeDir()
		if e != nil {
			err = e
			return
		}
		baseDir := filepath.Join(homeDir, DefaultBaseDir)
		defaultManager, err = NewManager(baseDir)
	})
	if err != nil {
		return nil, err
	}
	return defaultManager, nil
}

func NewManager(baseDir string) (*Manager, error) {
	m := &Manager{
		baseDir:    baseDir,
		usersDir:   filepath.Join(baseDir, UsersSubDir),
		globalFile: filepath.Join(baseDir, "global_config.toml"),
	}

	if err := m.ensureBaseDir(); err != nil {
		return nil, fmt.Errorf("failed to ensure base directory: %w", err)
	}

	return m, nil
}

func (m *Manager) ensureBaseDir() error {
	dirs := []string{
		m.baseDir,
		m.usersDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func (m *Manager) BaseDir() string {
	return m.baseDir
}

func (m *Manager) UsersDir() string {
	return m.usersDir
}

func (m *Manager) GlobalConfigPath() string {
	return m.globalFile
}

func (m *Manager) GlobalSecretsPath() string {
	return filepath.Join(m.baseDir, SecretsEnvName)
}

func (m *Manager) AuthDBPath() string {
	return filepath.Join(m.baseDir, "auth.db")
}

func (m *Manager) ModelCatalogPath() string {
	return filepath.Join(m.baseDir, "model_catalog.json")
}

func (m *Manager) AgentTemplatesDir() string {
	return filepath.Join(m.baseDir, "agent_templates")
}

func (m *Manager) IsOwner(username string) bool {
	return username == "owner"
}

func (m *Manager) UserDir(username string) string {
	if m.IsOwner(username) {
		return m.baseDir
	}
	return filepath.Join(m.usersDir, sanitizeUsername(username))
}

func (m *Manager) UserConfigPath(username string) string {
	if m.IsOwner(username) {
		return filepath.Join(m.baseDir, ConfigFileName)
	}
	return filepath.Join(m.UserDir(username), ConfigFileName)
}

func (m *Manager) UserDBPath(username string) string {
	if m.IsOwner(username) {
		return filepath.Join(m.baseDir, DBFileName)
	}
	return filepath.Join(m.UserDir(username), DBFileName)
}

func (m *Manager) UserSkillsDir(username string) string {
	if m.IsOwner(username) {
		return filepath.Join(m.baseDir, SkillsSubDir)
	}
	return filepath.Join(m.UserDir(username), SkillsSubDir)
}

func (m *Manager) UserSessionsDir(username string) string {
	if m.IsOwner(username) {
		return filepath.Join(m.baseDir, SessionsSubDir)
	}
	return filepath.Join(m.UserDir(username), SessionsSubDir)
}

func (m *Manager) UserAgentsDir(username string) string {
	if m.IsOwner(username) {
		return filepath.Join(m.baseDir, AgentsSubDir)
	}
	return filepath.Join(m.UserDir(username), AgentsSubDir)
}

func (m *Manager) UserSecretsPath(username string) string {
	if m.IsOwner(username) {
		return m.GlobalSecretsPath()
	}
	return filepath.Join(m.UserDir(username), SecretsEnvName)
}

func (m *Manager) UserPaths(username string) *UserPaths {
	if m.IsOwner(username) {
		return &UserPaths{
			BaseDir:     m.baseDir,
			ConfigPath:  filepath.Join(m.baseDir, ConfigFileName),
			DBPath:      filepath.Join(m.baseDir, DBFileName),
			SkillsDir:   filepath.Join(m.baseDir, SkillsSubDir),
			SessionsDir: filepath.Join(m.baseDir, SessionsSubDir),
			AgentsDir:   filepath.Join(m.baseDir, AgentsSubDir),
			SecretsPath: m.GlobalSecretsPath(),
		}
	}
	return &UserPaths{
		BaseDir:     m.UserDir(username),
		ConfigPath:  m.UserConfigPath(username),
		DBPath:      m.UserDBPath(username),
		SkillsDir:   m.UserSkillsDir(username),
		SessionsDir: m.UserSessionsDir(username),
		AgentsDir:   m.UserAgentsDir(username),
		SecretsPath: m.UserSecretsPath(username),
	}
}

func (m *Manager) EnsureUserDir(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.IsOwner(username) {
		dirs := []string{
			m.baseDir,
			m.UserSkillsDir(username),
			m.UserSessionsDir(username),
			m.UserAgentsDir(username),
		}

		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create owner directory %s: %w", dir, err)
			}
		}
		return nil
	}

	userDir := m.UserDir(username)

	dirs := []string{
		userDir,
		m.UserSkillsDir(username),
		m.UserSessionsDir(username),
		m.UserAgentsDir(username),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create user directory %s: %w", dir, err)
		}
	}

	return nil
}

func (m *Manager) UserExists(username string) bool {
	userDir := m.UserDir(username)
	info, err := os.Stat(userDir)
	return err == nil && info.IsDir()
}

func (m *Manager) ListUsers() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(m.usersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read users directory: %w", err)
	}

	var users []string
	for _, entry := range entries {
		if entry.IsDir() {
			users = append(users, entry.Name())
		}
	}

	return users, nil
}

func (m *Manager) DeleteUserDir(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	userDir := m.UserDir(username)
	if _, err := os.Stat(userDir); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(userDir)
}

func (m *Manager) MoveUserDir(oldUsername, newUsername string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldDir := m.UserDir(oldUsername)
	newDir := m.UserDir(newUsername)

	if _, err := os.Stat(oldDir); os.IsNotExist(err) {
		return fmt.Errorf("source user directory does not exist: %s", oldDir)
	}

	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("destination user directory already exists: %s", newDir)
	}

	return os.Rename(oldDir, newDir)
}

type UserPaths struct {
	BaseDir     string
	ConfigPath  string
	DBPath      string
	SkillsDir   string
	SessionsDir string
	AgentsDir   string
	SecretsPath string
}

func (p *UserPaths) String() string {
	return fmt.Sprintf("UserPaths{BaseDir: %s, ConfigPath: %s, DBPath: %s}",
		p.BaseDir, p.ConfigPath, p.DBPath)
}

func sanitizeUsername(username string) string {
	username = strings.ToLower(username)
	username = strings.ReplaceAll(username, " ", "_")
	username = strings.ReplaceAll(username, "/", "_")
	username = strings.ReplaceAll(username, "\\", "_")
	username = strings.ReplaceAll(username, "..", "_")
	username = strings.TrimSpace(username)

	if len(username) > 64 {
		username = username[:64]
	}

	return username
}

func GetDefaultBaseDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, DefaultBaseDir), nil
}

func InitDefaultDirectories() error {
	mgr, err := GetDefaultManager()
	if err != nil {
		return err
	}

	dirs := []string{
		mgr.AgentTemplatesDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func LoadUserSecrets(username string) (map[string]string, error) {
	mgr, err := GetDefaultManager()
	if err != nil {
		return nil, err
	}

	var secretsPath string
	if username == "" { // owner, global secrets
		secretsPath = mgr.GlobalSecretsPath()
	} else {
		secretsPath = mgr.UserSecretsPath(username)
	}
	secrets := make(map[string]string)

	data, err := os.ReadFile(secretsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return secrets, nil
		}
		return nil, fmt.Errorf("failed to read secrets file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = strings.Trim(value, "\"")
			}
			secrets[key] = value
		}
	}

	return secrets, nil
}

func SaveUserSecrets(username string, secrets map[string]string) error {
	mgr, err := GetDefaultManager()
	if err != nil {
		return err
	}

	if username != "" {
		if err := mgr.EnsureUserDir(username); err != nil {
			return err
		}
	}

	var secretsPath string
	if username == "" {
		secretsPath = mgr.GlobalSecretsPath()
	} else {
		secretsPath = mgr.UserSecretsPath(username)
	}

	var lines []string
	for key, value := range secrets {
		if strings.ContainsAny(value, " \t\n\"") {
			value = fmt.Sprintf("\"%s\"", value)
		}
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	data := []byte(strings.Join(lines, "\n") + "\n")
	return os.WriteFile(secretsPath, data, 0600)
}

func SetUserSecret(username, key, value string) error {
	secrets, err := LoadUserSecrets(username)
	if err != nil {
		return err
	}

	secrets[key] = value
	return SaveUserSecrets(username, secrets)
}

func DeleteUserSecret(username, key string) error {
	secrets, err := LoadUserSecrets(username)
	if err != nil {
		return err
	}

	delete(secrets, key)
	return SaveUserSecrets(username, secrets)
}
