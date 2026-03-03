// Package types provides core data structures for OpenFang.
package types

import (
	"time"
)

// SkillRuntimeType represents the runtime type for a skill.
type SkillRuntimeType string

const (
	SkillRuntimePython  SkillRuntimeType = "python"
	SkillRuntimeNode    SkillRuntimeType = "node"
	SkillRuntimeWasm    SkillRuntimeType = "wasm"
	SkillRuntimeBuiltin SkillRuntimeType = "builtin"
	SkillRuntimePrompt  SkillRuntimeType = "prompt"
)

// SkillRuntime represents the runtime configuration for a skill.
type SkillRuntime struct {
	RuntimeType SkillRuntimeType `json:"runtime_type"`
	Entry       string            `json:"entry"`
	Version     string            `json:"version,omitempty"`
}

// SkillToolDefinition defines a tool provided by a skill.
type SkillToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// SkillTools represents the tools provided and required by a skill.
type SkillTools struct {
	Provided []SkillToolDefinition `json:"provided"`
	Required []string               `json:"required,omitempty"`
}

// SkillRequirements represents the requirements for a skill.
type SkillRequirements struct {
	Python []string `json:"python,omitempty"`
	Node   []string `json:"node,omitempty"`
	System []string `json:"system,omitempty"`
}

// SkillManifest represents the manifest for a skill.
type SkillManifest struct {
	Version        string            `json:"version"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Author         string            `json:"author,omitempty"`
	Runtime        SkillRuntime      `json:"runtime"`
	Tools          SkillTools        `json:"tools"`
	Requirements   SkillRequirements `json:"requirements,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	PromptContext  string            `json:"prompt_context,omitempty"`
}

// SkillToolResult represents the result of executing a skill tool.
type SkillToolResult struct {
	Output  interface{} `json:"output"`
	IsError bool        `json:"is_error"`
}

// Skill represents an installed skill.
type Skill struct {
	ID          string        `json:"id"`
	Manifest    SkillManifest `json:"manifest"`
	InstallPath string        `json:"install_path"`
	InstalledAt time.Time     `json:"installed_at"`
	Enabled     bool          `json:"enabled"`
}

// SkillError represents an error that occurred during skill execution.
type SkillError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e SkillError) Error() string {
	return e.Message
}

// NewSkillError creates a new SkillError.
func NewSkillError(typ, message string) SkillError {
	return SkillError{Type: typ, Message: message}
}
