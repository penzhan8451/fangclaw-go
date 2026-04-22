package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	MemoryFileName = "MEMORY.md"
	UserFileName   = "USER.md"
	SectionSep     = "\n§\n"
	MemCharLimit   = 100000
	UserCharLimit  = 50000
)

// FileStore handles file-based memory storage (MEMORY.md and USER.md)
type FileStore struct {
	baseDir      string
	memPath      string
	userPath     string
	mu           sync.RWMutex
	memSnapshot  string
	userSnapshot string
}

// NewFileStore creates a new file store
func NewFileStore(baseDir string) (*FileStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	fs := &FileStore{
		baseDir:  baseDir,
		memPath:  filepath.Join(baseDir, MemoryFileName),
		userPath: filepath.Join(baseDir, UserFileName),
	}

	if err := fs.TakeSnapshot(); err != nil {
		return nil, err
	}

	return fs, nil
}

// TakeSnapshot captures current state of memory files
func (fs *FileStore) TakeSnapshot() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	memContent, _ := os.ReadFile(fs.memPath)
	userContent, _ := os.ReadFile(fs.userPath)

	fs.memSnapshot = string(memContent)
	fs.userSnapshot = string(userContent)

	return nil
}

// GetSnapshot returns current snapshot content
func (fs *FileStore) GetSnapshot(target string) string {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	switch target {
	case "memory":
		return fs.memSnapshot
	case "user":
		return fs.userSnapshot
	default:
		return ""
	}
}

// Read reads current content
func (fs *FileStore) Read(target string) (string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var path string
	switch target {
	case "memory":
		path = fs.memPath
	case "user":
		path = fs.userPath
	default:
		return "", fmt.Errorf("invalid target: %s (must be 'memory' or 'user')", target)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// Write appends new content
func (fs *FileStore) Write(target, content string) error {
	if content == "" {
		return fmt.Errorf("content cannot be empty")
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	var path string
	var limit int
	switch target {
	case "memory":
		path = fs.memPath
		limit = MemCharLimit
	case "user":
		path = fs.userPath
		limit = UserCharLimit
	default:
		return fmt.Errorf("invalid target: %s (must be 'memory' or 'user')", target)
	}

	if err := fs.validate(content); err != nil {
		return err
	}

	existing := ""
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		if contentBytes, err := os.ReadFile(path); err == nil {
			existing = string(contentBytes)
		}
	}

	newContent := existing
	if existing != "" {
		newContent = existing + SectionSep + content
	} else {
		newContent = content
	}

	if len(newContent) > limit {
		return fmt.Errorf("content would exceed limit of %d chars", limit)
	}

	if err := fs.writeAtomic(path, newContent); err != nil {
		return err
	}

	if target == "memory" {
		fs.memSnapshot = newContent
	} else {
		fs.userSnapshot = newContent
	}

	return nil
}

// Replace replaces old content with new
func (fs *FileStore) Replace(target, oldStr, newStr string) error {
	if oldStr == "" {
		return fmt.Errorf("old text required for replace")
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	var path string
	var limit int
	switch target {
	case "memory":
		path = fs.memPath
		limit = MemCharLimit
	case "user":
		path = fs.userPath
		limit = UserCharLimit
	default:
		return fmt.Errorf("invalid target: %s (must be 'memory' or 'user')", target)
	}

	if err := fs.validate(newStr); err != nil {
		return err
	}

	existingBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	existing := string(existingBytes)

	if !strings.Contains(existing, oldStr) {
		return fmt.Errorf("old text not found")
	}

	updated := strings.ReplaceAll(existing, oldStr, newStr)

	if len(updated) > limit {
		return fmt.Errorf("content would exceed limit of %d chars", limit)
	}

	if err := fs.writeAtomic(path, updated); err != nil {
		return err
	}

	if target == "memory" {
		fs.memSnapshot = updated
	} else {
		fs.userSnapshot = updated
	}

	return nil
}

// Remove removes specific text
func (fs *FileStore) Remove(target, text string) error {
	return fs.Replace(target, text, "")
}

// Clear clears all content
func (fs *FileStore) Clear(target string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	var path string
	switch target {
	case "memory":
		path = fs.memPath
	case "user":
		path = fs.userPath
	default:
		return fmt.Errorf("invalid target: %s (must be 'memory' or 'user')", target)
	}

	if err := fs.writeAtomic(path, ""); err != nil {
		return err
	}

	if target == "memory" {
		fs.memSnapshot = ""
	} else {
		fs.userSnapshot = ""
	}

	return nil
}

func (fs *FileStore) writeAtomic(path, content string) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename file: %w", err)
	}
	return nil
}

func (fs *FileStore) validate(content string) error {
	dangerous := []string{
		"curl http", "curl https", "wget http", "wget https",
		"nc -e", "bash -i", "/dev/tcp",
		"ssh -i", "chmod +s",
	}

	lower := strings.ToLower(content)
	for _, pat := range dangerous {
		if strings.Contains(lower, pat) {
			return fmt.Errorf("content contains potentially dangerous patterns")
		}
	}

	return nil
}
