package projects

import (
	"time"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// ProjectID 唯一标识一个项目
type ProjectID = uuid.UUID

// NewProjectID 创建一个新项目 ID
func NewProjectID() ProjectID {
	return uuid.New()
}

// ParseProjectID 解析项目 ID 字符串
func ParseProjectID(s string) (ProjectID, error) {
	return uuid.Parse(s)
}

// Project 代表一个多 Agent 协作项目
type Project struct {
	ID          ProjectID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Members     []ProjectMember `json:"members"`
	ChatHistory []ChatMessage   `json:"chat_history"`

	PMAgentID        types.AgentID            `json:"pm_agent_id"`
	PMKeywords       []string                 `json:"pm_keywords,omitempty"`
	WorkflowID       *string                  `json:"workflow_id,omitempty"`
	WorkflowBindings []ProjectWorkflowBinding `json:"workflow_bindings,omitempty"`
	CronBindings     []ProjectCronBinding     `json:"cron_bindings,omitempty"`
	CronResults      []CronResult             `json:"cron_results,omitempty"`

	Workspace string `json:"workspace"`
}

// ProjectMember 代表项目中的 Agent
type ProjectMember struct {
	ID     types.AgentID `json:"id"`
	Name   string        `json:"name"`
	Role   string        `json:"role"` // "researcher", "analyst", "coder", "pm", "custom"
	Active bool          `json:"active"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
	ID        string         `json:"id"`
	Role      string         `json:"role"` // "user", "assistant", "agent", "pm"
	AgentID   *types.AgentID `json:"agent_id,omitempty"`
	AgentName *string        `json:"agent_name,omitempty"`
	Content   string         `json:"content"`
	Timestamp time.Time      `json:"timestamp"`
	Meta      map[string]any `json:"meta,omitempty"`
}

type ProjectEvent string

const (
	EventAgentAdded    ProjectEvent = "agent_added"
	EventAgentRemoved  ProjectEvent = "agent_removed"
	EventTaskStarted   ProjectEvent = "task_started"
	EventTaskCompleted ProjectEvent = "task_completed"
	EventCollabStarted ProjectEvent = "collab_started"
	EventCollabDone    ProjectEvent = "collab_done"
)

type WorkflowTriggerMode string

const (
	TriggerModeAuto    WorkflowTriggerMode = "auto"
	TriggerModeManual  WorkflowTriggerMode = "manual"
	TriggerModeKeyword WorkflowTriggerMode = "keyword"
)

type ProjectWorkflowBinding struct {
	WorkflowID   string              `json:"workflow_id"`
	WorkflowName string              `json:"workflow_name"`
	TriggerMode  WorkflowTriggerMode `json:"trigger_mode"`
	Keywords     []string            `json:"keywords,omitempty"`
	Enabled      bool                `json:"enabled"`
}

type CronBindingStatus string

const (
	CronBindingActive        CronBindingStatus = "active"
	CronBindingOrphaned      CronBindingStatus = "orphaned"
	CronBindingAgentMismatch CronBindingStatus = "agent_mismatch"
)

type ProjectCronBinding struct {
	JobID   string            `json:"job_id"`
	JobName string            `json:"job_name"`
	Enabled bool              `json:"enabled"`
	Status  CronBindingStatus `json:"status"`
}

type CronResult struct {
	ID        string    `json:"id"`
	JobID     string    `json:"job_id"`
	JobName   string    `json:"job_name"`
	AgentID   string    `json:"agent_id"`
	AgentName string    `json:"agent_name"`
	Result    string    `json:"result"`
	Status    string    `json:"status"`
	FiredAt   time.Time `json:"fired_at"`
}
