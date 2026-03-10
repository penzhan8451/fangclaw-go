package kernel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/approvals"
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
	workflowEngine := NewWorkflowEngine()
	k := &Kernel{
		config:          kernelConfig,
		eventBus:        eventbus.NewEventBus(),
		scheduler:       NewScheduler(),
		cronScheduler:   cronScheduler,
		modelCatalog:    modelCatalog,
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
		startTime:       time.Now(),
		stopping:        make(chan struct{}),
	}

	mcpCallbacks := &agent.McpCallbacks{
		GetMcpTools: k.GetMcpTools,
		CallMcpTool: k.CallMcpTool,
	}

	agentRuntime := agent.NewRuntime(semanticStore, sessionStore, knowledgeStore, usageStore, skillLoader, embeddingDriver, modelCatalog, mcpCallbacks)

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
		}

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

	k.started = true

	event := eventbus.NewEvent(eventbus.EventTypeSystem, "kernel", eventbus.EventTargetSystem)
	k.eventBus.Publish(event)

	// Connect to MCP servers in background
	go k.ConnectMcpServers(context.Background())

	// Start cron tick loop
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		persistCounter := 0

		// Skip first tick
		<-ticker.C

		for {
			select {
			case <-ticker.C:
				select {
				case <-k.stopping:
					return
				default:
				}

				due := k.cronScheduler.DueJobs()
				for _, job := range due {
					jobID := job.ID
					jobName := job.Name

					log.Debug().Str("job", jobName).Msg("Cron: firing job")

					switch job.Action.Kind {
					case types.CronActionKindSystemEvent:
						if job.Action.Text != nil {
							log.Debug().Str("job", jobName).Msg("Cron: firing system event")
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
						log.Debug().Str("job", jobName).Str("agent", job.AgentID.String()).Msg("Cron: firing agent turn")
						// TODO: Implement AgentTurn action handling
						log.Warn().Str("job", jobName).Msg("Cron: AgentTurn action not yet implemented")
						k.cronScheduler.RecordFailure(jobID, "AgentTurn action not yet implemented")
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

func (k *Kernel) ModelCatalog() *model_catalog.ModelCatalog {
	return k.modelCatalog
}

func (k *Kernel) WorkflowEngine() *WorkflowEngine {
	return k.workflowEngine
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

	config := types.KernelConfig{
		DataDir: dataDir,
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
		return "", err
	}
	return result.Response, nil
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

		if err := k.agentRegistry.Register(entry); err != nil {
			return
		}

		agentName = manifest.Name
		agentSystemPrompt = manifest.SystemPrompt
		agentTools = manifest.Tools
		agentSkills = manifest.Skills
		agentSkillPromptContext = manifest.SkillPromptContext
	}()

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
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
}

// GetMcpServers returns all MCP servers.
func (k *Kernel) GetMcpServers() []McpServerInfo {
	var servers []McpServerInfo
	k.mcpConnections.Range(func(key, value interface{}) bool {
		if name, ok := key.(string); ok {
			servers = append(servers, McpServerInfo{
				Name:      name,
				Connected: true,
			})
		}
		return true
	})
	return servers
}
