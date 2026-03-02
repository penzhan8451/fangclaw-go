// Package vault provides secure credential storage for OpenFang.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrVaultLocked    = errors.New("vault locked")
	ErrInvalidKey     = errors.New("invalid encryption key")
	ErrCredentialNotFound = errors.New("credential not found")
)

// Credential represents a stored credential.
type Credential struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"` // "api_key", "oauth", "password", "certificate"
	Data        map[string]string      `json:"data"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// VaultConfig represents the configuration for the vault.
type VaultConfig struct {
	StoragePath string
	Key         []byte
}

// Vault is a secure credential storage system.
type Vault struct {
	mu         sync.RWMutex
	config     VaultConfig
	locked     bool
	credentials map[string]*Credential
}

// NewVault creates a new vault.
func NewVault(config VaultConfig) (*Vault, error) {
	if config.Key == nil || len(config.Key) != 32 {
		return nil, ErrInvalidKey
	}

	if config.StoragePath == "" {
		config.StoragePath = filepath.Join(os.Getenv("HOME"), ".fangclaw", "vault")
	}

	v := &Vault{
		config:     config,
		locked:     true,
		credentials: make(map[string]*Credential),
	}

	return v, nil
}

// Unlock unlocks the vault with the encryption key.
func (v *Vault) Unlock(key []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if len(key) != 32 {
		return ErrInvalidKey
	}

	v.config.Key = key
	v.locked = false

	if err := v.load(); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

// Lock locks the vault.
func (v *Vault) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.locked = true
	v.credentials = make(map[string]*Credential)
}

// IsLocked checks if the vault is locked.
func (v *Vault) IsLocked() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.locked
}

// Set stores a credential in the vault.
func (v *Vault) Set(cred *Credential) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.locked {
		return ErrVaultLocked
	}

	cred.UpdatedAt = time.Now()
	if cred.CreatedAt.IsZero() {
		cred.CreatedAt = cred.UpdatedAt
	}

	v.credentials[cred.ID] = cred
	return v.save()
}

// Get retrieves a credential from the vault.
func (v *Vault) Get(id string) (*Credential, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.locked {
		return nil, ErrVaultLocked
	}

	cred, ok := v.credentials[id]
	if !ok {
		return nil, ErrCredentialNotFound
	}

	return cred, nil
}

// GetByName retrieves a credential by name.
func (v *Vault) GetByName(name string) (*Credential, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.locked {
		return nil, ErrVaultLocked
	}

	for _, cred := range v.credentials {
		if cred.Name == name {
			return cred, nil
		}
	}

	return nil, ErrCredentialNotFound
}

// List lists all credentials in the vault.
func (v *Vault) List() ([]*Credential, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.locked {
		return nil, ErrVaultLocked
	}

	creds := make([]*Credential, 0, len(v.credentials))
	for _, cred := range v.credentials {
		creds = append(creds, cred)
	}

	return creds, nil
}

// Delete deletes a credential from the vault.
func (v *Vault) Delete(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.locked {
		return ErrVaultLocked
	}

	if _, ok := v.credentials[id]; !ok {
		return ErrCredentialNotFound
	}

	delete(v.credentials, id)
	return v.save()
}

// encrypt encrypts data using AES-256-GCM.
func (v *Vault) encrypt(data []byte) (string, error) {
	block, err := aes.NewCipher(v.config.Key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts data using AES-256-GCM.
func (v *Vault) decrypt(encrypted string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(v.config.Key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// save saves the vault to disk.
func (v *Vault) save() error {
	if err := os.MkdirAll(filepath.Dir(v.config.StoragePath), 0700); err != nil {
		return err
	}

	data, err := json.Marshal(v.credentials)
	if err != nil {
		return err
	}

	encrypted, err := v.encrypt(data)
	if err != nil {
		return err
	}

	return os.WriteFile(v.config.StoragePath, []byte(encrypted), 0600)
}

// load loads the vault from disk.
func (v *Vault) load() error {
	encryptedData, err := os.ReadFile(v.config.StoragePath)
	if err != nil {
		return err
	}

	data, err := v.decrypt(string(encryptedData))
	if err != nil {
		return err
	}

	var creds map[string]*Credential
	if err := json.Unmarshal(data, &creds); err != nil {
		return err
	}

	v.credentials = creds
	return nil
}

// GenerateKey generates a random 32-byte AES key.
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}
