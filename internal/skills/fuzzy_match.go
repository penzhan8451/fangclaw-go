package skills

import (
	"strings"
	"unicode"
)

type FuzzyMatchStrategy string

const (
	StrategyExact              FuzzyMatchStrategy = "exact"
	StrategyLineTrimmed        FuzzyMatchStrategy = "line_trimmed"
	StrategyWhitespaceNormalized FuzzyMatchStrategy = "whitespace_normalized"
	StrategyIndentationFlexible FuzzyMatchStrategy = "indentation_flexible"
	StrategyTrimmedBoundary    FuzzyMatchStrategy = "trimmed_boundary"
)

type FuzzyMatchResult struct {
	Success    bool
	NewContent string
	Count      int
	Strategy   FuzzyMatchStrategy
	Error      string
}

func FuzzyFindAndReplace(content, oldStr, newStr string) FuzzyMatchResult {
	if oldStr == "" {
		return FuzzyMatchResult{Success: false, Error: "old string cannot be empty"}
	}
	if oldStr == newStr {
		return FuzzyMatchResult{Success: false, Error: "old and new strings are identical"}
	}

	strategies := []struct {
		name FuzzyMatchStrategy
		fn   func(string, string) (int, int, bool)
	}{
		{StrategyExact, matchExact},
		{StrategyLineTrimmed, matchLineTrimmed},
		{StrategyWhitespaceNormalized, matchWhitespaceNormalized},
		{StrategyIndentationFlexible, matchIndentationFlexible},
		{StrategyTrimmedBoundary, matchTrimmedBoundary},
	}

	for _, strategy := range strategies {
		start, end, found := strategy.fn(content, oldStr)
		if found {
			newContent := content[:start] + newStr + content[end:]
			return FuzzyMatchResult{
				Success:    true,
				NewContent: newContent,
				Count:      1,
				Strategy:   strategy.name,
			}
		}
	}

	return FuzzyMatchResult{
		Success: false,
		Error:   "could not find a match using any strategy",
	}
}

func matchExact(content, pattern string) (int, int, bool) {
	idx := strings.Index(content, pattern)
	if idx == -1 {
		return 0, 0, false
	}
	return idx, idx + len(pattern), true
}

func matchLineTrimmed(content, pattern string) (int, int, bool) {
	patternLines := strings.Split(pattern, "\n")
	trimmedPatternLines := make([]string, len(patternLines))
	for i, line := range patternLines {
		trimmedPatternLines[i] = strings.TrimSpace(line)
	}
	trimmedPattern := strings.Join(trimmedPatternLines, "\n")

	contentLines := strings.Split(content, "\n")
	trimmedContentLines := make([]string, len(contentLines))
	for i, line := range contentLines {
		trimmedContentLines[i] = strings.TrimSpace(line)
	}

	return findInNormalizedLines(content, contentLines, trimmedContentLines, pattern, trimmedPattern)
}

func matchWhitespaceNormalized(content, pattern string) (int, int, bool) {
	normalize := func(s string) string {
		var result strings.Builder
		inSpace := false
		for _, r := range s {
			if unicode.IsSpace(r) {
				if !inSpace {
					result.WriteRune(' ')
					inSpace = true
				}
			} else {
				result.WriteRune(r)
				inSpace = false
			}
		}
		return result.String()
	}

	normPattern := normalize(pattern)
	normContent := normalize(content)

	idx := strings.Index(normContent, normPattern)
	if idx == -1 {
		return 0, 0, false
	}

	return mapNormPosToOrig(content, normContent, idx, idx+len(normPattern))
}

func matchIndentationFlexible(content, pattern string) (int, int, bool) {
	contentLines := strings.Split(content, "\n")
	strippedContentLines := make([]string, len(contentLines))
	for i, line := range contentLines {
		strippedContentLines[i] = strings.TrimLeftFunc(line, unicode.IsSpace)
	}

	patternLines := strings.Split(pattern, "\n")
	strippedPatternLines := make([]string, len(patternLines))
	for i, line := range patternLines {
		strippedPatternLines[i] = strings.TrimLeftFunc(line, unicode.IsSpace)
	}
	strippedPattern := strings.Join(strippedPatternLines, "\n")

	return findInNormalizedLines(content, contentLines, strippedContentLines, pattern, strippedPattern)
}

func matchTrimmedBoundary(content, pattern string) (int, int, bool) {
	patternLines := strings.Split(pattern, "\n")
	if len(patternLines) == 0 {
		return 0, 0, false
	}

	modifiedPatternLines := make([]string, len(patternLines))
	copy(modifiedPatternLines, patternLines)
	modifiedPatternLines[0] = strings.TrimSpace(modifiedPatternLines[0])
	if len(modifiedPatternLines) > 1 {
		modifiedPatternLines[len(modifiedPatternLines)-1] = strings.TrimSpace(modifiedPatternLines[len(modifiedPatternLines)-1])
	}
	modifiedPattern := strings.Join(modifiedPatternLines, "\n")

	contentLines := strings.Split(content, "\n")
	patternLineCount := len(patternLines)

	for i := 0; i <= len(contentLines)-patternLineCount; i++ {
		blockLines := contentLines[i : i+patternLineCount]
		checkLines := make([]string, len(blockLines))
		copy(checkLines, blockLines)
		checkLines[0] = strings.TrimSpace(checkLines[0])
		if len(checkLines) > 1 {
			checkLines[len(checkLines)-1] = strings.TrimSpace(checkLines[len(checkLines)-1])
		}
		if strings.Join(checkLines, "\n") == modifiedPattern {
			return calculateLinePositions(contentLines, i, i+patternLineCount, len(content))
		}
	}

	return 0, 0, false
}

func findInNormalizedLines(content string, contentLines, normContentLines []string, pattern, normPattern string) (int, int, bool) {
	patternNormLines := strings.Split(normPattern, "\n")
	numPatternLines := len(patternNormLines)

	for i := 0; i <= len(normContentLines)-numPatternLines; i++ {
		block := strings.Join(normContentLines[i:i+numPatternLines], "\n")
		if block == normPattern {
			return calculateLinePositions(contentLines, i, i+numPatternLines, len(content))
		}
	}

	return 0, 0, false
}

func calculateLinePositions(contentLines []string, startLine, endLine, contentLen int) (int, int, bool) {
	startPos := 0
	for i := 0; i < startLine; i++ {
		startPos += len(contentLines[i]) + 1
	}

	endPos := startPos
	for i := startLine; i < endLine; i++ {
		endPos += len(contentLines[i]) + 1
	}
	if endPos > 0 {
		endPos--
	}
	if endPos > contentLen {
		endPos = contentLen
	}

	return startPos, endPos, true
}

func mapNormPosToOrig(original, normalized string, normStart, normEnd int) (int, int, bool) {
	origToNorm := make([]int, len(original)+1)
	normPos := 0
	for i := 0; i < len(original); i++ {
		origToNorm[i] = normPos
		if i < len(normalized) && original[i] == normalized[normPos] {
			normPos++
		} else if original[i] == ' ' || original[i] == '\t' {
		} else {
			normPos++
		}
	}
	origToNorm[len(original)] = normPos

	origStart := -1
	for i := 0; i < len(origToNorm); i++ {
		if origToNorm[i] >= normStart {
			origStart = i
			break
		}
	}

	origEnd := len(original)
	for i := len(origToNorm) - 1; i >= 0; i-- {
		if origToNorm[i] <= normEnd {
			origEnd = i
			break
		}
	}

	if origStart == -1 {
		return 0, 0, false
	}

	return origStart, origEnd, true
}
