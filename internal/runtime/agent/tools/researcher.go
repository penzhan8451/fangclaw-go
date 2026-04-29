package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ============ PDF Extract Tool (Researcher) ============

// PDFExtractTool extracts text content from PDF files
type PDFExtractTool struct{}

// NewPDFExtractTool creates a new PDF extraction tool
func NewPDFExtractTool() *PDFExtractTool { return &PDFExtractTool{} }

func (t *PDFExtractTool) Name() string { return "pdf_extract" }

func (t *PDFExtractTool) Description() string {
	return "Extract text content from PDF files either from a local file path or a URL"
}

func (t *PDFExtractTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "pdf_extract",
			"description": "Extract text content from a PDF file. Supports both local file paths and URLs.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source":               map[string]interface{}{"type": "string", "description": "Local file path or URL to the PDF file"},
					"max_pages":            map[string]interface{}{"type": "integer", "description": "Maximum number of pages to extract (default: all pages)", "default": 0},
					"include_page_numbers": map[string]interface{}{"type": "boolean", "description": "Include page numbers in output (default: true)", "default": true},
				},
				"required": []string{"source"},
			},
		},
	}
}

func (t *PDFExtractTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	source, _ := args["source"].(string)
	if source == "" {
		return "", fmt.Errorf("source required (file path or URL)")
	}

	maxPages := 0
	if v, ok := args["max_pages"].(int); ok && v > 0 {
		maxPages = v
	}

	includePageNumbers := true
	if v, ok := args["include_page_numbers"].(bool); ok {
		includePageNumbers = v
	}

	var pdfData []byte
	var err error

	// Check if source is a URL or local file
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		// Download PDF from URL
		pdfData, err = downloadPDF(ctx, source)
		if err != nil {
			return "", fmt.Errorf("failed to download PDF from URL: %w", err)
		}
	} else {
		// Read from local file
		pdfData, err = os.ReadFile(source)
		if err != nil {
			return "", fmt.Errorf("failed to read PDF file: %w", err)
		}
	}

	// Validate PDF header
	if len(pdfData) < 4 || string(pdfData[:4]) != "%PDF" {
		return "", fmt.Errorf("invalid PDF file: missing PDF header")
	}

	// Extract text from PDF
	text, err := extractTextFromPDFData(pdfData, maxPages, includePageNumbers)
	if err != nil {
		return "", fmt.Errorf("failed to extract text from PDF: %w", err)
	}

	result := fmt.Sprintf("PDF Content Extracted from: %s\n", source)
	result += fmt.Sprintf("Total Pages Processed: %d\n\n", getPageCount(pdfData))
	result += text

	return result, nil
}

// downloadPDF downloads a PDF file from a URL
func downloadPDF(ctx context.Context, urlStr string) ([]byte, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; OpenFangAgent/0.1)")
	req.Header.Set("Accept", "application/pdf,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Limit download size to 50MB for safety
	maxSize := int64(50 * 1024 * 1024)
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}

// getPageCount estimates the number of pages in a PDF
func getPageCount(pdfData []byte) int {
	content := string(pdfData)
	count := strings.Count(content, "/Type /Page")
	count += strings.Count(content, "/Type/Page")

	// Also check for page objects
	count += strings.Count(content, "<<\n/Type /Page")
	count += strings.Count(content, "<< /Type /Page")

	if count == 0 {
		count = 1 // Default to at least 1 page
	}
	return count
}

// extractTextFromPDFData extracts text from PDF data using simple text extraction
// Note: This is a basic implementation that works for text-based PDFs
// For advanced PDF processing, external libraries like unipdf would be needed
func extractTextFromPDFData(pdfData []byte, maxPages int, includePageNumbers bool) (string, error) {
	content := string(pdfData)

	// Remove binary streams and metadata
	content = removePDFStreams(content)
	content = removePDFMetadata(content)

	// Extract text objects
	textObjects := extractPDFTextObjects(content)

	var result strings.Builder
	currentPage := 1

	for _, textObj := range textObjects {
		if maxPages > 0 && currentPage > maxPages {
			break
		}

		if includePageNumbers {
			result.WriteString(fmt.Sprintf("\n--- Page %d ---\n", currentPage))
		}

		// Clean up the extracted text
		cleanText := cleanPDFFormattedText(textObj)
		if strings.TrimSpace(cleanText) != "" {
			result.WriteString(cleanText)
			result.WriteString("\n")
		}

		currentPage++
	}

	if result.Len() == 0 {
		return "", fmt.Errorf("no text content found in PDF - this may be an image-based PDF that requires OCR")
	}

	return result.String(), nil
}

// removePDFStreams removes binary streams from PDF content
func removePDFStreams(content string) string {
	// Remove stream objects that contain binary data
	start := 0
	var result strings.Builder

	for {
		streamStart := strings.Index(content[start:], "stream")
		if streamStart == -1 {
			result.WriteString(content[start:])
			break
		}

		streamStart += start
		result.WriteString(content[start:streamStart])

		// Find endstream
		streamEnd := strings.Index(content[streamStart:], "endstream")
		if streamEnd == -1 {
			break
		}

		streamEnd += streamStart
		start = streamEnd + len("endstream")
	}

	return result.String()
}

// removePDFMetadata removes metadata sections
func removePDFMetadata(content string) string {
	// Remove Info dictionary
	infoStart := strings.Index(content, "<<")
	if infoStart != -1 {
		infoSection := content[infoStart:]
		if strings.Contains(infoSection, "/Producer") || strings.Contains(infoSection, "/Creator") {
			// Skip metadata section
			infoEnd := strings.Index(infoSection, ">>")
			if infoEnd != -1 {
				content = content[:infoStart] + content[infoStart+infoEnd+2:]
			}
		}
	}
	return content
}

// extractPDFTextObjects extracts text content from PDF text objects
func extractPDFTextObjects(content string) []string {
	var textObjects []string

	// Look for BT/ET blocks (Begin Text/End Text)
	start := 0
	for {
		btPos := strings.Index(content[start:], "BT")
		if btPos == -1 {
			break
		}

		btPos += start
		etPos := strings.Index(content[btPos:], "ET")
		if etPos == -1 {
			break
		}

		etPos += btPos
		textBlock := content[btPos+2 : etPos]

		// Extract text strings from the block
		textContent := extractTextFromBlock(textBlock)
		if strings.TrimSpace(textContent) != "" {
			textObjects = append(textObjects, textContent)
		}

		start = etPos + 2
	}

	return textObjects
}

// extractTextFromBlock extracts readable text from a PDF text block
func extractTextFromBlock(block string) string {
	var result strings.Builder

	// Look for text strings marked with parentheses
	i := 0
	for i < len(block) {
		if i < len(block)-1 && block[i] == '(' && block[i+1] != ')' {
			// Found start of text string
			j := i + 1
			for j < len(block) && block[j] != ')' {
				// Handle escaped characters
				if j < len(block)-1 && block[j] == '\\' {
					j += 2
				} else {
					result.WriteByte(block[j])
					j++
				}
			}
			result.WriteByte(' ')
			i = j + 1
		} else {
			i++
		}
	}

	return result.String()
}

// cleanPDFFormattedText cleans up extracted text
func cleanPDFFormattedText(text string) string {
	// Remove extra whitespace
	text = strings.Join(strings.Fields(text), " ")

	// Remove common PDF artifacts
	text = strings.ReplaceAll(text, "\x00", "")
	text = strings.ReplaceAll(text, "\x01", "")
	text = strings.ReplaceAll(text, "\x02", "")

	// Handle common ligatures
	text = strings.ReplaceAll(text, "fi", "fi")
	text = strings.ReplaceAll(text, "fl", "fl")
	text = strings.ReplaceAll(text, "ff", "ff")

	return strings.TrimSpace(text)
}

// ============ Web Search Tool (Researcher) ============

// WebSearchTool searches the web using Tavily, Serper, Google Custom Search API, or Baidu
type WebSearchTool struct {
	apiKey    string
	apiSecret string
	provider  string // "tavily", "serper", "google", "baidu-qianfan", "baidu"
}

// NewWebSearchTool creates a new web search tool. It reads API key from environment.
// Priority: TAVILY_API_KEY > BAIDU_QIANFAN_API_KEY > SERPER_API_KEY > GOOGLE_API_KEY+GOOGLE_CX > BAIDU_ENABLED
func NewWebSearchTool() *WebSearchTool {
	tool := &WebSearchTool{}

	// Try Tavily first
	if apiKey := os.Getenv("TAVILY_API_KEY"); apiKey != "" {
		tool.apiKey = apiKey
		tool.provider = "tavily"
		return tool
	}

	// Try Baidu Qianfan AI Search (simpler API - only needs API key)
	if apiKey := os.Getenv("BAIDU_QIANFAN_API_KEY"); apiKey != "" {
		tool.apiKey = apiKey
		tool.provider = "baidu-qianfan"
		return tool
	}

	// Try Serper
	if apiKey := os.Getenv("SERPER_API_KEY"); apiKey != "" {
		tool.apiKey = apiKey
		tool.provider = "serper"
		return tool
	}

	// Try Google (requires both API key and CX)
	googleAPIKey := os.Getenv("GOOGLE_API_KEY")
	googleCX := os.Getenv("GOOGLE_CX")
	if googleAPIKey != "" && googleCX != "" {
		tool.apiKey = googleAPIKey + ":" + googleCX
		tool.provider = "google"
		return tool
	}

	// Try Baidu (no API key needed for basic search)
	if true || (os.Getenv("BAIDU_ENABLED") == "true" || os.Getenv("BAIDU_ENABLED") == "1") {
		tool.provider = "baidu"
		return tool
	}

	// No API key configured - will return error on execute
	tool.provider = "none"
	return tool
}

func (t *WebSearchTool) Name() string { return "web_search" }
func (t *WebSearchTool) Description() string {
	return "Search the web for information using search APIs"
}
func (t *WebSearchTool) Provider() string { return t.provider }

func (t *WebSearchTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "web_search",
			"description": "Search the web for information. Returns a list of relevant results with titles, URLs, and snippets.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query",
					},
					"max_results": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results to return (default: 5, max: 10)",
						"default":     5,
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query required")
	}

	maxResults := 5
	if v, ok := args["max_results"].(int); ok && v > 0 {
		if v > 10 {
			maxResults = 10
		} else {
			maxResults = v
		}
	}
	if v, ok := args["max_results"].(float64); ok && v > 0 {
		if int(v) > 10 {
			maxResults = 10
		} else {
			maxResults = int(v)
		}
	}

	if t.provider == "none" || t.apiKey == "" {
		return "", fmt.Errorf("no search API key configured. Set TAVILY_API_KEY, SERPER_API_KEY, or GOOGLE_API_KEY+GOOGLE_CX environment variable")
	}

	var results []SearchResult
	var err error

	switch t.provider {
	case "tavily":
		results, err = t.searchTavily(ctx, query, maxResults)
	case "baidu-qianfan":
		results, err = t.searchBaiduQianfan(ctx, query, maxResults)
	case "serper":
		results, err = t.searchSerper(ctx, query, maxResults)
	case "google":
		results, err = t.searchGoogle(ctx, query, maxResults)
	case "baidu":
		results, err = t.searchBaidu(ctx, query, maxResults)
	default:
		return "", fmt.Errorf("unknown search provider: %s", t.provider)
	}

	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "No results found for query: " + query, nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search results for: \"%s\"\n\n", query))
	for i, r := range results {
		output.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, r.Title))
		output.WriteString(fmt.Sprintf("   URL: %s\n", r.URL))
		output.WriteString(fmt.Sprintf("   %s\n\n", r.Snippet))
	}

	return output.String(), nil
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// searchTavily searches using Tavily API
func (t *WebSearchTool) searchTavily(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	reqBody := map[string]interface{}{
		"api_key":             t.apiKey,
		"query":               query,
		"max_results":         maxResults,
		"include_raw_content": false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var tavilyResp struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tavilyResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var results []SearchResult
	for _, r := range tavilyResp.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: safeTruncate(r.Content, 300),
		})
	}
	return results, nil
}

// searchSerper searches using Serper API
func (t *WebSearchTool) searchSerper(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	reqBody := map[string]interface{}{
		"q":   query,
		"num": maxResults,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://google.serper.dev/search", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", t.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var serperResp struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&serperResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var results []SearchResult
	for _, r := range serperResp.Organic {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.Link,
			Snippet: safeTruncate(r.Snippet, 300),
		})
	}
	return results, nil
}

// searchGoogle searches using Google Custom Search API
func (t *WebSearchTool) searchGoogle(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	parts := strings.SplitN(t.apiKey, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid Google API key format")
	}
	apiKey := parts[0]
	cx := parts[1]

	apiURL := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s&num=%d",
		apiKey, cx, url.QueryEscape(query), maxResults)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var googleResp struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var results []SearchResult
	for _, r := range googleResp.Items {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.Link,
			Snippet: safeTruncate(r.Snippet, 300),
		})
	}
	return results, nil
}

// searchBaiduQianfan searches using Baidu Qianfan AI Search API (v2 - simpler API)
func (t *WebSearchTool) searchBaiduQianfan(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	if t.apiKey == "" {
		return nil, fmt.Errorf("BAIDU_QIANFAN_API_KEY must be set")
	}

	// Use the simpler v2 API that only requires API key
	apiURL := "https://qianfan.baidubce.com/v2/ai_search/web_search"

	reqBody := map[string]interface{}{
		"query":       query,
		"top_k":       maxResults,
		"need_filter": false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Key", t.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var qianfanResp struct {
		Results []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&qianfanResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var results []SearchResult
	for _, r := range qianfanResp.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.Link,
			Snippet: safeTruncate(r.Snippet, 300),
		})
	}

	fmt.Println("✅Return baidu qianfan search results")
	return results, nil
}

// searchBaidu searches using Baidu (no API key needed)
func (t *WebSearchTool) searchBaidu(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	apiURL := fmt.Sprintf("https://www.baidu.com/s?wd=%s&rn=%d", url.QueryEscape(query), maxResults)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read baidu search response: %w", err)
	}

	return parseBaiduSearchResults(string(bodyBytes), maxResults), nil
}

// parseBaiduSearchResults parses Baidu search results from HTML into SearchResult
func parseBaiduSearchResults(html string, max int) []SearchResult {
	var results []SearchResult

	chunks := strings.Split(html, "class=\"result c-container\"")
	for _, chunk := range chunks[1:] {
		if len(results) >= max {
			break
		}

		title := extractBetween(chunk, "data-title=\"", "\"")
		if title == "" {
			title = extractBetween(chunk, "<h3", "</h3>")
			title = extractBetween(title, ">", "<")
		}

		url := extractBetween(chunk, "href=\"", "\"")
		if url == "" || strings.HasPrefix(url, "/") || strings.HasPrefix(url, "#") {
			continue
		}

		snippet := extractBetween(chunk, "class=\"c-abstract\"", "</div>")
		snippet = strings.TrimSpace(stripHTMLTags(snippet))

		title = cleanBaiduSearchTitle(title)

		if title != "" && url != "" {
			results = append(results, SearchResult{
				Title:   title,
				URL:     url,
				Snippet: safeTruncate(snippet, 300),
			})
		}
	}

	fmt.Println("✅Return baidu search results")
	return results
}

// cleanBaiduSearchTitle cleans Baidu search result title
func cleanBaiduSearchTitle(title string) string {
	title = strings.ReplaceAll(title, "&quot;", "\"")
	title = strings.ReplaceAll(title, "&amp;", "&")
	title = strings.ReplaceAll(title, "&lt;", "<")
	title = strings.ReplaceAll(title, "&gt;", ">")
	title = strings.ReplaceAll(title, "&#39;", "'")
	return strings.TrimSpace(title)
}

// ============ Web Scrape Tool (Researcher) ============

type WebScrapeTool struct{}

func NewWebScrapeTool() *WebScrapeTool { return &WebScrapeTool{} }

func (t *WebScrapeTool) Name() string { return "web_scrape" }
func (t *WebScrapeTool) Description() string {
	return "Scrape and extract structured data from web pages"
}

func (t *WebScrapeTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "web_scrape",
			"description": "Scrape a web page and extract structured content including text, links, headings, etc.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url":              map[string]interface{}{"type": "string", "description": "URL of the web page to scrape"},
					"extract_links":    map[string]interface{}{"type": "boolean", "description": "Whether to extract all links (default: true)"},
					"extract_headings": map[string]interface{}{"type": "boolean", "description": "Whether to extract headings (default: true)"},
				},
				"required": []string{"url"},
			},
		},
	}
}

func (t *WebScrapeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	urlStr, _ := args["url"].(string)
	if urlStr == "" {
		return "", fmt.Errorf("url required")
	}

	extractLinks := true
	if v, ok := args["extract_links"].(bool); ok {
		extractLinks = v
	}
	extractHeadings := true
	if v, ok := args["extract_headings"].(bool); ok {
		extractHeadings = v
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

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; OpenFangAgent/0.1)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	maxResponseBytes := 10 * 1024 * 1024
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxResponseBytes)))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	html := string(bodyBytes)
	title := extractTitle(html)
	mainText := extractTextFromHTML(html)
	headings := []string{}
	links := []string{}

	if extractHeadings {
		headings = extractHeadingsFromHTML(html)
	}
	if extractLinks {
		links = extractLinksFromHTML(html, urlStr)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Scraped: %s\n", urlStr))
	result.WriteString(fmt.Sprintf("Title: %s\n\n", title))
	result.WriteString("--- CONTENT ---\n")
	result.WriteString(safeTruncate(mainText, 10000))
	if len(mainText) > 10000 {
		result.WriteString("... [content truncated]\n")
	}

	if extractHeadings && len(headings) > 0 {
		result.WriteString("\n--- HEADINGS ---\n")
		for i, h := range headings {
			if i >= 20 {
				result.WriteString(fmt.Sprintf("... and %d more headings\n", len(headings)-20))
				break
			}
			result.WriteString(fmt.Sprintf("- %s\n", h))
		}
	}

	if extractLinks && len(links) > 0 {
		result.WriteString("\n--- LINKS ---\n")
		for i, l := range links {
			if i >= 30 {
				result.WriteString(fmt.Sprintf("... and %d more links\n", len(links)-30))
				break
			}
			result.WriteString(fmt.Sprintf("- %s\n", l))
		}
	}

	return result.String(), nil
}

func extractTitle(html string) string {
	title := extractBetween(html, "<title>", "</title>")
	title = stripHTMLTags(title)
	title = strings.TrimSpace(title)
	if title == "" {
		return "Untitled"
	}
	return title
}

func extractHeadingsFromHTML(html string) []string {
	var headings []string
	headingTags := []string{"h1", "h2", "h3", "h4", "h5", "h6"}

	for _, tag := range headingTags {
		openTag := "<" + tag
		closeTag := "</" + tag + ">"

		remaining := html
		for {
			startIdx := strings.Index(strings.ToLower(remaining), openTag)
			if startIdx == -1 {
				break
			}
			startCloseTag := strings.Index(remaining[startIdx:], ">")
			if startCloseTag == -1 {
				break
			}
			contentStart := startIdx + startCloseTag + 1
			endIdx := strings.Index(strings.ToLower(remaining[contentStart:]), closeTag)
			if endIdx == -1 {
				break
			}
			content := remaining[contentStart : contentStart+endIdx]
			content = stripHTMLTags(content)
			content = strings.TrimSpace(content)
			if content != "" {
				headings = append(headings, content)
			}
			remaining = remaining[contentStart+endIdx+len(closeTag):]
		}
	}
	return headings
}

func extractLinksFromHTML(html, baseURL string) []string {
	var links []string
	seen := make(map[string]bool)

	parsedBase, _ := url.Parse(baseURL)
	remaining := html

	for {
		startIdx := strings.Index(strings.ToLower(remaining), "<a")
		if startIdx == -1 {
			break
		}
		hrefStart := strings.Index(strings.ToLower(remaining[startIdx:]), "href=")
		if hrefStart == -1 {
			remaining = remaining[startIdx+2:]
			continue
		}
		hrefStart += startIdx + 5
		if hrefStart >= len(remaining) {
			break
		}
		quoteChar := remaining[hrefStart]
		if quoteChar != '"' && quoteChar != '\'' {
			remaining = remaining[hrefStart:]
			continue
		}
		hrefStart++
		hrefEnd := strings.IndexByte(remaining[hrefStart:], quoteChar)
		if hrefEnd == -1 {
			break
		}
		link := remaining[hrefStart : hrefStart+hrefEnd]
		link = strings.TrimSpace(link)

		if parsedBase != nil && link != "" {
			if parsedLink, err := parsedBase.Parse(link); err == nil {
				link = parsedLink.String()
			}
		}

		if link != "" && !seen[link] && (strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://")) {
			seen[link] = true
			links = append(links, link)
		}
		remaining = remaining[hrefStart+hrefEnd+1:]
	}
	return links
}

// ============ Citation Generator Tool (Researcher) ============

type CitationGeneratorTool struct{}

func NewCitationGeneratorTool() *CitationGeneratorTool { return &CitationGeneratorTool{} }

func (t *CitationGeneratorTool) Name() string { return "citation_generator" }
func (t *CitationGeneratorTool) Description() string {
	return "Generate citations in APA, MLA, or Chicago format"
}

func (t *CitationGeneratorTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "citation_generator",
			"description": "Generate citations for web pages, books, articles in APA 7th, MLA 9th, or Chicago format.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of source (webpage, book, journal, newspaper)",
						"enum":        []string{"webpage", "book", "journal", "newspaper"},
					},
					"authors": map[string]interface{}{
						"type":        "array",
						"description": "List of authors (each with 'first_name' and 'last_name')",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"first_name": map[string]interface{}{"type": "string"},
								"last_name":  map[string]interface{}{"type": "string"},
							},
						},
					},
					"title":            map[string]interface{}{"type": "string", "description": "Title of the work"},
					"url":              map[string]interface{}{"type": "string", "description": "URL (for web sources)"},
					"publication_year": map[string]interface{}{"type": "string", "description": "Year of publication"},
					"publisher":        map[string]interface{}{"type": "string", "description": "Publisher (for books)"},
					"journal_title":    map[string]interface{}{"type": "string", "description": "Journal title (for articles)"},
					"volume":           map[string]interface{}{"type": "string", "description": "Volume number"},
					"issue":            map[string]interface{}{"type": "string", "description": "Issue number"},
					"pages":            map[string]interface{}{"type": "string", "description": "Page range"},
					"style": map[string]interface{}{
						"type":        "string",
						"description": "Citation style (apa, mla, chicago)",
						"enum":        []string{"apa", "mla", "chicago"},
					},
				},
				"required": []string{"source_type", "title", "style"},
			},
		},
	}
}

type Author struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (t *CitationGeneratorTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	sourceType, _ := args["source_type"].(string)
	title, _ := args["title"].(string)
	style, _ := args["style"].(string)
	url, _ := args["url"].(string)
	publicationYear, _ := args["publication_year"].(string)
	publisher, _ := args["publisher"].(string)
	journalTitle, _ := args["journal_title"].(string)
	volume, _ := args["volume"].(string)
	issue, _ := args["issue"].(string)
	pages, _ := args["pages"].(string)

	var authors []Author
	if authorsRaw, ok := args["authors"].([]interface{}); ok {
		for _, a := range authorsRaw {
			if authorMap, ok := a.(map[string]interface{}); ok {
				authors = append(authors, Author{
					FirstName: fmt.Sprintf("%v", authorMap["first_name"]),
					LastName:  fmt.Sprintf("%v", authorMap["last_name"]),
				})
			}
		}
	}

	if title == "" {
		return "", fmt.Errorf("title required")
	}
	if style == "" {
		style = "apa"
	}

	var citation string
	switch strings.ToLower(style) {
	case "apa":
		citation = generateAPACitation(sourceType, authors, title, url, publicationYear, publisher, journalTitle, volume, issue, pages)
	case "mla":
		citation = generateMLACitation(sourceType, authors, title, url, publicationYear, publisher, journalTitle, volume, issue, pages)
	case "chicago":
		citation = generateChicagoCitation(sourceType, authors, title, url, publicationYear, publisher, journalTitle, volume, issue, pages)
	default:
		citation = generateAPACitation(sourceType, authors, title, url, publicationYear, publisher, journalTitle, volume, issue, pages)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Citation (%s):\n\n", strings.ToUpper(style)))
	result.WriteString(citation)
	result.WriteString("\n\n")
	result.WriteString("In-text citation:\n")
	result.WriteString(generateInTextCitation(authors, publicationYear, style))

	return result.String(), nil
}

func formatAuthorsAPA(authors []Author) string {
	if len(authors) == 0 {
		return ""
	}
	var parts []string
	for _, a := range authors {
		if a.LastName != "" {
			initials := ""
			if a.FirstName != "" {
				initials = strings.ToUpper(a.FirstName[:1]) + "."
			}
			parts = append(parts, fmt.Sprintf("%s, %s", a.LastName, initials))
		}
	}
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 2 {
		return parts[0] + " & " + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " & " + parts[len(parts)-1]
}

func formatAuthorsMLA(authors []Author) string {
	if len(authors) == 0 {
		return ""
	}
	var parts []string
	for i, a := range authors {
		if a.LastName != "" {
			if i == 0 {
				parts = append(parts, fmt.Sprintf("%s, %s", a.LastName, a.FirstName))
			} else {
				parts = append(parts, fmt.Sprintf("%s %s", a.FirstName, a.LastName))
			}
		}
	}
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 2 {
		return parts[0] + " and " + parts[1]
	}
	return parts[0] + ", et al."
}

func generateAPACitation(sourceType string, authors []Author, title, url, year, publisher, journalTitle, volume, issue, pages string) string {
	authorStr := formatAuthorsAPA(authors)
	if year == "" {
		year = "n.d."
	}

	switch sourceType {
	case "webpage":
		if authorStr != "" {
			return fmt.Sprintf("%s. (%s). %s. %s", authorStr, year, title, url)
		}
		return fmt.Sprintf("%s. (%s). %s. %s", title, year, title, url)
	case "book":
		if authorStr != "" {
			if publisher != "" {
				return fmt.Sprintf("%s. (%s). %s. %s", authorStr, year, title, publisher)
			}
			return fmt.Sprintf("%s. (%s). %s", authorStr, year, title)
		}
		return fmt.Sprintf("%s. (%s). %s", title, year, title)
	case "journal":
		if authorStr != "" {
			if volume != "" && issue != "" {
				return fmt.Sprintf("%s. (%s). %s. %s, %s(%s), %s. %s", authorStr, year, title, journalTitle, volume, issue, pages, url)
			}
			return fmt.Sprintf("%s. (%s). %s. %s. %s", authorStr, year, title, journalTitle, url)
		}
		return fmt.Sprintf("%s. (%s). %s. %s. %s", title, year, title, journalTitle, url)
	default:
		if authorStr != "" {
			return fmt.Sprintf("%s. (%s). %s. %s", authorStr, year, title, url)
		}
		return fmt.Sprintf("%s. (%s). %s. %s", title, year, title, url)
	}
}

func generateMLACitation(sourceType string, authors []Author, title, url, year, publisher, journalTitle, volume, issue, pages string) string {
	authorStr := formatAuthorsMLA(authors)
	accessDate := time.Now().Format("2 Jan. 2006")

	switch sourceType {
	case "webpage":
		if authorStr != "" {
			return fmt.Sprintf("%s. \"%s.\" %s, %s. Accessed %s.", authorStr, title, url, year, accessDate)
		}
		return fmt.Sprintf("\"%s.\" %s, %s. Accessed %s.", title, url, year, accessDate)
	case "book":
		if authorStr != "" {
			return fmt.Sprintf("%s. %s. %s, %s.", authorStr, title, publisher, year)
		}
		return fmt.Sprintf("%s. %s, %s.", title, publisher, year)
	case "journal":
		if authorStr != "" {
			if volume != "" && issue != "" {
				return fmt.Sprintf("%s. \"%s.\" %s, vol. %s, no. %s, %s, pp. %s.", authorStr, title, journalTitle, volume, issue, year, pages)
			}
			return fmt.Sprintf("%s. \"%s.\" %s, %s.", authorStr, title, journalTitle, year)
		}
		return fmt.Sprintf("\"%s.\" %s, %s.", title, journalTitle, year)
	default:
		if authorStr != "" {
			return fmt.Sprintf("%s. \"%s.\" %s, %s. Accessed %s.", authorStr, title, url, year, accessDate)
		}
		return fmt.Sprintf("\"%s.\" %s, %s. Accessed %s.", title, url, year, accessDate)
	}
}

func generateChicagoCitation(sourceType string, authors []Author, title, url, year, publisher, journalTitle, volume, issue, pages string) string {
	authorStr := formatAuthorsMLA(authors)
	accessDate := time.Now().Format("January 2, 2006")

	switch sourceType {
	case "webpage":
		if authorStr != "" {
			return fmt.Sprintf("%s. \"%s.\" Accessed %s. %s.", authorStr, title, accessDate, url)
		}
		return fmt.Sprintf("\"%s.\" Accessed %s. %s.", title, accessDate, url)
	case "book":
		if authorStr != "" {
			return fmt.Sprintf("%s. %s. %s, %s.", authorStr, title, publisher, year)
		}
		return fmt.Sprintf("%s. %s, %s.", title, publisher, year)
	case "journal":
		if authorStr != "" {
			if volume != "" && issue != "" {
				return fmt.Sprintf("%s. \"%s.\" %s %s, no. %s (%s): %s.", authorStr, title, journalTitle, volume, issue, year, pages)
			}
			return fmt.Sprintf("%s. \"%s.\" %s (%s).", authorStr, title, journalTitle, year)
		}
		return fmt.Sprintf("\"%s.\" %s (%s).", title, journalTitle, year)
	default:
		if authorStr != "" {
			return fmt.Sprintf("%s. \"%s.\" Accessed %s. %s.", authorStr, title, accessDate, url)
		}
		return fmt.Sprintf("\"%s.\" Accessed %s. %s.", title, accessDate, url)
	}
}

func generateInTextCitation(authors []Author, year, style string) string {
	if len(authors) == 0 {
		if year == "" {
			return "(Anonymous)"
		}
		return fmt.Sprintf("(Anonymous, %s)", year)
	}

	switch style {
	case "apa", "mla":
		if len(authors) == 1 {
			if year == "" {
				return fmt.Sprintf("(%s)", authors[0].LastName)
			}
			return fmt.Sprintf("(%s, %s)", authors[0].LastName, year)
		}
		if len(authors) == 2 {
			if year == "" {
				return fmt.Sprintf("(%s & %s)", authors[0].LastName, authors[1].LastName)
			}
			return fmt.Sprintf("(%s & %s, %s)", authors[0].LastName, authors[1].LastName, year)
		}
		if year == "" {
			return fmt.Sprintf("(%s et al.)", authors[0].LastName)
		}
		return fmt.Sprintf("(%s et al., %s)", authors[0].LastName, year)
	case "chicago":
		if len(authors) == 1 {
			if year == "" {
				return fmt.Sprintf("(%s)", authors[0].LastName)
			}
			return fmt.Sprintf("(%s %s)", authors[0].LastName, year)
		}
		if len(authors) == 2 {
			if year == "" {
				return fmt.Sprintf("(%s and %s)", authors[0].LastName, authors[1].LastName)
			}
			return fmt.Sprintf("(%s and %s %s)", authors[0].LastName, authors[1].LastName, year)
		}
		if year == "" {
			return fmt.Sprintf("(%s et al.)", authors[0].LastName)
		}
		return fmt.Sprintf("(%s et al. %s)", authors[0].LastName, year)
	default:
		if len(authors) == 1 {
			if year == "" {
				return fmt.Sprintf("(%s)", authors[0].LastName)
			}
			return fmt.Sprintf("(%s, %s)", authors[0].LastName, year)
		}
		if year == "" {
			return fmt.Sprintf("(%s et al.)", authors[0].LastName)
		}
		return fmt.Sprintf("(%s et al., %s)", authors[0].LastName, year)
	}
}
