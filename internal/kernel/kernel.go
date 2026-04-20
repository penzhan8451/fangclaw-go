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

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/a2a"
	"github.com/penzhan8451/fangclaw-go/internal/approvals"
	"github.com/penzhan8451/fangclaw-go/internal/audit"
	"github.com/penzhan8451/fangclaw-go/internal/auth"
	"github.com/penzhan8451/fangclaw-go/internal/autoreply"
	"github.com/penzhan8451/fangclaw-go/internal/browser"
	"github.com/penzhan8451/fangclaw-go/internal/capabilities"
	"github.com/penzhan8451/fangclaw-go/internal/channels"
	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/penzhan8451/fangclaw-go/internal/configreload"
	"github.com/penzhan8451/fangclaw-go/internal/cron"
	deliv "github.com/penzhan8451/fangclaw-go/internal/delivery"
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
	"github.com/penzhan8451/fangclaw-go/internal/scheduler"
	"github.com/penzhan8451/fangclaw-go/internal/security"
	"github.com/penzhan8451/fangclaw-go/internal/skills"
	"github.com/penzhan8451/fangclaw-go/internal/triggers"
	"github.com/penzhan8451/fangclaw-go/internal/types"
	"github.com/penzhan8451/fangclaw-go/internal/userdir"
	"github.com/penzhan8451/fangclaw-go/internal/vector"
	"github.com/rs/zerolog/log"
)

type Kernel struct {
	config          types.KernelConfig
	eventBus        *eventbus.EventBus
	scheduler       *Scheduler
	cronScheduler   *cron.CronScheduler
	agentScheduler  *scheduler.AgentScheduler
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
	sharedRegistry  *channels.Registry
	agentRegistry   *AgentRegistry
	handRegistry    *hands.Registry
	triggerEngine   *triggers.TriggerEngine
	approvalMgr     *approvals.ApprovalManager
	capabilityMgr   *capabilities.CapabilityManager
	deliveryReg     *deliv.DeliveryRegistry
	deliveryTracker *deliv.DeliveryTracker
	pairingManager  *pairing.PairingManager
	workflowEngine  *WorkflowEngine
	agentRuntime    *agent.Runtime
	autoReplyEngine *autoreply.AutoReplyEngine
	mcpConnections  sync.Map
	mcpTools        sync.Map
	auditLog        *audit.AuditLog
	a2aTaskStore    *a2a.A2ATaskStore
	a2aClient       *a2a.A2AClient
	a2aEventStore   *a2a.A2AEventStore
	authManager     *auth.AuthManager
	userDirMgr      *userdir.Manager
	secrets         map[string]string
	secretsMu       sync.RWMutex
	mu              sync.RWMutex
	started         bool
	startTime       time.Time
	stopping        chan struct{}
}

func NewKernel(kernelConfig types.KernelConfig) (*Kernel, error) {
	return NewKernelWithShared(kernelConfig, nil, nil)
}

func NewKernelWithShared(kernelConfig types.KernelConfig, sharedModelCatalog *model_catalog.ModelCatalog, sharedAgentTemplates *agent_templates.AgentTemplates) (*Kernel, error) {
	var dataDir string
	var err error
	var userDirMgr *userdir.Manager

	userDirMgr, err = userdir.GetDefaultManager()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get userdir manager, some features may not work")
	}

	if kernelConfig.DataDir != "" {
		dataDir, err = expandPath(kernelConfig.DataDir)
		if err != nil {
			return nil, fmt.Errorf("invalid data directory: %w", err)
		}
		log.Info().Str("user", kernelConfig.Username).Str("dataDir", dataDir).Str("configDataDir", kernelConfig.DataDir).Msg("Using configured data directory")
	} else if kernelConfig.Username != "" && kernelConfig.Auth.Enabled && userDirMgr != nil {
		if err := userDirMgr.EnsureUserDir(kernelConfig.Username); err != nil {
			return nil, fmt.Errorf("failed to ensure user directory: %w", err)
		}

		dataDir = userDirMgr.UserDir(kernelConfig.Username)
		log.Info().Str("user", kernelConfig.Username).Str("dataDir", dataDir).Msg("Using user-specific data directory")
	} else {
		dataDir, err = expandPath(kernelConfig.DataDir)
		if err != nil {
			return nil, fmt.Errorf("invalid data directory: %w", err)
		}
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	kernelConfig.DataDir = dataDir

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
	triggerEngine := triggers.NewTriggerEngine(db)
	if err := triggerEngine.LoadFromDB(); err != nil {
		log.Warn().Err(err).Msg("Failed to load triggers from database")
	}
	approvalPolicy := approvals.DefaultApprovalPolicy()
	approvalMgr := approvals.NewApprovalManager(approvalPolicy)
	deliveryReg := deliv.NewDeliveryRegistry()
	deliveryTracker := deliv.NewDeliveryTracker()
	ntfyURL := "https://ntfy.sh"
	ntfyTopic := "fangclaw-go-notifications"
	pairingConfig := pairing.PairingConfig{
		Enabled:         true,
		MaxDevices:      10,
		TokenExpirySecs: 300,
		PushProvider:    "ntfy",
		NtfyURL:         &ntfyURL,
		NtfyTopic:       &ntfyTopic,
	}
	pairingManager := pairing.NewPairingManager(pairingConfig)

	cronPersistDir := dataDir
	cronScheduler := cron.NewCronScheduler(cronPersistDir, 100)
	// If this is the global kernel (no username), set it as the global scheduler
	if kernelConfig.Username == "" {
		cron.SetGlobalScheduler(cronScheduler)
	}
	// qouta trace and check
	agentScheduler := scheduler.NewAgentScheduler()

	modelCatalogPath := filepath.Join(dataDir, "model_catalog.json")
	var modelCatalog *model_catalog.ModelCatalog
	if sharedModelCatalog != nil {
		sharedModels := sharedModelCatalog.GetSharedModels()
		sharedAliases := sharedModelCatalog.GetSharedAliases()
		sharedProviders := sharedModelCatalog.GetSharedProviders()
		modelCatalog = model_catalog.NewModelCatalogWithShared(modelCatalogPath, sharedModels, sharedAliases, sharedProviders)
	} else {
		modelCatalog = model_catalog.NewModelCatalog(modelCatalogPath)
	}

	var agentTemplates *agent_templates.AgentTemplates
	if sharedAgentTemplates != nil {
		sharedTemplates := sharedAgentTemplates.GetSharedTemplates()
		agentTemplates = agent_templates.NewAgentTemplatesWithShared(dataDir, sharedTemplates)
	} else {
		agentTemplates = agent_templates.NewAgentTemplatesWithDataDir(dataDir)
	}
	if err := agentTemplates.Load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load agent templates")
	}
	log.Debug().Str("dataDir", dataDir).Msg("Creating WorkflowEngine")
	workflowEngine := NewWorkflowEngine(dataDir)
	log.Debug().Msg("WorkflowEngine created")

	autoReplyConfig := autoreply.AutoReplyConfig{
		Enabled:          true,
		MaxConcurrent:    3,
		SuppressPatterns: []string{"/stop", "/pause", "谢谢", "好的，知道了", "人工客服", "我要找人工客服"},
	}
	autoReplyEngine := autoreply.NewAutoReplyEngine(autoReplyConfig)

	k := &Kernel{
		config:          kernelConfig,
		eventBus:        eventbus.NewEventBus(),
		scheduler:       NewScheduler(),
		cronScheduler:   cronScheduler,
		agentScheduler:  agentScheduler,
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
		deliveryTracker: deliveryTracker,
		pairingManager:  pairingManager,
		workflowEngine:  workflowEngine,
		autoReplyEngine: autoReplyEngine,
		auditLog:        audit.NewAuditLog(),
		a2aTaskStore:    a2a.NewA2ATaskStore(1000),
		a2aClient:       a2a.NewA2AClient(),
		a2aEventStore:   a2a.NewA2AEventStore(1000),
		capabilityMgr:   capabilities.NewCapabilityManager(),
		userDirMgr:      userDirMgr,
		secrets:         make(map[string]string),
		startTime:       time.Now(),
		stopping:        make(chan struct{}),
	}

	authDBPath := kernelConfig.Auth.DBPath
	if authDBPath == "" {
		authDBPath = filepath.Join(dataDir, "auth.db")
	}
	authManager, err := auth.NewAuthManager(authDBPath)
	if err != nil {
		semanticStore.Close()
		sessionStore.Close()
		db.Close()
		return nil, fmt.Errorf("failed to create auth manager: %w", err)
	}
	authManager.SetGitHubOAuthConfig(
		kernelConfig.Auth.GitHub.ClientID,
		kernelConfig.Auth.GitHub.ClientSecret,
		kernelConfig.Auth.GitHub.RedirectURL,
		kernelConfig.Auth.GitHub.Enabled,
	)
	k.authManager = authManager
	log.Info().Str("path", authDBPath).Msg("Auth manager initialized")

	// Link event store to A2A components
	k.a2aTaskStore.SetEventStore(k.a2aEventStore)
	k.a2aClient.SetEventStore(k.a2aEventStore)
	k.a2aClient.SetTaskStore(k.a2aTaskStore)

	// Set event publisher for workflow engine
	workflowEngine.SetEventPublisher(func(event *eventbus.Event) {
		k.PublishEvent(event)
	})

	k.workflowEngine.SetChannelSender(func(channelName, recipient, message string) error {
		_, err := channels.SendMessageToChannelName(k.registry, channelName, recipient, message, nil)
		return err
	})

	mcpCallbacks := &agent.McpCallbacks{
		GetMcpTools: k.GetMcpTools,
		CallMcpTool: k.CallMcpTool,
	}

	agentRuntime := agent.NewRuntime(semanticStore, sessionStore, knowledgeStore, usageStore, skillLoader, embeddingDriver, modelCatalog, mcpCallbacks, approvalMgr, agentScheduler)

	kernelConfig.DataDir = dataDir
	k.config = kernelConfig
	k.agentRuntime = agentRuntime

	agentRuntime.SetCapabilityChecker(func(agentID string, capType string, resource string) bool {
		cap := capabilities.Capability{Type: capabilities.CapabilityType(capType), Resource: resource}
		result := k.capabilityMgr.CheckWithDefault(agentID, cap)
		return result.Granted()
	})

	// Initialize CDP Browser Manager if enabled
	if kernelConfig.Browser.Enabled {
		browserConfig := browser.CDPBrowserConfig{
			ChromiumPath:   kernelConfig.Browser.ChromiumPath,
			Headless:       kernelConfig.Browser.Headless,
			ViewportWidth:  kernelConfig.Browser.ViewportWidth,
			ViewportHeight: kernelConfig.Browser.ViewportHeight,
			MaxSessions:    kernelConfig.Browser.MaxSessions,
		}
		if browserConfig.ViewportWidth == 0 {
			browserConfig.ViewportWidth = 1280
		}
		if browserConfig.ViewportHeight == 0 {
			browserConfig.ViewportHeight = 720
		}
		if browserConfig.MaxSessions == 0 {
			browserConfig.MaxSessions = 5
		}
		cdpBrowserMgr := browser.NewCDPBrowserManager(browserConfig)
		agentRuntime.SetBrowserManager(cdpBrowserMgr)
		log.Info().Msg("CDP Browser Manager initialized")
	}

	// Register all built-in tools to AgentRuntime
	tools.RegisterAllTools(agentRuntime)

	// Register schedule tools with this kernel's scheduler
	agentRuntime.RegisterTool(tools.NewScheduleCreateTool(cronScheduler))
	agentRuntime.RegisterTool(tools.NewScheduleListTool(cronScheduler))
	agentRuntime.RegisterTool(tools.NewScheduleDeleteTool(cronScheduler))

	// Register skill_manage tool with skill loader
	skillManageTool := tools.NewSkillManageTool(skillLoader)
	agentRuntime.RegisterTool(skillManageTool)

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

			// Get API key from secrets
			apiKey := k.GetSecret(agentAPIKeyEnv)
			if apiKey == "" {
				log.Warn().Str("agent", agentEntry.Name).Str("env", agentAPIKeyEnv).Msg("API key not set, skipping registration to runtime")
				continue
			}

			// Create LLM driver
			driver, err := llm.NewDriver(agentProvider, apiKey, agentModel)
			if err != nil {
				log.Warn().Str("agent", agentEntry.Name).Err(err).Msg("Failed to create LLM driver, skipping registration to runtime")
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
				agentEntry.Files,
			)
			if err != nil {
				log.Warn().Str("agent", agentEntry.Name).Err(err).Msg("Failed to register agent in runtime, skipping")
			} else {
				caps := manifestToCapabilities(agentEntry.Manifest)
				for _, cap := range caps {
					fmt.Printf("***manifestToCapailities Type %s to grant to agent:%s\n", cap.Type, agentEntry.ID.String())
					fmt.Printf("***manifestToCapailities Resource %s to grant to agent:%s\n", cap.Resource, agentEntry.ID.String())
				}
				k.capabilityMgr.Grant(agentEntry.ID.String(), caps)

				// Register agent quota
				agentIDStr := agentEntry.ID.String()
				quota := k.config.Quotas.Default
				if agentQuota, ok := k.config.Quotas.Agents[agentIDStr]; ok {
					quota = agentQuota
				}
				schedulerQuota := scheduler.ResourceQuota{
					MaxTokensPerHour:    quota.MaxTokensPerHour,
					MaxToolCallsPerHour: quota.MaxToolCallsPerHour,
					MaxCostPerHourUSD:   quota.MaxCostPerHourUSD,
				}
				k.agentScheduler.Register(agentIDStr, schedulerQuota)

				log.Debug().Str("agent", agentEntry.Name).Str("id", agentEntry.ID.String()).Msg("Registered agent from disk")
			}
		} //_for loop for agent in _agentRegistry

		// Restore hand instances from agents
		log.Debug().Int("agents", len(agents)).Msg("Restoring hand instances from agents")
		var agentEntriesForHands []hands.AgentEntry
		for _, agent := range agents {
			agentEntriesForHands = append(agentEntriesForHands, agent)
		}
		handRegistry.RestoreInstancesFromAgents(agentEntriesForHands)
		log.Debug().Int("instances", len(handRegistry.ListInstances())).Msg("Restored hand instances")
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

	// Load agents from agentRegistry and register to agentRuntime
	agentEntries := k.agentRegistry.List()
	fmt.Printf("[Kernel] Found %d agents in registry to register to runtime\n", len(agentEntries))
	for _, entry := range agentEntries {
		fmt.Printf("[Kernel] Registering agent from registry: ID=%s, Name=%s\n", entry.ID.String(), entry.Name)

		manifest := entry.Manifest

		// Get API key
		apiKeyEnv := manifest.Model.APIKeyEnv
		apiKey := k.GetSecret(apiKeyEnv)
		if apiKey == "" {
			fmt.Printf("[Kernel] Warning: API key not found for agent %s (env=%s), skipping\n", entry.Name, apiKeyEnv)
			continue
		}

		// Create and register driver
		driver, err := llm.NewDriver(manifest.Model.Provider, apiKey, manifest.Model.Model)
		if err != nil {
			fmt.Printf("[Kernel] Warning: Failed to create LLM driver for agent %s: %v, skipping\n", entry.Name, err)
			continue
		}
		k.agentRuntime.RegisterDriver(manifest.Model.Provider, driver)

		// Register agent to runtime with the original ID
		_, err = k.agentRuntime.RegisterAgent(
			context.Background(),
			entry.ID.String(),
			entry.Name,
			manifest.Model.Provider,
			manifest.Model.Model,
			manifest.SystemPrompt,
			manifest.Tools,
			manifest.Skills,
			manifest.SkillPromptContext,
			entry.Files,
		)
		if err != nil {
			fmt.Printf("[Kernel] Warning: Failed to register agent %s in runtime: %v\n", entry.Name, err)
		} else {
			fmt.Printf("[Kernel] Successfully registered agent %s (ID=%s) to runtime\n", entry.Name, entry.ID.String())

			// Grant capabilities
			caps := manifestToCapabilities(manifest)
			k.capabilityMgr.Grant(entry.ID.String(), caps)

			// Register agent quota
			agentIDStr := entry.ID.String()
			quota := k.config.Quotas.Default
			if agentQuota, ok := k.config.Quotas.Agents[agentIDStr]; ok {
				quota = agentQuota
			}
			schedulerQuota := scheduler.ResourceQuota{
				MaxTokensPerHour:    quota.MaxTokensPerHour,
				MaxToolCallsPerHour: quota.MaxToolCallsPerHour,
				MaxCostPerHourUSD:   quota.MaxCostPerHourUSD,
			}
			k.agentScheduler.Register(agentIDStr, schedulerQuota)
		}
	}

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

									payload := map[string]interface{}{
										"type":         fmt.Sprintf("cron.%s", jobName),
										"text":         *job.Action.Message,
										"job_id":       jobID.String(),
										"agent_id":     job.AgentID.String(),
										"agent_result": res.result,
									}
									event := eventbus.NewEvent(
										eventbus.EventTypeSystem,
										"cron",
										eventbus.EventTargetBroadcast,
									).WithPayload(payload)
									k.PublishEvent(event)
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
								env, workDir, prepErr := security.PrepareSecureExec(security.DefaultSecureExecConfig())
								if prepErr != nil {
									log.Warn().Str("job", jobName).Err(prepErr).Msg("Cron: failed to prepare secure execution, using default environment")
								}

								cmd := exec.CommandContext(ctxTimeout, command, args...)
								var stdoutBuf, stderrBuf bytes.Buffer
								cmd.Stdout = &stdoutBuf
								cmd.Stderr = &stderrBuf
								cmd.Env = env
								if workDir != "" {
									cmd.Dir = workDir
								}

								log.Info().Str("job", jobName).Msg("Cron: starting command execution")
								execErr := cmd.Run()
								stdout := stdoutBuf.String()
								stderr := stderrBuf.String()

								log.Info().Str("job", jobName).Str("stdout", stdout).Str("stderr", stderr).Err(execErr).Msg("Cron: command execution finished")
								resultChan <- struct {
									stdout string
									stderr string
									err    error
								}{stdout, stderr, execErr}
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
	if k.authManager != nil {
		k.authManager.Close()
	}
	k.started = false

	event := eventbus.NewEvent(eventbus.EventTypeSystem, "kernel", eventbus.EventTargetSystem)
	k.eventBus.Publish(event)

	return nil
}

func (k *Kernel) Config() types.KernelConfig {
	return k.config
}

func (k *Kernel) GetSecret(key string) string {
	k.secretsMu.RLock()
	defer k.secretsMu.RUnlock()
	return k.secrets[key]
}

func (k *Kernel) SetSecret(key, value string) {
	k.secretsMu.Lock()
	defer k.secretsMu.Unlock()
	k.secrets[key] = value
}

func (k *Kernel) SetSecrets(secrets map[string]string) {
	k.secretsMu.Lock()
	defer k.secretsMu.Unlock()
	k.secrets = secrets
}

func (k *Kernel) GetSecrets() map[string]string {
	k.secretsMu.RLock()
	defer k.secretsMu.RUnlock()
	result := make(map[string]string)
	for key, value := range k.secrets {
		result[key] = value
	}
	return result
}

func (k *Kernel) ReloadSecrets() error {
	secrets, err := userdir.LoadUserSecrets(k.config.Username)
	if err != nil {
		return err
	}
	k.SetSecrets(secrets)
	return nil
}

func (k *Kernel) EventBus() *eventbus.EventBus {
	return k.eventBus
}

func (k *Kernel) Scheduler() *Scheduler {
	return k.scheduler
}

// agent quota
func (k *Kernel) AgentScheduler() *scheduler.AgentScheduler {
	return k.agentScheduler
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

func (k *Kernel) IsSetupComplete() bool {
	var cfg *config.Config
	var err error
	if k.config.Username != "" {
		cfg, err = config.LoadUserConfig(k.config.Username)
	} else {
		cfg, err = config.Load("")
	}
	if err != nil {
		cfg = config.DefaultConfig()
	}

	if cfg.DefaultModel.APIKeyEnv == "" {
		return false
	}

	apiKey := k.GetSecret(cfg.DefaultModel.APIKeyEnv)
	return apiKey != ""
}

func (k *Kernel) SkillLoader() *skills.Loader {
	return k.skillLoader
}

// channel registry
func (k *Kernel) Registry() *channels.Registry {
	if k.sharedRegistry != nil {
		return k.sharedRegistry
	}
	return k.registry
}

func (k *Kernel) LocalRegistry() *channels.Registry {
	return k.registry
}

func (k *Kernel) SetSharedRegistry(registry *channels.Registry) {
	k.sharedRegistry = registry
}

func (k *Kernel) AgentRegistry() *AgentRegistry {
	return k.agentRegistry
}

func (k *Kernel) UpdateAgentRuntimeSkills(agentID string, skills []string) {
	k.agentRuntime.UpdateAgentSkills(agentID, skills)
}

func (k *Kernel) UpdateAgentRuntimeSystemPrompt(agentID string, systemPrompt string) {
	k.agentRuntime.UpdateAgentSystemPrompt(agentID, systemPrompt)
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

func (k *Kernel) CapabilityManager() *capabilities.CapabilityManager {
	return k.capabilityMgr
}

func (k *Kernel) AuthManager() *auth.AuthManager {
	return k.authManager
}

func (k *Kernel) IsAuthEnabled() bool {
	return k.config.Auth.Enabled
}

func (k *Kernel) Username() string {
	return k.config.Username
}

func (k *Kernel) UserID() string {
	return k.config.UserID
}

func (k *Kernel) UserDirManager() *userdir.Manager {
	return k.userDirMgr
}

func (k *Kernel) DeliveryRegistry() *deliv.DeliveryRegistry {
	return k.deliveryReg
}

func (k *Kernel) DeliveryTracker() *deliv.DeliveryTracker {
	return k.deliveryTracker
}

func (k *Kernel) PairingManager() *pairing.PairingManager {
	return k.pairingManager
}

func (k *Kernel) CronScheduler() *cron.CronScheduler {
	return k.cronScheduler
}

func (k *Kernel) RunCronJob(ctx context.Context, jobID types.CronJobID) error {
	job := k.cronScheduler.GetJob(jobID)
	if job == nil {
		return fmt.Errorf("job not found: %s", jobID)
	}

	log.Info().Str("job", job.Name).Msg("Cron: manually firing job")

	switch job.Action.Kind {
	case types.CronActionKindSystemEvent:
		if job.Action.Text != nil {
			log.Info().Str("job", job.Name).Msg("Cron: firing system event")
			payload := map[string]interface{}{
				"type":   fmt.Sprintf("cron.%s", job.Name),
				"text":   *job.Action.Text,
				"job_id": jobID.String(),
			}
			event := eventbus.NewEvent(
				eventbus.EventTypeSystem,
				"cron",
				eventbus.EventTargetBroadcast,
			).WithPayload(payload)
			k.PublishEvent(event)
			k.cronScheduler.RecordSuccess(jobID)
		}
	case types.CronActionKindAgentTurn:
		log.Info().Str("job", job.Name).Str("agent", job.AgentID.String()).Msg("Cron: firing agent turn")
		if job.Action.Message != nil {
			log.Info().Str("job", job.Name).Str("message", *job.Action.Message).Msg("Cron: sending message to agent")
			timeoutSecs := uint64(120)
			if job.Action.TimeoutSecs != nil {
				timeoutSecs = *job.Action.TimeoutSecs
			}
			timeout := time.Duration(timeoutSecs) * time.Second
			ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			delivery := job.Delivery
			resultChan := make(chan struct {
				result string
				err    error
			}, 1)

			go func() {
				log.Info().Str("job", job.Name).Msg("Cron: calling SendMessage")
				result, err := k.SendMessage(ctxTimeout, job.AgentID.String(), *job.Action.Message)
				log.Info().Str("job", job.Name).Err(err).Msg("Cron: SendMessage returned")
				resultChan <- struct {
					result string
					err    error
				}{result, err}
			}()

			select {
			case <-ctxTimeout.Done():
				log.Warn().Str("job", job.Name).Uint64("timeout_s", timeoutSecs).Msg("Cron job timed out")
				k.cronScheduler.RecordFailure(jobID, fmt.Sprintf("timed out after %ds", timeoutSecs))
			case res := <-resultChan:
				if res.err != nil {
					errMsg := res.err.Error()
					log.Warn().Str("job", job.Name).Err(res.err).Msg("Cron job failed")
					k.cronScheduler.RecordFailure(jobID, errMsg)
				} else {
					log.Info().Str("job", job.Name).Str("result", res.result).Msg("Cron job completed successfully")
					k.cronScheduler.RecordSuccess(jobID)
					k.cronDeliverResponse(job.AgentID, res.result, &delivery)

					payload := map[string]interface{}{
						"type":         fmt.Sprintf("cron.%s", job.Name),
						"text":         *job.Action.Message,
						"job_id":       jobID.String(),
						"agent_id":     job.AgentID.String(),
						"agent_result": res.result,
					}
					event := eventbus.NewEvent(
						eventbus.EventTypeSystem,
						"cron",
						eventbus.EventTargetBroadcast,
					).WithPayload(payload)
					k.PublishEvent(event)
				}
			}
		}
	case types.CronActionKindExecuteShell:
		log.Info().Str("job", job.Name).Msg("Cron: firing execute shell")
		if job.Action.Command != nil {
			command := *job.Action.Command
			args := job.Action.Args
			log.Info().Str("job", job.Name).Str("command", command).Strs("args", args).Msg("Cron: executing shell command")

			if err := ValidateShellCommand(command, args, k.config.CronShellSecurity); err != nil {
				errMsg := fmt.Sprintf("security validation failed: %v", err)
				log.Warn().Str("job", job.Name).Err(err).Msg("Cron job blocked by security")
				k.cronScheduler.RecordFailure(jobID, errMsg)
				return nil
			}

			timeoutSecs := uint64(60)
			if job.Action.TimeoutSecs != nil {
				timeoutSecs = *job.Action.TimeoutSecs
			}
			timeout := time.Duration(timeoutSecs) * time.Second
			ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			delivery := job.Delivery
			resultChan := make(chan struct {
				stdout string
				stderr string
				err    error
			}, 1)

			go func() {
				env, workDir, prepErr := security.PrepareSecureExec(security.DefaultSecureExecConfig())
				if prepErr != nil {
					log.Warn().Str("job", job.Name).Err(prepErr).Msg("Cron: failed to prepare secure execution, using default environment")
				}

				cmd := exec.CommandContext(ctxTimeout, command, args...)
				var stdoutBuf, stderrBuf bytes.Buffer
				cmd.Stdout = &stdoutBuf
				cmd.Stderr = &stderrBuf
				cmd.Env = env
				if workDir != "" {
					cmd.Dir = workDir
				}

				log.Info().Str("job", job.Name).Msg("Cron: starting command execution")
				execErr := cmd.Run()
				stdout := stdoutBuf.String()
				stderr := stderrBuf.String()

				log.Info().Str("job", job.Name).Str("stdout", stdout).Str("stderr", stderr).Err(execErr).Msg("Cron: command execution finished")
				resultChan <- struct {
					stdout string
					stderr string
					err    error
				}{stdout, stderr, execErr}
			}()

			select {
			case <-ctxTimeout.Done():
				log.Warn().Str("job", job.Name).Uint64("timeout_s", timeoutSecs).Msg("Cron job timed out")
				k.cronScheduler.RecordFailure(jobID, fmt.Sprintf("timed out after %ds", timeoutSecs))
			case res := <-resultChan:
				if res.err != nil {
					errMsg := fmt.Sprintf("command failed: %v, stderr: %s", res.err, res.stderr)
					log.Warn().Str("job", job.Name).Err(res.err).Str("stderr", res.stderr).Msg("Cron job failed")
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
					log.Info().Str("job", job.Name).Str("stdout", res.stdout).Msg("Cron job completed successfully")
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

	return nil
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

func (k *Kernel) A2ATaskStore() *a2a.A2ATaskStore {
	return k.a2aTaskStore
}

func (k *Kernel) A2AClient() *a2a.A2AClient {
	return k.a2aClient
}

func (k *Kernel) A2AEventStore() *a2a.A2AEventStore {
	return k.a2aEventStore
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

	kernelConfig := types.KernelConfig{
		DataDir:    dataDir,
		McpServers: cfg.McpServers,
		Browser: types.BrowserConfig{
			Enabled:        cfg.Browser.Enabled,
			ChromiumPath:   cfg.Browser.ChromiumPath,
			Headless:       cfg.Browser.Headless,
			ViewportWidth:  cfg.Browser.ViewportWidth,
			ViewportHeight: cfg.Browser.ViewportHeight,
			MaxSessions:    cfg.Browser.MaxSessions,
		},
		Auth: cfg.Auth,
	}

	k, err := NewKernel(kernelConfig)
	if err != nil {
		return nil, err
	}

	if err := k.ReloadSecrets(); err != nil {
		log.Warn().Err(err).Msg("Failed to load global secrets")
	}
	// install embedded skills for owner (global kernel)
	if err := k.SkillLoader().InstallAllEmbeddedSkills(); err != nil {
		log.Warn().Err(err).Msg("Failed to install embedded skills for owner")
	}

	return k, nil
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

	var savedCrons []types.CronJob
	var oldAgentID types.AgentID

	existingAgent := k.agentRegistry.FindByName(def.Agent.Name)
	if existingAgent != nil {
		oldAgentID = existingAgent.ID
		savedCrons = k.cronScheduler.TakeAgentJobs(oldAgentID)
		log.Info().Str("old_agent", oldAgentID.String()).Int("saved_crons", len(savedCrons)).Msg("Saved cron jobs before reactivating hand")

		if _, err := k.agentRegistry.Remove(oldAgentID); err != nil {
			log.Warn().Err(err).Str("old_agent", oldAgentID.String()).Msg("Failed to remove old agent")
		}
		k.agentRuntime.DeleteAgent(oldAgentID.String())
	}

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

	// Resolve hand settings into prompt block
	resolved := hands.ResolveSettings(def.Settings, handConfig)

	// Build the system prompt
	systemPrompt := def.Agent.SystemPrompt
	if resolved.PromptBlock != "" {
		systemPrompt = fmt.Sprintf("%s\n\n---\n\n%s", systemPrompt, resolved.PromptBlock)
	}

	manifest := types.AgentManifest{
		Name:         def.Agent.Name,
		Description:  def.Agent.Description,
		SystemPrompt: systemPrompt,
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
	agentSystemPrompt := systemPrompt // Use the system prompt with user config
	agentTools := def.Tools
	agentSkillPromptContext := ""

	if hand, _ := hands.GetBundledHand(handID); hand != nil {
		agentSkillPromptContext = hand.SkillContent
	}

	k.mu.Unlock()

	if len(savedCrons) > 0 {
		restoredCount := 0
		for _, job := range savedCrons {
			job.AgentID = agentID
			job.NextRun = nil
			job.LastRun = nil
			job.Enabled = true
			if _, err := k.cronScheduler.AddJob(job, false); err == nil {
				restoredCount++
			} else {
				log.Warn().Err(err).Str("job_id", job.ID.String()).Msg("Failed to restore cron job")
			}
		}
		log.Info().Str("agent", agentID.String()).Int("restored", restoredCount).Msg("Restored cron jobs after hand reactivation")
		if err := k.cronScheduler.Persist(); err != nil {
			log.Warn().Err(err).Msg("Failed to persist cron jobs after restoration")
		}
	}

	apiKey := k.GetSecret(agentAPIKeyEnv)
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
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register agent in runtime: %w", err)
	}
	// Send initialization message to agent for schedule setup
	go func() {
		time.Sleep(2 * time.Second)
		initMessage := "Please initialize by setting up any scheduled tasks according to the Schedule Management instructions in your system prompt. Use schedule_list first to check for existing schedules, then create any new schedules as needed."
		ctx := context.Background()
		if _, err := k.SendMessage(ctx, agentID.String(), initMessage); err != nil {
			log.Warn().Err(err).Str("agent", agentID.String()).Msg("Failed to send initialization message to agent")
		} else {
			log.Info().Str("agent", agentID.String()).Msg("Sent initialization message to agent for schedule setup")
		}
	}()

	k.mu.Lock()
	updatedInstance, _ := k.handRegistry.GetInstance(instance.InstanceID)
	k.mu.Unlock()

	return updatedInstance, nil
}

// DeactivateHand deactivates a hand and kills the agent.
func (k *Kernel) DeactivateHand(instanceID string) error {
	k.mu.Lock()

	instance, ok := k.handRegistry.GetInstance(instanceID)
	if !ok {
		k.mu.Unlock()
		return fmt.Errorf("hand instance not found: %s", instanceID)
	}

	if err := k.handRegistry.DeactivateInstance(instanceID); err != nil {
		k.mu.Unlock()
		return err
	}

	var agentID types.AgentID
	var hasAgent bool
	if instance.AgentID != "" {
		if id, err := types.ParseAgentID(instance.AgentID); err == nil {
			agentID = id
			hasAgent = true
		}
	}

	k.mu.Unlock()
	if hasAgent {
		removedCount := k.cronScheduler.RemoveAgentJobs(agentID)
		log.Info().Str("agent", agentID.String()).Int("removed", removedCount).Msg("Removed cron jobs for deactivated hand")
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	if hasAgent {
		if _, err := k.agentRegistry.Remove(agentID); err != nil {
			log.Warn().Err(err).Str("agent", agentID.String()).Msg("Failed to remove agent")
		}
		k.agentRuntime.DeleteAgent(instance.AgentID)
	}

	return nil
}

// PauseHand pauses a hand instance.
func (k *Kernel) PauseHand(instanceID string) error {
	k.mu.Lock()

	instance, ok := k.handRegistry.GetInstance(instanceID)
	if !ok {
		k.mu.Unlock()
		return fmt.Errorf("hand instance not found: %s", instanceID)
	}

	if err := k.handRegistry.PauseInstance(instanceID); err != nil {
		k.mu.Unlock()
		return err
	}

	var agentID types.AgentID
	var hasAgent bool
	if instance.AgentID != "" {
		if id, err := types.ParseAgentID(instance.AgentID); err == nil {
			agentID = id
			hasAgent = true
		}
	}

	k.mu.Unlock()

	if hasAgent {
		disabledCount := k.cronScheduler.SetAgentJobsEnabled(agentID, false)
		log.Info().Str("agent", agentID.String()).Int("disabled", disabledCount).Msg("Disabled cron jobs for paused hand")
	}

	return nil
}

// ResumeHand resumes a paused hand instance.
func (k *Kernel) ResumeHand(instanceID string) error {
	k.mu.Lock()

	instance, ok := k.handRegistry.GetInstance(instanceID)
	if !ok {
		k.mu.Unlock()
		return fmt.Errorf("hand instance not found: %s", instanceID)
	}

	if err := k.handRegistry.ResumeInstance(instanceID); err != nil {
		k.mu.Unlock()
		return err
	}

	var agentID types.AgentID
	var hasAgent bool
	if instance.AgentID != "" {
		if id, err := types.ParseAgentID(instance.AgentID); err == nil {
			agentID = id
			hasAgent = true
		}
	}

	k.mu.Unlock()

	if hasAgent {
		enabledCount := k.cronScheduler.SetAgentJobsEnabled(agentID, true)
		log.Info().Str("agent", agentID.String()).Int("enabled", enabledCount).Msg("Enabled cron jobs for resumed hand")
	}

	return nil
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
	apiKey := k.GetSecret(agentAPIKeyEnv)
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
		nil,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to register agent in runtime: %w", err)
	}

	caps := manifestToCapabilities(manifest)
	k.capabilityMgr.Grant(agentID.String(), caps)

	k.auditLog.Record("system", agentID.String(), audit.ActionAgentSpawn, fmt.Sprintf("name=%s", agentName), "ok")

	event := eventbus.NewEvent(
		eventbus.EventTypeAgentCreated,
		agentID.String(),
		eventbus.EventTargetAgent,
	).WithPayload(map[string]interface{}{
		"name": agentName,
	})
	k.PublishEvent(event)

	event = eventbus.NewEvent(
		eventbus.EventTypeAgentStarted,
		agentID.String(),
		eventbus.EventTargetAgent,
	).WithPayload(map[string]interface{}{
		"name": agentName,
	})
	k.PublishEvent(event)

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

	var agentName string
	k.mu.Lock()
	entry := k.agentRegistry.Get(id)
	if entry != nil {
		agentName = entry.Name
	}
	_, err = k.agentRegistry.Remove(id)
	k.mu.Unlock()

	if err != nil {
		return err
	}

	k.agentRuntime.DeleteAgent(agentIDStr)

	k.capabilityMgr.RevokeAll(agentIDStr)

	event := eventbus.NewEvent(
		eventbus.EventTypeAgentDeleted,
		agentIDStr,
		eventbus.EventTargetAgent,
	).WithPayload(map[string]interface{}{
		"name": agentName,
	})
	k.PublishEvent(event)

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
			log.Warn().Str("server", serverConfig.Name).Err(err).Msg("Failed to connect to MCP server")
			continue
		}

		k.mcpConnections.Store(serverConfig.Name, conn)

		tools := conn.Tools()
		k.mcpTools.Store(serverConfig.Name, tools)

		log.Debug().Str("server", serverConfig.Name).Int("tools", len(tools)).Msg("Connected to MCP server")
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
	connected := []map[string]interface{}{}
	configured := []map[string]interface{}{}

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

		env := serverConfig.Env
		if env == nil {
			env = []string{}
		}

		transport := serverConfig.Transport
		if transport.Type == "" {
			transport.Type = "stdio"
		}

		configured = append(configured, map[string]interface{}{
			"name":      serverConfig.Name,
			"transport": transport,
			"env":       env,
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
			k.deliveryTracker.Record(agentID, deliv.FailedReceipt(agentID, *delivery.ChannelName, recipient, err.Error()))
		} else {
			k.deliveryTracker.Record(agentID, deliv.SentReceipt(agentID, *delivery.ChannelName, recipient))
		}

	case types.CronDeliveryKindLastChannel:
		store := memory.NewStructuredStore(k.db)
		raw, err := store.Get(agentID, "delivery.last_channel")
		if err != nil || raw == nil {
			log.Info().Str("agent", agentID.String()).Msg("Cron delivery: LastChannel — no previous channel recorded, skipping")
			return
		}
		kvMap, ok := raw.(map[string]interface{})
		if !ok {
			log.Warn().Str("agent", agentID.String()).Msg("Cron delivery: LastChannel — stored value is not a map, skipping")
			return
		}
		lastChannelName, _ := kvMap["channel"].(string)
		lastRecipient, _ := kvMap["recipient"].(string)
		if lastChannelName == "" || lastRecipient == "" {
			log.Warn().Str("agent", agentID.String()).Msg("Cron delivery: LastChannel — channel or recipient empty, skipping")
			return
		}

		channelList := k.registry.ListChannels()
		var lastTargetChannel *channels.Channel
		for _, ch := range channelList {
			if ch.Name == lastChannelName {
				lastTargetChannel = ch
				break
			}
		}
		if lastTargetChannel == nil {
			log.Warn().Str("channel", lastChannelName).Msg("Cron delivery: LastChannel — channel not found")
			return
		}
		adapter, ok := k.registry.GetAdapter(lastTargetChannel.ID)
		if !ok {
			log.Warn().Str("channel", lastTargetChannel.ID).Msg("Cron delivery: LastChannel — adapter not found")
			return
		}
		lastMsg := &channels.Message{
			ID:        fmt.Sprintf("cron_last_%d", time.Now().UnixNano()),
			ChannelID: lastTargetChannel.ID,
			Content:   response,
			Recipient: lastRecipient,
			CreatedAt: time.Now(),
		}
		if err := adapter.Send(lastMsg); err != nil {
			log.Warn().Err(err).Str("channel", lastTargetChannel.ID).Msg("Cron delivery: LastChannel — failed to send message")
			k.deliveryTracker.Record(agentID, deliv.FailedReceipt(agentID, lastChannelName, lastRecipient, err.Error()))
		} else {
			log.Info().Str("channel", lastChannelName).Str("recipient", lastRecipient).Msg("Cron delivery: LastChannel — message sent")
			k.deliveryTracker.Record(agentID, deliv.SentReceipt(agentID, lastChannelName, lastRecipient))
		}

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

// describeEventForHistory generates a human-readable description of an event.
func describeEventForHistory(event *eventbus.Event) string {
	switch event.Type {
	case eventbus.EventTypeAgentCreated:
		name, _ := event.Payload["name"].(string)
		return fmt.Sprintf("Agent created: %s (id: %s)", name, event.AgentID)
	case eventbus.EventTypeAgentStarted:
		name, _ := event.Payload["name"].(string)
		return fmt.Sprintf("Agent started: %s (id: %s)", name, event.AgentID)
	case eventbus.EventTypeAgentStopped:
		name, _ := event.Payload["name"].(string)
		return fmt.Sprintf("Agent stopped: %s (id: %s)", name, event.AgentID)
	case eventbus.EventTypeMessageReceived:
		channel, _ := event.Payload["channel"].(string)
		content, _ := event.Payload["content"].(string)
		if len(content) > 200 {
			content = content[:197] + "..."
		}
		return fmt.Sprintf("Message received on %s: %s", channel, content)
	case eventbus.EventTypeMessageSent:
		channel, _ := event.Payload["channel"].(string)
		content, _ := event.Payload["content"].(string)
		if len(content) > 200 {
			content = content[:197] + "..."
		}
		return fmt.Sprintf("Message sent to %s: %s", channel, content)
	case eventbus.EventTypeSystem:
		subtype, _ := event.Payload["subtype"].(string)
		if subtype != "" {
			return fmt.Sprintf("System event: %s", subtype)
		}
		return "System event"
	case eventbus.EventTypeTriggerFired:
		triggerID, _ := event.Payload["trigger_id"].(string)
		return fmt.Sprintf("Trigger fired: %s", triggerID)
	case eventbus.EventTypeWorkflowStarted:
		workflowID, _ := event.Payload["workflow_id"].(string)
		return fmt.Sprintf("Workflow started: %s", workflowID)
	case eventbus.EventTypeWorkflowCompleted:
		workflowID, _ := event.Payload["workflow_id"].(string)
		return fmt.Sprintf("Workflow completed: %s", workflowID)
	default:
		return fmt.Sprintf("Event: %s", event.Type)
	}
}

// PublishEvent publishes an event to the event bus and evaluates triggers.
// Returns a list of (agent_id, message) pairs that were triggered.
func (k *Kernel) PublishEvent(event *eventbus.Event) []triggers.MatchResult {
	// Evaluate triggers before publishing
	triggered := k.triggerEngine.Evaluate(event)

	// Publish to event bus
	k.eventBus.Publish(event)

	// Get event description for history
	eventDesc := describeEventForHistory(event)

	// Send triggered messages to agents in background
	for _, pair := range triggered {
		triggerID := pair.TriggerID.String()
		agentID := pair.AgentID
		message := pair.Message
		go func() {
			log.Info().
				Str("trigger_id", triggerID).
				Str("agent_id", agentID).
				Str("message", message).
				Msg("Sending triggered message to agent")

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			response, err := k.SendMessage(ctx, agentID, message)
			if err != nil {
				log.Warn().
					Err(err).
					Str("agent_id", agentID).
					Msg("Failed to send triggered message to agent")
				response = fmt.Sprintf("Error: %v", err)
			}

			// Get session ID for this agent
			var sessionID string
			// TODO: Implement session ID retrieval from agent entry if available

			// Save trigger history
			historyID := uuid.NewString()
			historyRecord := &memory.TriggerHistoryRecord{
				ID:               historyID,
				TriggerID:        triggerID,
				AgentID:          agentID,
				EventType:        string(event.Type),
				EventDescription: eventDesc,
				SentMessage:      message,
				AgentResponse:    response,
				SessionID:        sessionID,
				CreatedAt:        time.Now(),
			}

			if err := k.db.SaveTriggerHistory(historyRecord); err != nil {
				log.Warn().
					Err(err).
					Str("trigger_id", triggerID).
					Msg("Failed to save trigger history")
			}
		}()
	}

	return triggered
}

// GetAutoReplyEngine returns the auto-reply engine.
func (k *Kernel) GetAutoReplyEngine() *autoreply.AutoReplyEngine {
	return k.autoReplyEngine
}

// RecordDelivery records a channel delivery receipt and persists the last-channel info
// into the agent's KV store (for CronDelivery LastChannel feature).
func (k *Kernel) RecordDelivery(_ context.Context, agentIDStr, channel, recipient string, success bool, errMsg string) {
	agentID, err := types.ParseAgentID(agentIDStr)
	if err != nil {
		log.Warn().Str("agent_id", agentIDStr).Msg("RecordDelivery: invalid agent ID, skipping")
		return
	}

	var receipt deliv.DeliveryReceipt
	if success {
		receipt = deliv.SentReceipt(agentID, channel, recipient)
		// Persist last successful channel for CronDelivery::LastChannel
		kv := map[string]interface{}{
			"channel":   channel,
			"recipient": recipient,
		}
		store := memory.NewStructuredStore(k.db)
		if err := store.Set(agentID, "delivery.last_channel", kv); err != nil {
			log.Warn().Err(err).Str("agent", agentIDStr).Msg("RecordDelivery: failed to persist last_channel")
		}
	} else {
		receipt = deliv.FailedReceipt(agentID, channel, recipient, errMsg)
	}

	k.deliveryTracker.Record(agentID, receipt)
}

func manifestToCapabilities(manifest types.AgentManifest) []capabilities.Capability {
	var caps []capabilities.Capability

	if manifest.Capabilities == nil {
		return caps
	}

	mc := manifest.Capabilities

	for _, pattern := range mc.Network {
		caps = append(caps, capabilities.Capability{
			Type:     capabilities.CapNetConnect,
			Resource: pattern,
		})
	}

	for _, pattern := range mc.Shell {
		caps = append(caps, capabilities.Capability{
			Type:     capabilities.CapShellExec,
			Resource: pattern,
		})
	}

	for _, pattern := range mc.MemoryRead {
		caps = append(caps, capabilities.Capability{
			Type:     capabilities.CapMemoryRead,
			Resource: pattern,
		})
	}

	for _, pattern := range mc.MemoryWrite {
		caps = append(caps, capabilities.Capability{
			Type:     capabilities.CapMemoryWrite,
			Resource: pattern,
		})
	}

	if mc.AgentSpawn {
		caps = append(caps, capabilities.Capability{
			Type: capabilities.CapAgentSpawn,
		})
	}

	for _, pattern := range mc.AgentMessage {
		caps = append(caps, capabilities.Capability{
			Type:     capabilities.CapAgentMessage,
			Resource: pattern,
		})
	}

	if mc.Schedule {
		caps = append(caps, capabilities.Capability{
			Type: capabilities.CapSchedule,
		})
	}

	for _, toolID := range manifest.Tools {
		caps = append(caps, capabilities.Capability{
			Type:     capabilities.CapToolInvoke,
			Resource: toolID,
		})
	}

	return caps
}
