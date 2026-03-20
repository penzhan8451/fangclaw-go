package tools

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ============ CSV Export Tool (Lead) ============

type CSVExportTool struct{}

func NewCSVExportTool() *CSVExportTool { return &CSVExportTool{} }

func (t *CSVExportTool) Name() string        { return "csv_export" }
func (t *CSVExportTool) Description() string { return "Export data to CSV format" }

func (t *CSVExportTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "csv_export",
			"description": "Export an array of objects to CSV format. Returns the CSV content as a string.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"data": map[string]interface{}{
						"type":        "array",
						"description": "Array of objects to export",
						"items":       map[string]interface{}{"type": "object"},
					},
					"filename":        map[string]interface{}{"type": "string", "description": "Filename to save (optional)"},
					"include_headers": map[string]interface{}{"type": "boolean", "description": "Include header row (default: true)"},
				},
				"required": []string{"data"},
			},
		},
	}
}

func (t *CSVExportTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	dataRaw, ok := args["data"].([]interface{})
	if !ok {
		return "", fmt.Errorf("data must be an array")
	}
	if len(dataRaw) == 0 {
		return "", fmt.Errorf("data array is empty")
	}

	filename, _ := args["filename"].(string)
	includeHeaders := true
	if v, ok := args["include_headers"].(bool); ok {
		includeHeaders = v
	}

	var records [][]string
	var headers []string
	headerMap := make(map[string]bool)

	for i, item := range dataRaw {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if i == 0 {
			for k := range itemMap {
				if !headerMap[k] {
					headerMap[k] = true
					headers = append(headers, k)
				}
			}
			sort.Strings(headers)
		}

		var record []string
		for _, h := range headers {
			val := itemMap[h]
			if val == nil {
				record = append(record, "")
			} else {
				record = append(record, fmt.Sprintf("%v", val))
			}
		}
		records = append(records, record)
	}

	var buf strings.Builder
	w := csv.NewWriter(&buf)

	if includeHeaders && len(headers) > 0 {
		if err := w.Write(headers); err != nil {
			return "", fmt.Errorf("failed to write headers: %w", err)
		}
	}

	if err := w.WriteAll(records); err != nil {
		return "", fmt.Errorf("failed to write CSV: %w", err)
	}

	csvContent := buf.String()

	if filename != "" {
		if err := os.WriteFile(filename, []byte(csvContent), 0644); err != nil {
			return "", fmt.Errorf("failed to save file: %w", err)
		}
		return fmt.Sprintf("CSV exported successfully to: %s\n\n%s", filename, csvContent), nil
	}

	return csvContent, nil
}
