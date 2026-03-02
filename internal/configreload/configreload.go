package configreload

import (
	"reflect"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

type HotAction int

const (
	HotActionReloadChannels HotAction = iota
	HotActionReloadSkills
	HotActionUpdateUsageFooter
	HotActionReloadWebConfig
	HotActionReloadBrowserConfig
	HotActionUpdateApprovalPolicy
	HotActionUpdateCronConfig
	HotActionUpdateWebhookConfig
	HotActionReloadExtensions
	HotActionReloadMcpServers
	HotActionReloadA2aConfig
	HotActionReloadFallbackProviders
	HotActionReloadProviderUrls
)

type ReloadMode int

const (
	ReloadModeHot ReloadMode = iota
	ReloadModeRestart
)

type ReloadPlan struct {
	RestartRequired bool
	HotActions      []HotAction
	ChangedFields   []string
}

func BuildReloadPlan(oldConfig, newConfig types.KernelConfig) *ReloadPlan {
	plan := &ReloadPlan{
		HotActions:    make([]HotAction, 0),
		ChangedFields: make([]string, 0),
	}

	if !reflect.DeepEqual(oldConfig.Skills, newConfig.Skills) {
		plan.HotActions = append(plan.HotActions, HotActionReloadSkills)
		plan.ChangedFields = append(plan.ChangedFields, "Skills")
	}

	if !reflect.DeepEqual(oldConfig.Extensions, newConfig.Extensions) {
		plan.HotActions = append(plan.HotActions, HotActionReloadExtensions)
		plan.ChangedFields = append(plan.ChangedFields, "Extensions")
	}

	if oldConfig.LogLevel != newConfig.LogLevel {
		plan.ChangedFields = append(plan.ChangedFields, "LogLevel")
	}

	if !reflect.DeepEqual(oldConfig.Include, newConfig.Include) {
		plan.ChangedFields = append(plan.ChangedFields, "Include")
	}

	if !reflect.DeepEqual(oldConfig.API, newConfig.API) ||
		!reflect.DeepEqual(oldConfig.Models, newConfig.Models) ||
		!reflect.DeepEqual(oldConfig.Memory, newConfig.Memory) ||
		!reflect.DeepEqual(oldConfig.Security, newConfig.Security) {
		plan.RestartRequired = true
	}

	return plan
}

func (rp *ReloadPlan) NeedsRestart() bool {
	return rp.RestartRequired
}

func (rp *ReloadPlan) HasHotActions() bool {
	return len(rp.HotActions) > 0
}

func (rp *ReloadPlan) GetHotActions() []HotAction {
	return rp.HotActions
}

func (rp *ReloadPlan) GetChangedFields() []string {
	return rp.ChangedFields
}
