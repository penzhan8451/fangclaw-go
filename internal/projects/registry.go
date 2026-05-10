package projects

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// Registry 项目注册表，管理 Project 的持久化和检索
type Registry struct {
	mu       sync.RWMutex
	projects map[ProjectID]*Project
	dataDir  string
}

// NewRegistry 创建一个新的项目注册表
func NewRegistry(dataDir string) *Registry {
	r := &Registry{
		projects: make(map[ProjectID]*Project),
		dataDir:  dataDir,
	}
	r.loadFromDisk()
	return r
}

// projectDataDir 返回项目数据目录
func (r *Registry) projectDataDir(projectID ProjectID) string {
	return filepath.Join(r.dataDir, "projects", projectID.String())
}

// projectFile 返回项目配置文件路径
func (r *Registry) projectFile(projectID ProjectID) string {
	return filepath.Join(r.projectDataDir(projectID), "project.json")
}

// loadFromDisk 从磁盘加载所有项目
func (r *Registry) loadFromDisk() {
	projectsDir := filepath.Join(r.dataDir, "projects")
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectID, err := uuid.Parse(entry.Name())
		if err != nil {
			continue
		}

		project, err := r.loadProject(projectID)
		if err == nil {
			r.projects[projectID] = project
		}
	}
}

// loadProject 加载单个项目
func (r *Registry) loadProject(projectID ProjectID) (*Project, error) {
	path := r.projectFile(projectID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, err
	}

	return &project, nil
}

// saveProject 保存项目到磁盘
func (r *Registry) saveProject(project *Project) error {
	projectDir := r.projectDataDir(project.ID)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err
	}

	workspaceDir := filepath.Join(projectDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(r.projectFile(project.ID), data, 0644)
}

// Register 注册一个新项目
func (r *Registry) Register(project *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.projects[project.ID]; exists {
		return fmt.Errorf("project already exists: %s", project.ID)
	}

	r.projects[project.ID] = project
	return r.saveProject(project)
}

// Save 保存项目
func (r *Registry) Save(project *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	project.UpdatedAt = time.Now()
	r.projects[project.ID] = project
	return r.saveProject(project)
}

// Get 获取项目
func (r *Registry) Get(projectID ProjectID) *Project {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.projects[projectID]
}

// List 列出所有项目
func (r *Registry) List() []*Project {
	r.mu.RLock()
	defer r.mu.RUnlock()

	projects := make([]*Project, 0, len(r.projects))
	for _, project := range r.projects {
		projects = append(projects, project)
	}
	return projects
}

// Delete 删除项目
func (r *Registry) Delete(projectID ProjectID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.projects[projectID]; !exists {
		return fmt.Errorf("project not found: %s", projectID)
	}

	delete(r.projects, projectID)
	projectDir := r.projectDataDir(projectID)
	return os.RemoveAll(projectDir)
}

// Create 创建一个新项目
func (r *Registry) Create(name, description, owner string, pmKeywords []string) (*Project, error) {
	pmAgentID := types.NewAgentID()

	project := &Project{
		ID:          NewProjectID(),
		Name:        name,
		Description: description,
		Owner:       owner,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Members: []ProjectMember{
			{
				ID:     pmAgentID,
				Name:   "PM Agent",
				Role:   "pm",
				Active: true,
			},
		},
		ChatHistory: []ChatMessage{},
		PMAgentID:   pmAgentID,
		PMKeywords:  pmKeywords,
		Workspace:   "workspace",
	}

	if err := r.Register(project); err != nil {
		return nil, err
	}

	return project, nil
}

// AddMember 添加成员到项目
func (r *Registry) AddMember(projectID ProjectID, agentID types.AgentID, name, role string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	project, exists := r.projects[projectID]
	if !exists {
		return fmt.Errorf("project not found: %s", projectID)
	}

	for _, m := range project.Members {
		if m.ID == agentID {
			return fmt.Errorf("agent already in project: %s", agentID)
		}
	}

	member := ProjectMember{
		ID:     agentID,
		Name:   name,
		Role:   role,
		Active: true,
	}

	project.Members = append(project.Members, member)
	project.UpdatedAt = time.Now()
	return r.saveProject(project)
}

// RemoveMember 从项目中移除成员
func (r *Registry) RemoveMember(projectID ProjectID, agentID types.AgentID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	project, exists := r.projects[projectID]
	if !exists {
		return fmt.Errorf("project not found: %s", projectID)
	}

	newMembers := []ProjectMember{}
	found := false
	for _, m := range project.Members {
		if m.ID != agentID {
			newMembers = append(newMembers, m)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("agent not in project: %s", agentID)
	}

	project.Members = newMembers
	project.UpdatedAt = time.Now()
	return r.saveProject(project)
}

// AddChatMessage 添加聊天消息
func (r *Registry) AddChatMessage(projectID ProjectID, msg ChatMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	project, exists := r.projects[projectID]
	if !exists {
		return fmt.Errorf("project not found: %s", projectID)
	}

	project.ChatHistory = append(project.ChatHistory, msg)
	project.UpdatedAt = time.Now()
	return r.saveProject(project)
}

func (r *Registry) BindWorkflow(projectID ProjectID, binding ProjectWorkflowBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	project, exists := r.projects[projectID]
	if !exists {
		return fmt.Errorf("project not found: %s", projectID)
	}

	for i, b := range project.WorkflowBindings {
		if b.WorkflowID == binding.WorkflowID {
			project.WorkflowBindings[i] = binding
			project.UpdatedAt = time.Now()
			return r.saveProject(project)
		}
	}

	project.WorkflowBindings = append(project.WorkflowBindings, binding)
	project.UpdatedAt = time.Now()
	return r.saveProject(project)
}

func (r *Registry) UnbindWorkflow(projectID ProjectID, workflowID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	project, exists := r.projects[projectID]
	if !exists {
		return fmt.Errorf("project not found: %s", projectID)
	}

	newBindings := make([]ProjectWorkflowBinding, 0, len(project.WorkflowBindings))
	found := false
	for _, b := range project.WorkflowBindings {
		if b.WorkflowID == workflowID {
			found = true
			continue
		}
		newBindings = append(newBindings, b)
	}

	if !found {
		return fmt.Errorf("workflow not bound to project: %s", workflowID)
	}

	project.WorkflowBindings = newBindings
	project.UpdatedAt = time.Now()
	return r.saveProject(project)
}

func (r *Registry) BindCron(projectID ProjectID, binding ProjectCronBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	project, exists := r.projects[projectID]
	if !exists {
		return fmt.Errorf("project not found: %s", projectID)
	}

	for i, b := range project.CronBindings {
		if b.JobID == binding.JobID {
			project.CronBindings[i] = binding
			project.UpdatedAt = time.Now()
			return r.saveProject(project)
		}
	}

	project.CronBindings = append(project.CronBindings, binding)
	project.UpdatedAt = time.Now()
	return r.saveProject(project)
}

func (r *Registry) UnbindCron(projectID ProjectID, jobID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	project, exists := r.projects[projectID]
	if !exists {
		return fmt.Errorf("project not found: %s", projectID)
	}

	newBindings := make([]ProjectCronBinding, 0, len(project.CronBindings))
	found := false
	for _, b := range project.CronBindings {
		if b.JobID == jobID {
			found = true
			continue
		}
		newBindings = append(newBindings, b)
	}

	if !found {
		return fmt.Errorf("cron job not bound to project: %s", jobID)
	}

	project.CronBindings = newBindings
	project.UpdatedAt = time.Now()
	return r.saveProject(project)
}

func (r *Registry) AddCronResult(projectID ProjectID, result CronResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	project, exists := r.projects[projectID]
	if !exists {
		return fmt.Errorf("project not found: %s", projectID)
	}

	const maxResultsPerJob = 50
	var resultsForJob []CronResult
	for _, cr := range project.CronResults {
		if cr.JobID == result.JobID {
			resultsForJob = append(resultsForJob, cr)
		}
	}
	if len(resultsForJob) >= maxResultsPerJob {
		newResults := make([]CronResult, 0, len(project.CronResults))
		removed := false
		for _, cr := range project.CronResults {
			if !removed && cr.JobID == result.JobID {
				removed = true
				continue
			}
			newResults = append(newResults, cr)
		}
		project.CronResults = newResults
	}

	project.CronResults = append(project.CronResults, result)
	project.UpdatedAt = time.Now()
	return r.saveProject(project)
}
