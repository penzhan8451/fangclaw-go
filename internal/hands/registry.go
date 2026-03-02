// Package hands provides autonomous capability packages (Hands) for OpenFang.
package hands

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Registry manages Hands.
type Registry struct {
	mu           sync.RWMutex
	hands        map[string]*Hand
	definitions  map[string]*HandDefinition
	instances    map[string]*HandInstance
	factories    map[string]HandFactory
	approvalGate *ApprovalGate
	handSkills   map[string]string
}

// NewRegistry creates a new Hand registry.
func NewRegistry() *Registry {
	r := &Registry{
		hands:        make(map[string]*Hand),
		definitions:  make(map[string]*HandDefinition),
		instances:    make(map[string]*HandInstance),
		factories:    make(map[string]HandFactory),
		approvalGate: NewApprovalGate(),
		handSkills:   make(map[string]string),
	}
	r.registerDefaultHands()
	r.loadBundledDefinitions()
	return r
}

// loadBundledDefinitions loads the bundled Hand definitions.
func (r *Registry) loadBundledDefinitions() {
	for _, def := range GetBundledHands() {
		r.definitions[def.ID] = def
	}
}

// GetApprovalGate returns the approval gate.
func (r *Registry) GetApprovalGate() *ApprovalGate {
	return r.approvalGate
}

// LoadHandsFromDirectory loads Hands from a directory.
func (r *Registry) LoadHandsFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			handDir := filepath.Join(dir, entry.Name())
			manifestPath := filepath.Join(handDir, "HAND.toml")
			if _, err := os.Stat(manifestPath); err == nil {
				if err := r.LoadHandFromDirectory(handDir); err != nil {
					continue
				}
			}
		}
	}

	return nil
}

// LoadHandFromDirectory loads a single Hand from a directory.
func (r *Registry) LoadHandFromDirectory(dir string) error {
	manifest, err := LoadHandManifestFromDir(dir)
	if err != nil {
		return err
	}

	hand := ManifestToHand(manifest)
	r.RegisterHand(hand)

	skill, err := LoadSkill(dir)
	if err == nil {
		r.mu.Lock()
		r.handSkills[hand.ID] = skill
		r.mu.Unlock()
	}

	return nil
}

// GetSkill returns the skill documentation for a Hand.
func (r *Registry) GetSkill(handID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	skill, ok := r.handSkills[handID]
	return skill, ok
}

// registerDefaultHands registers the default Hands.
func (r *Registry) registerDefaultHands() {
	// Register Clip Hand
	r.RegisterHand(&Hand{
		ID:          "clip",
		Name:        "Clip",
		Description: "Takes a YouTube URL, downloads it, identifies the best moments, cuts them into vertical shorts with captions and thumbnails, optionally adds AI voice-over, and publishes to Telegram and WhatsApp.",
		Category:    HandCategoryContent,
		Icon:        "🎬",
		State:       HandStateIdle,
		Config: HandConfig{
			Tools: []string{"youtube", "ffmpeg", "caption", "publish"},
			Settings: map[string]string{
				"output_format": "vertical",
				"max_duration":  "60",
			},
		},
	})

	// Register Lead Hand
	r.RegisterHand(&Hand{
		ID:          "lead",
		Name:        "Lead",
		Description: "Runs daily. Discovers prospects matching your ICP, enriches them with web research, scores 0-100, deduplicates against your existing database, and delivers qualified leads in CSV/JSON/Markdown.",
		Category:    HandCategoryProductivity,
		Icon:        "🎯",
		State:       HandStateIdle,
		Schedule:    "0 6 * * *",
		Config: HandConfig{
			Tools: []string{"search", "enrich", "score", "export"},
			Settings: map[string]string{
				"icp":           "",
				"output_format": "csv",
			},
		},
	})

	// Register Collector Hand
	r.RegisterHand(&Hand{
		ID:          "collector",
		Name:        "Collector",
		Description: "OSINT-grade intelligence. You give it a target (company, person, topic). It monitors continuously — change detection, sentiment tracking, knowledge graph construction, and critical alerts when something important shifts.",
		Category:    HandCategoryData,
		Icon:        "🔍",
		State:       HandStateIdle,
		Config: HandConfig{
			Tools: []string{"monitor", "analyze", "alert"},
			Settings: map[string]string{
				"target":         "",
				"check_interval": "3600",
			},
		},
	})

	// Register Predictor Hand
	r.RegisterHand(&Hand{
		ID:          "predictor",
		Name:        "Predictor",
		Description: "Superforecasting engine. Collects signals from multiple sources, builds calibrated reasoning chains, makes predictions with confidence intervals, and tracks its own accuracy using Brier scores.",
		Category:    HandCategoryData,
		Icon:        "📊",
		State:       HandStateIdle,
		Config: HandConfig{
			Tools: []string{"collect", "analyze", "predict"},
			Settings: map[string]string{
				"question":  "",
				"timeframe": "30d",
			},
		},
	})

	// Register Researcher Hand
	r.RegisterHand(&Hand{
		ID:          "researcher",
		Name:        "Researcher",
		Description: "Deep autonomous researcher. Cross-references multiple sources, evaluates credibility using CRAAP criteria, generates cited reports with APA formatting, supports multiple languages.",
		Category:    HandCategoryProductivity,
		Icon:        "📚",
		State:       HandStateIdle,
		Config: HandConfig{
			Tools: []string{"search", "fetch", "analyze", "cite"},
			Settings: map[string]string{
				"topic":  "",
				"format": "apa",
			},
		},
	})

	// Register Twitter Hand
	r.RegisterHand(&Hand{
		ID:          "twitter",
		Name:        "Twitter",
		Description: "Autonomous Twitter/X account manager. Creates content in 7 rotating formats, schedules posts for optimal engagement, responds to mentions, tracks performance metrics.",
		Category:    HandCategoryCommunication,
		Icon:        "🐦",
		State:       HandStateIdle,
		Schedule:    "0 */4 * * *",
		Config: HandConfig{
			Tools: []string{"create", "schedule", "respond", "analyze"},
			Settings: map[string]string{
				"account":       "",
				"content_types": "thread,quote,image,link,question,poll,thread",
			},
		},
	})

	// Register Browser Hand
	r.RegisterHand(&Hand{
		ID:          "browser",
		Name:        "Browser",
		Description: "Web automation agent. Navigates sites, fills forms, clicks buttons, handles multi-step workflows. Uses Playwright bridge with session persistence.",
		Category:    HandCategoryProductivity,
		Icon:        "🌐",
		State:       HandStateIdle,
		Config: HandConfig{
			Tools: []string{"navigate", "fill", "click", "extract"},
			Settings: map[string]string{
				"headless": "true",
				"timeout":  "300",
			},
		},
	})
}

// RegisterHand registers a Hand.
func (r *Registry) RegisterHand(hand *Hand) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if hand.ID == "" {
		hand.ID = uuid.New().String()
	}
	r.hands[hand.ID] = hand
}

// Get returns a Hand by ID.
func (r *Registry) Get(id string) (*Hand, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	hand, ok := r.hands[id]
	return hand, ok
}

// List returns all Hands.
func (r *Registry) List() []*Hand {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hands := make([]*Hand, 0, len(r.hands))
	for _, hand := range r.hands {
		hands = append(hands, hand)
	}
	return hands
}

// Remove removes a Hand by ID.
func (r *Registry) Remove(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.hands[id]; ok {
		delete(r.hands, id)
		return true
	}
	return false
}

// Count returns the number of Hands.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.hands)
}

// Activate activates a Hand.
func (r *Registry) Activate(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	hand, ok := r.hands[id]
	if !ok {
		return nil
	}

	hand.State = HandStateRunning
	now := time.Now()
	hand.LastRun = &now
	return nil
}

// Pause pauses a Hand.
func (r *Registry) Pause(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	hand, ok := r.hands[id]
	if !ok {
		return nil
	}

	hand.State = HandStatePaused
	return nil
}

// Stop stops a Hand.
func (r *Registry) Stop(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	hand, ok := r.hands[id]
	if !ok {
		return nil
	}

	hand.State = HandStateIdle
	return nil
}

// RegisterFactory registers a Hand factory.
func (r *Registry) RegisterFactory(id string, factory HandFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[id] = factory
}

// GetFactory returns a Hand factory by ID.
func (r *Registry) GetFactory(id string) (HandFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.factories[id]
	return factory, ok
}

// RegisterDefinition registers a Hand definition.
func (r *Registry) RegisterDefinition(def *HandDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.definitions[def.ID] = def
}

// GetDefinition returns a Hand definition by ID.
func (r *Registry) GetDefinition(id string) (*HandDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.definitions[id]
	return def, ok
}

// ListDefinitions returns all Hand definitions.
func (r *Registry) ListDefinitions() []*HandDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]*HandDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		defs = append(defs, def)
	}
	return defs
}

// ActivateHand activates a Hand and creates an instance.
func (r *Registry) ActivateHand(handID, agentName string, config map[string]interface{}) (*HandInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.hands[handID]
	if !ok {
		return nil, ErrHandNotFound
	}

	now := time.Now()
	instance := &HandInstance{
		InstanceID:  uuid.New().String(),
		HandID:      handID,
		Status:      HandStatusActive,
		AgentName:   agentName,
		Config:      config,
		ActivatedAt: now,
		UpdatedAt:   now,
	}

	r.instances[instance.InstanceID] = instance

	if hand, ok := r.hands[handID]; ok {
		hand.State = HandStateRunning
		hand.LastRun = &now
	}

	return instance, nil
}

// GetInstance returns a Hand instance by ID.
func (r *Registry) GetInstance(instanceID string) (*HandInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	instance, ok := r.instances[instanceID]
	return instance, ok
}

// ListInstances returns all Hand instances.
func (r *Registry) ListInstances() []*HandInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances := make([]*HandInstance, 0, len(r.instances))
	for _, instance := range r.instances {
		instances = append(instances, instance)
	}
	return instances
}

// ListInstancesByHand returns Hand instances by Hand ID.
func (r *Registry) ListInstancesByHand(handID string) []*HandInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances := make([]*HandInstance, 0)
	for _, instance := range r.instances {
		if instance.HandID == handID {
			instances = append(instances, instance)
		}
	}
	return instances
}

// PauseInstance pauses a Hand instance.
func (r *Registry) PauseInstance(instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instance, ok := r.instances[instanceID]
	if !ok {
		return ErrHandNotFound
	}

	instance.Status = HandStatusPaused
	instance.UpdatedAt = time.Now()
	return nil
}

// ResumeInstance resumes a Hand instance.
func (r *Registry) ResumeInstance(instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instance, ok := r.instances[instanceID]
	if !ok {
		return ErrHandNotFound
	}

	instance.Status = HandStatusActive
	instance.UpdatedAt = time.Now()
	return nil
}

// DeactivateInstance deactivates a Hand instance.
func (r *Registry) DeactivateInstance(instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instance, ok := r.instances[instanceID]
	if !ok {
		return ErrHandNotFound
	}

	instance.Status = HandStatusInactive
	instance.UpdatedAt = time.Now()

	if hand, ok := r.hands[instance.HandID]; ok {
		hand.State = HandStateIdle
	}

	return nil
}

// UpdateInstanceAgent updates the agent ID for a Hand instance.
func (r *Registry) UpdateInstanceAgent(instanceID, agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instance, ok := r.instances[instanceID]
	if !ok {
		return ErrHandNotFound
	}

	instance.AgentID = agentID
	instance.UpdatedAt = time.Now()
	return nil
}
