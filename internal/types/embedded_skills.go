package types

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed skills/*
var embeddedSkillsFS embed.FS

const embeddedSkillsDir = "skills"

// EmbeddedSkill represents an embedded skill
type EmbeddedSkill struct {
	ID      string
	FS      embed.FS
	BaseDir string
}

// ListEmbeddedSkills returns a list of all embedded skills
func ListEmbeddedSkills() ([]*EmbeddedSkill, error) {
	var skills []*EmbeddedSkill

	entries, err := embeddedSkillsFS.ReadDir(embeddedSkillsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded skills directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			skills = append(skills, &EmbeddedSkill{
				ID:      entry.Name(),
				FS:      embeddedSkillsFS,
				BaseDir: filepath.Join(embeddedSkillsDir, entry.Name()),
			})
		}
	}

	return skills, nil
}

// GetEmbeddedSkill gets an embedded skill by ID
func GetEmbeddedSkill(skillID string) (*EmbeddedSkill, error) {
	skillPath := filepath.Join(embeddedSkillsDir, skillID)
	if _, err := fs.Stat(embeddedSkillsFS, skillPath); err != nil {
		return nil, fmt.Errorf("embedded skill not found: %s", skillID)
	}

	return &EmbeddedSkill{
		ID:      skillID,
		FS:      embeddedSkillsFS,
		BaseDir: skillPath,
	}, nil
}

// Extract extracts the embedded skill to the destination directory
func (es *EmbeddedSkill) Extract(destPath string) error {
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	return fs.WalkDir(es.FS, es.BaseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(es.BaseDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		destFilePath := filepath.Join(destPath, relPath)

		if d.IsDir() {
			return os.MkdirAll(destFilePath, 0755)
		}

		data, err := fs.ReadFile(es.FS, path)
		if err != nil {
			return err
		}

		return os.WriteFile(destFilePath, data, 0644)
	})
}

// ExtractEmbeddedSkill extracts a specific embedded skill to the destination directory
func ExtractEmbeddedSkill(skillID, destPath string) error {
	skill, err := GetEmbeddedSkill(skillID)
	if err != nil {
		return err
	}
	return skill.Extract(destPath)
}

// ExtractAllEmbeddedSkills extracts all embedded skills to the destination directory
func ExtractAllEmbeddedSkills(destBasePath string) error {
	skills, err := ListEmbeddedSkills()
	if err != nil {
		return err
	}

	for _, skill := range skills {
		destPath := filepath.Join(destBasePath, skill.ID)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			if err := skill.Extract(destPath); err != nil {
				return fmt.Errorf("failed to extract skill %s: %w", skill.ID, err)
			}
		}
	}

	return nil
}
