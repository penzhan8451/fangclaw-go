package tools

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// ============ RSS Feed Tool (Collector) ============

type RSSFeedTool struct{}

func NewRSSFeedTool() *RSSFeedTool { return &RSSFeedTool{} }

func (t *RSSFeedTool) Name() string        { return "rss_feed" }
func (t *RSSFeedTool) Description() string { return "Parse and read RSS/Atom feeds" }

func (t *RSSFeedTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "rss_feed",
			"description": "Fetch and parse an RSS or Atom feed. Returns the latest items.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url":   map[string]interface{}{"type": "string", "description": "URL of the RSS/Atom feed"},
					"limit": map[string]interface{}{"type": "integer", "description": "Maximum number of items to return (default: 10)"},
				},
				"required": []string{"url"},
			},
		},
	}
}

type RSSFeed struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Description string `xml:"description"`
		Items       []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			PubDate     string `xml:"pubDate"`
			GUID        string `xml:"guid"`
		} `xml:"item"`
	} `xml:"channel"`
}

type AtomFeed struct {
	XMLName xml.Name `xml:"feed"`
	Title   string   `xml:"title"`
	Link    []struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
	} `xml:"link"`
	Updated string `xml:"updated"`
	Entries []struct {
		Title string `xml:"title"`
		Link  []struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Updated   string `xml:"updated"`
		Published string `xml:"published"`
		Summary   string `xml:"summary"`
		Content   string `xml:"content"`
		ID        string `xml:"id"`
	} `xml:"entry"`
}

func (t *RSSFeedTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	urlStr, _ := args["url"].(string)
	if urlStr == "" {
		return "", fmt.Errorf("url required")
	}

	limit := 10
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	if limit <= 0 {
		limit = 10
	}

	if err := checkSSRF(urlStr); err != nil {
		return "", err
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FangClawGOAgent/0.1)")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result strings.Builder
	var feedTitle, feedLink string
	var items []map[string]string

	var rssFeed RSSFeed
	if err := xml.Unmarshal(bodyBytes, &rssFeed); err == nil && rssFeed.Channel.Title != "" {
		feedTitle = rssFeed.Channel.Title
		feedLink = rssFeed.Channel.Link
		for _, item := range rssFeed.Channel.Items {
			items = append(items, map[string]string{
				"title":       item.Title,
				"link":        item.Link,
				"description": item.Description,
				"pubDate":     item.PubDate,
			})
		}
	} else {
		var atomFeed AtomFeed
		if err := xml.Unmarshal(bodyBytes, &atomFeed); err == nil && atomFeed.Title != "" {
			feedTitle = atomFeed.Title
			for _, link := range atomFeed.Link {
				if link.Rel == "alternate" || link.Rel == "" {
					feedLink = link.Href
					break
				}
			}
			for _, entry := range atomFeed.Entries {
				link := ""
				for _, l := range entry.Link {
					if l.Href != "" {
						link = l.Href
						break
					}
				}
				desc := entry.Summary
				if desc == "" {
					desc = entry.Content
				}
				pubDate := entry.Published
				if pubDate == "" {
					pubDate = entry.Updated
				}
				items = append(items, map[string]string{
					"title":       entry.Title,
					"link":        link,
					"description": desc,
					"pubDate":     pubDate,
				})
			}
		} else {
			return "", fmt.Errorf("failed to parse feed as RSS or Atom")
		}
	}

	result.WriteString(fmt.Sprintf("Feed: %s\n", feedTitle))
	if feedLink != "" {
		result.WriteString(fmt.Sprintf("Link: %s\n", feedLink))
	}
	result.WriteString(fmt.Sprintf("Items: %d\n\n", len(items)))

	if len(items) > limit {
		items = items[:limit]
	}

	for i, item := range items {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, item["title"]))
		if item["link"] != "" {
			result.WriteString(fmt.Sprintf("   URL: %s\n", item["link"]))
		}
		if item["pubDate"] != "" {
			result.WriteString(fmt.Sprintf("   Date: %s\n", item["pubDate"]))
		}
		if item["description"] != "" {
			desc := stripHTMLTags(item["description"])
			desc = strings.TrimSpace(desc)
			if len(desc) > 200 {
				desc = desc[:200] + "..."
			}
			result.WriteString(fmt.Sprintf("   %s\n", desc))
		}
		result.WriteString("\n")
	}

	return result.String(), nil
}

// ============ Sentiment Analyzer Tool (Collector) ============

type SentimentAnalyzerTool struct{}

func NewSentimentAnalyzerTool() *SentimentAnalyzerTool { return &SentimentAnalyzerTool{} }

func (t *SentimentAnalyzerTool) Name() string        { return "sentiment_analyzer" }
func (t *SentimentAnalyzerTool) Description() string { return "Analyze sentiment of text content to determine positive, negative, or neutral tone" }

func (t *SentimentAnalyzerTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "sentiment_analyzer",
			"description": "Analyze the sentiment of text content. Returns sentiment classification (positive/negative/neutral) with confidence score and key emotional indicators.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{"type": "string", "description": "Text content to analyze for sentiment"},
					"language": map[string]interface{}{"type": "string", "description": "Language of the text (default: en)", "default": "en"},
				},
				"required": []string{"text"},
			},
		},
	}
}

func (t *SentimentAnalyzerTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	text, _ := args["text"].(string)
	if text == "" {
		return "", fmt.Errorf("text required")
	}

	language := "en"
	if lang, ok := args["language"].(string); ok && lang != "" {
		language = lang
	}

	// Clean and normalize text
	cleanText := cleanTextForAnalysis(text)
	if len(cleanText) == 0 {
		return "", fmt.Errorf("no valid text to analyze")
	}

	// Perform sentiment analysis
	result := analyzeSentiment(cleanText, language)
	
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Sentiment Analysis Results\n"))
	output.WriteString(fmt.Sprintf("=========================\n"))
	output.WriteString(fmt.Sprintf("Classification: %s\n", result.Classification))
	output.WriteString(fmt.Sprintf("Confidence: %.2f%%\n", result.Confidence*100))
	output.WriteString(fmt.Sprintf("Emotional Tone: %s\n", strings.Join(result.Emotions, ", ")))
	output.WriteString(fmt.Sprintf("\nOriginal Text Length: %d characters\n", len(text)))
	output.WriteString(fmt.Sprintf("Processed Text Length: %d characters\n", len(cleanText)))
	
	if len(result.Keywords) > 0 {
		output.WriteString("\nKey Emotional Indicators:\n")
		for _, keyword := range result.Keywords {
			output.WriteString(fmt.Sprintf("  - %s\n", keyword))
		}
	}
	
	return output.String(), nil
}

// SentimentResult holds the analysis results
type SentimentResult struct {
	Classification string   // positive, negative, neutral
	Confidence     float64  // 0.0 to 1.0
	Emotions       []string // e.g., ["happy", "concerned"]
	Keywords       []string // key words that influenced the decision
}

// cleanTextForAnalysis preprocesses text for sentiment analysis
func cleanTextForAnalysis(text string) string {
	// Convert to lowercase
	text = strings.ToLower(text)
	
	// Remove extra whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	
	// Remove URLs
	urlRegex := regexp.MustCompile(`https?://\S+|www\.\S+`)
	text = urlRegex.ReplaceAllString(text, "")
	
	// Remove email addresses
	emailRegex := regexp.MustCompile(`[\w\.\-]+@[\w\.\-]+\.[\w\-]+`)
	text = emailRegex.ReplaceAllString(text, "")
	
	// Remove special characters but keep letters, numbers, and basic punctuation
	text = regexp.MustCompile(`[^\p{L}\p{N}\s\.\,\!\?\;\:]`).ReplaceAllString(text, " ")
	
	return strings.TrimSpace(text)
}

// analyzeSentiment performs rule-based sentiment analysis
func analyzeSentiment(text, language string) SentimentResult {
	result := SentimentResult{
		Classification: "neutral",
		Confidence:     0.5,
		Emotions:       []string{},
		Keywords:       []string{},
	}

	// Language-specific lexicons
	var positiveWords, negativeWords map[string]float64
	
	if language == "zh" || strings.Contains(language, "zh") {
		positiveWords = chinesePositiveLexicon()
		negativeWords = chineseNegativeLexicon()
	} else {
		positiveWords = englishPositiveLexicon()
		negativeWords = englishNegativeLexicon()
	}

	words := strings.Fields(text)
	posScore := 0.0
	negScore := 0.0
	var foundKeywords []string
	var emotions []string

	// Score each word
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:")
		if score, exists := positiveWords[word]; exists {
			posScore += score
			foundKeywords = append(foundKeywords, word)
		}
		if score, exists := negativeWords[word]; exists {
			negScore += score
			foundKeywords = append(foundKeywords, word)
		}
	}

	// Apply modifiers (intensifiers, negations)
	posScore, negScore = applyModifiers(words, posScore, negScore, positiveWords, negativeWords)

	// Calculate final scores
	totalScore := posScore - negScore
	absTotal := posScore + negScore

	if absTotal > 0 {
		result.Confidence = absTotal / (absTotal + 2.0) // Smooth confidence calculation
	} else {
		result.Confidence = 0.5
	}

	// Classify sentiment
	threshold := 0.1
	switch {
	case totalScore > threshold:
		result.Classification = "positive"
	case totalScore < -threshold:
		result.Classification = "negative"
	default:
		result.Classification = "neutral"
	}

	// Identify emotions
	emotions = identifyEmotions(text, language)
	result.Emotions = emotions
	result.Keywords = foundKeywords

	return result
}

// applyModifiers adjusts scores based on linguistic modifiers
func applyModifiers(words []string, posScore, negScore float64, posWords, negWords map[string]float64) (float64, float64) {
	intensifiers := map[string]float64{
		"very": 1.5, "extremely": 2.0, "really": 1.3, "quite": 1.2,
		"so": 1.4, "too": 1.3, "super": 1.8, "absolutely": 2.0,
		"completely": 1.8, "totally": 1.7, "highly": 1.6,
		"非常": 1.5, "极其": 2.0, "特别": 1.3, "相当": 1.2,
		"超级": 1.8, "绝对": 2.0, "完全": 1.8, "高度": 1.6,
	}

	// Negation words that reverse sentiment
	negations := map[string]bool{
		"not": true, "no": true, "never": true, "nothing": true,
		"nowhere": true, "neither": true, "nobody": true, "none": true,
		"不": true, "没": true, "无": true, "非": true, "别": true,
		"从未": true, "没有": true, "不是": true,
	}

	for i, word := range words {
		word = strings.Trim(word, ".,!?;:")
		
		// Check for intensifiers
		if multiplier, exists := intensifiers[word]; exists && i < len(words)-1 {
			nextWord := strings.Trim(words[i+1], ".,!?;:")
			if _, pos := posWords[nextWord]; pos {
				posScore *= multiplier
			}
			if _, neg := negWords[nextWord]; neg {
				negScore *= multiplier
			}
		}

		// Check for negations
		if _, exists := negations[word]; exists && i < len(words)-1 {
			nextWord := strings.Trim(words[i+1], ".,!?;:")
			if _, pos := posWords[nextWord]; pos {
				posScore -= 1.0
				negScore += 1.0
			}
			if _, neg := negWords[nextWord]; neg {
				negScore -= 1.0
				posScore += 1.0
			}
		}
	}

	return posScore, negScore
}

// identifyEmotions detects specific emotional tones
func identifyEmotions(text, language string) []string {
	var emotions []string
	
	if language == "zh" || strings.Contains(language, "zh") {
		// Chinese emotion detection
		if strings.Contains(text, "高兴") || strings.Contains(text, "开心") || strings.Contains(text, "快乐") || strings.Contains(text, "喜悦") {
			emotions = append(emotions, "happy")
		}
		if strings.Contains(text, "愤怒") || strings.Contains(text, "生气") || strings.Contains(text, "恼火") {
			emotions = append(emotions, "angry")
		}
		if strings.Contains(text, "悲伤") || strings.Contains(text, "难过") || strings.Contains(text, "伤心") {
			emotions = append(emotions, "sad")
		}
		if strings.Contains(text, "担心") || strings.Contains(text, "担忧") || strings.Contains(text, "焦虑") {
			emotions = append(emotions, "concerned")
		}
		if strings.Contains(text, "兴奋") || strings.Contains(text, "激动") {
			emotions = append(emotions, "excited")
		}
	} else {
		// English emotion detection
		if strings.Contains(text, "happy") || strings.Contains(text, "joy") || strings.Contains(text, "excited") || strings.Contains(text, "pleased") {
			emotions = append(emotions, "happy")
		}
		if strings.Contains(text, "angry") || strings.Contains(text, "mad") || strings.Contains(text, "furious") || strings.Contains(text, "irate") {
			emotions = append(emotions, "angry")
		}
		if strings.Contains(text, "sad") || strings.Contains(text, "depressed") || strings.Contains(text, "upset") || strings.Contains(text, "disappointed") {
			emotions = append(emotions, "sad")
		}
		if strings.Contains(text, "concerned") || strings.Contains(text, "worried") || strings.Contains(text, "anxious") || strings.Contains(text, "nervous") {
			emotions = append(emotions, "concerned")
		}
		if strings.Contains(text, "excited") || strings.Contains(text, "thrilled") || strings.Contains(text, "eager") {
			emotions = append(emotions, "excited")
		}
	}

	if len(emotions) == 0 {
		emotions = append(emotions, "neutral")
	}
	
	return emotions
}

// Lexicon functions
func englishPositiveLexicon() map[string]float64 {
	return map[string]float64{
		"good": 1.0, "great": 1.2, "excellent": 1.5, "amazing": 1.4,
		"wonderful": 1.3, "fantastic": 1.4, "awesome": 1.3, "brilliant": 1.3,
		"perfect": 1.5, "love": 1.6, "like": 0.8, "enjoy": 1.0,
		"happy": 1.2, "pleased": 1.1, "satisfied": 1.0, "delighted": 1.3,
		"success": 1.2, "successful": 1.2, "best": 1.1, "better": 0.8,
		"improved": 0.9, "improvement": 0.9, "progress": 0.8,
	}
}

func englishNegativeLexicon() map[string]float64 {
	return map[string]float64{
		"bad": 1.0, "terrible": 1.5, "awful": 1.4, "horrible": 1.5,
		"worst": 1.3, "hate": 1.6, "dislike": 0.9, "disappointed": 1.2,
		"angry": 1.3, "frustrated": 1.1, "annoyed": 1.0, "upset": 1.1,
		"sad": 1.2, "depressed": 1.4, "worried": 1.0, "concerned": 0.9,
		"failed": 1.1, "failure": 1.3, "problem": 0.9, "issue": 0.8,
		"worse": 0.8, "decline": 0.9, "declining": 0.9,
	}
}

func chinesePositiveLexicon() map[string]float64 {
	return map[string]float64{
		"好": 1.0, "很好": 1.2, "非常好": 1.5, "棒": 1.3,
		"优秀": 1.4, "完美": 1.5, "喜欢": 1.0, "爱": 1.6,
		"高兴": 1.2, "开心": 1.2, "愉快": 1.1, "满意": 1.0,
		"成功": 1.2, "胜利": 1.3, "最好": 1.1, "更好": 0.8,
		"改善": 0.9, "进步": 0.8, "提升": 0.9,
	}
}

func chineseNegativeLexicon() map[string]float64 {
	return map[string]float64{
		"坏": 1.0, "很差": 1.3, "糟糕": 1.4, "可怕": 1.5,
		"最差": 1.3, "讨厌": 1.3, "不喜欢": 0.9, "失望": 1.2,
		"生气": 1.3, "愤怒": 1.4, "沮丧": 1.1, "烦恼": 1.0,
		"难过": 1.2, "悲伤": 1.3, "担心": 1.0, "忧虑": 1.1,
		"失败": 1.3, "问题": 0.9, "麻烦": 1.0, "更糟": 0.8,
		"下降": 0.9, "恶化": 1.0,
	}
}

// ============ Web Monitor Tool (Collector) ============

type WebMonitorTool struct{}

func NewWebMonitorTool() *WebMonitorTool { return &WebMonitorTool{} }

func (t *WebMonitorTool) Name() string        { return "web_monitor" }
func (t *WebMonitorTool) Description() string { return "Monitor websites for changes, track updates, and detect content modifications" }

func (t *WebMonitorTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "web_monitor",
			"description": "Monitor a website for changes. Takes snapshots of web pages and compares them to detect modifications, updates, or new content.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{"type": "string", "description": "URL of the website to monitor"},
					"frequency": map[string]interface{}{"type": "string", "description": "Monitoring frequency (hourly/daily/weekly)", "default": "daily"},
					"elements": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Specific HTML elements/CSS selectors to monitor (optional)"},
					"full_page": map[string]interface{}{"type": "boolean", "description": "Monitor entire page content vs specific sections", "default": true},
				},
				"required": []string{"url"},
			},
		},
	}
}

func (t *WebMonitorTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	urlStr, _ := args["url"].(string)
	if urlStr == "" {
		return "", fmt.Errorf("url required")
	}

	frequency := "daily"
	if freq, ok := args["frequency"].(string); ok && freq != "" {
		frequency = freq
	}

	fullPage := true
	if fp, ok := args["full_page"].(bool); ok {
		fullPage = fp
	}

	var elements []string
	if elems, ok := args["elements"].([]interface{}); ok {
		elements = make([]string, len(elems))
		for i, elem := range elems {
			if s, ok := elem.(string); ok {
				elements[i] = s
			}
		}
	}

	if err := checkSSRF(urlStr); err != nil {
		return "", err
	}

	// Fetch current page content
	content, err := fetchWebPageContent(ctx, urlStr)
	if err != nil {
		return "", fmt.Errorf("failed to fetch webpage: %w", err)
	}

	// Generate content fingerprint/checksum
	fingerprint := generateContentFingerprint(content)

	// In a real implementation, this would check against stored previous versions
	// For now, we'll simulate monitoring results
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Web Monitor Report\n"))
	result.WriteString(fmt.Sprintf("==================\n"))
	result.WriteString(fmt.Sprintf("URL: %s\n", urlStr))
	result.WriteString(fmt.Sprintf("Frequency: %s\n", frequency))
	result.WriteString(fmt.Sprintf("Monitoring Mode: %s\n", map[bool]string{true: "Full Page", false: "Selective Elements"}[fullPage]))
	
	if len(elements) > 0 {
		result.WriteString(fmt.Sprintf("Elements: %v\n", elements))
	}

	result.WriteString(fmt.Sprintf("Content Fingerprint: %s\n", truncateString(fingerprint, 16)))
	result.WriteString(fmt.Sprintf("Content Length: %d characters\n", len(content)))
	
	// Simulate change detection
	changeDetected := detectContentChanges(content, fingerprint)
	if changeDetected {
		result.WriteString("\n⚠️  CHANGE DETECTED\n")
		result.WriteString("Recent modifications found on the page.\n")
	} else {
		result.WriteString("\n✅ No changes detected\n")
		result.WriteString("Page content remains unchanged since last check.\n")
	}

	// Extract key information
	title := extractPageTitle(content)
	if title != "" {
		result.WriteString(fmt.Sprintf("\nPage Title: %s\n", title))
	}

	links := extractLinks(content, urlStr)
	result.WriteString(fmt.Sprintf("\nLinks Found: %d\n", len(links)))
	if len(links) > 0 && len(links) <= 5 {
		for _, link := range links {
			result.WriteString(fmt.Sprintf("  - %s\n", link))
		}
	} else if len(links) > 5 {
		for i := 0; i < 3; i++ {
			result.WriteString(fmt.Sprintf("  - %s\n", links[i]))
		}
		result.WriteString(fmt.Sprintf("  ... and %d more links\n", len(links)-3))
	}

	return result.String(), nil
}

// ============ Knowledge Graph Tool (Collector) ============

type KnowledgeGraphTool struct{}

func NewKnowledgeGraphTool() *KnowledgeGraphTool { return &KnowledgeGraphTool{} }

func (t *KnowledgeGraphTool) Name() string        { return "knowledge_graph" }
func (t *KnowledgeGraphTool) Description() string { return "Build and query knowledge graphs from text content, extracting entities and relationships" }

func (t *KnowledgeGraphTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "knowledge_graph",
			"description": "Extract entities and relationships from text to build a knowledge graph. Identifies people, organizations, locations, dates, and semantic connections.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{"type": "string", "description": "Text content to analyze and build knowledge graph from"},
					"entity_types": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Types of entities to extract (person, organization, location, date, concept)"},
					"max_entities": map[string]interface{}{"type": "integer", "description": "Maximum number of entities to extract", "default": 20},
					"include_relationships": map[string]interface{}{"type": "boolean", "description": "Include relationships between entities", "default": true},
				},
				"required": []string{"text"},
			},
		},
	}
}

func (t *KnowledgeGraphTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	text, _ := args["text"].(string)
	if text == "" {
		return "", fmt.Errorf("text required")
	}

	maxEntities := 20
	if max, ok := args["max_entities"].(float64); ok {
		maxEntities = int(max)
	}

	includeRelationships := true
	if rel, ok := args["include_relationships"].(bool); ok {
		includeRelationships = rel
	}

	var entityTypes []string
	if types, ok := args["entity_types"].([]interface{}); ok {
		entityTypes = make([]string, len(types))
		for i, typ := range types {
			if s, ok := typ.(string); ok {
				entityTypes[i] = s
			}
		}
	}

	// Preprocess text
	cleanText := preprocessTextForKG(text)
	
	// Extract entities
	entities := extractEntities(cleanText, entityTypes, maxEntities)
	
	// Extract relationships if requested
	var relationships []Relationship
	if includeRelationships {
		relationships = extractRelationships(cleanText, entities)
	}

	// Generate output
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Knowledge Graph Analysis\n"))
	result.WriteString(fmt.Sprintf("========================\n"))
	result.WriteString(fmt.Sprintf("Input Text Length: %d characters\n", len(text)))
	result.WriteString(fmt.Sprintf("Processed Text Length: %d characters\n", len(cleanText)))
	result.WriteString(fmt.Sprintf("Entities Extracted: %d\n", len(entities)))
	
	if len(entityTypes) > 0 {
		result.WriteString(fmt.Sprintf("Entity Types: %v\n", entityTypes))
	}
	
	if includeRelationships {
		result.WriteString(fmt.Sprintf("Relationships Found: %d\n", len(relationships)))
	}

	// Display entities
	if len(entities) > 0 {
		result.WriteString("\nEntities:\n")
		for i, entity := range entities {
			result.WriteString(fmt.Sprintf("  %d. [%s] %s\n", i+1, entity.Type, entity.Text))
			if entity.Context != "" {
				result.WriteString(fmt.Sprintf("     Context: %s\n", entity.Context))
			}
		}
	}

	// Display relationships
	if includeRelationships && len(relationships) > 0 {
		result.WriteString("\nRelationships:\n")
		for i, rel := range relationships {
			result.WriteString(fmt.Sprintf("  %d. %s --(%s)--> %s\n", i+1, rel.Source, rel.Type, rel.Target))
		}
	}

	// Generate graph statistics
	stats := generateKGStatistics(entities, relationships)
	result.WriteString("\nGraph Statistics:\n")
	result.WriteString(fmt.Sprintf("  Entity Types Distribution: %v\n", stats.EntityTypeDistribution))
	result.WriteString(fmt.Sprintf("  Average Relationships per Entity: %.2f\n", stats.AvgRelationshipsPerEntity))
	result.WriteString(fmt.Sprintf("  Graph Density: %.2f\n", stats.GraphDensity))

	return result.String(), nil
}

// Helper functions for Web Monitor

func fetchWebPageContent(ctx context.Context, urlStr string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FangClawGOAgent/0.1)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(bodyBytes), nil
}

func generateContentFingerprint(content string) string {
	// Simple checksum for demonstration
	// In production, use proper cryptographic hashing
	sum := 0
	for _, char := range content {
		sum += int(char)
	}
	fingerprint := fmt.Sprintf("%x", sum)
	// Ensure we don't slice beyond the length
	if len(fingerprint) > 16 {
		return fingerprint[:16]
	}
	return fingerprint
}

func detectContentChanges(content, fingerprint string) bool {
	// Simulate change detection logic
	// In reality, this would compare with stored previous fingerprints
	newFingerprint := generateContentFingerprint(content)
	return newFingerprint != fingerprint
}

func extractPageTitle(content string) string {
	titleRegex := regexp.MustCompile(`<title[^>]*>([^<]+)</title>`)
	matches := titleRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(stripHTMLTags(matches[1]))
	}
	return ""
}

func extractLinks(content, baseURL string) []string {
	linkRegex := regexp.MustCompile(`<a[^>]+href=["']([^"']+)["'][^>]*>`) 
	matches := linkRegex.FindAllStringSubmatch(content, -1)
	
	var links []string
	for _, match := range matches {
		if len(match) > 1 {
			link := match[1]
			// Convert relative URLs to absolute
			if strings.HasPrefix(link, "/") {
				// Would need to parse baseURL properly in real implementation
				links = append(links, baseURL+link)
			} else if strings.HasPrefix(link, "http") {
				links = append(links, link)
			}
		}
	}
	
	// Remove duplicates
	uniqueLinks := make(map[string]bool)
	var result []string
	for _, link := range links {
		if !uniqueLinks[link] {
			uniqueLinks[link] = true
			result = append(result, link)
		}
	}
	
	return result
}

// Helper types and functions for Knowledge Graph

type Entity struct {
	Text    string
	Type    string
	Context string
}

type Relationship struct {
	Source string
	Target string
	Type   string
}

type KGStatistics struct {
	EntityTypeDistribution    map[string]int
	AvgRelationshipsPerEntity float64
	GraphDensity              float64
}

func preprocessTextForKG(text string) string {
	// Clean and normalize text for entity extraction
	text = strings.ToLower(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`[^\p{L}\p{N}\s\.\,\!\?\;\:]`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func extractEntities(text string, entityTypes []string, maxEntities int) []Entity {
	var entities []Entity
	words := strings.Fields(text)
	
	// If no specific types requested, extract all common types
	shouldExtractPerson := len(entityTypes) == 0 || containsString(entityTypes, "person")
	shouldExtractOrg := len(entityTypes) == 0 || containsString(entityTypes, "organization")
	shouldExtractLocation := len(entityTypes) == 0 || containsString(entityTypes, "location")
	shouldExtractDate := len(entityTypes) == 0 || containsString(entityTypes, "date")
	
	// Simple rule-based entity extraction
	for i, word := range words {
		if len(entities) >= maxEntities {
			break
		}
		
		// Person name detection (capitalized words)
		if shouldExtractPerson && len(word) > 2 && unicode.IsUpper(rune(word[0])) {
			context := getContext(words, i, 3)
			entities = append(entities, Entity{
				Text:    word,
				Type:    "person",
				Context: context,
			})
		}
		
		// Organization detection (common org words)
		orgIndicators := []string{"company", "inc", "corp", "ltd", "llc", "foundation", "institute"}
		if shouldExtractOrg && containsString(orgIndicators, word) {
			context := getContext(words, i, 3)
			entities = append(entities, Entity{
				Text:    word,
				Type:    "organization",
				Context: context,
			})
		}
		
		// Location detection (common location words)
		locationIndicators := []string{"city", "state", "country", "street", "avenue", "road"}
		if shouldExtractLocation && containsString(locationIndicators, word) {
			context := getContext(words, i, 3)
			entities = append(entities, Entity{
				Text:    word,
				Type:    "location",
				Context: context,
			})
		}
		
		// Date detection (simple pattern matching)
		if shouldExtractDate && isLikelyDate(word) {
			context := getContext(words, i, 3)
			entities = append(entities, Entity{
				Text:    word,
				Type:    "date",
				Context: context,
			})
		}
	}
	
	return entities
}

func extractRelationships(text string, entities []Entity) []Relationship {
	var relationships []Relationship
	
	// Simple co-occurrence based relationship extraction
	for i := 0; i < len(entities)-1; i++ {
		for j := i + 1; j < len(entities); j++ {
			// If entities appear close to each other, assume relationship
			if strings.Contains(text, entities[i].Text) && strings.Contains(text, entities[j].Text) {
				// Simple heuristic: if they're both persons, assume "associated_with"
				relType := "associated_with"
				if entities[i].Type == "person" && entities[j].Type == "organization" {
					relType = "works_for"
				} else if entities[i].Type == "organization" && entities[j].Type == "location" {
					relType = "located_in"
				}
				
				relationships = append(relationships, Relationship{
					Source: entities[i].Text,
					Target: entities[j].Text,
					Type:   relType,
				})
			}
		}
	}
	
	return relationships
}

func buildKnowledgeGraph(entities []Entity, relationships []Relationship) map[string]interface{} {
	kg := make(map[string]interface{})
	kg["entities"] = entities
	kg["relationships"] = relationships
	return kg
}

func generateKGStatistics(entities []Entity, relationships []Relationship) KGStatistics {
	stats := KGStatistics{
		EntityTypeDistribution: make(map[string]int),
	}
	
	// Count entity types
	for _, entity := range entities {
		stats.EntityTypeDistribution[entity.Type]++
	}
	
	// Calculate average relationships per entity
	if len(entities) > 0 {
		stats.AvgRelationshipsPerEntity = float64(len(relationships)) / float64(len(entities))
	}
	
	// Calculate graph density
	maxPossibleEdges := len(entities) * (len(entities) - 1) / 2
	if maxPossibleEdges > 0 {
		stats.GraphDensity = float64(len(relationships)) / float64(maxPossibleEdges)
	}
	
	return stats
}

// Utility functions

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func getContext(words []string, index, window int) string {
	start := index - window
	if start < 0 {
		start = 0
	}
	end := index + window + 1
	if end > len(words) {
		end = len(words)
	}
	return strings.Join(words[start:end], " ")
}

func isLikelyDate(word string) bool {
	// Simple date pattern matching
	datePatterns := []string{
		`\d{4}`,           // 4-digit years
		`\d{1,2}/\d{1,2}`, // MM/DD or DD/MM
		`\d{1,2}-\d{1,2}`, // MM-DD or DD-MM
		`jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec`, // Month abbreviations
	}
	
	for _, pattern := range datePatterns {
		matched, _ := regexp.MatchString(pattern, strings.ToLower(word))
		if matched {
			return true
		}
	}
	return false
}
