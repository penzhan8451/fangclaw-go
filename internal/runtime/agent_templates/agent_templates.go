package agent_templates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

type AgentTemplates struct {
	templates    []types.AgentTemplate
	templatesDir string
	mu           sync.RWMutex
	lastModTime  time.Time
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

func NewAgentTemplates() *AgentTemplates {
	homeDir, _ := os.UserHomeDir()
	templatesDir := filepath.Join(homeDir, ".fangclaw-go", "agent_templates")

	return &AgentTemplates{
		templates:    types.GetDefaultAgentTemplates(),
		templatesDir: templatesDir,
		stopChan:     make(chan struct{}),
	}
}

func (at *AgentTemplates) getTemplatesDir() string {
	return at.templatesDir
}

func (at *AgentTemplates) ensureTemplatesDir() error {
	if _, err := os.Stat(at.templatesDir); os.IsNotExist(err) {
		if err := os.MkdirAll(at.templatesDir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (at *AgentTemplates) writeDefaultTemplates() error {
	defaultTemplates := types.GetDefaultAgentTemplates()
	for _, tpl := range defaultTemplates {
		filename := fmt.Sprintf("%s.json", tpl.ID)
		filepath := filepath.Join(at.templatesDir, filename)

		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			data, err := json.MarshalIndent(tpl, "", "  ")
			if err != nil {
				continue
			}
			os.WriteFile(filepath, data, 0644)
		}
	}
	return nil
}

func (at *AgentTemplates) loadTemplatesFromDir() ([]types.AgentTemplate, error) {
	templateMap := make(map[string]types.AgentTemplate)

	for _, t := range types.GetDefaultAgentTemplates() {
		templateMap[t.ID] = t
	}

	if _, err := os.Stat(at.templatesDir); err == nil {
		files, err := os.ReadDir(at.templatesDir)
		if err == nil {
			for _, file := range files {
				if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
					filepath := filepath.Join(at.templatesDir, file.Name())
					data, err := os.ReadFile(filepath)
					if err != nil {
						continue
					}
					var tpl types.AgentTemplate
					if err := json.Unmarshal(data, &tpl); err == nil {
						if tpl.ID != "" {
							templateMap[tpl.ID] = tpl
						}
					}
				}
			}
		}
	}

	templates := make([]types.AgentTemplate, 0, len(templateMap))
	for _, t := range templateMap {
		templates = append(templates, t)
	}
	return templates, nil
}

func (at *AgentTemplates) getDirModTime() time.Time {
	var latestModTime time.Time

	if _, err := os.Stat(at.templatesDir); err == nil {
		files, err := os.ReadDir(at.templatesDir)
		if err == nil {
			for _, file := range files {
				if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
					filepath := filepath.Join(at.templatesDir, file.Name())
					if info, err := os.Stat(filepath); err == nil {
						if info.ModTime().After(latestModTime) {
							latestModTime = info.ModTime()
						}
					}
				}
			}
		}
	}
	return latestModTime
}

func (at *AgentTemplates) Load() error {
	at.mu.Lock()
	defer at.mu.Unlock()

	if err := at.ensureTemplatesDir(); err != nil {
		return err
	}

	at.writeDefaultTemplates()

	templates, err := at.loadTemplatesFromDir()
	if err != nil {
		return err
	}

	at.templates = templates
	at.lastModTime = at.getDirModTime()
	return nil
}

func (at *AgentTemplates) StartWatching() {
	at.wg.Add(1)
	go func() {
		defer at.wg.Done()
		ticker := time.NewTicker(2 * 60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				at.checkAndReload()
			case <-at.stopChan:
				return
			}
		}
	}()
}

func (at *AgentTemplates) StopWatching() {
	close(at.stopChan)
	at.wg.Wait()
}

func (at *AgentTemplates) checkAndReload() {
	at.mu.Lock()
	defer at.mu.Unlock()

	currentModTime := at.getDirModTime()
	if currentModTime.After(at.lastModTime) {
		templates, err := at.loadTemplatesFromDir()
		if err == nil {
			at.templates = templates
			at.lastModTime = currentModTime
		}
	}
}

func (at *AgentTemplates) ListTemplates() []types.AgentTemplate {
	at.mu.RLock()
	defer at.mu.RUnlock()
	result := make([]types.AgentTemplate, len(at.templates))
	copy(result, at.templates)
	return result
}

func (at *AgentTemplates) GetTemplate(id string) *types.AgentTemplate {
	at.mu.RLock()
	defer at.mu.RUnlock()
	for i := range at.templates {
		if at.templates[i].ID == id {
			return &at.templates[i]
		}
	}
	return nil
}
