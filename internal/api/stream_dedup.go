package api

import (
	"strings"
	"unicode"
)

const (
	MinDedupLength = 10
	DedupWindow    = 50
)

// StreamDedup detects duplicate content in streaming responses.
type StreamDedup struct {
	recentChunks     []string
	recentNormalized []string
}

// NewStreamDedup creates a new stream deduplicator.
func NewStreamDedup() *StreamDedup {
	return &StreamDedup{
		recentChunks:     make([]string, 0, DedupWindow),
		recentNormalized: make([]string, 0, DedupWindow),
	}
}

// IsDuplicate checks if text is a duplicate of recently sent content.
func (d *StreamDedup) IsDuplicate(text string) bool {
	if len(text) < MinDedupLength {
		return false
	}

	for _, chunk := range d.recentChunks {
		if chunk == text {
			return true
		}
	}

	normalized := normalize(text)
	for _, norm := range d.recentNormalized {
		if norm == normalized {
			return true
		}
	}

	return false
}

// RecordSent records text that was successfully sent.
func (d *StreamDedup) RecordSent(text string) {
	if len(text) < MinDedupLength {
		return
	}

	if len(d.recentChunks) >= DedupWindow {
		d.recentChunks = d.recentChunks[1:]
		d.recentNormalized = d.recentNormalized[1:]
	}

	d.recentChunks = append(d.recentChunks, text)
	d.recentNormalized = append(d.recentNormalized, normalize(text))
}

// Clear clears the deduplication window.
func (d *StreamDedup) Clear() {
	d.recentChunks = d.recentChunks[:0]
	d.recentNormalized = d.recentNormalized[:0]
}

// normalize normalizes text for fuzzy matching: lowercase + collapse whitespace.
func normalize(text string) string {
	var result strings.Builder
	result.Grow(len(text))
	lastWasSpace := false

	for _, ch := range text {
		if unicode.IsSpace(ch) {
			if !lastWasSpace {
				result.WriteRune(' ')
				lastWasSpace = true
			}
		} else {
			result.WriteRune(unicode.ToLower(ch))
			lastWasSpace = false
		}
	}

	return strings.TrimSpace(result.String())
}
