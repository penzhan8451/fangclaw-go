// Package skills implements the skill system runtime for OpenFang.
package skills

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
	"gopkg.in/yaml.v3"
)

const (
	// DefaultTimeout is the default timeout for skill execution
	DefaultTimeout = 120 * time.Second
	// MaxRecursionDepth is the maximum depth for config includes
	MaxRecursionDepth = 10
)

// Loader handles loading and executing skills from various runtimes.
type Loader struct {
	skillsPath string
	mu         sync.RWMutex
	registry   map[string]*types.Skill
}

// NewLoader creates a new skill loader.
func NewLoader(skillsPath string) (*Loader, error) {
	if err := os.MkdirAll(skillsPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create skills directory: %w", err)
	}

	return &Loader{
		skillsPath: skillsPath,
		registry:   make(map[string]*types.Skill),
	}, nil
}

// LoadSkill loads a skill from a directory.
func (l *Loader) LoadSkill(skillID string) (*types.Skill, error) {
	// fmt.Println("[DEBUG] LoadSkill called for:", skillID)
	l.mu.RLock()
	if skill, exists := l.registry[skillID]; exists {
		l.mu.RUnlock()
		// fmt.Println("[DEBUG] LoadSkill found in registry:", skillID)
		return skill, nil
	}
	l.mu.RUnlock()

	skillDir := filepath.Join(l.skillsPath, skillID)
	// fmt.Println("[DEBUG] LoadSkill skillDir:", skillDir)
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		// fmt.Println("[DEBUG] LoadSkill skill not found:", skillID)
		return nil, fmt.Errorf("skill not found: %s", skillID)
	}

	// fmt.Println("[DEBUG] LoadSkill loading manifest...")
	manifest, err := l.loadManifest(skillDir)
	if err != nil {
		// fmt.Println("[DEBUG] LoadSkill failed to load manifest:", err)
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}
	// fmt.Println("[DEBUG] LoadSkill manifest loaded successfully")

	skill := &types.Skill{
		ID:          skillID,
		Manifest:    manifest,
		InstallPath: skillDir,
		InstalledAt: time.Now(),
		Enabled:     true,
	}

	l.mu.Lock()
	l.registry[skillID] = skill
	l.mu.Unlock()

	// fmt.Println("[DEBUG] LoadSkill complete for:", skillID)
	return skill, nil
}

// loadManifest loads and parses the skill manifest.
func (l *Loader) loadManifest(skillDir string) (types.SkillManifest, error) {
	// fmt.Println("[DEBUG] loadManifest called, skillDir:", skillDir)
	manifestPath := filepath.Join(skillDir, "skill.toml")
	// fmt.Println("[DEBUG] loadManifest checking skill.toml...")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		manifestPath = filepath.Join(skillDir, "manifest.json")
		// fmt.Println("[DEBUG] loadManifest checking manifest.json...")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			manifestPath = filepath.Join(skillDir, "SKILL.md")
			// fmt.Println("[DEBUG] loadManifest checking SKILL.md...")
		}
	}
	// fmt.Println("[DEBUG] loadManifest using path:", manifestPath)

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// fmt.Println("[DEBUG] loadManifest failed to read file:", err)
		return types.SkillManifest{}, fmt.Errorf("failed to read manifest: %w", err)
	}
	// fmt.Println("[DEBUG] loadManifest read", len(data), "bytes")

	var manifest types.SkillManifest
	if filepath.Ext(manifestPath) == ".md" {
		// fmt.Println("[DEBUG] loadManifest parsing as SKILL.md...")
		return parseSKILLMD(data)
	} else if filepath.Ext(manifestPath) == ".toml" {
		return types.SkillManifest{}, fmt.Errorf("TOML manifest not yet supported")
	}

	if err := json.Unmarshal(data, &manifest); err != nil {
		// fmt.Println("[DEBUG] loadManifest failed to parse JSON:", err)
		return types.SkillManifest{}, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// fmt.Println("[DEBUG] loadManifest complete")
	return manifest, nil
}

// parseSKILLMD parses a SKILL.md file with YAML frontmatter.
func parseSKILLMD(data []byte) (types.SkillManifest, error) {
	// fmt.Println("[DEBUG] parseSKILLMD called")
	content := string(data)
	// fmt.Println("[DEBUG] parseSKILLMD content length:", len(content))

	content = strings.ReplaceAll(content, "\r\n", "\n")

	var frontmatter string
	var body string

	if strings.HasPrefix(content, "---\n") {
		remaining := content[4:]
		idx := strings.Index(remaining, "\n---")
		if idx == -1 {
			idx = strings.Index(remaining, "---\n")
			if idx == -1 {
				idx = strings.Index(remaining, "---")
			}
		}

		if idx != -1 {
			frontmatter = remaining[:idx]
			bodyStart := idx + 1
			if strings.HasPrefix(remaining[idx:], "\n---\n") {
				bodyStart = idx + 5
			} else if strings.HasPrefix(remaining[idx:], "\n---") {
				bodyStart = idx + 4
			} else if strings.HasPrefix(remaining[idx:], "---\n") {
				bodyStart = idx + 4
			} else {
				bodyStart = idx + 3
			}
			body = remaining[bodyStart:]
		}
	}

	// fmt.Println("[DEBUG] parseSKILLMD frontmatter length:", len(frontmatter))
	// fmt.Println("[DEBUG] parseSKILLMD body length:", len(body))

	if frontmatter == "" {
		// fmt.Println("[DEBUG] parseSKILLMD invalid format, missing frontmatter")
		return types.SkillManifest{}, fmt.Errorf("invalid SKILL.md format: missing frontmatter")
	}

	// fmt.Println("[DEBUG] parseSKILLMD frontmatter:", frontmatter[:min(200, len(frontmatter))])

	type FrontMatter struct {
		Name        string   `yaml:"name"`
		Description string   `yaml:"description"`
		Version     string   `yaml:"version"`
		Author      string   `yaml:"author"`
		Tags        []string `yaml:"tags,omitempty"`
	}

	var fm FrontMatter
	// fmt.Println("[DEBUG] parseSKILLMD unmarshaling frontmatter...")
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		// fmt.Println("[DEBUG] parseSKILLMD failed to parse frontmatter:", err)
		return types.SkillManifest{}, fmt.Errorf("failed to parse frontmatter: %w", err)
	}
	// fmt.Println("[DEBUG] parseSKILLMD frontmatter parsed, name:", fm.Name)

	if fm.Version == "" {
		fm.Version = "1.0.0"
	}

	manifest := types.SkillManifest{
		Version:     fm.Version,
		Name:        fm.Name,
		Description: fm.Description,
		Author:      fm.Author,
		Runtime: types.SkillRuntime{
			RuntimeType: types.SkillRuntimePrompt,
		},
		Tools: types.SkillTools{
			Provided: []types.SkillToolDefinition{},
		},
		Metadata:      make(map[string]string),
		PromptContext: strings.TrimSpace(body),
	}

	if len(fm.Tags) > 0 {
		manifest.Metadata["tags"] = strings.Join(fm.Tags, ",")
	}

	// fmt.Println("[DEBUG] parseSKILLMD complete, prompt context length:", len(manifest.PromptContext))
	return manifest, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ExecuteTool executes a skill tool.
func (l *Loader) ExecuteTool(skillID, toolName string, input interface{}) (*types.SkillToolResult, error) {
	skill, err := l.LoadSkill(skillID)
	if err != nil {
		return nil, err
	}

	toolFound := false
	for _, tool := range skill.Manifest.Tools.Provided {
		if tool.Name == toolName {
			toolFound = true
			break
		}
	}

	if !toolFound {
		return nil, fmt.Errorf("tool %s not found in skill %s", toolName, skillID)
	}

	switch skill.Manifest.Runtime.RuntimeType {
	case types.SkillRuntimePython:
		return l.executePython(skill, toolName, input)
	case types.SkillRuntimeNode:
		return l.executeNode(skill, toolName, input)
	case types.SkillRuntimeWasm:
		return nil, fmt.Errorf("WASM runtime not yet implemented")
	case types.SkillRuntimeBuiltin:
		return nil, fmt.Errorf("builtin skills are handled by the kernel")
	case types.SkillRuntimePrompt:
		return &types.SkillToolResult{
			Output: map[string]interface{}{
				"note": "Prompt-context skill — instructions are in your system prompt. Use built-in tools directly.",
			},
			IsError: false,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", skill.Manifest.Runtime.RuntimeType)
	}
}

// executePython executes a Python skill script.
func (l *Loader) executePython(skill *types.Skill, toolName string, input interface{}) (*types.SkillToolResult, error) {
	scriptPath := filepath.Join(skill.InstallPath, skill.Manifest.Runtime.Entry)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Python script not found: %s", scriptPath)
	}

	payload := map[string]interface{}{
		"tool":  toolName,
		"input": input,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize payload: %w", err)
	}

	python := findPython()
	if python == "" {
		return nil, fmt.Errorf("Python not found. Install Python 3.8+ to run Python skills")
	}

	cmd := exec.Command(python, scriptPath)
	cmd.Dir = skill.InstallPath
	cmd.Env = []string{}

	// Preserve essential environment variables
	if path := os.Getenv("PATH"); path != "" {
		cmd.Env = append(cmd.Env, "PATH="+path)
	}
	if home := os.Getenv("HOME"); home != "" {
		cmd.Env = append(cmd.Env, "HOME="+home)
	}
	if runtime.GOOS == "windows" {
		if systemroot := os.Getenv("SYSTEMROOT"); systemroot != "" {
			cmd.Env = append(cmd.Env, "SYSTEMROOT="+systemroot)
		}
		if temp := os.Getenv("TEMP"); temp != "" {
			cmd.Env = append(cmd.Env, "TEMP="+temp)
		}
	}
	cmd.Env = append(cmd.Env, "PYTHONIOENCODING=utf-8")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Python: %w", err)
	}

	if _, err := stdin.Write(payloadBytes); err != nil {
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}
	stdin.Close()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return &types.SkillToolResult{
				Output:  map[string]interface{}{"error": stderr.String()},
				IsError: true,
			}, nil
		}
	case <-time.After(DefaultTimeout):
		cmd.Process.Kill()
		return nil, fmt.Errorf("skill execution timed out")
	}

	var result types.SkillToolResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return &types.SkillToolResult{
			Output:  map[string]interface{}{"result": stdout.String()},
			IsError: false,
		}, nil
	}

	return &result, nil
}

// executeNode executes a Node.js skill script.
func (l *Loader) executeNode(skill *types.Skill, toolName string, input interface{}) (*types.SkillToolResult, error) {
	scriptPath := filepath.Join(skill.InstallPath, skill.Manifest.Runtime.Entry)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Node.js script not found: %s", scriptPath)
	}

	payload := map[string]interface{}{
		"tool":  toolName,
		"input": input,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize payload: %w", err)
	}

	node := findNode()
	if node == "" {
		return nil, fmt.Errorf("Node.js not found. Install Node.js 18+ to run Node.js skills")
	}

	cmd := exec.Command(node, scriptPath)
	cmd.Dir = skill.InstallPath
	cmd.Env = []string{}

	if path := os.Getenv("PATH"); path != "" {
		cmd.Env = append(cmd.Env, "PATH="+path)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Node.js: %w", err)
	}

	if _, err := stdin.Write(payloadBytes); err != nil {
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}
	stdin.Close()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return &types.SkillToolResult{
				Output:  map[string]interface{}{"error": stderr.String()},
				IsError: true,
			}, nil
		}
	case <-time.After(DefaultTimeout):
		cmd.Process.Kill()
		return nil, fmt.Errorf("skill execution timed out")
	}

	var result types.SkillToolResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return &types.SkillToolResult{
			Output:  map[string]interface{}{"result": stdout.String()},
			IsError: false,
		}, nil
	}

	return &result, nil
}

// findPython finds the Python executable.
func findPython() string {
	candidates := []string{"python3", "python"}
	for _, cmd := range candidates {
		if path, err := exec.LookPath(cmd); err == nil {
			return path
		}
	}
	return ""
}

// findNode finds the Node.js executable.
func findNode() string {
	candidates := []string{"node", "nodejs"}
	for _, cmd := range candidates {
		if path, err := exec.LookPath(cmd); err == nil {
			return path
		}
	}
	return ""
}

// LoadAll loads all installed skills from the skills directory.
func (l *Loader) LoadAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	entries, err := os.ReadDir(l.skillsPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			skillID := entry.Name()
			skillDir := filepath.Join(l.skillsPath, skillID)
			if manifest, err := l.loadManifest(skillDir); err == nil {
				skill := &types.Skill{
					ID:          skillID,
					Manifest:    manifest,
					InstallPath: skillDir,
					InstalledAt: time.Now(),
					Enabled:     true,
				}
				l.registry[skillID] = skill
			}
		}
	}

	return nil
}

// FindToolProvider finds a skill that provides the given tool.
func (l *Loader) FindToolProvider(toolName string) (*types.Skill, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, skill := range l.registry {
		if !skill.Enabled {
			continue
		}
		for _, tool := range skill.Manifest.Tools.Provided {
			if tool.Name == toolName {
				return skill, true
			}
		}
	}

	return nil, false
}

// GetSkill gets a skill by ID.
func (l *Loader) GetSkill(skillID string) (*types.Skill, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	skill, exists := l.registry[skillID]
	return skill, exists
}

// ListSkills lists all installed skills.
func (l *Loader) ListSkills() ([]*types.Skill, error) {
	// fmt.Println("[DEBUG] ListSkills called, skillsPath:", l.skillsPath)

	entries, err := os.ReadDir(l.skillsPath)
	if err != nil {
		// fmt.Println("[DEBUG] ListSkills failed to read dir:", err)
		return nil, err
	}
	// fmt.Println("[DEBUG] ListSkills found", len(entries), "entries")

	var dirNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirNames = append(dirNames, entry.Name())
		}
	}

	var skills []*types.Skill
	for _, dirName := range dirNames {
		// fmt.Println("[DEBUG] Loading skill:", dirName)
		if skill, err := l.LoadSkill(dirName); err == nil {
			// fmt.Println("[DEBUG] Successfully loaded skill:", dirName)
			skills = append(skills, skill)
		} else {
			// fmt.Println("[DEBUG] Failed to load skill", dirName, ":", err)
		}
	}
	// fmt.Println("[DEBUG] ListSkills returning", len(skills), "skills")

	return skills, nil
}

// InstallSkill installs a skill from a directory.
func (l *Loader) InstallSkill(sourcePath, skillID string) (*types.Skill, error) {
	destPath := filepath.Join(l.skillsPath, skillID)
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		return nil, fmt.Errorf("skill %s already exists", skillID)
	}

	if err := copyDir(sourcePath, destPath); err != nil {
		return nil, fmt.Errorf("failed to copy skill: %w", err)
	}

	return l.LoadSkill(skillID)
}

// UninstallSkill uninstalls a skill.
func (l *Loader) UninstallSkill(skillID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	skillPath := filepath.Join(l.skillsPath, skillID)
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		return fmt.Errorf("skill not found: %s", skillID)
	}

	if err := os.RemoveAll(skillPath); err != nil {
		return fmt.Errorf("failed to remove skill: %w", err)
	}

	delete(l.registry, skillID)
	return nil
}

// copyDir copies a directory recursively.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}
