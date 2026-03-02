package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const (
	DefaultWarnThreshold        = 3
	DefaultBlockThreshold       = 5
	DefaultGlobalCircuitBreaker = 30
	DefaultPollMultiplier       = 3
	HistorySize                 = 30
)

// LoopGuardVerdict represents the verdict from the loop guard.
type LoopGuardVerdict string

const (
	LoopGuardVerdictAllow        LoopGuardVerdict = "allow"
	LoopGuardVerdictWarn         LoopGuardVerdict = "warn"
	LoopGuardVerdictBlock        LoopGuardVerdict = "block"
	LoopGuardVerdictCircuitBreak LoopGuardVerdict = "circuit_break"
)

// LoopGuardConfig configures the loop guard behavior.
type LoopGuardConfig struct {
	WarnThreshold        uint32
	BlockThreshold       uint32
	GlobalCircuitBreaker uint32
	PollMultiplier       uint32
}

// DefaultLoopGuardConfig returns the default loop guard configuration.
func DefaultLoopGuardConfig() LoopGuardConfig {
	return LoopGuardConfig{
		WarnThreshold:        DefaultWarnThreshold,
		BlockThreshold:       DefaultBlockThreshold,
		GlobalCircuitBreaker: DefaultGlobalCircuitBreaker,
		PollMultiplier:       DefaultPollMultiplier,
	}
}

// LoopGuard tracks tool calls to detect and prevent loops.
type LoopGuard struct {
	config       LoopGuardConfig
	callCounts   map[string]uint32
	totalCalls   uint32
	recentCalls  []string
	hashToTool   map[string]string
	blockedCalls uint32
}

// NewLoopGuard creates a new loop guard with default configuration.
func NewLoopGuard() *LoopGuard {
	return NewLoopGuardWithConfig(DefaultLoopGuardConfig())
}

// NewLoopGuardWithConfig creates a new loop guard with custom configuration.
func NewLoopGuardWithConfig(config LoopGuardConfig) *LoopGuard {
	return &LoopGuard{
		config:      config,
		callCounts:  make(map[string]uint32),
		recentCalls: make([]string, 0, HistorySize),
		hashToTool:  make(map[string]string),
	}
}

// Check checks if a tool call should proceed.
func (g *LoopGuard) Check(toolName string, params map[string]interface{}) (LoopGuardVerdict, string) {
	g.totalCalls++

	if g.totalCalls > g.config.GlobalCircuitBreaker {
		g.blockedCalls++
		return LoopGuardVerdictCircuitBreak, fmt.Sprintf(
			"Circuit breaker: exceeded %d total tool calls in this loop. The agent appears to be stuck.",
			g.config.GlobalCircuitBreaker,
		)
	}

	hash := g.computeHash(toolName, params)
	g.hashToTool[hash] = toolName

	if len(g.recentCalls) >= HistorySize {
		g.recentCalls = g.recentCalls[1:]
	}
	g.recentCalls = append(g.recentCalls, hash)

	count := g.callCounts[hash]
	count++
	g.callCounts[hash] = count

	effectiveWarn := g.config.WarnThreshold
	effectiveBlock := g.config.BlockThreshold

	if count >= effectiveBlock {
		g.blockedCalls++
		return LoopGuardVerdictBlock, fmt.Sprintf(
			"Blocked: tool '%s' called %d times with identical parameters. Try a different approach or different parameters.",
			toolName, count,
		)
	}

	if count >= effectiveWarn {
		return LoopGuardVerdictWarn, fmt.Sprintf(
			"Warning: tool '%s' has been called %d times with identical parameters. Consider a different approach.",
			toolName, count,
		)
	}

	return LoopGuardVerdictAllow, ""
}

// computeHash computes a SHA-256 hash of the tool name and parameters.
func (g *LoopGuard) computeHash(toolName string, params map[string]interface{}) string {
	hasher := sha256.New()
	hasher.Write([]byte(toolName))
	hasher.Write([]byte("|"))

	paramsJSON, _ := json.Marshal(params)
	hasher.Write(paramsJSON)

	return hex.EncodeToString(hasher.Sum(nil))
}

// Stats returns statistics about the loop guard state.
type LoopGuardStats struct {
	TotalCalls   uint32
	UniqueCalls  uint32
	BlockedCalls uint32
}

// Stats returns the current loop guard statistics.
func (g *LoopGuard) Stats() LoopGuardStats {
	return LoopGuardStats{
		TotalCalls:   g.totalCalls,
		UniqueCalls:  uint32(len(g.callCounts)),
		BlockedCalls: g.blockedCalls,
	}
}
