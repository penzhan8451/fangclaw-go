package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
	"github.com/rs/zerolog/log"
)

type AgentRunner interface {
	RunAgent(ctx context.Context, agentID types.AgentID, prompt string) (string, error)
}

type StepAgentRef struct {
	ID   *string
	Name *string
}

type AgentResolverFunc func(agent StepAgentRef) (string, string, bool)
type MessageSenderFunc func(agentID, prompt string) (string, uint64, uint64, error)

type WorkflowInfo struct {
	ID          string
	Name        string
	Description string
	Steps       []WorkflowStepInfo
}

type WorkflowStepInfo struct {
	Name  string
	Agent StepAgentRef
}

type TemplateInfo struct {
	ID              string
	Name            string
	Description     string
	Category        string
	TriggerKeywords []string
	RequiredRoles   []string
	WorkflowID      string
}

type WorkflowExecutor interface {
	CreateRun(workflowID string, input string) *string
	ExecuteRun(runID string, resolver AgentResolverFunc, sender MessageSenderFunc) (string, error)
	GetWorkflow(id string) *WorkflowInfo
	ListTemplates() []TemplateInfo
	GetTemplate(id string) *TemplateInfo
	CreateWorkflowFromTemplate(templateID string) (string, error)
	Register(workflow types.Workflow) types.WorkflowID
}

type AgentFinder interface {
	FindAgentByName(ctx context.Context, name string) (string, bool)
}

type CronJobManager interface {
	GetJob(id types.CronJobID) *types.CronJob
	ListJobs(agentID types.AgentID) []types.CronJob
	ListAllJobs() []types.CronJob
}

type PMAgent struct {
	registry       *Registry
	runner         AgentRunner
	workflowEngine WorkflowExecutor
	cronManager    CronJobManager
	agentFinder    AgentFinder
	eventChan      chan ProjectEvent
	autoGenMu      sync.Mutex
	autoGenUsage   map[string]*autoGenCounter
	runningChats   map[ProjectID]context.CancelFunc
	runningChatsMu sync.Mutex
}

const autoGenDailyLimit = 10

type autoGenCounter struct {
	Date  string
	Count int
}

func NewPMAgent(registry *Registry, runner AgentRunner) *PMAgent {
	return &PMAgent{
		registry:     registry,
		runner:       runner,
		eventChan:    make(chan ProjectEvent, 100),
		autoGenUsage: make(map[string]*autoGenCounter),
		runningChats: make(map[ProjectID]context.CancelFunc),
	}
}

func (pm *PMAgent) SetWorkflowEngine(engine WorkflowExecutor) {
	pm.workflowEngine = engine
}

func (pm *PMAgent) SetAgentFinder(finder AgentFinder) {
	pm.agentFinder = finder
}

func (pm *PMAgent) SetCronManager(manager CronJobManager) {
	pm.cronManager = manager
}

func (pm *PMAgent) RegisterChatCancel(projectID ProjectID, cancel context.CancelFunc) {
	pm.runningChatsMu.Lock()
	defer pm.runningChatsMu.Unlock()
	pm.runningChats[projectID] = cancel
}

func (pm *PMAgent) UnregisterChatCancel(projectID ProjectID) {
	pm.runningChatsMu.Lock()
	defer pm.runningChatsMu.Unlock()
	delete(pm.runningChats, projectID)
}

func (pm *PMAgent) CancelProjectChat(projectID ProjectID) bool {
	pm.runningChatsMu.Lock()
	cancel, ok := pm.runningChats[projectID]
	if ok {
		delete(pm.runningChats, projectID)
	}
	pm.runningChatsMu.Unlock()
	if ok && cancel != nil {
		cancel()
		return true
	}
	return false
}

func (pm *PMAgent) IsChatRunning(projectID ProjectID) bool {
	pm.runningChatsMu.Lock()
	defer pm.runningChatsMu.Unlock()
	_, ok := pm.runningChats[projectID]
	return ok
}

func (pm *PMAgent) ListCronBindings(ctx context.Context, projectID ProjectID) []ProjectCronBinding {
	project := pm.registry.Get(projectID)
	if project == nil {
		return nil
	}

	var result []ProjectCronBinding
	for _, binding := range project.CronBindings {
		if pm.cronManager != nil {
			job := pm.cronManager.GetJob(types.CronJobID(binding.JobID))
			if job == nil {
				binding.Status = CronBindingOrphaned
				binding.Enabled = false
			} else if job.Action.Kind == types.CronActionKindAgentTurn {
				isMember := false
				for _, m := range project.Members {
					if m.Active && m.ID == job.AgentID {
						isMember = true
						break
					}
				}
				if !isMember {
					binding.Status = CronBindingAgentMismatch
				} else {
					binding.Status = CronBindingActive
				}
			} else {
				binding.Status = CronBindingActive
			}
		} else {
			binding.Status = CronBindingActive
		}
		result = append(result, binding)
	}

	return result
}

func (pm *PMAgent) HandleUserInput(ctx context.Context, projectID ProjectID, userInput string) (*ChatMessage, error) {
	project := pm.registry.Get(projectID)
	if project == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	// Tier 1: Project-level workflow bindings
	if pm.workflowEngine != nil {
		for i := range project.WorkflowBindings {
			binding := &project.WorkflowBindings[i]
			if !binding.Enabled {
				continue
			}

			shouldTrigger := false
			switch binding.TriggerMode {
			case TriggerModeAuto:
				shouldTrigger = true
			case TriggerModeKeyword:
				shouldTrigger = pm.matchKeywords(userInput, binding.Keywords)
			case TriggerModeManual:
				continue
			}

			if shouldTrigger {
				if binding.TriggerMode == TriggerModeAuto {
					binding.TriggerMode = TriggerModeKeyword
					pm.registry.BindWorkflow(projectID, *binding)
				}
				log.Debug().Str("workflowName", binding.WorkflowName).Msg("[HandleUserInput]Tier 1: binding workflow triggered")
				msg, err := pm.executeWorkflow(ctx, projectID, binding.WorkflowID, userInput, project)
				if err != nil {
					return nil, err
				}
				if msg.Meta == nil {
					msg.Meta = make(map[string]any)
				}
				msg.Meta["workflow_source"] = "binding"
				msg.Meta["workflow_name"] = binding.WorkflowName
				return msg, nil
			}
		}
	}

	// Tier 2: LLM auto-generate workflow for complex tasks
	if pm.workflowEngine != nil && pm.shouldAutoGenerateWorkflow(userInput, project) {
		genCtx, genCancel := context.WithTimeout(ctx, 80*time.Second)
		defer genCancel()

		rateLimitWarning := pm.checkAutoGenRateLimit(projectID)

		log.Info().Msg("[HandleUserInput]Tier 2: shouldAutoGenerateWorkflow is true, auto-generate workflow started")
		result, genErr := pm.GenerateWorkflowByLLM(genCtx, projectID, userInput)
		log.Debug().Str("WorkflowJSON Content", result.WorkflowJSON).Msg("[HandleUserInput]Tier 2: auto-generated workflow content")
		log.Debug().Strs("Workflow Keywords", result.Keywords).Msg("[HandleUserInput]Tier 2: auto-generated workflow Keywords")
		if genErr == nil && result != nil {
			result.Workflow.AutoGenerated = true
			workflowID := pm.workflowEngine.Register(result.Workflow)

			binding := ProjectWorkflowBinding{
				WorkflowID:   string(workflowID),
				WorkflowName: result.Workflow.Name,
				TriggerMode:  TriggerModeManual,
				Keywords:     result.Keywords,
				Enabled:      true,
			}
			pm.registry.BindWorkflow(projectID, binding)

			pm.recordAutoGenUsage(projectID)

			log.Debug().Str("workflowName", result.Workflow.Name).Str("workflowID", string(workflowID)).Msg("[HandleUserInput]Tier 2: auto-generated workflow")
			msg, execErr := pm.executeWorkflow(ctx, projectID, string(workflowID), userInput, project)
			if execErr == nil {
				meta := make(map[string]any)
				if msg.Meta != nil {
					meta = msg.Meta
				}
				meta["auto_generated"] = true
				meta["workflow_source"] = "auto_generated"
				meta["workflow_name"] = result.Workflow.Name
				meta["step_count"] = len(result.Workflow.Steps)
				if rateLimitWarning != "" {
					meta["rate_limit_warning"] = rateLimitWarning
				}
				msg.Meta = meta
				return msg, nil
			}
			log.Warn().Err(execErr).Msg("[HandleUserInput]Tier 2: auto-generated workflow execution failed, falling back to template matching")
		} else if genErr != nil {
			log.Warn().Err(genErr).Msg("[HandleUserInput]Tier 2: auto-generate workflow failed, falling back to template matching")
		}
	}

	// Tier 3: Template library keyword matching
	if pm.workflowEngine != nil {
		matchedTemplate := pm.matchTemplate(userInput, project)
		if matchedTemplate != nil {
			workflowID, err := pm.workflowEngine.CreateWorkflowFromTemplate(matchedTemplate.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to create workflow from template: %w", err)
			}

			binding := ProjectWorkflowBinding{
				WorkflowID:   workflowID,
				WorkflowName: matchedTemplate.Name,
				TriggerMode:  TriggerModeAuto,
				Keywords:     matchedTemplate.TriggerKeywords,
				Enabled:      true,
			}
			pm.registry.BindWorkflow(projectID, binding)
			log.Debug().Str("workflowName", matchedTemplate.Name).Msg("[HandleUserInput]Tier 3: workflow created from template")
			msg, err := pm.executeWorkflow(ctx, projectID, workflowID, userInput, project)
			if err != nil {
				return nil, err
			}
			if msg.Meta == nil {
				msg.Meta = make(map[string]any)
			}
			msg.Meta["workflow_source"] = "template"
			msg.Meta["workflow_name"] = matchedTemplate.Name
			return msg, nil
		}
	}

	return pm.directReply(ctx, projectID, userInput, project)
}

func (pm *PMAgent) ExecuteWorkflowDirectly(ctx context.Context, projectID ProjectID, workflowID string, input string) (*ChatMessage, error) {
	project := pm.registry.Get(projectID)
	if project == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	if pm.workflowEngine == nil {
		return nil, fmt.Errorf("workflow engine not available")
	}

	return pm.executeWorkflow(ctx, projectID, workflowID, input, project)
}

func (pm *PMAgent) executeWorkflow(ctx context.Context, projectID ProjectID, workflowID string, input string, project *Project) (*ChatMessage, error) {
	runIDPtr := pm.workflowEngine.CreateRun(workflowID, input)
	if runIDPtr == nil {
		return nil, fmt.Errorf("invalid workflow ID: %s", workflowID)
	}

	resolver := pm.buildProjectResolver(ctx, project)
	sender := pm.buildSender(ctx)

	output, err := pm.workflowEngine.ExecuteRun(*runIDPtr, resolver, sender)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "agent not found") {
			return pm.makeSystemMessage(projectID, "workflow中的agent没有找到，请先将agent作为member加入到project中来")
		}
		return nil, err
	}

	pmName := "PM Agent"
	msg := ChatMessage{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Role:      "assistant",
		AgentID:   &project.PMAgentID,
		AgentName: &pmName,
		Content:   output,
		Timestamp: time.Now(),
		Meta: map[string]any{
			"workflow_id": workflowID,
			"run_id":      *runIDPtr,
			"type":        "workflow_result",
		},
	}

	pm.registry.AddChatMessage(projectID, msg)
	return &msg, nil
}

func (pm *PMAgent) buildProjectResolver(ctx context.Context, project *Project) AgentResolverFunc {
	return func(agent StepAgentRef) (string, string, bool) {
		if agent.Name != nil {
			for _, member := range project.Members {
				if member.Active && strings.EqualFold(member.Name, *agent.Name) {
					return member.ID.String(), member.Name, true
				}
			}
			for _, member := range project.Members {
				if member.Active && strings.EqualFold(member.Role, *agent.Name) {
					return member.ID.String(), member.Name, true
				}
			}
			if pm.agentFinder != nil {
				if agentID, ok := pm.agentFinder.FindAgentByName(ctx, *agent.Name); ok {
					parsedID, err := types.ParseAgentID(agentID)
					if err == nil {
						role := inferRoleFromName(*agent.Name)
						if addErr := pm.registry.AddMember(project.ID, parsedID, *agent.Name, role); addErr == nil {
							log.Info().Str("agent", *agent.Name).Str("role", role).Str("project", project.ID.String()).Msg("Auto-added agent to project")
						}
					}
					return agentID, *agent.Name, true
				}
			}
		}

		if agent.ID != nil {
			for _, member := range project.Members {
				if member.Active && member.ID.String() == *agent.ID {
					return member.ID.String(), member.Name, true
				}
			}
			return *agent.ID, "", true
		}

		return "", "", false
	}
}

func (pm *PMAgent) buildSender(ctx context.Context) MessageSenderFunc {
	return func(agentID, prompt string) (string, uint64, uint64, error) {
		parsedID, err := types.ParseAgentID(agentID)
		if err != nil {
			return "", 0, 0, fmt.Errorf("invalid agent ID: %s", agentID)
		}
		output, err := pm.runner.RunAgent(ctx, parsedID, prompt)
		if err != nil {
			return "", 0, 0, err
		}
		return output, 0, 0, nil
	}
}

func (pm *PMAgent) directReply(ctx context.Context, projectID ProjectID, userInput string, project *Project) (*ChatMessage, error) {
	var defaultMember *ProjectMember
	for i := range project.Members {
		if project.Members[i].Active && project.Members[i].Role != "pm" {
			defaultMember = &project.Members[i]
			break
		}
	}

	if defaultMember == nil {
		return pm.makeSystemMessage(projectID, "No available member agent in this project. Please add an agent as a project member first.")
	}

	output, err := pm.runner.RunAgent(ctx, defaultMember.ID, userInput)
	if err != nil {
		return nil, fmt.Errorf("agent %s failed: %w", defaultMember.Name, err)
	}

	msg := ChatMessage{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Role:      "agent",
		AgentID:   &defaultMember.ID,
		AgentName: &defaultMember.Name,
		Content:   output,
		Timestamp: time.Now(),
	}

	pm.registry.AddChatMessage(projectID, msg)
	return &msg, nil
}

func (pm *PMAgent) makeSystemMessage(projectID ProjectID, content string) (*ChatMessage, error) {
	msg := ChatMessage{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
	}

	pm.registry.AddChatMessage(projectID, msg)
	return &msg, nil
}

func (pm *PMAgent) checkAutoGenRateLimit(projectID ProjectID) string {
	pm.autoGenMu.Lock()
	defer pm.autoGenMu.Unlock()

	today := time.Now().Format("2006-01-02")
	key := projectID.String()

	counter, ok := pm.autoGenUsage[key]
	if !ok || counter.Date != today {
		return ""
	}

	if counter.Count >= autoGenDailyLimit {
		return fmt.Sprintf("This project has used %d/%d auto-generated workflows today. Consider using bound workflows or templates for repeated tasks.", counter.Count, autoGenDailyLimit)
	}

	if counter.Count >= autoGenDailyLimit/2 {
		return fmt.Sprintf("This project has used %d/%d auto-generated workflows today.", counter.Count, autoGenDailyLimit)
	}

	return ""
}

func (pm *PMAgent) recordAutoGenUsage(projectID ProjectID) {
	pm.autoGenMu.Lock()
	defer pm.autoGenMu.Unlock()

	today := time.Now().Format("2006-01-02")
	key := projectID.String()

	counter, ok := pm.autoGenUsage[key]
	if !ok || counter.Date != today {
		pm.autoGenUsage[key] = &autoGenCounter{Date: today, Count: 1}
		return
	}

	counter.Count++
}

func (pm *PMAgent) matchKeywords(input string, keywords []string) bool {
	inputLower := strings.ToLower(input)
	for _, kw := range keywords {
		if strings.Contains(inputLower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func (pm *PMAgent) shouldAutoGenerateWorkflow(input string, project *Project) bool {
	trimmed := strings.TrimSpace(input)
	if len(trimmed) < 10 {
		return false
	}

	activeCount := 0
	for _, m := range project.Members {
		if m.Active && m.Role != "pm" {
			activeCount++
		}
	}
	if activeCount < 2 {
		return false
	}

	if pm.isSimpleQuestion(trimmed) {
		return false
	}

	return pm.isMultiStepTask(trimmed)
}

func (pm *PMAgent) isSimpleQuestion(input string) bool {
	lower := strings.ToLower(input)
	simplePatterns := []string{
		"什么是", "解释", "翻译", "告诉我", "说明", "定义", "计算", "换算",
		"查询", "搜索", "查找", "what is", "explain", "translate", "tell me",
		"define", "calculate", "convert", "search", "find",
	}
	for _, p := range simplePatterns {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}

	if len([]rune(input)) < 15 {
		return true
	}

	return false
}

func (pm *PMAgent) isMultiStepTask(input string) bool {
	lower := strings.ToLower(input)

	multiStepKeywords := []string{
		"然后", "之后", "接着", "并且", "同时", "分别", "最后", "再", "对比",
		"以及", "完成后", "基础上", "and then", "after that", "next",
		"finally", "additionally", "meanwhile", "respectively", "compare",
	}
	for _, kw := range multiStepKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	compoundPatterns := []string{
		"并写", "并生成", "并输出", "并制作", "并创建", "并给出", "并总结", "并分析", "对比并分析",
		"分析.*建议", "调研.*报告", "收集.*整理", "设计.*实现",
		"and write", "and generate", "and create", "and summarize", "compare and analyze",
	}
	for _, p := range compoundPatterns {
		matched, _ := regexp.MatchString(p, lower)
		if matched {
			return true
		}
	}

	return false
}

func (pm *PMAgent) matchTemplate(input string, project *Project) *TemplateInfo {
	templates := pm.workflowEngine.ListTemplates()
	if len(templates) == 0 {
		return nil
	}

	inputLower := strings.ToLower(input)

	projectRoles := make(map[string]bool)
	for _, member := range project.Members {
		if member.Active && member.Role != "pm" {
			projectRoles[strings.ToLower(member.Role)] = true
		}
	}

	var bestMatch *TemplateInfo
	bestScore := 0

	for i := range templates {
		t := &templates[i]
		matchedKeywords := 0
		for _, kw := range t.TriggerKeywords {
			if strings.Contains(inputLower, strings.ToLower(kw)) {
				matchedKeywords++
			}
		}

		if matchedKeywords == 0 {
			continue
		}

		rolesSatisfied := true
		for _, requiredRole := range t.RequiredRoles {
			if !projectRoles[strings.ToLower(requiredRole)] {
				rolesSatisfied = false
				break
			}
		}

		if !rolesSatisfied {
			continue
		}

		if matchedKeywords > bestScore {
			bestScore = matchedKeywords
			bestMatch = t
		}
	}

	return bestMatch
}

type GeneratedWorkflow struct {
	Workflow     types.Workflow `json:"workflow"`
	WorkflowJSON string         `json:"workflow_json"`
	Summary      string         `json:"summary"`
	Keywords     []string       `json:"keywords,omitempty"`
}

func (pm *PMAgent) GenerateWorkflowByLLM(ctx context.Context, projectID ProjectID, userDescription string) (*GeneratedWorkflow, error) {
	project := pm.registry.Get(projectID)
	if project == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	var agentNames []string
	for _, m := range project.Members {
		if m.Active && m.Role != "pm" {
			agentNames = append(agentNames, m.Name)
		}
	}

	if len(agentNames) == 0 {
		return nil, fmt.Errorf("no active non-PM members in this project")
	}

	prompt := fmt.Sprintf(`Design a workflow. Decide if each step runs sequentially or in parallel (fan_out).
- Use fan_out when 2+ steps can run simultaneously (no dependency between them)
- fan_out steps MUST be consecutive (at least 2 in a row)

Available agents:
%s

User requirement: %s

Output format:
1. Workflow name (on a line by itself)
2. Workflow description (on a line by itself)
3. Steps: one per line
   - Sequential: "Step[number]. [description] - [agent name]"
   - Parallel:   "Step[number](fan_out). [description] - [agent name]"
4. Trigger keywords: a comma-separated list of 3-5 short keywords or phrases that would match similar future requests (on a line by itself)

Examples:
  Step1. Collect data - Data Analyst
  Step2(fan_out). Technical analysis - Data Analyst
  Step3(fan_out). Business analysis - Data Analyst
  Step4. Synthesize report - Writer
  Keywords: analyze, summarize, report, data analysis

IMPORTANT:
- The agent name MUST be exactly one of the available agents from the list above.
- Trigger keywords should be concise words or short phrases that capture the intent of this workflow.`, strings.Join(agentNames, "\n"), userDescription)

	var llmMember *ProjectMember
	for i := range project.Members {
		if project.Members[i].Active && project.Members[i].Role != "pm" {
			llmMember = &project.Members[i]
			break
		}
	}

	if llmMember == nil {
		return nil, fmt.Errorf("no available member agent to execute LLM call")
	}

	output, err := pm.runner.RunAgent(ctx, llmMember.ID, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call via agent %s failed: %w", llmMember.Name, err)
	}

	log.Debug().Str("output", output).Msg("LLM output for workflow generation")
	workflow := pm.parseWorkflowFromLLMOutput(output, agentNames)
	if workflow == nil {
		return nil, fmt.Errorf("failed to generate workflow from LLM output. Raw output: %s", output)
	}

	summary := fmt.Sprintf("Workflow: %s\nDescription: %s\nSteps:\n", workflow.Name, workflow.Description)
	for i, step := range workflow.Steps {
		agentName := "unknown"
		if step.Agent.Name != nil {
			agentName = *step.Agent.Name
		}
		summary += fmt.Sprintf("  %d. %s (agent: %s)\n", i+1, step.Name, agentName)
	}

	workflowJSON, _ := json.MarshalIndent(workflow, "", "  ")

	keywords := parseKeywordsFromLLMOutput(output)
	if len(keywords) == 0 {
		keywords = extractKeywordsFromInput(userDescription)
	}

	return &GeneratedWorkflow{
		Workflow:     *workflow,
		WorkflowJSON: string(workflowJSON),
		Summary:      summary,
		Keywords:     keywords,
	}, nil
}

func (pm *PMAgent) parseWorkflowFromLLMOutput(output string, validAgentNames []string) *types.Workflow {
	workflow := pm.buildWorkflowFromText(output, validAgentNames)
	if workflow != nil && len(workflow.Steps) > 0 {
		return workflow
	}

	jsonStr := extractJSON(output)
	if jsonStr != "" {
		var workflow types.Workflow
		if err := json.Unmarshal([]byte(jsonStr), &workflow); err == nil {
			pm.normalizeWorkflow(&workflow, validAgentNames)
			if len(workflow.Steps) > 0 {
				return &workflow
			}
		}
	}

	return workflow
}

func (pm *PMAgent) normalizeWorkflow(workflow *types.Workflow, validAgentNames []string) {
	if workflow.Name == "" {
		workflow.Name = "Generated Workflow"
	}
	for i := range workflow.Steps {
		if workflow.Steps[i].Name == "" {
			workflow.Steps[i].Name = fmt.Sprintf("step-%d", i+1)
		}
		if workflow.Steps[i].PromptTemplate == "" {
			workflow.Steps[i].PromptTemplate = "{{input}}"
		}
		if workflow.Steps[i].Mode.Type == "" {
			workflow.Steps[i].Mode.Type = "sequential"
		}
		if workflow.Steps[i].TimeoutSecs == 0 {
			workflow.Steps[i].TimeoutSecs = 120
		}
		if workflow.Steps[i].ErrorMode.Type == "" {
			workflow.Steps[i].ErrorMode.Type = "fail"
		}
		if workflow.Steps[i].Agent.Name != nil {
			matched := pm.matchAgentName(*workflow.Steps[i].Agent.Name, validAgentNames)
			if matched != "" {
				workflow.Steps[i].Agent.Name = &matched
			}
		}
	}
}

func (pm *PMAgent) matchAgentName(input string, validNames []string) string {
	inputLower := strings.ToLower(input)
	for _, name := range validNames {
		if strings.EqualFold(name, input) {
			return name
		}
	}
	for _, name := range validNames {
		if strings.Contains(strings.ToLower(name), inputLower) || strings.Contains(inputLower, strings.ToLower(name)) {
			return name
		}
	}
	return ""
}

func parseKeywordsFromLLMOutput(output string) []string {
	lines := strings.Split(output, "\n")
	kwHeaderRegex := regexp.MustCompile(`(?i)^(?:#{1,4}\s*|\d+\.\s*)?(trigger\s*)?keywords?\s*:?\s*$`)
	kwInlineRegex := regexp.MustCompile(`(?i)^(?:#{1,4}\s*|\d+\.\s*)?(trigger\s*)?keywords?\s*:\s*(.+)$`)
	kwPlainRegex := regexp.MustCompile(`(?i)^(trigger\s*)?keywords?\s*:\s*(.+)$`)

	inKeywordsSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if kwHeaderRegex.MatchString(trimmed) {
			inKeywordsSection = true
			continue
		}

		if matches := kwInlineRegex.FindStringSubmatch(trimmed); len(matches) >= 3 {
			return parseKeywordList(matches[len(matches)-1])
		}

		if matches := kwPlainRegex.FindStringSubmatch(trimmed); len(matches) >= 3 {
			return parseKeywordList(matches[2])
		}

		if inKeywordsSection {
			stripped := regexp.MustCompile(`^#{1,4}\s*|^\d+\.\s*`).ReplaceAllString(trimmed, "")
			stripped = strings.Trim(stripped, "- ")
			if stripped != "" {
				return parseKeywordList(stripped)
			}
		}
	}

	return nil
}

func parseKeywordList(raw string) []string {
	var keywords []string
	for _, kw := range strings.Split(raw, ",") {
		kw = strings.TrimSpace(kw)
		kw = strings.Trim(kw, "\"'`")
		if kw != "" {
			keywords = append(keywords, kw)
		}
	}
	return keywords
}

func extractKeywordsFromInput(input string) []string {
	stopWords := map[string]bool{
		"的": true, "了": true, "在": true, "是": true, "和": true, "与": true, "及": true,
		"对": true, "比": true, "把": true, "被": true, "让": true, "给": true,
		"the": true, "a": true, "an": true, "is": true, "are": true, "was": true,
		"and": true, "or": true, "of": true, "in": true, "to": true, "for": true,
		"with": true, "on": true, "at": true, "by": true, "from": true, "it": true,
		"this": true, "that": true, "its": true, "how": true, "what": true,
	}

	segments := regexp.MustCompile(`[,，、；;：:\s]+`).Split(input, -1)
	var keywords []string
	seen := make(map[string]bool)
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		seg = strings.Trim(seg, "\"'`()（）[]【】")
		if len(seg) < 2 || len(seg) > 20 {
			continue
		}
		lower := strings.ToLower(seg)
		if stopWords[lower] {
			continue
		}
		if !seen[lower] {
			seen[lower] = true
			keywords = append(keywords, seg)
		}
		if len(keywords) >= 5 {
			break
		}
	}
	log.Debug().Strs("Extract Keywords", keywords).Msg("[HandleUserInput]Tier 2: extracted Keywords From Input")
	return keywords
}

func (pm *PMAgent) buildWorkflowFromText(output string, validAgentNames []string) *types.Workflow {
	if len(validAgentNames) == 0 {
		return nil
	}

	lines := strings.Split(output, "\n")
	var nonEmptyLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	if len(nonEmptyLines) == 0 {
		return nil
	}

	var (
		workflowName string
		description  string
		parsedSteps  []*parsedStep
	)

	stripNumPrefix := func(s string) string {
		re := regexp.MustCompile(`^\d+\.\s+`)
		return strings.TrimSpace(re.ReplaceAllString(s, ""))
	}

	sectionRegex := regexp.MustCompile(`(?i)^\d+\.\s*(workflow\s*name|name|description|steps?)\s*:?\s*$`)
	nameRegex := regexp.MustCompile(`(?i)^\d+\.\s*workflow\s*name\s*:?\s*$`)
	descRegex := regexp.MustCompile(`(?i)^\d+\.\s*(workflow\s*)?description\s*:?\s*$`)
	stepsRegex := regexp.MustCompile(`(?i)^\d+\.\s*steps?\s*:?\s*$`)

	inNameSection := false
	inDescSection := false
	inStepsSection := false

	for _, line := range nonEmptyLines {
		if nameRegex.MatchString(line) {
			inNameSection = true
			inDescSection = false
			inStepsSection = false
			continue
		}
		if descRegex.MatchString(line) {
			inNameSection = false
			inDescSection = true
			inStepsSection = false
			continue
		}
		if stepsRegex.MatchString(line) {
			inNameSection = false
			inDescSection = false
			inStepsSection = true
			continue
		}

		if sectionRegex.MatchString(line) {
			continue
		}

		if inNameSection && workflowName == "" {
			workflowName = stripNumPrefix(line)
			continue
		}
		if inDescSection {
			if description == "" {
				description = stripNumPrefix(line)
			} else {
				description = description + " " + stripNumPrefix(line)
			}
			continue
		}
		if inStepsSection {
			step := pm.parseStepFromLine(line, validAgentNames, len(parsedSteps))
			if step != nil {
				parsedSteps = append(parsedSteps, step)
			}
			continue
		}

		if workflowName == "" {
			workflowName = stripNumPrefix(line)
		} else if description == "" {
			description = stripNumPrefix(line)
		} else {
			step := pm.parseStepFromLine(line, validAgentNames, len(parsedSteps))
			if step != nil {
				parsedSteps = append(parsedSteps, step)
			}
		}
	}

	if workflowName == "" {
		workflowName = "Generated Workflow"
	}
	if description == "" {
		description = "Auto-generated workflow from AI description"
	}

	var steps []types.WorkflowStep
	for i, ps := range parsedSteps {
		var promptTmpl string
		var modeType string

		if ps.IsFanOut {
			modeType = "fan_out"
			promptTmpl = fmt.Sprintf("Please %s: {{input}}", ps.Description)
		} else {
			modeType = "sequential"
			if i == 0 {
				promptTmpl = "{{input}}"
			} else {
				prev := parsedSteps[i-1]
				if prev.IsFanOut {
					fanOutStepNames := []string{}
					j := i - 1
					for j >= 0 && parsedSteps[j].IsFanOut {
						fanOutStepNames = append([]string{parsedSteps[j].Name}, fanOutStepNames...)
						j--
					}
					var refs []string
					for _, sn := range fanOutStepNames {
						refs = append(refs, fmt.Sprintf("{{steps.%s.output}}", sn))
					}
					promptTmpl = fmt.Sprintf("Based on the previous results:\n%s\n\nPlease %s", strings.Join(refs, "\n"), ps.Description)
				} else {
					promptTmpl = fmt.Sprintf("Based on the previous input: {{input}}\n\nPlease %s", ps.Description)
				}
			}
		}

		steps = append(steps, types.WorkflowStep{
			Name:           ps.Name,
			Agent:          types.StepAgent{Name: &ps.AgentName},
			PromptTemplate: promptTmpl,
			Mode:           types.StepMode{Type: modeType},
			TimeoutSecs:    120,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		})
	}

	if len(steps) == 0 {
		firstAgent := validAgentNames[0]
		steps = append(steps, types.WorkflowStep{
			Name:           "step-1",
			Agent:          types.StepAgent{Name: &firstAgent},
			PromptTemplate: "{{input}}",
			Mode:           types.StepMode{Type: "sequential"},
			TimeoutSecs:    120,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		})
	}

	workflow := &types.Workflow{
		Name:        workflowName,
		Description: description,
		Steps:       steps,
	}
	return workflow
}

type parsedStep struct {
	Name        string
	AgentName   string
	Description string
	IsFanOut    bool
}

func (pm *PMAgent) parseStepFromLine(line string, validAgentNames []string, stepIndex int) *parsedStep {
	var agentName string

	isFanOut := strings.Contains(line, "(fan_out)")

	lineLower := strings.ToLower(line)

	var bestMatch struct {
		name  string
		index int
	}
	bestMatch.index = -1

	for _, name := range validAgentNames {
		nameLower := strings.ToLower(name)
		if idx := strings.LastIndex(lineLower, nameLower); idx != -1 {
			if idx > bestMatch.index {
				bestMatch.index = idx
				bestMatch.name = name
			}
		}
	}

	if bestMatch.name == "" {
		return nil
	}

	agentName = bestMatch.name

	agentEndIndex := bestMatch.index + len(agentName)
	line = strings.TrimSpace(line[:agentEndIndex])

	var description string
	stepPrefixRegex := regexp.MustCompile(`^(?i)Step\s*\d+\s*(?:\(fan_out\))?\s*[.\s]+`)
	matches := stepPrefixRegex.FindStringIndex(line)
	if matches != nil {
		description = strings.TrimSpace(line[matches[1]:])
	} else {
		numStepRegex := regexp.MustCompile(`^(\d+[.\d]*)\s+`)
		numMatches := numStepRegex.FindStringIndex(line)
		if numMatches != nil {
			description = strings.TrimSpace(line[numMatches[1]:])
		} else {
			description = strings.TrimSpace(line)
		}
	}

	descLower := strings.ToLower(description)
	agentNameLower := strings.ToLower(agentName)
	sepIdx := strings.LastIndex(descLower, agentNameLower)

	if sepIdx != -1 {
		description = strings.TrimSpace(description[:sepIdx])
		description = strings.TrimRight(description, "-:–")
		description = strings.TrimSpace(description)
	}

	stepName := fmt.Sprintf("step-%d", stepIndex+1)

	return &parsedStep{
		Name:        stepName,
		AgentName:   agentName,
		Description: description,
		IsFanOut:    isFanOut,
	}
}

func inferRoleFromName(name string) string {
	nameLower := strings.ToLower(name)
	roleMap := map[string]string{
		"research":    "researcher",
		"search":      "researcher",
		"analyst":     "analyst",
		"analyz":      "analyst",
		"writer":      "writer",
		"author":      "writer",
		"coder":       "coder",
		"developer":   "coder",
		"programmer":  "coder",
		"code-review": "code-reviewer",
		"reviewer":    "code-reviewer",
		"designer":    "designer",
		"translator":  "translator",
	}
	for key, role := range roleMap {
		if strings.Contains(nameLower, key) {
			return role
		}
	}
	return "agent"
}

func extractJSON(output string) string {
	trimmed := strings.TrimSpace(output)

	if strings.HasPrefix(trimmed, "```") {
		firstNewline := strings.Index(trimmed, "\n")
		if firstNewline != -1 {
			trimmed = trimmed[firstNewline+1:]
		}
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}

	start := strings.Index(trimmed, "{")
	if start == -1 {
		return ""
	}

	depth := 0
	for i := start; i < len(trimmed); i++ {
		switch trimmed[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return trimmed[start : i+1]
			}
		}
	}

	return trimmed[start:]
}
