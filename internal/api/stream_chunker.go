package api

import (
	"strings"
)

const (
	DefaultMinChunkChars = 200
	DefaultMaxChunkChars = 2000
)

// StreamChunker implements Markdown-aware stream chunking.
type StreamChunker struct {
	buffer         string
	inCodeFence    bool
	fenceMarker    string
	minChunkChars  int
	maxChunkChars  int
}

// NewStreamChunker creates a new stream chunker with default values.
func NewStreamChunker() *StreamChunker {
	return NewStreamChunkerWithThresholds(DefaultMinChunkChars, DefaultMaxChunkChars)
}

// NewStreamChunkerWithThresholds creates a new stream chunker with custom thresholds.
func NewStreamChunkerWithThresholds(minChunkChars, maxChunkChars int) *StreamChunker {
	return &StreamChunker{
		buffer:        "",
		inCodeFence:   false,
		fenceMarker:   "",
		minChunkChars: minChunkChars,
		maxChunkChars: maxChunkChars,
	}
}

// Push adds new text to the buffer.
func (c *StreamChunker) Push(text string) {
	lines := strings.SplitAfter(text, "\n")
	for _, line := range lines {
		c.buffer += line
		
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if c.inCodeFence {
				if trimmed == "```" || strings.HasPrefix(trimmed, c.fenceMarker) {
					c.inCodeFence = false
					c.fenceMarker = ""
				}
			} else {
				c.inCodeFence = true
				c.fenceMarker = "```"
			}
		}
	}
}

// TryFlush attempts to flush a chunk from the buffer.
func (c *StreamChunker) TryFlush() *string {
	if len(c.buffer) < c.minChunkChars {
		return nil
	}

	if c.inCodeFence && len(c.buffer) < c.maxChunkChars {
		return nil
	}

	if c.inCodeFence && len(c.buffer) >= c.maxChunkChars {
		chunk := c.buffer
		chunk += "\n```\n"
		c.buffer = "```" + strings.TrimPrefix(c.fenceMarker, "```") + "\n"
		return &chunk
	}

	searchStart := c.minChunkChars
	searchEnd := min(len(c.buffer), c.maxChunkChars)
	searchRange := c.buffer[searchStart:searchEnd]

	if pos := strings.LastIndex(searchRange, "\n\n"); pos != -1 {
		breakAt := searchStart + pos + 2
		chunk := c.buffer[:breakAt]
		c.buffer = c.buffer[breakAt:]
		return &chunk
	}

	if pos := strings.LastIndex(searchRange, "\n"); pos != -1 {
		breakAt := searchStart + pos + 1
		chunk := c.buffer[:breakAt]
		c.buffer = c.buffer[breakAt:]
		return &chunk
	}

	for _, ending := range []string{". ", "! ", "? "} {
		if pos := strings.LastIndex(searchRange, ending); pos != -1 {
			breakAt := searchStart + pos + len(ending)
			chunk := c.buffer[:breakAt]
			c.buffer = c.buffer[breakAt:]
			return &chunk
		}
	}

	if len(c.buffer) >= c.maxChunkChars {
		breakAt := c.maxChunkChars
		chunk := c.buffer[:breakAt]
		c.buffer = c.buffer[breakAt:]
		return &chunk
	}

	return nil
}

// FlushRemaining flushes all remaining text from the buffer.
func (c *StreamChunker) FlushRemaining() *string {
	if c.buffer == "" {
		return nil
	}
	chunk := c.buffer
	c.buffer = ""
	return &chunk
}

// BufferedLen returns the current buffer length.
func (c *StreamChunker) BufferedLen() int {
	return len(c.buffer)
}

// IsInCodeFence returns whether we're currently inside a code fence.
func (c *StreamChunker) IsInCodeFence() bool {
	return c.inCodeFence
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
