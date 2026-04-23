package memory

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// MemoryReviewTask is a review task that analyzes and optimizes memories
type MemoryReviewTask struct {
	semanticStore *SemanticStore
	fileStore     *FileStore
	interval      time.Duration
}

// NewMemoryReviewTask creates a new MemoryReviewTask
func NewMemoryReviewTask(semanticStore *SemanticStore, fileStore *FileStore) *MemoryReviewTask {
	return &MemoryReviewTask{
		semanticStore: semanticStore,
		fileStore:     fileStore,
		interval:      1 * time.Hour, // Default: run every hour
	}
}

// Name returns the name of the task
func (mrt *MemoryReviewTask) Name() string {
	return "memory-review"
}

// Interval returns the interval at which the task should run
func (mrt *MemoryReviewTask) Interval() time.Duration {
	return mrt.interval
}

// Run executes the memory review task
func (mrt *MemoryReviewTask) Run(ctx context.Context) error {
	log.Println("Starting memory review task...")
	results := []string{}

	// Review semantic memory
	if mrt.semanticStore != nil {
		semanticResult, err := mrt.reviewSemanticMemory(ctx)
		if err != nil {
			log.Printf("Semantic memory review failed: %v", err)
		} else {
			results = append(results, semanticResult)
		}
	}

	// Review file memory (MEMORY.md and USER.md)
	if mrt.fileStore != nil {
		fileResult := mrt.reviewFileMemory()
		results = append(results, fileResult)
	}

	log.Println("Memory review task completed")
	if len(results) > 0 {
		log.Printf("Review results:\n%s", strings.Join(results, "\n\n"))
	}

	return nil
}

// reviewSemanticMemory reviews semantic memory for optimization
func (mrt *MemoryReviewTask) reviewSemanticMemory(ctx context.Context) (string, error) {
	log.Println("Reviewing semantic memory...")

	// Get all memories (in a real implementation, you might want to limit this)
	// For now, we'll just log that we're doing semantic review
	// Later we can add duplicate detection, summarization, etc.

	// Check for potential duplicates (simple heuristic: similar content)
	// This is a placeholder implementation

	return "Semantic memory review completed (placeholder implementation)", nil
}

// reviewFileMemory reviews file memory for potential improvements
func (mrt *MemoryReviewTask) reviewFileMemory() string {
	log.Println("Reviewing file memory...")
	var suggestions []string

	// Check MEMORY.md
	memContent, err := mrt.fileStore.Read("memory")
	if err == nil && memContent != "" {
		suggestions = append(suggestions, mrt.analyzeFileMemoryContent("MEMORY.md", memContent))
	}

	// Check USER.md
	userContent, err := mrt.fileStore.Read("user")
	if err == nil && userContent != "" {
		suggestions = append(suggestions, mrt.analyzeFileMemoryContent("USER.md", userContent))
	}

	if len(suggestions) == 0 {
		return "File memory review completed - no suggestions"
	}

	return fmt.Sprintf("File memory review completed:\n\n%s", strings.Join(suggestions, "\n\n"))
}

// analyzeFileMemoryContent analyzes file memory content for improvements
func (mrt *MemoryReviewTask) analyzeFileMemoryContent(filename, content string) string {
	lines := strings.Split(content, "\n")
	var suggestions []string

	// Simple heuristics for suggestions
	if len(lines) > 50 {
		suggestions = append(suggestions, "  - Consider organizing into sections or splitting into multiple entries")
	}

	// Check for obvious duplicates (simple heuristic)
	seenLines := make(map[string]bool)
	duplicates := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if seenLines[line] {
			duplicates++
		}
		seenLines[line] = true
	}
	if duplicates > 3 {
		suggestions = append(suggestions, fmt.Sprintf("  - Found %d duplicate lines, consider removing redundancy", duplicates))
	}

	if len(suggestions) == 0 {
		return fmt.Sprintf("%s: No suggestions for improvement", filename)
	}

	return fmt.Sprintf("%s:\n%s", filename, strings.Join(suggestions, "\n"))
}

// SetInterval sets the interval for the task
func (mrt *MemoryReviewTask) SetInterval(interval time.Duration) {
	mrt.interval = interval
}
