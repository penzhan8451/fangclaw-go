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
	"github.com/penzhan8451/fangclaw-go/internal/delivery"
	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
	"github.com/penzhan8451/fangclaw-go/internal/memory"
	"github.com/penzhan8451/fangclaw-go/internal/pairing"
	"github.com/penzhan8451/fangclaw-go/internal/skills"
	"github.com/penzhan8451/fangclaw-go/internal/triggers"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

type Kernel struct {
	config         types.KernelConfig
	eventBus       *eventbus.EventBus
	scheduler      *Scheduler
	db             *memory.DB
	semantic       *memory.SemanticStore
	sessions       *memory.SessionStore
	knowledge      *memory.KnowledgeStore
	skillLoader    *skills.Loader
	registry       *channels.Registry
	agentRegistry  *AgentRegistry
	triggerEngine  *triggers.TriggerEngine
	approvalMgr    *approvals.ApprovalManager
	deliveryReg    *delivery.DeliveryRegistry
	pairingManager *pairing.PairingManager
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

	skillsPath := filepath.Join(dataDir, "skills")
	skillLoader, err := skills.NewLoader(skillsPath)
	if err != nil {
		semanticStore.Close()
		sessionStore.Close()
		db.Close()
		return nil, fmt.Errorf("failed to create skill loader: %w", err)
	}

	registry := channels.NewRegistry()
	agentRegistry := NewAgentRegistry()
	triggerEngine := triggers.NewTriggerEngine()
	approvalPolicy := approvals.DefaultApprovalPolicy()
	approvalMgr := approvals.NewApprovalManager(approvalPolicy)
	deliveryReg := delivery.NewDeliveryRegistry()
	pairingConfig := pairing.PairingConfig{
		Enabled:    true,
		MaxDevices: 10,
	}
	pairingManager := pairing.NewPairingManager(pairingConfig)

	return &Kernel{
		config:         config,
		eventBus:       eventbus.NewEventBus(),
		scheduler:      NewScheduler(),
		db:             db,
		semantic:       semanticStore,
		sessions:       sessionStore,
		knowledge:      knowledgeStore,
		skillLoader:    skillLoader,
		registry:       registry,
		agentRegistry:  agentRegistry,
		triggerEngine:  triggerEngine,
		approvalMgr:    approvalMgr,
		deliveryReg:    deliveryReg,
		pairingManager: pairingManager,
	}, nil
}

func (k *Kernel) Start(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.started {
		return fmt.Errorf("kernel already started")
	}

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

func (k *Kernel) SkillLoader() *skills.Loader {
	return k.skillLoader
}

func (k *Kernel) Registry() *channels.Registry {
	return k.registry
}

func (k *Kernel) AgentRegistry() *AgentRegistry {
	return k.agentRegistry
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
