package types

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

const (
	MAX_JOBS_PER_AGENT      = 50
	MAX_CONSECUTIVE_ERRORS  = 5
	MAX_NAME_LENGTH         = 100
	MAX_TEXT_LENGTH         = 4096
	MAX_CHANNEL_NAME_LENGTH = 100
	MAX_RECIPIENT_LENGTH    = 256
	MAX_WEBHOOK_URL_LENGTH  = 2048
	MAX_EVERY_SECS          = 31536000
	MIN_EVERY_SECS          = 1
)

type CronJobID string

func NewCronJobID() CronJobID {
	return CronJobID(uuid.New().String())
}

func (id CronJobID) String() string {
	return string(id)
}

func ParseCronJobID(s string) (CronJobID, error) {
	if s == "" {
		return "", fmt.Errorf("invalid cron job ID")
	}
	_, err := uuid.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid cron job ID: %w", err)
	}
	return CronJobID(s), nil
}

func GenerateUUID() string {
	return uuid.New().String()
}

type CronScheduleKind string

const (
	CronScheduleKindAt    CronScheduleKind = "at"
	CronScheduleKindEvery CronScheduleKind = "every"
	CronScheduleKindCron  CronScheduleKind = "cron"
)

type CronSchedule struct {
	Kind      CronScheduleKind `json:"kind"`
	At        *time.Time       `json:"at,omitempty"`
	EverySecs *uint64          `json:"every_secs,omitempty"`
	Expr      *string          `json:"expr,omitempty"`
	Tz        *string          `json:"tz,omitempty"`
}

func NewCronScheduleAt(at time.Time) CronSchedule {
	return CronSchedule{
		Kind: CronScheduleKindAt,
		At:   &at,
	}
}

func NewCronScheduleEvery(everySecs uint64) CronSchedule {
	return CronSchedule{
		Kind:      CronScheduleKindEvery,
		EverySecs: &everySecs,
	}
}

func NewCronScheduleCron(expr string, tz *string) CronSchedule {
	return CronSchedule{
		Kind: CronScheduleKindCron,
		Expr: &expr,
		Tz:   tz,
	}
}

type CronActionKind string

const (
	CronActionKindSystemEvent  CronActionKind = "system_event"
	CronActionKindAgentTurn    CronActionKind = "agent_turn"
	CronActionKindExecuteShell CronActionKind = "execute_shell"
)

type CronAction struct {
	Kind          CronActionKind `json:"kind"`
	Text          *string        `json:"text,omitempty"`
	Message       *string        `json:"message,omitempty"`
	ModelOverride *string        `json:"model_override,omitempty"`
	TimeoutSecs   *uint64        `json:"timeout_secs,omitempty"`
	Command       *string        `json:"command,omitempty"`
	Args          []string       `json:"args,omitempty"`
}

func NewCronActionSystemEvent(text string) CronAction {
	return CronAction{
		Kind: CronActionKindSystemEvent,
		Text: &text,
	}
}

func NewCronActionAgentTurn(message string, modelOverride *string, timeoutSecs *uint64) CronAction {
	return CronAction{
		Kind:          CronActionKindAgentTurn,
		Message:       &message,
		ModelOverride: modelOverride,
		TimeoutSecs:   timeoutSecs,
	}
}

func NewCronActionExecuteShell(command string, args []string, timeoutSecs *uint64) CronAction {
	return CronAction{
		Kind:        CronActionKindExecuteShell,
		Command:     &command,
		Args:        args,
		TimeoutSecs: timeoutSecs,
	}
}

type CronDeliveryKind string

const (
	CronDeliveryKindNone        CronDeliveryKind = "none"
	CronDeliveryKindChannel     CronDeliveryKind = "channel"
	CronDeliveryKindLastChannel CronDeliveryKind = "last_channel"
	CronDeliveryKindWebhook     CronDeliveryKind = "webhook"
	CronDeliveryKindProject     CronDeliveryKind = "project"
)

type CronDelivery struct {
	Kind        CronDeliveryKind `json:"kind"`
	ChannelName *string          `json:"channel_name,omitempty"`
	Recipient   *string          `json:"recipient,omitempty"`
	Url         *string          `json:"url,omitempty"`
	ProjectID   *string          `json:"project_id,omitempty"`
}

func NewCronDeliveryNone() CronDelivery {
	return CronDelivery{
		Kind: CronDeliveryKindNone,
	}
}

func NewCronDeliveryChannel(channelName string, recipient *string) CronDelivery {
	return CronDelivery{
		Kind:        CronDeliveryKindChannel,
		ChannelName: &channelName,
		Recipient:   recipient,
	}
}

func NewCronDeliveryLastChannel() CronDelivery {
	return CronDelivery{
		Kind: CronDeliveryKindLastChannel,
	}
}

func NewCronDeliveryWebhook(url string) CronDelivery {
	return CronDelivery{
		Kind: CronDeliveryKindWebhook,
		Url:  &url,
	}
}

func NewCronDeliveryProject(projectID string) CronDelivery {
	return CronDelivery{
		Kind:      CronDeliveryKindProject,
		ProjectID: &projectID,
	}
}

type CronJob struct {
	ID        CronJobID    `json:"id"`
	AgentID   AgentID      `json:"agent_id"`
	Name      string       `json:"name"`
	Enabled   bool         `json:"enabled"`
	Schedule  CronSchedule `json:"schedule"`
	Action    CronAction   `json:"action"`
	Delivery  CronDelivery `json:"delivery"`
	CreatedAt time.Time    `json:"created_at"`
	LastRun   *time.Time   `json:"last_run,omitempty"`
	NextRun   *time.Time   `json:"next_run,omitempty"`
}

func NewCronJob(agentID AgentID, name string, enabled bool, schedule CronSchedule, action CronAction, delivery CronDelivery) CronJob {
	return CronJob{
		ID:        NewCronJobID(),
		AgentID:   agentID,
		Name:      name,
		Enabled:   enabled,
		Schedule:  schedule,
		Action:    action,
		Delivery:  delivery,
		CreatedAt: time.Now().UTC(),
	}
}

func (j *CronJob) Validate(agentJobCount int) error {
	if len(j.Name) == 0 {
		return fmt.Errorf("name is required")
	}
	if len(j.Name) > MAX_NAME_LENGTH {
		return fmt.Errorf("name too long (max %d chars)", MAX_NAME_LENGTH)
	}
	if agentJobCount >= MAX_JOBS_PER_AGENT {
		return fmt.Errorf("per-agent job limit reached (%d)", MAX_JOBS_PER_AGENT)
	}
	if err := j.Schedule.Validate(); err != nil {
		return err
	}
	if err := j.Action.Validate(); err != nil {
		return err
	}
	if err := j.Delivery.Validate(); err != nil {
		return err
	}
	return nil
}

func (s *CronSchedule) Validate() error {
	switch s.Kind {
	case CronScheduleKindAt:
		if s.At == nil {
			return fmt.Errorf("at time is required for at schedule")
		}
		now := time.Now().UTC()
		if s.At.Before(now) {
			return fmt.Errorf("at time must be in the future")
		}
	case CronScheduleKindEvery:
		if s.EverySecs == nil {
			return fmt.Errorf("every_secs is required for every schedule")
		}
		if *s.EverySecs < MIN_EVERY_SECS {
			return fmt.Errorf("every_secs must be at least %d", MIN_EVERY_SECS)
		}
		if *s.EverySecs > MAX_EVERY_SECS {
			return fmt.Errorf("every_secs must be at most %d", MAX_EVERY_SECS)
		}
	case CronScheduleKindCron:
		if s.Expr == nil || len(*s.Expr) == 0 {
			return fmt.Errorf("cron expression is required for cron schedule")
		}
		if _, err := cron.ParseStandard(*s.Expr); err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
	default:
		return fmt.Errorf("unknown schedule kind: %s", s.Kind)
	}
	return nil
}

func (a *CronAction) Validate() error {
	switch a.Kind {
	case CronActionKindSystemEvent:
		if a.Text == nil || len(*a.Text) == 0 {
			return fmt.Errorf("text is required for system_event action")
		}
		if len(*a.Text) > MAX_TEXT_LENGTH {
			return fmt.Errorf("text too long (max %d chars)", MAX_TEXT_LENGTH)
		}
	case CronActionKindAgentTurn:
		if a.Message == nil || len(*a.Message) == 0 {
			return fmt.Errorf("message is required for agent_turn action")
		}
		if len(*a.Message) > MAX_TEXT_LENGTH {
			return fmt.Errorf("message too long (max %d chars)", MAX_TEXT_LENGTH)
		}
		if a.TimeoutSecs != nil && *a.TimeoutSecs == 0 {
			return fmt.Errorf("timeout_secs must be positive")
		}
	case CronActionKindExecuteShell:
		if a.Command == nil || len(*a.Command) == 0 {
			return fmt.Errorf("command is required for execute_shell action")
		}
		if a.TimeoutSecs != nil && *a.TimeoutSecs == 0 {
			return fmt.Errorf("timeout_secs must be positive")
		}
	default:
		return fmt.Errorf("unknown action kind: %s", a.Kind)
	}
	return nil
}

func (d *CronDelivery) Validate() error {
	switch d.Kind {
	case CronDeliveryKindNone:
	case CronDeliveryKindChannel:
		if d.ChannelName == nil || len(*d.ChannelName) == 0 {
			return fmt.Errorf("channel_name is required for channel delivery")
		}
		if len(*d.ChannelName) > MAX_CHANNEL_NAME_LENGTH {
			return fmt.Errorf("channel_name too long (max %d chars)", MAX_CHANNEL_NAME_LENGTH)
		}
		if d.Recipient != nil && len(*d.Recipient) > MAX_RECIPIENT_LENGTH {
			return fmt.Errorf("recipient too long (max %d chars)", MAX_RECIPIENT_LENGTH)
		}
	case CronDeliveryKindLastChannel:
	case CronDeliveryKindWebhook:
		if d.Url == nil || len(*d.Url) == 0 {
			return fmt.Errorf("url is required for webhook delivery")
		}
		if len(*d.Url) > MAX_WEBHOOK_URL_LENGTH {
			return fmt.Errorf("url too long (max %d chars)", MAX_WEBHOOK_URL_LENGTH)
		}
	case CronDeliveryKindProject:
	default:
		return fmt.Errorf("unknown delivery kind: %s", d.Kind)
	}
	return nil
}

func UnmarshalCronSchedule(data []byte) (CronSchedule, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return CronSchedule{}, err
	}
	kind, ok := raw["kind"].(string)
	if !ok {
		return CronSchedule{}, fmt.Errorf("missing or invalid kind field")
	}
	switch CronScheduleKind(kind) {
	case CronScheduleKindAt:
		atStr, ok := raw["at"].(string)
		if !ok {
			return CronSchedule{}, fmt.Errorf("missing or invalid at field")
		}
		at, err := time.Parse(time.RFC3339, atStr)
		if err != nil {
			return CronSchedule{}, err
		}
		return NewCronScheduleAt(at.UTC()), nil
	case CronScheduleKindEvery:
		everySecs, ok := raw["every_secs"].(float64)
		if !ok {
			return CronSchedule{}, fmt.Errorf("missing or invalid every_secs field")
		}
		es := uint64(everySecs)
		return NewCronScheduleEvery(es), nil
	case CronScheduleKindCron:
		expr, ok := raw["expr"].(string)
		if !ok {
			return CronSchedule{}, fmt.Errorf("missing or invalid expr field")
		}
		var tz *string
		if tzVal, ok := raw["tz"]; ok && tzVal != nil {
			tzStr, ok := tzVal.(string)
			if ok {
				tz = &tzStr
			}
		}
		return NewCronScheduleCron(expr, tz), nil
	default:
		return CronSchedule{}, fmt.Errorf("unknown schedule kind: %s", kind)
	}
}

func UnmarshalCronAction(data []byte) (CronAction, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return CronAction{}, err
	}
	kind, ok := raw["kind"].(string)
	if !ok {
		return CronAction{}, fmt.Errorf("missing or invalid kind field")
	}
	switch CronActionKind(kind) {
	case CronActionKindSystemEvent:
		text, ok := raw["text"].(string)
		if !ok {
			return CronAction{}, fmt.Errorf("missing or invalid text field")
		}
		return NewCronActionSystemEvent(text), nil
	case CronActionKindAgentTurn:
		message, ok := raw["message"].(string)
		if !ok {
			return CronAction{}, fmt.Errorf("missing or invalid message field")
		}
		var modelOverride *string
		if moVal, ok := raw["model_override"]; ok && moVal != nil {
			moStr, ok := moVal.(string)
			if ok {
				modelOverride = &moStr
			}
		}
		var timeoutSecs *uint64
		if tsVal, ok := raw["timeout_secs"]; ok && tsVal != nil {
			tsFloat, ok := tsVal.(float64)
			if ok {
				ts := uint64(tsFloat)
				timeoutSecs = &ts
			}
		}
		return NewCronActionAgentTurn(message, modelOverride, timeoutSecs), nil
	case CronActionKindExecuteShell:
		command, ok := raw["command"].(string)
		if !ok {
			return CronAction{}, fmt.Errorf("missing or invalid command field")
		}
		var args []string
		if argsVal, ok := raw["args"]; ok && argsVal != nil {
			if argsSlice, ok := argsVal.([]interface{}); ok {
				for _, arg := range argsSlice {
					if argStr, ok := arg.(string); ok {
						args = append(args, argStr)
					}
				}
			}
		}
		var timeoutSecs *uint64
		if tsVal, ok := raw["timeout_secs"]; ok && tsVal != nil {
			tsFloat, ok := tsVal.(float64)
			if ok {
				ts := uint64(tsFloat)
				timeoutSecs = &ts
			}
		}
		return NewCronActionExecuteShell(command, args, timeoutSecs), nil
	default:
		return CronAction{}, fmt.Errorf("unknown action kind: %s", kind)
	}
}

func UnmarshalCronDelivery(data []byte) (CronDelivery, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return CronDelivery{}, err
	}
	kind, ok := raw["kind"].(string)
	if !ok {
		return CronDelivery{}, fmt.Errorf("missing or invalid kind field")
	}
	switch CronDeliveryKind(kind) {
	case CronDeliveryKindNone:
		return NewCronDeliveryNone(), nil
	case CronDeliveryKindChannel:
		channelName, ok := raw["channel_name"].(string)
		if !ok {
			return CronDelivery{}, fmt.Errorf("missing or invalid channel_name field")
		}
		var recipient *string
		if rVal, ok := raw["recipient"]; ok && rVal != nil {
			rStr, ok := rVal.(string)
			if ok {
				recipient = &rStr
			}
		}
		return NewCronDeliveryChannel(channelName, recipient), nil
	case CronDeliveryKindLastChannel:
		return NewCronDeliveryLastChannel(), nil
	case CronDeliveryKindWebhook:
		url, ok := raw["url"].(string)
		if !ok {
			return CronDelivery{}, fmt.Errorf("missing or invalid url field")
		}
		return NewCronDeliveryWebhook(url), nil
	case CronDeliveryKindProject:
		projectID, ok := raw["project_id"].(string)
		if !ok {
			return CronDelivery{}, fmt.Errorf("missing or invalid project_id field")
		}
		return NewCronDeliveryProject(projectID), nil
	default:
		return CronDelivery{}, fmt.Errorf("unknown delivery kind: %s", kind)
	}
}
