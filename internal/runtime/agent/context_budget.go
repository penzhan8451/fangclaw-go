package agent

import (
	"fmt"
	"strings"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// ContextBudget manages the dynamic context budget for tool result truncation.
type ContextBudget struct {
	ContextWindowTokens  int
	ToolCharsPerToken    float64
	GeneralCharsPerToken float64
}

// NewContextBudget creates a new ContextBudget with default values.
func NewContextBudget(contextWindowTokens int) *ContextBudget {
	return &ContextBudget{
		ContextWindowTokens:  contextWindowTokens,
		ToolCharsPerToken:    2.0,
		GeneralCharsPerToken: 4.0,
	}
}

// DefaultContextBudget creates a default context budget (200K tokens).
func DefaultContextBudget() *ContextBudget {
	return NewContextBudget(200000)
}

// PerResultCap returns the per-result character cap (30% of context window).
func (b *ContextBudget) PerResultCap() int {
	tokensForTool := int(float64(b.ContextWindowTokens) * 0.30)
	return int(float64(tokensForTool) * b.ToolCharsPerToken)
}

// SingleResultMax returns the single result absolute max (50% of context window).
func (b *ContextBudget) SingleResultMax() int {
	tokens := int(float64(b.ContextWindowTokens) * 0.50)
	return int(float64(tokens) * b.ToolCharsPerToken)
}

// TotalToolHeadroomChars returns the total tool result headroom (75% of context window).
func (b *ContextBudget) TotalToolHeadroomChars() int {
	tokens := int(float64(b.ContextWindowTokens) * 0.75)
	return int(float64(tokens) * b.ToolCharsPerToken)
}

// TruncateToolResultDynamic truncates a single tool result dynamically based on budget.
func TruncateToolResultDynamic(content string, budget *ContextBudget) string {
	cap := budget.PerResultCap()
	if len(content) <= cap {
		return content
	}

	searchStart := cap - 200
	if searchStart < 0 {
		searchStart = 0
	}
	endSearch := cap
	if endSearch > len(content) {
		endSearch = len(content)
	}
	searchRange := content[searchStart:endSearch]
	breakPoint := strings.LastIndex(searchRange, "\n")

	var finalBreakPoint int
	if breakPoint != -1 {
		finalBreakPoint = searchStart + breakPoint
	} else {
		finalBreakPoint = cap - 100
		if finalBreakPoint < 0 {
			finalBreakPoint = 0
		}
		if finalBreakPoint > len(content) {
			finalBreakPoint = len(content)
		}
	}

	return fmt.Sprintf(
		"%s\n\n[TRUNCATED: result was %d chars, showing first %d (budget: 30%% of %dK context window)]",
		content[:finalBreakPoint],
		len(content),
		finalBreakPoint,
		budget.ContextWindowTokens/1000,
	)
}

// ToolResultLoc represents the location of a tool result in the message history.
type ToolResultLoc struct {
	MsgIdx   int
	BlockIdx int
	CharLen  int
}

// ApplyContextGuard applies context guard to compact tool results if needed.
func ApplyContextGuard(messages []types.Message, budget *ContextBudget) ([]types.Message, int) {
	headroom := budget.TotalToolHeadroomChars()

	locations := make([]ToolResultLoc, 0)
	totalChars := 0

	for msgIdx, msg := range messages {
		if msg.Role == "user" {
			len := len(msg.Content)
			totalChars += len
			locations = append(locations, ToolResultLoc{
				MsgIdx:  msgIdx,
				CharLen: len,
			})
		}
	}

	if totalChars <= headroom {
		return messages, 0
	}

	compacted := 0
	newMessages := make([]types.Message, len(messages))
	copy(newMessages, messages)

	compactTarget := 2000
	for _, loc := range locations {
		if totalChars <= headroom {
			break
		}
		if loc.CharLen <= compactTarget {
			continue
		}
		if newMessages[loc.MsgIdx].Role == "user" && len(newMessages[loc.MsgIdx].Content) > compactTarget {
			oldLen := len(newMessages[loc.MsgIdx].Content)
			newMessages[loc.MsgIdx].Content = TruncateTo(newMessages[loc.MsgIdx].Content, compactTarget)
			totalChars -= oldLen
			totalChars += len(newMessages[loc.MsgIdx].Content)
			compacted++
		}
	}

	return newMessages, compacted
}

// TruncateTo truncates content to max_chars with a marker.
func TruncateTo(content string, maxChars int) string {
	if len(content) <= maxChars {
		return content
	}
	keep := maxChars - 80
	if keep < 0 {
		keep = 0
	}

	searchStart := keep - 100
	if searchStart < 0 {
		searchStart = 0
	}
	endSearch := keep
	if endSearch > len(content) {
		endSearch = len(content)
	}
	searchRange := content[searchStart:endSearch]
	breakPoint := strings.LastIndex(searchRange, "\n")

	var finalBreakPoint int
	if breakPoint != -1 {
		finalBreakPoint = searchStart + breakPoint
	} else {
		finalBreakPoint = keep
		if finalBreakPoint < 0 {
			finalBreakPoint = 0
		}
		if finalBreakPoint > len(content) {
			finalBreakPoint = len(content)
		}
	}

	return fmt.Sprintf(
		"%s\n\n[COMPACTED: %d → %d chars by context guard]",
		content[:finalBreakPoint],
		len(content),
		finalBreakPoint,
	)
}
