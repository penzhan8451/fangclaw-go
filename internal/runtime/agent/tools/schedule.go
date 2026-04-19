package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/cron"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

type ScheduleCreateTool struct {
	scheduler *cron.CronScheduler
}

func NewScheduleCreateTool(scheduler *cron.CronScheduler) *ScheduleCreateTool {
	return &ScheduleCreateTool{scheduler: scheduler}
}

func (t *ScheduleCreateTool) Name() string {
	return "schedule_create"
}

func (t *ScheduleCreateTool) Description() string {
	return "Schedule a recurring task using natural language or cron syntax"
}

func (t *ScheduleCreateTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "schedule_create",
			"description": "Schedule a recurring task using natural language OR standard cron syntax. Both formats are supported. Natural language examples: 'every 5 minutes', 'daily at 9am', 'weekly on Friday at 9am', 'weekdays at 6pm'. Standard cron examples: '0 */5 * * *', '0 9 * * *', '0 9 * * 5'.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"description": map[string]interface{}{"type": "string", "description": "What this schedule does (e.g., 'Check for new emails')"},
					"schedule":    map[string]interface{}{"type": "string", "description": "Natural language schedule OR standard cron expression (both supported)"},
					"message":     map[string]interface{}{"type": "string", "description": "Message to send when the schedule triggers"},
				},
				"required": []string{"description", "schedule"},
			},
		},
	}
}

func (t *ScheduleCreateTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	description, _ := args["description"].(string)
	if description == "" {
		return "", fmt.Errorf("description required")
	}

	scheduleStr, _ := args["schedule"].(string)
	if scheduleStr == "" {
		return "", fmt.Errorf("schedule required")
	}

	message, _ := args["message"].(string)
	if message == "" {
		message = "Scheduled task: " + description
	}

	if t.scheduler == nil {
		return "", fmt.Errorf("cron scheduler not available")
	}

	cronExpr, err := parseScheduleToCron(scheduleStr)
	if err != nil {
		return "", err
	}

	agentIDStr, ok := ctx.Value("agent_id").(string)
	if !ok || agentIDStr == "" {
		return "", fmt.Errorf("agent ID not available in context")
	}

	agentID, err := types.ParseAgentID(agentIDStr)
	if err != nil {
		return "", fmt.Errorf("invalid agent ID: %w", err)
	}

	job := types.CronJob{
		ID:        types.CronJobID(uuid.New().String()),
		Name:      description,
		AgentID:   agentID,
		Enabled:   true,
		Schedule:  types.NewCronScheduleCron(cronExpr, nil),
		Action:    types.NewCronActionAgentTurn(message, nil, nil),
		Delivery:  types.NewCronDeliveryLastChannel(),
		CreatedAt: time.Now().UTC(),
	}

	jobID, err := t.scheduler.AddJob(job, false)
	if err != nil {
		return "", fmt.Errorf("failed to create schedule: %w", err)
	}

	if err := t.scheduler.Persist(); err != nil {
		return "", fmt.Errorf("failed to persist schedule: %w", err)
	}

	return fmt.Sprintf("Schedule created:\n  ID: %s\n  Description: %s\n  Cron: %s\n  Original: %s",
		jobID, description, cronExpr, scheduleStr), nil
}

type ScheduleListTool struct {
	scheduler *cron.CronScheduler
}

func NewScheduleListTool(scheduler *cron.CronScheduler) *ScheduleListTool {
	return &ScheduleListTool{scheduler: scheduler}
}

func (t *ScheduleListTool) Name() string {
	return "schedule_list"
}

func (t *ScheduleListTool) Description() string {
	return "List all scheduled tasks"
}

func (t *ScheduleListTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "schedule_list",
			"description": "List all scheduled tasks with their IDs, descriptions, schedules, and next run times.",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

func (t *ScheduleListTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.scheduler == nil {
		return "", fmt.Errorf("cron scheduler not available")
	}

	agentIDStr, ok := ctx.Value("agent_id").(string)
	if !ok || agentIDStr == "" {
		return "", fmt.Errorf("agent ID not available in context")
	}

	agentID, err := types.ParseAgentID(agentIDStr)
	if err != nil {
		return "", fmt.Errorf("invalid agent ID: %w", err)
	}

	jobs := t.scheduler.ListJobs(agentID)
	if len(jobs) == 0 {
		return "No scheduled tasks.", nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Scheduled tasks (%d):\n\n", len(jobs)))
	for _, j := range jobs {
		status := "active"
		if !j.Enabled {
			status = "paused"
		}
		cronExpr := ""
		if j.Schedule.Kind == types.CronScheduleKindCron {
			if j.Schedule.Expr != nil {
				cronExpr = *j.Schedule.Expr
			}
		} else if j.Schedule.Kind == types.CronScheduleKindEvery {
			if j.Schedule.EverySecs != nil {
				cronExpr = fmt.Sprintf("every %ds", *j.Schedule.EverySecs)
			}
		}
		nextRun := "-"
		if j.NextRun != nil {
			nextRun = j.NextRun.Format(time.RFC3339)
		}
		output.WriteString(fmt.Sprintf(
			"  [%s] %s — %s\n    Cron: %s\n    Next run: %s\n\n",
			status, j.ID, j.Name, cronExpr, nextRun,
		))
	}

	return output.String(), nil
}

type ScheduleDeleteTool struct {
	scheduler *cron.CronScheduler
}

func NewScheduleDeleteTool(scheduler *cron.CronScheduler) *ScheduleDeleteTool {
	return &ScheduleDeleteTool{scheduler: scheduler}
}

func (t *ScheduleDeleteTool) Name() string {
	return "schedule_delete"
}

func (t *ScheduleDeleteTool) Description() string {
	return "Remove a scheduled task by its ID"
}

func (t *ScheduleDeleteTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "schedule_delete",
			"description": "Remove a scheduled task by its ID.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "string", "description": "The schedule ID to remove"},
				},
				"required": []string{"id"},
			},
		},
	}
}

func (t *ScheduleDeleteTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id required")
	}

	if t.scheduler == nil {
		return "", fmt.Errorf("cron scheduler not available")
	}

	jobID := types.CronJobID(id)
	_, err := t.scheduler.RemoveJob(jobID)
	if err != nil {
		return "", fmt.Errorf("failed to delete schedule: %w", err)
	}

	if err := t.scheduler.Persist(); err != nil {
		return "", fmt.Errorf("failed to persist after deletion: %w", err)
	}

	return fmt.Sprintf("Schedule '%s' deleted.", id), nil
}

func parseScheduleToCron(input string) (string, error) {
	input = strings.TrimSpace(strings.ToLower(input))

	if strings.Count(input, " ") == 4 {
		return input, nil
	}

	if strings.HasPrefix(input, "every ") {
		rest := strings.TrimPrefix(input, "every ")
		if strings.HasSuffix(rest, " minutes") {
			nStr := strings.TrimSuffix(rest, " minutes")
			n, err := strconv.Atoi(strings.TrimSpace(nStr))
			if err == nil && n > 0 && n < 60 {
				return fmt.Sprintf("*/%d * * * *", n), nil
			}
		}
		if rest == "minute" || rest == "1 minute" {
			return "* * * * *", nil
		}
		if rest == "hour" || rest == "1 hour" {
			return "0 * * * *", nil
		}
		if strings.HasSuffix(rest, " hours") {
			nStr := strings.TrimSuffix(rest, " hours")
			n, err := strconv.Atoi(strings.TrimSpace(nStr))
			if err == nil && n > 0 && n < 24 {
				return fmt.Sprintf("0 */%d * * *", n), nil
			}
		}
		if rest == "day" || rest == "1 day" {
			return "0 0 * * *", nil
		}
		if rest == "week" || rest == "1 week" {
			return "0 0 * * 0", nil
		}
	}

	if strings.HasPrefix(input, "daily at ") {
		timeStr := strings.TrimPrefix(input, "daily at ")
		hour, err := parseTimeToHour(timeStr)
		if err == nil {
			return fmt.Sprintf("0 %d * * *", hour), nil
		}
	}

	if strings.HasPrefix(input, "weekdays at ") {
		timeStr := strings.TrimPrefix(input, "weekdays at ")
		hour, err := parseTimeToHour(timeStr)
		if err == nil {
			return fmt.Sprintf("0 %d * * 1-5", hour), nil
		}
	}

	if strings.HasPrefix(input, "weekends at ") {
		timeStr := strings.TrimPrefix(input, "weekends at ")
		hour, err := parseTimeToHour(timeStr)
		if err == nil {
			return fmt.Sprintf("0 %d * * 0,6", hour), nil
		}
	}

	if strings.HasPrefix(input, "weekly on ") {
		rest := strings.TrimPrefix(input, "weekly on ")
		parts := strings.SplitN(rest, " at ", 2)
		if len(parts) == 2 {
			dayStr := parts[0]
			timeStr := parts[1]
			dayNum, err := parseDayToNumber(dayStr)
			if err == nil {
				hour, err := parseTimeToHour(timeStr)
				if err == nil {
					return fmt.Sprintf("0 %d * * %d", hour, dayNum), nil
				}
			}
		}
	}

	if strings.Contains(input, " on ") && strings.Contains(input, " at ") {
		parts := strings.SplitN(input, " on ", 2)
		if len(parts) == 2 {
			rest := parts[1]
			dayTimeParts := strings.SplitN(rest, " at ", 2)
			if len(dayTimeParts) == 2 {
				dayStr := dayTimeParts[0]
				timeStr := dayTimeParts[1]
				dayNum, err := parseDayToNumber(dayStr)
				if err == nil {
					hour, err := parseTimeToHour(timeStr)
					if err == nil {
						return fmt.Sprintf("0 %d * * %d", hour, dayNum), nil
					}
				}
			}
		}
	}

	switch input {
	case "hourly":
		return "0 * * * *", nil
	case "daily":
		return "0 0 * * *", nil
	case "weekly":
		return "0 0 * * 0", nil
	case "monthly":
		return "0 0 1 * *", nil
	}

	return "", fmt.Errorf("could not parse schedule '%s'. Try: 'every 5 minutes', 'daily at 9am', 'weekdays at 6pm', 'weekly on friday at 9am', or a cron expression like '0 */5 * * *'", input)
}

func parseTimeToHour(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	if strings.HasSuffix(s, "am") {
		hStr := strings.TrimSuffix(s, "am")
		h, err := strconv.Atoi(strings.TrimSpace(hStr))
		if err == nil {
			if h == 12 {
				return 0, nil
			}
			if h >= 1 && h <= 11 {
				return h, nil
			}
		}
	}

	if strings.HasSuffix(s, "pm") {
		hStr := strings.TrimSuffix(s, "pm")
		h, err := strconv.Atoi(strings.TrimSpace(hStr))
		if err == nil {
			if h == 12 {
				return 12, nil
			}
			if h >= 1 && h <= 11 {
				return h + 12, nil
			}
		}
	}

	if strings.Contains(s, ":") {
		parts := strings.SplitN(s, ":", 2)
		h, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err == nil && h >= 0 && h <= 23 {
			return h, nil
		}
	}

	h, err := strconv.Atoi(s)
	if err == nil && h >= 0 && h <= 23 {
		return h, nil
	}

	return 0, fmt.Errorf("invalid time: %s", s)
}

func parseDayToNumber(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "sunday", "sun":
		return 0, nil
	case "monday", "mon":
		return 1, nil
	case "tuesday", "tue":
		return 2, nil
	case "wednesday", "wed":
		return 3, nil
	case "thursday", "thu":
		return 4, nil
	case "friday", "fri":
		return 5, nil
	case "saturday", "sat":
		return 6, nil
	}
	return 0, fmt.Errorf("invalid day: %s", s)
}
