package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/approvals"
	"github.com/penzhan8451/fangclaw-go/internal/audit"
	"github.com/penzhan8451/fangclaw-go/internal/channels"
	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/penzhan8451/fangclaw-go/internal/configreload"
	"github.com/penzhan8451/fangclaw-go/internal/cron"
	"github.com/penzhan8451/fangclaw-go/internal/delivery"
	"github.com/penzhan8451/fangclaw-go/internal/embedding"
	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
	"github.com/penzhan8451/fangclaw-go/internal/hands"
	"github.com/penzhan8451/fangclaw-go/internal/mcp"
	"github.com/penzhan8451/fangclaw-go/internal/memory"
	"github.com/penzhan8451/fangclaw-go/internal/pairing"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/agent"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/agent/tools"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/agent_templates"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/llm"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/model_catalog"
	"github.com/penzhan8451/fangclaw-go/internal/skills"
	"github.com/penzhan8451/fangclaw-go/internal/triggers"
	"github.com/penzhan8451/fangclaw-go/internal/types"
	"github.com/penzhan8451/fangclaw-go/internal/vector"
	"github.com/rs/zerolog/log"
)

type Kernel struct {
	config          types.KernelConfig
	eventBus        *eventbus.EventBus
	scheduler       *Scheduler
	cronScheduler   *cron.CronScheduler
	modelCatalog    *model_catalog.ModelCatalog
	agentTemplates  *agent_templates.AgentTemplates
	db              *memory.DB
	semantic        *memory.SemanticStore
	sessions        *memory.SessionStore
	knowledge       *memory.KnowledgeStore
	usage           *memory.UsageStore
	skillLoader     *skills.Loader
	embeddingDriver *embedding.EmbeddingDriver
	registry        *channels.Registry
	agentRegistry   *AgentRegistry
	handRegistry    *hands.Registry
	triggerEngine   *triggers.TriggerEngine
	approvalMgr     *approvals.ApprovalManager
	deliveryReg     *delivery.DeliveryRegistry
	pairingManager  *pairing.PairingManager
	workflowEngine  *WorkflowEngine
	agentRuntime    *agent.Runtime
	mcpConnections  sync.Map
	mcpTools        sync.Map
	auditLog        *audit.AuditLog
	mu              sync.RWMutex
	started         bool
	startTime       time.Time
	stopping        chan struct{}
}

func NewKernel(kernelConfig types.KernelConfig) (*Kernel, error) {
	dataDir, err := expandPath(kernelConfig.DataDir)
	if err != nil {
		return nil, fmt.Errorf("invalid data directory: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "fangclaw.db")

	db, err := memory.NewDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	semanticStore, err := memory.NewSemanticStore(dbPath)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create semantic store: %w", err)
	}

	sessionStore, err := memory.NewSessionStore(dbPath)
	if err != nil {
		semanticStore.Close()
		db.Close()
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}

	knowledgeStore := memory.NewKnowledgeStore(db)
	if err := knowledgeStore.Init(); err != nil {
		semanticStore.Close()
		sessionStore.Close()
		db.Close()
		return nil, fmt.Errorf("failed to initialize knowledge store: %w", err)
	}

	usageStore := memory.NewUsageStore(db)

	skillsPath := filepath.Join(dataDir, "skills")
	skillLoader, err := skills.NewLoader(skillsPath)
	if err != nil {
		semanticStore.Close()
		sessionStore.Close()
		db.Close()
		return nil, fmt.Errorf("failed to create skill loader: %w", err)
	}

	embeddingDriver := embedding.NewEmbeddingDriver()
	openAIEmbedder := vector.NewOpenAIEmbedder("", "")
	embeddingAdapter := embedding.NewVectorEmbedderAdapter(openAIEmbedder)
	embeddingDriver.Register("openai", embeddingAdapter)

	registry := channels.NewRegistry()
	agentRegistry := NewAgentRegistry(dataDir)
	agentRegistry.LoadFromDisk()
	handRegistry := hands.NewRegistry()
	triggerEngine := triggers.NewTriggerEngine()
	approvalPolicy := approvals.DefaultApprovalPolicy()
	approvalMgr := approvals.NewApprovalManager(approvalPolicy)
	deliveryReg := delivery.NewDeliveryRegistry()
	pairingConfig := pairing.PairingConfig{
		Enabled:    true,
		MaxDevices: 10,
	}
	pairingManager := pairing.NewPairingManager(pairingConfig)

	cronPersistDir := dataDir
	cronScheduler := cron.NewCronScheduler(cronPersistDir, 100)

	modelCatalogPath := filepath.Join(dataDir, "model_catalog.json")
	modelCatalog := model_catalog.NewModelCatalog(modelCatalogPath)
	agentTemplates := agent_templates.NewAgentTemplates()
	if err := agentTemplates.Load(); err != nil {
		fmt.Printf("[Kernel] Warning: Failed to load agent templates: %v\n", err)
	}
	fmt.Printf("[Kernel] Creating WorkflowEngine with dataDir: '%s'\n", dataDir)
	workflowEngine := NewWorkflowEngine(dataDir)
	fmt.Printf("[Kernel] WorkflowEngine created\n")

	k := &Kernel{
		config:          kernelConfig,
		eventBus:        eventbus.NewEventBus(),
		scheduler:       NewScheduler(),
		cronScheduler:   cronScheduler,
		modelCatalog:    modelCatalog,
		agentTemplates:  agentTemplates,
		db:              db,
		semantic:        semanticStore,
		sessions:        sessionStore,
		knowledge:       knowledgeStore,
		usage:           usageStore,
		skillLoader:     skillLoader,
		embeddingDriver: embeddingDriver,
		registry:        registry,
		agentRegistry:   agentRegistry,
		handRegistry:    handRegistry,
		triggerEngine:   triggerEngine,
		approvalMgr:     approvalMgr,
		deliveryReg:     deliveryReg,
		pairingManager:  pairingManager,
		workflowEngine:  workflowEngine,
		auditLog:        audit.NewAuditLog(),
		startTime:       time.Now(),
		stopping:        make(chan struct{}),
	}

	k.workflowEngine.SetChannelSender(func(channelName, recipient, message string) error {
		_, err := channels.SendMessageToChannelName(k.registry, channelName, recipient, message, nil)
		return err
	})

	mcpCallbacks := &agent.McpCallbacks{
		GetMcpTools: k.GetMcpTools,
		CallMcpTool: k.CallMcpTool,
	}

	agentRuntime := agent.NewRuntime(semanticStore, sessionStore, knowledgeStore, usageStore, skillLoader, embeddingDriver, modelCatalog, mcpCallbacks, approvalMgr)

	kernelConfig.DataDir = dataDir
	k.config = kernelConfig
	k.agentRuntime = agentRuntime

	// Register all built-in tools to AgentRuntime
	tools.RegisterAllTools(agentRuntime)

	// Agents loaded from disk also need to be registered in AgentRuntime
	agents := agentRegistry.List()
	if len(agents) > 0 {
		// Load config file
		cfg, err := config.Load("")
		if err != nil {
			cfg = config.DefaultConfig()
		}

		for _, agentEntry := range agents {
			// Declare variables
			var agentProvider, agentModel, agentAPIKeyEnv string

			// If any config is missing in agent entry (provider, model, or APIKeyEnv), use defaults from config.toml
			if agentEntry.Manifest.Model.Provider == "" || agentEntry.Manifest.Model.Model == "" || agentEntry.Manifest.Model.APIKeyEnv == "" {
				agentProvider = cfg.DefaultModel.Provider
				agentModel = cfg.DefaultModel.Model
				agentAPIKeyEnv = cfg.DefaultModel.APIKeyEnv
			} else {
				// Otherwise use config from agent entry
				agentProvider = agentEntry.Manifest.Model.Provider
				agentModel = agentEntry.Manifest.Model.Model
				agentAPIKeyEnv = agentEntry.Manifest.Model.APIKeyEnv
			}

			// Get API key
			apiKey := os.Getenv(agentAPIKeyEnv)
			if apiKey == "" {
				fmt.Printf("Warning: API key not set for agent %s: %s, skipping registration to runtime\n", agentEntry.Name, agentAPIKeyEnv)
				continue
			}

			// Create LLM driver
			driver, err := llm.NewDriver(agentProvider, apiKey, agentModel)
			if err != nil {
				fmt.Printf("Warning: Failed to create LLM driver for agent %s: %v, skipping registration to runtime\n", agentEntry.Name, err)
				continue
			}

			// Register driver to AgentRuntime
			agentRuntime.RegisterDriver(agentProvider, driver)

			// Register agent to AgentRuntime
			_, err = agentRuntime.RegisterAgent(
				context.Background(),
				agentEntry.ID.String(),
				agentEntry.Name,
				agentProvider,
				agentModel,
				agentEntry.Manifest.SystemPrompt,
				agentEntry.Manifest.Tools,
				agentEntry.Manifest.Skills,
				agentEntry.Manifest.SkillPromptContext,
			)
			if err != nil {
				fmt.Printf("Warning: Failed to register agent %s in runtime: %v, skipping\n", agentEntry.Name, err)
			} else {
				fmt.Printf("Registered agent from disk: %s (ID: %s)\n", agentEntry.Name, agentEntry.ID.String())
			}
		} //_for loop for agent in _agentRegistry

		// Restore hand instances from agents
		fmt.Printf("Restoring hand instances from %d agents...\n", len(agents))
		var agentEntriesForHands []hands.AgentEntry
		for _, agent := range agents {
			agentEntriesForHands = append(agentEntriesForHands, agent)
		}
		handRegistry.RestoreInstancesFromAgents(agentEntriesForHands)
		fmt.Printf("Restored %d hand instances\n", len(handRegistry.ListInstances()))
	}

	return k, nil
}

func (k *Kernel) Start(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.started {
		return fmt.Errorf("kernel already started")
	}

	if _, err := k.cronScheduler.Load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load cron jobs")
	}

	k.modelCatalog.DetectAuth()

	k.agentTemplates.StartWatching()

	k.started = true

	event := eventbus.NewEvent(eventbus.EventTypeSystem, "kernel", eventbus.EventTargetSystem)
	k.eventBus.Publish(event)

	// Connect to MCP servers in background
	go k.ConnectMcpServers(context.Background())

	// Start cron tick loop
	go func() {
		log.Info().Msg("Cron: tick loop starting")
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		persistCounter := 0

		log.Info().Msg("Cron: skipping first tick")
		// Skip first tick
		<-ticker.C
		log.Info().Msg("Cron: first tick skipped, starting main loop")

		for {
			select {
			case <-ticker.C:
				log.Info().Msg("Cron: tick received")
				select {
				case <-k.stopping:
					log.Info().Msg("Cron: stopping tick loop")
					return
				default:
				}

				due := k.cronScheduler.DueJobs()
				log.Info().Int("count", len(due)).Msg("Cron: checking for due jobs")
				for _, job := range due {
					jobID := job.ID
					jobName := job.Name

					log.Info().Str("job", jobName).Msg("Cron: firing job")

					switch job.Action.Kind {
					case types.CronActionKindSystemEvent:
						if job.Action.Text != nil {
							log.Info().Str("job", jobName).Msg("Cron: firing system event")
							payload := map[string]interface{}{
								"type":   fmt.Sprintf("cron.%s", jobName),
								"text":   *job.Action.Text,
								"job_id": jobID.String(),
							}
							event := eventbus.NewEvent(
								eventbus.EventTypeSystem,
								"cron",
								eventbus.EventTargetBroadcast,
							).WithPayload(payload)
							k.eventBus.Publish(event)
							k.cronScheduler.RecordSuccess(jobID)
						}
					case types.CronActionKindAgentTurn:
						log.Info().Str("job", jobName).Str("agent", job.AgentID.String()).Msg("Cron: firing agent turn")
						if job.Action.Message != nil {
							log.Info().Str("job", jobName).Str("message", *job.Action.Message).Msg("Cron: sending message to agent")
							timeoutSecs := uint64(120)
							if job.Action.TimeoutSecs != nil {
								timeoutSecs = *job.Action.TimeoutSecs
							}
							timeout := time.Duration(timeoutSecs) * time.Second
							ctxTimeout, cancel := context.WithTimeout(context.Background(), timeout)
							defer cancel()

							delivery := job.Delivery
							resultChan := make(chan struct {
								result string
								err    error
							}, 1)

							go func() {
								log.Info().Str("job", jobName).Msg("Cron: calling SendMessage")
								result, err := k.SendMessage(ctxTimeout, job.AgentID.String(), *job.Action.Message)
								log.Info().Str("job", jobName).Err(err).Msg("Cron: SendMessage returned")
								resultChan <- struct {
									result string
									err    error
								}{result, err}
							}()

							select {
							case <-ctxTimeout.Done():
								log.Warn().Str("job", jobName).Uint64("timeout_s", timeoutSecs).Msg("Cron job timed out")
								k.cronScheduler.RecordFailure(jobID, fmt.Sprintf("timed out after %ds", timeoutSecs))
							case res := <-resultChan:
								if res.err != nil {
									errMsg := res.err.Error()
									log.Warn().Str("job", jobName).Err(res.err).Msg("Cron job failed")
									k.cronScheduler.RecordFailure(jobID, errMsg)
								} else {
									log.Info().Str("job", jobName).Str("result", res.result).Msg("Cron job completed successfully")
									k.cronScheduler.RecordSuccess(jobID)
									k.cronDeliverResponse(job.AgentID, res.result, &delivery)
								}
							}
						}
					case types.CronActionKindExecuteShell:
						log.Info().Str("job", jobName).Msg("Cron: firing execute shell")
						if job.Action.Command != nil {
							command := *job.Action.Command
							args := job.Action.Args
							log.Info().Str("job", jobName).Str("command", command).Strs("args", args).Msg("Cron: executing shell command")

							if err := ValidateShellCommand(command, args, k.config.CronShellSecurity); err != nil {
								errMsg := fmt.Sprintf("security validation failed: %v", err)
								log.Warn().Str("job", jobName).Err(err).Msg("Cron job blocked by security")
								k.cronScheduler.RecordFailure(jobID, errMsg)
								continue
							}

							timeoutSecs := uint64(60)
							if job.Action.TimeoutSecs != nil {
								timeoutSecs = *job.Action.TimeoutSecs
							}
							timeout := time.Duration(timeoutSecs) * time.Second
							ctxTimeout, cancel := context.WithTimeout(context.Background(), timeout)
							defer cancel()

							delivery := job.Delivery
							resultChan := make(chan struct {
								stdout string
								stderr string
								err    error
							}, 1)

							go func() {
								cmd := exec.CommandContext(ctxTimeout, command, args...)
								var stdoutBuf, stderrBuf bytes.Buffer
								cmd.Stdout = &stdoutBuf
								cmd.Stderr = &stderrBuf

								log.Info().Str("job", jobName).Msg("Cron: starting command execution")
								err := cmd.Run()
								stdout := stdoutBuf.String()
								stderr := stderrBuf.String()

								log.Info().Str("job", jobName).Str("stdout", stdout).Str("stderr", stderr).Err(err).Msg("Cron: command execution finished")
								resultChan <- struct {
									stdout string
									stderr string
									err    error
								}{stdout, stderr, err}
							}()

							select {
							case <-ctxTimeout.Done():
								log.Warn().Str("job", jobName).Uint64("timeout_s", timeoutSecs).Msg("Cron job timed out")
								k.cronScheduler.RecordFailure(jobID, fmt.Sprintf("timed out after %ds", timeoutSecs))
							case res := <-resultChan:
								if res.err != nil {
									errMsg := fmt.Sprintf("command failed: %v, stderr: %s", res.err, res.stderr)
									log.Warn().Str("job", jobName).Err(res.err).Str("stderr", res.stderr).Msg("Cron job failed")
									k.cronScheduler.RecordFailure(jobID, errMsg)
									var fullResult string
									if res.stdout != "" && res.stderr != "" {
										fullResult = fmt.Sprintf("❌ 命令执行失败\n\n📝 命令: %s %s\n\n📤 输出:\n%s\n\n📥 错误:\n%s\n\n⚠️  错误信息: %v", command, strings.Join(args, " "), res.stdout, res.stderr, res.err)
									} else if res.stdout != "" {
										fullResult = fmt.Sprintf("❌ 命令执行失败\n\n📝 命令: %s %s\n\n📤 输出:\n%s\n\n⚠️  错误信息: %v", command, strings.Join(args, " "), res.stdout, res.err)
									} else if res.stderr != "" {
										fullResult = fmt.Sprintf("❌ 命令执行失败\n\n📝 命令: %s %s\n\n📥 错误:\n%s\n\n⚠️  错误信息: %v", command, strings.Join(args, " "), res.stderr, res.err)
									} else {
										fullResult = fmt.Sprintf("❌ 命令执行失败\n\n📝 命令: %s %s\n\n⚠️  错误信息: %v", command, strings.Join(args, " "), res.err)
									}
									k.cronDeliverResponse(job.AgentID, fullResult, &delivery)
								} else {
									log.Info().Str("job", jobName).Str("stdout", res.stdout).Msg("Cron job completed successfully")
									k.cronScheduler.RecordSuccess(jobID)
									var fullResult string
									if res.stdout != "" && res.stderr != "" {
										fullResult = fmt.Sprintf("✅ 命令执行成功\n\n📝 命令: %s %s\n\n📤 输出:\n%s\n\n📥 警告:\n%s", command, strings.Join(args, " "), res.stdout, res.stderr)
									} else if res.stdout != "" {
										fullResult = fmt.Sprintf("✅ 命令执行成功\n\n📝 命令: %s %s\n\n📤 输出:\n%s", command, strings.Join(args, " "), res.stdout)
									} else if res.stderr != "" {
										fullResult = fmt.Sprintf("✅ 命令执行成功\n\n📝 命令: %s %s\n\n📥 警告:\n%s", command, strings.Join(args, " "), res.stderr)
									} else {
										fullResult = fmt.Sprintf("✅ 命令执行成功\n\n📝 命令: %s %s\n\n(无输出)", command, strings.Join(args, " "))
									}
									k.cronDeliverResponse(job.AgentID, fullResult, &delivery)
								}
							}
						}
					}
				}

				// Persist every ~5 minutes (20 ticks * 15s)
				persistCounter++
				if persistCounter >= 20 {
					persistCounter = 0
					if err := k.cronScheduler.Persist(); err != nil {
						log.Warn().Err(err).Msg("Cron persist failed")
					}
				}
			case <-k.stopping:
				return
			}
		}
	}()

	if k.cronScheduler.TotalJobs() > 0 {
		log.Info().Int("count", k.cronScheduler.TotalJobs()).Msg("Cron scheduler active")
	}

	k.cronScheduler.StartHotReload()

	return nil
}

func (k *Kernel) Stop(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.started {
		return nil
	}

	// Stop hot reload first
	k.cronScheduler.StopHotReload()
	k.agentTemplates.StopWatching()

	// Signal stopping
	close(k.stopping)

	// Persist cron jobs on shutdown
	if err := k.cronScheduler.Persist(); err != nil {
		log.Warn().Err(err).Msg("Failed to persist cron jobs on shutdown")
	}

	k.scheduler.Shutdown()
	_ = k.registry.DisconnectAll()

	// Close MCP connections
	k.CloseMcpConnections()

	k.semantic.Close()
	k.sessions.Close()
	k.db.Close()
	k.started = false

	event := eventbus.NewEvent(eventbus.EventTypeSystem, "kernel", eventbus.EventTargetSystem)
	k.eventBus.Publish(event)

	return nil
}

func (k *Kernel) Config() types.KernelConfig {
	return k.config
}

func (k *Kernel) EventBus() *eventbus.EventBus {
	return k.eventBus
}

func (k *Kernel) Scheduler() *Scheduler {
	return k.scheduler
}

func (k *Kernel) DB() *memory.DB {
	return k.db
}

func (k *Kernel) SemanticStore() *memory.SemanticStore {
	return k.semantic
}

func (k *Kernel) SessionStore() *memory.SessionStore {
	return k.sessions
}

func (k *Kernel) KnowledgeStore() *memory.KnowledgeStore {
	return k.knowledge
}

func (k *Kernel) UsageStore() *memory.UsageStore {
	return k.usage
}

func (k *Kernel) GetUptime() time.Duration {
	return time.Since(k.startTime)
}

func (k *Kernel) SkillLoader() *skills.Loader {
	return k.skillLoader
}

// channel registry
func (k *Kernel) Registry() *channels.Registry {
	return k.registry
}

func (k *Kernel) AgentRegistry() *AgentRegistry {
	return k.agentRegistry
}

func (k *Kernel) HandRegistry() *hands.Registry {
	return k.handRegistry
}

func (k *Kernel) TriggerEngine() *triggers.TriggerEngine {
	return k.triggerEngine
}

func (k *Kernel) ApprovalManager() *approvals.ApprovalManager {
	return k.approvalMgr
}

func (k *Kernel) DeliveryRegistry() *delivery.DeliveryRegistry {
	return k.deliveryReg
}

func (k *Kernel) PairingManager() *pairing.PairingManager {
	return k.pairingManager
}

func (k *Kernel) CronScheduler() *cron.CronScheduler {
	return k.cronScheduler
}

func (k *Kernel) AuditLog() *audit.AuditLog {
	return k.auditLog
}

func (k *Kernel) ModelCatalog() *model_catalog.ModelCatalog {
	return k.modelCatalog
}

func (k *Kernel) AgentTemplates() *agent_templates.AgentTemplates {
	return k.agentTemplates
}

func (k *Kernel) WorkflowEngine() *WorkflowEngine {
	return k.workflowEngine
}

// ExecuteWorkflow executes a workflow by ID with the given input.
func (k *Kernel) ExecuteWorkflow(ctx context.Context, workflowID types.WorkflowID, input string) (string, error) {
	runID := k.workflowEngine.CreateRun(workflowID, input)
	if runID == nil {
		return "", fmt.Errorf("workflow not found: %s", workflowID)
	}

	resolver := func(agent types.StepAgent) (string, string, bool) {
		if agent.Name != nil {
			agentID, ok := k.FindAgentByName(ctx, *agent.Name)
			if ok {
				return agentID, *agent.Name, true
			}
		}
		if agent.ID != nil {
			return *agent.ID, "", true
		}
		return "", "", false
	}

	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		return k.SendMessageWithUsage(ctx, agentID, prompt)
	}

	return k.workflowEngine.ExecuteRun(*runID, resolver, sender)
}

// RegisterWorkflow registers a new workflow with the kernel.
func (k *Kernel) RegisterWorkflow(workflow types.Workflow) types.WorkflowID {
	return k.workflowEngine.Register(workflow)
}

// GetWorkflow gets a workflow by ID.
func (k *Kernel) GetWorkflow(id types.WorkflowID) *types.Workflow {
	return k.workflowEngine.GetWorkflow(id)
}

// ListWorkflows lists all registered workflows.
func (k *Kernel) ListWorkflows() []types.Workflow {
	return k.workflowEngine.ListWorkflows()
}

// RemoveWorkflow removes a workflow by ID.
func (k *Kernel) RemoveWorkflow(id types.WorkflowID) bool {
	return k.workflowEngine.RemoveWorkflow(id)
}

// GetWorkflowRun gets a workflow run by ID.
func (k *Kernel) GetWorkflowRun(id types.WorkflowRunID) *types.WorkflowRun {
	return k.workflowEngine.GetRun(id)
}

// ListWorkflowRuns lists all workflow runs (optionally filtered by state and/or workflow ID).
func (k *Kernel) ListWorkflowRuns(stateFilter *string, workflowID *types.WorkflowID) []types.WorkflowRun {
	return k.workflowEngine.ListRuns(stateFilter, workflowID)
}

// ListWorkflowTemplates lists all available workflow templates.
func (k *Kernel) ListWorkflowTemplates() []types.WorkflowTemplate {
	return k.workflowEngine.ListTemplates()
}

// GetWorkflowTemplate gets a workflow template by ID.
func (k *Kernel) GetWorkflowTemplate(id types.WorkflowTemplateID) *types.WorkflowTemplate {
	return k.workflowEngine.GetTemplate(id)
}

// CreateWorkflowFromTemplate creates a new workflow from a template.
func (k *Kernel) CreateWorkflowFromTemplate(templateID types.WorkflowTemplateID, customName, customDescription string) (*types.Workflow, error) {
	return k.workflowEngine.CreateFromTemplate(templateID, customName, customDescription)
}

// ExecuteWorkflowWithDelivery executes a workflow and delivers the result.
func (k *Kernel) ExecuteWorkflowWithDelivery(ctx context.Context, workflowID types.WorkflowID, input string, delivery *types.DeliveryConfig) (string, error) {
	runID := k.workflowEngine.CreateRun(workflowID, input)
	if runID == nil {
		return "", fmt.Errorf("workflow not found: %s", workflowID)
	}

	resolver := func(agent types.StepAgent) (string, string, bool) {
		if agent.Name != nil {
			agentID, ok := k.FindAgentByName(ctx, *agent.Name)
			if ok {
				return agentID, *agent.Name, true
			}
		}
		if agent.ID != nil {
			return *agent.ID, "", true
		}
		return "", "", false
	}

	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		return k.SendMessageWithUsage(ctx, agentID, prompt)
	}

	output, err := k.workflowEngine.ExecuteRun(*runID, resolver, sender)
	if err != nil {
		return "", err
	}

	if delivery != nil {
		if deliveryErr := k.workflowEngine.DeliverResult(workflowID, output, delivery); deliveryErr != nil {
			log.Warn().Err(deliveryErr).Str("workflow_id", string(workflowID)).Msg("Failed to deliver workflow result")
		}
	}

	return output, nil
}

func (k *Kernel) ReloadConfig(newConfig types.KernelConfig) *configreload.ReloadPlan {
	k.mu.Lock()
	defer k.mu.Unlock()

	plan := configreload.BuildReloadPlan(k.config, newConfig)

	if !plan.RestartRequired {
		k.config = newConfig
	}

	return plan
}

// Boot boots the kernel with default configuration.
func Boot(dataDir string) (*Kernel, error) {
	if dataDir == "" {
		dataDir = "~/.fangclaw-go"
	}

	cfg, err := config.Load("")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	config := types.KernelConfig{
		DataDir:    dataDir,
		McpServers: cfg.McpServers,
	}

	return NewKernel(config)
}

// ActivateHand activates a hand and spawns an agent.
func (k *Kernel) ActivateHand(handID string, handConfig map[string]interface{}) (*hands.HandInstance, error) {
	k.mu.Lock()

	def, ok := k.handRegistry.GetDefinition(handID)
	if !ok {
		k.mu.Unlock()
		return nil, fmt.Errorf("hand not found: %s", handID)
	}

	instance, err := k.handRegistry.ActivateHand(handID, def.Agent.Name, handConfig)
	if err != nil {
		k.mu.Unlock()
		return nil, err
	}

	agentID := types.NewAgentID()

	// Load config file to get default model configuration
	cfg, err := config.Load("")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Declare variables
	var agentProvider, agentModel, agentAPIKeyEnv string

	// If any config is missing in hand definition (provider, model, or APIKeyEnv), use defaults from config.toml
	if def.Agent.Provider == "" || def.Agent.Model == "" || def.Agent.APIKeyEnv == "" {
		agentProvider = cfg.DefaultModel.Provider
		agentModel = cfg.DefaultModel.Model
		agentAPIKeyEnv = cfg.DefaultModel.APIKeyEnv
	} else {
		// Otherwise use config from hand definition
		agentProvider = def.Agent.Provider
		agentModel = def.Agent.Model
		agentAPIKeyEnv = def.Agent.APIKeyEnv
	}

	manifest := types.AgentManifest{
		Name:         def.Agent.Name,
		Description:  def.Agent.Description,
		SystemPrompt: def.Agent.SystemPrompt,
		Model: types.ModelConfig{
			Provider:  agentProvider,
			Model:     agentModel,
			APIKeyEnv: agentAPIKeyEnv,
		},
		Tools: def.Tools,
	}

	entry := &AgentEntry{
		ID:         agentID,
		Name:       def.Agent.Name,
		State:      types.AgentStateRunning,
		Mode:       "auto",
		Tags:       []string{"hand:" + handID, "hand_instance:" + instance.InstanceID},
		Manifest:   manifest,
		CreatedAt:  instance.ActivatedAt,
		LastActive: instance.ActivatedAt,
	}

	if err := k.agentRegistry.Register(entry); err != nil {
		k.mu.Unlock()
		return nil, err
	}

	if err := k.handRegistry.UpdateInstanceAgent(instance.InstanceID, agentID.String()); err != nil {
		k.mu.Unlock()
		return nil, err
	}

	agentName := def.Agent.Name
	agentSystemPrompt := def.Agent.SystemPrompt
	agentTools := def.Tools
	agentSkillPromptContext := ""

	if hand, _ := hands.GetBundledHand(handID); hand != nil {
		agentSkillPromptContext = hand.SkillContent
	}

	k.mu.Unlock()

	apiKey := os.Getenv(agentAPIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("API key not set for agent %s: %s", agentName, agentAPIKeyEnv)
	}

	driver, err := llm.NewDriver(agentProvider, apiKey, agentModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM driver for %s: %w", agentProvider, err)
	}

	k.agentRuntime.RegisterDriver(agentProvider, driver)

	_, err = k.agentRuntime.RegisterAgent(
		context.Background(),
		agentID.String(),
		agentName,
		agentProvider,
		agentModel,
		agentSystemPrompt,
		agentTools,
		[]string{},
		agentSkillPromptContext,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register agent in runtime: %w", err)
	}

	k.mu.Lock()
	updatedInstance, _ := k.handRegistry.GetInstance(instance.InstanceID)
	k.mu.Unlock()

	return updatedInstance, nil
}

// DeactivateHand deactivates a hand and kills the agent.
func (k *Kernel) DeactivateHand(instanceID string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	instance, ok := k.handRegistry.GetInstance(instanceID)
	if !ok {
		return fmt.Errorf("hand instance not found: %s", instanceID)
	}

	if err := k.handRegistry.DeactivateInstance(instanceID); err != nil {
		return err
	}

	if instance.AgentID != "" {
		agentID, err := types.ParseAgentID(instance.AgentID)
		if err == nil {
			if _, err := k.agentRegistry.Remove(agentID); err != nil {
				return err
			}
		}
		// Also delete agent from AgentRuntime
		k.agentRuntime.DeleteAgent(instance.AgentID)
	}

	return nil
}

// PauseHand pauses a hand instance.
func (k *Kernel) PauseHand(instanceID string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.handRegistry.PauseInstance(instanceID)
}

// ResumeHand resumes a paused hand instance.
func (k *Kernel) ResumeHand(instanceID string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.handRegistry.ResumeInstance(instanceID)
}

// expandPath expands a path with ~ to the user's home directory.
func expandPath(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, path[1:]), nil
}

// SendMessage sends a message to an agent and gets the response.
func (k *Kernel) SendMessage(ctx context.Context, agentID string, message string) (string, error) {
	runner := agent.NewAgentRunner(k.agentRuntime)
	result, err := runner.RunAgent(ctx, agentID, message, nil, nil)
	if err != nil {
		k.auditLog.Record("system", agentID, audit.ActionAgentMessage, "agent loop failed", fmt.Sprintf("error: %v", err))
		return "", err
	}
	k.auditLog.Record("system", agentID, audit.ActionAgentMessage, fmt.Sprintf("tokens_in=%d, tokens_out=%d", result.TotalUsage.PromptTokens, result.TotalUsage.CompletionTokens), "ok")
	return result.Response, nil
}

// SendMessageWithUsage sends a message to an agent and gets the response with token usage.
func (k *Kernel) SendMessageWithUsage(ctx context.Context, agentID string, message string) (string, uint64, uint64, error) {
	runner := agent.NewAgentRunner(k.agentRuntime)
	result, err := runner.RunAgent(ctx, agentID, message, nil, nil)
	if err != nil {
		k.auditLog.Record("system", agentID, audit.ActionAgentMessage, "agent loop failed", fmt.Sprintf("error: %v", err))
		return "", 0, 0, err
	}
	k.auditLog.Record("system", agentID, audit.ActionAgentMessage, fmt.Sprintf("tokens_in=%d, tokens_out=%d", result.TotalUsage.PromptTokens, result.TotalUsage.CompletionTokens), "ok")
	return result.Response, uint64(result.TotalUsage.PromptTokens), uint64(result.TotalUsage.CompletionTokens), nil
}

// FindAgentByName finds an agent by name.
func (k *Kernel) FindAgentByName(ctx context.Context, name string) (string, bool) {
	entry := k.agentRegistry.FindByName(name)
	if entry == nil {
		return "", false
	}
	return entry.ID.String(), true
}

// ListAgents lists all running agents.
func (k *Kernel) ListAgents(ctx context.Context) ([]channels.AgentInfo, error) {
	entries := k.agentRegistry.List()
	infos := make([]channels.AgentInfo, 0, len(entries))
	for _, entry := range entries {
		infos = append(infos, channels.AgentInfo{
			ID:   entry.ID.String(),
			Name: entry.Name,
		})
	}
	return infos, nil
}

// ListAgentEntries lists all agent entries with full information.
func (k *Kernel) ListAgentEntries() []*mcp.AgentEntry {
	entries := k.agentRegistry.List()
	result := make([]*mcp.AgentEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, &mcp.AgentEntry{
			ID:       entry.ID.String(),
			Name:     entry.Name,
			Manifest: entry.Manifest,
		})
	}
	return result
}

// SpawnAgent spawns a new agent from a manifest.
func (k *Kernel) SpawnAgent(manifest types.AgentManifest) (string, string, error) {
	var agentID types.AgentID
	var agentName, agentSystemPrompt, agentProvider, agentModel, agentAPIKeyEnv string
	var agentTools, agentSkills []string
	var agentSkillPromptContext string
	var registerErr error

	// Part 1: Operations that require a lock
	func() {
		k.mu.Lock()
		defer k.mu.Unlock()

		agentID = types.NewAgentID()

		// Load config file to get default model configuration
		cfg, err := config.Load("")
		if err != nil {
			cfg = config.DefaultConfig()
		}

		// If any config is missing in manifest (provider, model, or APIKeyEnv), use defaults from config.toml
		if manifest.Model.Provider == "" || manifest.Model.Model == "" || manifest.Model.APIKeyEnv == "" {
			agentProvider = cfg.DefaultModel.Provider
			agentModel = cfg.DefaultModel.Model
			agentAPIKeyEnv = cfg.DefaultModel.APIKeyEnv
		} else {
			// Otherwise use config from manifest
			agentProvider = manifest.Model.Provider
			agentModel = manifest.Model.Model
			agentAPIKeyEnv = manifest.Model.APIKeyEnv
		}

		// Update model config in manifest
		manifest.Model.Provider = agentProvider
		manifest.Model.Model = agentModel
		manifest.Model.APIKeyEnv = agentAPIKeyEnv

		entry := &AgentEntry{
			ID:         agentID,
			Name:       manifest.Name,
			State:      types.AgentStateRunning,
			Mode:       "auto",
			Tags:       []string{},
			Manifest:   manifest,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
		}

		if registerErr = k.agentRegistry.Register(entry); registerErr != nil {
			return
		}

		agentName = manifest.Name
		agentSystemPrompt = manifest.SystemPrompt
		agentTools = manifest.Tools
		agentSkills = manifest.Skills
		agentSkillPromptContext = manifest.SkillPromptContext
	}()

	// Check if register failed
	if registerErr != nil {
		return "", "", registerErr
	}

	// Part 2: Operations that don't require a lock
	apiKey := os.Getenv(agentAPIKeyEnv)
	if apiKey == "" {
		return "", "", fmt.Errorf("API key not set for agent %s: %s", agentName, agentAPIKeyEnv)
	}

	driver, err := llm.NewDriver(agentProvider, apiKey, agentModel)
	if err != nil {
		return "", "", fmt.Errorf("failed to create LLM driver for %s: %w", agentProvider, err)
	}

	k.agentRuntime.RegisterDriver(agentProvider, driver)

	_, err = k.agentRuntime.RegisterAgent(
		context.Background(),
		agentID.String(),
		agentName,
		agentProvider,
		agentModel,
		agentSystemPrompt,
		agentTools,
		agentSkills,
		agentSkillPromptContext,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to register agent in runtime: %w", err)
	}

	k.auditLog.Record("system", agentID.String(), audit.ActionAgentSpawn, fmt.Sprintf("name=%s", agentName), "ok")

	return agentID.String(), agentName, nil
}

// SpawnAgentByName spawns an agent by manifest name.
func (k *Kernel) SpawnAgentByName(ctx context.Context, manifestName string) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

// DeleteAgent deletes an agent from the registry and runtime.
func (k *Kernel) DeleteAgent(agentIDStr string) error {
	id, err := types.ParseAgentID(agentIDStr)
	if err != nil {
		return err
	}

	k.mu.Lock()
	_, err = k.agentRegistry.Remove(id)
	k.mu.Unlock()

	if err != nil {
		return err
	}

	// Also delete agent from AgentRuntime
	k.agentRuntime.DeleteAgent(agentIDStr)

	return nil
}

// AgentRuntime returns the agent runtime.
func (k *Kernel) AgentRuntime() *agent.Runtime {
	return k.agentRuntime
}

// ConnectMcpServers connects to all configured MCP servers.
func (k *Kernel) ConnectMcpServers(ctx context.Context) {
	for _, serverConfig := range k.config.McpServers {
		conn, err := mcp.Connect(ctx, serverConfig)
		if err != nil {
			fmt.Printf("Warning: Failed to connect to MCP server %s: %v\n", serverConfig.Name, err)
			continue
		}

		k.mcpConnections.Store(serverConfig.Name, conn)

		tools := conn.Tools()
		k.mcpTools.Store(serverConfig.Name, tools)

		fmt.Printf("Connected to MCP server %s, found %d tools\n", serverConfig.Name, len(tools))
	}
}

// GetMcpTools returns all MCP tools from connected servers.
func (k *Kernel) GetMcpTools() []types.ToolDefinition {
	var allTools []types.ToolDefinition

	k.mcpTools.Range(func(_, value interface{}) bool {
		if tools, ok := value.([]types.ToolDefinition); ok {
			allTools = append(allTools, tools...)
		}
		return true
	})

	return allTools
}

// GetMcpToolsForServer returns MCP tools from a specific server.
func (k *Kernel) GetMcpToolsForServer(serverName string) []types.ToolDefinition {
	if value, ok := k.mcpTools.Load(serverName); ok {
		if tools, ok := value.([]types.ToolDefinition); ok {
			return tools
		}
	}
	return nil
}

// CallMcpTool calls an MCP tool.
func (k *Kernel) CallMcpTool(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error) {
	serverName, ok := mcp.ExtractMcpServer(toolName)
	if !ok {
		return "", fmt.Errorf("not an MCP tool: %s", toolName)
	}

	value, ok := k.mcpConnections.Load(serverName)
	if !ok {
		return "", fmt.Errorf("MCP server not connected: %s", serverName)
	}

	conn, ok := value.(*mcp.McpConnection)
	if !ok {
		return "", fmt.Errorf("invalid MCP connection")
	}

	result, err := conn.CallTool(ctx, toolName, arguments)
	if err != nil {
		return "", err
	}

	var output strings.Builder
	for _, content := range result.Content {
		if content.Type == "text" {
			output.WriteString(content.Text)
			output.WriteString("\n")
		}
	}

	return output.String(), nil
}

// CloseMcpConnections closes all MCP connections.
func (k *Kernel) CloseMcpConnections() {
	k.mcpConnections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*mcp.McpConnection); ok {
			conn.Close()
		}
		k.mcpConnections.Delete(key)
		return true
	})
}

type McpServerInfo struct {
	Name      string                 `json:"name"`
	Connected bool                   `json:"connected"`
	Tools     []types.ToolDefinition `json:"tools,omitempty"`
	Transport types.McpTransport     `json:"transport,omitempty"`
	Env       []string               `json:"env,omitempty"`
}

// GetMcpServers returns all MCP servers.
func (k *Kernel) GetMcpServers() map[string]interface{} {
	var connected []map[string]interface{}
	var configured []map[string]interface{}

	for _, serverConfig := range k.config.McpServers {
		// Check if connected
		var isConnected bool
		var tools []types.ToolDefinition
		if connValue, ok := k.mcpConnections.Load(serverConfig.Name); ok {
			if _, ok := connValue.(*mcp.McpConnection); ok {
				isConnected = true
				if toolsValue, ok := k.mcpTools.Load(serverConfig.Name); ok {
					tools, _ = toolsValue.([]types.ToolDefinition)
				}
			}
		}

		if isConnected {
			connected = append(connected, map[string]interface{}{
				"name":        serverConfig.Name,
				"tools_count": len(tools),
				"tools":       tools,
			})
		}

		configured = append(configured, map[string]interface{}{
			"name":      serverConfig.Name,
			"transport": serverConfig.Transport,
			"env":       serverConfig.Env,
		})
	}

	return map[string]interface{}{
		"total_configured": len(configured),
		"total_connected":  len(connected),
		"connected":        connected,
		"configured":       configured,
	}
}

// cronDeliverResponse delivers the response from an agent turn to the configured destination.
func (k *Kernel) cronDeliverResponse(agentID types.AgentID, response string, delivery *types.CronDelivery) {
	log.Info().
		Str("agent", agentID.String()).
		Str("delivery_kind", string(delivery.Kind)).
		Msg("Cron: delivering response")

	switch delivery.Kind {
	case types.CronDeliveryKindNone:
		log.Info().Msg("Cron delivery: kind is None, skipping")
		return

	case types.CronDeliveryKindChannel:
		if delivery.ChannelName == nil {
			log.Warn().Str("agent", agentID.String()).Msg("Cron delivery: channel name not specified")
			return
		}

		var recipient string
		if delivery.Recipient != nil {
			recipient = *delivery.Recipient
		}

		log.Info().
			Str("channel_name", *delivery.ChannelName).
			Str("recipient", recipient).
			Msg("Cron delivery: preparing to send via channel")

		// Find channel by name
		channelList := k.registry.ListChannels()
		var targetChannel *channels.Channel
		var availableChannelNames []string
		for _, ch := range channelList {
			availableChannelNames = append(availableChannelNames, ch.Name)
			if ch.Name == *delivery.ChannelName {
				targetChannel = ch
				break
			}
		}

		log.Info().
			Strs("available_channels", availableChannelNames).
			Msg("Cron delivery: available channels")

		if targetChannel == nil {
			log.Warn().Str("channel", *delivery.ChannelName).Msg("Cron delivery: channel not found")
			return
		}

		adapter, ok := k.registry.GetAdapter(targetChannel.ID)
		if !ok {
			log.Warn().Str("channel", targetChannel.ID).Msg("Cron delivery: adapter not found")
			return
		}

		msg := &channels.Message{
			ID:        fmt.Sprintf("cron_%d", time.Now().UnixNano()),
			ChannelID: targetChannel.ID,
			Content:   response,
			Recipient: recipient,
			CreatedAt: time.Now(),
		}

		if err := adapter.Send(msg); err != nil {
			log.Warn().Err(err).Str("channel", targetChannel.ID).Msg("Cron delivery: failed to send message")
		}

	case types.CronDeliveryKindLastChannel:
		log.Warn().Msg("Cron delivery: LastChannel not implemented yet")

	case types.CronDeliveryKindWebhook:
		if delivery.Url == nil {
			log.Warn().Msg("Cron delivery: webhook URL not specified")
			return
		}

		// Prepare payload
		payload := map[string]interface{}{
			"agent_id":  agentID.String(),
			"response":  response,
			"timestamp": time.Now().UTC(),
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Warn().Err(err).Msg("Cron delivery: failed to marshal webhook payload")
			return
		}

		// Send webhook
		req, err := http.NewRequest("POST", *delivery.Url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			log.Warn().Err(err).Msg("Cron delivery: failed to create webhook request")
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Warn().Err(err).Msg("Cron delivery: failed to send webhook")
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Warn().Int("status", resp.StatusCode).Msg("Cron delivery: webhook returned non-success status")
		}
	}
}

// ValidateShellCommand validates a shell command against security configuration
func ValidateShellCommand(command string, args []string, config types.CronShellSecurityConfig) error {
	log.Info().
		Str("command", command).
		Strs("args", args).
		Str("security_mode", config.SecurityMode).
		Msg("Cron: validating shell command")

	if !config.EnableExecuteShell {
		return fmt.Errorf("execute_shell is disabled in configuration")
	}

	for _, forbidden := range config.ForbiddenCommands {
		if command == forbidden {
			log.Warn().Str("command", command).Msg("Cron: command is forbidden")
			return fmt.Errorf("command '%s' is forbidden", command)
		}
	}

	switch config.SecurityMode {
	case "strict":
		allowed := false
		for _, allowedCmd := range config.AllowedCommands {
			if command == allowedCmd {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Warn().Str("command", command).Strs("allowed_commands", config.AllowedCommands).Msg("Cron: command not in allowed list")
			return fmt.Errorf("command '%s' is not in allowed list", command)
		}

	case "path":
		cmdPath, err := exec.LookPath(command)
		if err != nil {
			return fmt.Errorf("command not found: %w", err)
		}
		cmdPath = filepath.Clean(cmdPath)

		allowed := false
		for _, allowedPath := range config.AllowedPaths {
			allowedPath = filepath.Clean(allowedPath)
			if strings.HasPrefix(cmdPath, allowedPath+string(os.PathSeparator)) || cmdPath == allowedPath {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Warn().Str("command", command).Str("cmd_path", cmdPath).Strs("allowed_paths", config.AllowedPaths).Msg("Cron: command path not in allowed paths")
			return fmt.Errorf("command path '%s' is not in allowed paths", cmdPath)
		}

	case "none":
		log.Warn().Str("command", command).Msg("Cron: execute_shell running in 'none' security mode")

	default:
		return fmt.Errorf("invalid security mode: %s", config.SecurityMode)
	}

	for _, arg := range args {
		for _, pattern := range config.ForbiddenArgsPatterns {
			matched, err := regexp.MatchString(pattern, arg)
			if err != nil {
				return fmt.Errorf("invalid pattern '%s': %w", pattern, err)
			}
			if matched {
				log.Warn().Str("argument", arg).Str("pattern", pattern).Msg("Cron: argument contains forbidden pattern")
				return fmt.Errorf("argument '%s' contains forbidden pattern", arg)
			}
		}
	}

	log.Info().Str("command", command).Msg("Cron: shell command validation passed")
	return nil
}
