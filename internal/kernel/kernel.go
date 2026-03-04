package kernel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/penzhan8451/fangclaw-go/internal/approvals"
	"github.com/penzhan8451/fangclaw-go/internal/channels"
	"github.com/penzhan8451/fangclaw-go/internal/configreload"
	"github.com/penzhan8451/fangclaw-go/internal/cron"
	"github.com/penzhan8451/fangclaw-go/internal/delivery"
	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
	"github.com/penzhan8451/fangclaw-go/internal/hands"
	"github.com/penzhan8451/fangclaw-go/internal/memory"
	"github.com/penzhan8451/fangclaw-go/internal/pairing"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/model_catalog"
	"github.com/penzhan8451/fangclaw-go/internal/skills"
	"github.com/penzhan8451/fangclaw-go/internal/triggers"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

type Kernel struct {
	config         types.KernelConfig
	eventBus       *eventbus.EventBus
	scheduler      *Scheduler
	cronScheduler  *cron.CronScheduler
	modelCatalog   *model_catalog.ModelCatalog
	db             *memory.DB
	semantic       *memory.SemanticStore
	sessions       *memory.SessionStore
	knowledge      *memory.KnowledgeStore
	usage          *memory.UsageStore
	skillLoader    *skills.Loader
	registry       *channels.Registry
	agentRegistry  *AgentRegistry
	handRegistry   *hands.Registry
	triggerEngine  *triggers.TriggerEngine
	approvalMgr    *approvals.ApprovalManager
	deliveryReg    *delivery.DeliveryRegistry
	pairingManager *pairing.PairingManager
	workflowEngine *WorkflowEngine
	mu             sync.RWMutex
	started        bool
}

func NewKernel(config types.KernelConfig) (*Kernel, error) {
	dataDir, err := expandPath(config.DataDir)
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

	cronPersistDir := filepath.Join(dataDir, "cron")
	cronScheduler := cron.NewCronScheduler(cronPersistDir, nil)

	modelCatalog := model_catalog.NewModelCatalog()
	workflowEngine := NewWorkflowEngine()

	config.DataDir = dataDir

	return &Kernel{
		config:         config,
		eventBus:       eventbus.NewEventBus(),
		scheduler:      NewScheduler(),
		cronScheduler:  cronScheduler,
		modelCatalog:   modelCatalog,
		db:             db,
		semantic:       semanticStore,
		sessions:       sessionStore,
		knowledge:      knowledgeStore,
		usage:          usageStore,
		skillLoader:    skillLoader,
		registry:       registry,
		agentRegistry:  agentRegistry,
		handRegistry:   handRegistry,
		triggerEngine:  triggerEngine,
		approvalMgr:    approvalMgr,
		deliveryReg:    deliveryReg,
		pairingManager: pairingManager,
		workflowEngine: workflowEngine,
	}, nil
}

func (k *Kernel) Start(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.started {
		return fmt.Errorf("kernel already started")
	}

	if err := k.cronScheduler.Start(); err != nil {
		return fmt.Errorf("failed to start cron scheduler: %w", err)
	}

	k.modelCatalog.DetectAuth()

	k.started = true

	event := eventbus.NewEvent(eventbus.EventTypeSystem, "kernel", eventbus.EventTargetSystem)
	k.eventBus.Publish(event)

	return nil
}

func (k *Kernel) Stop(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.started {
		return nil
	}

	k.cronScheduler.Stop()
	k.scheduler.Shutdown()
	_ = k.registry.DisconnectAll()
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

func (k *Kernel) SkillLoader() *skills.Loader {
	return k.skillLoader
}

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
func (k *Kernel) ActivateHand(handID string, config map[string]interface{}) (*hands.HandInstance, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	def, ok := k.handRegistry.GetDefinition(handID)
	if !ok {
		return nil, fmt.Errorf("hand not found: %s", handID)
	}

	instance, err := k.handRegistry.ActivateHand(handID, def.Agent.Name, config)
	if err != nil {
		return nil, err
	}

	agentID := types.NewAgentID()
	manifest := types.AgentManifest{
		Name:         def.Agent.Name,
		Description:  def.Agent.Description,
		SystemPrompt: def.Agent.SystemPrompt,
		Model: types.ModelConfig{
			Provider:  def.Agent.Provider,
			Model:     def.Agent.Model,
			APIKeyEnv: def.Agent.APIKeyEnv,
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
		return nil, err
	}

	if err := k.handRegistry.UpdateInstanceAgent(instance.InstanceID, agentID.String()); err != nil {
		return nil, err
	}

	updatedInstance, _ := k.handRegistry.GetInstance(instance.InstanceID)
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
