package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"
)

// ============ Brier Scorer Tool (Predictor) ============

type BrierScorerTool struct{}

func NewBrierScorerTool() *BrierScorerTool { return &BrierScorerTool{} }

func (t *BrierScorerTool) Name() string { return "brier_scorer" }
func (t *BrierScorerTool) Description() string {
	return "Calculate Brier scores for prediction accuracy"
}

func (t *BrierScorerTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "brier_scorer",
			"description": "Calculate Brier scores for a set of predictions. Brier score measures prediction accuracy (0 = perfect, 0.25 = guessing, 1 = worst).",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"predictions": map[string]interface{}{
						"type":        "array",
						"description": "Array of predictions with 'probability' (0-1) and 'actual' (0 or 1)",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"probability": map[string]interface{}{"type": "number", "description": "Predicted probability (0-1)"},
								"actual":      map[string]interface{}{"type": "integer", "description": "Actual outcome (0 or 1)"},
								"question":    map[string]interface{}{"type": "string", "description": "Question or prediction description (optional)"},
							},
							"required": []string{"probability", "actual"},
						},
					},
				},
				"required": []string{"predictions"},
			},
		},
	}
}

func (t *BrierScorerTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	predictionsRaw, ok := args["predictions"].([]interface{})
	if !ok {
		return "", fmt.Errorf("predictions must be an array")
	}
	if len(predictionsRaw) == 0 {
		return "", fmt.Errorf("predictions array is empty")
	}

	type Prediction struct {
		Probability float64
		Actual      int
		Question    string
	}

	var predictions []Prediction
	for _, p := range predictionsRaw {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		prob, _ := pm["probability"].(float64)
		actual, _ := pm["actual"].(float64)
		question, _ := pm["question"].(string)
		predictions = append(predictions, Prediction{
			Probability: prob,
			Actual:      int(actual),
			Question:    question,
		})
	}

	if len(predictions) == 0 {
		return "", fmt.Errorf("no valid predictions found")
	}

	var totalScore float64
	var calibrationScore float64
	var resolutionScore float64
	var refinementScore float64

	var outcomes []int
	var sumActual float64

	for _, p := range predictions {
		outcomes = append(outcomes, p.Actual)
		sumActual += float64(p.Actual)
	}

	baseRate := sumActual / float64(len(predictions))

	for _, p := range predictions {
		prob := p.Probability
		actual := float64(p.Actual)

		brierComponent := math.Pow(prob-actual, 2)
		totalScore += brierComponent

		calibrationComponent := math.Pow(prob-baseRate, 2)
		calibrationScore += calibrationComponent

		resolutionComponent := math.Pow(baseRate-actual, 2)
		resolutionScore += resolutionComponent
	}

	n := float64(len(predictions))
	totalScore /= n
	calibrationScore /= n
	resolutionScore /= n
	refinementScore = totalScore - calibrationScore + resolutionScore

	var result strings.Builder
	result.WriteString("=== Brier Score Analysis ===\n\n")
	result.WriteString(fmt.Sprintf("Total predictions: %d\n", len(predictions)))
	result.WriteString(fmt.Sprintf("Base rate: %.2f%%\n\n", baseRate*100))

	result.WriteString("Scores:\n")
	result.WriteString(fmt.Sprintf("- Brier Score: %.4f\n", totalScore))
	result.WriteString(fmt.Sprintf("- Calibration: %.4f\n", calibrationScore))
	result.WriteString(fmt.Sprintf("- Resolution: %.4f\n", resolutionScore))
	result.WriteString(fmt.Sprintf("- Refinement: %.4f\n\n", refinementScore))

	result.WriteString("Interpretation:\n")
	if totalScore < 0.1 {
		result.WriteString("- Excellent prediction accuracy\n")
	} else if totalScore < 0.2 {
		result.WriteString("- Good prediction accuracy\n")
	} else if totalScore < 0.25 {
		result.WriteString("- Better than random guessing\n")
	} else if totalScore == 0.25 {
		result.WriteString("- Equivalent to random guessing\n")
	} else {
		result.WriteString("- Worse than random guessing\n")
	}

	if len(predictions) <= 10 {
		result.WriteString("\n=== Individual Predictions ===\n\n")
		for i, p := range predictions {
			score := math.Pow(p.Probability-float64(p.Actual), 2)
			result.WriteString(fmt.Sprintf("%d. ", i+1))
			if p.Question != "" {
				result.WriteString(fmt.Sprintf("%s\n", p.Question))
				result.WriteString(fmt.Sprintf("   "))
			}
			result.WriteString(fmt.Sprintf("Predicted: %.0f%%, Actual: %d, Score: %.4f\n",
				p.Probability*100, p.Actual, score))
		}
	}

	return result.String(), nil
}

// ============ Prediction Tracker Tool (Predictor) ============

type PredictionTrackerTool struct{}

func NewPredictionTrackerTool() *PredictionTrackerTool { return &PredictionTrackerTool{} }

func (t *PredictionTrackerTool) Name() string        { return "prediction_tracker" }
func (t *PredictionTrackerTool) Description() string { return "Track and manage prediction records" }

func (t *PredictionTrackerTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "prediction_tracker",
			"description": "Track predictions - can add, list, update, or resolve predictions.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action to perform (add, list, update, resolve)",
						"enum":        []string{"add", "list", "update", "resolve"},
					},
					"storage_path":    map[string]interface{}{"type": "string", "description": "File path to store predictions (default: predictions.json)"},
					"question":        map[string]interface{}{"type": "string", "description": "Prediction question (for add/update/resolve)"},
					"probability":     map[string]interface{}{"type": "number", "description": "Predicted probability 0-1 (for add/update)"},
					"resolution_date": map[string]interface{}{"type": "string", "description": "Date when prediction will be resolved (YYYY-MM-DD)"},
					"actual":          map[string]interface{}{"type": "integer", "description": "Actual outcome (0 or 1) (for resolve)"},
					"id":              map[string]interface{}{"type": "string", "description": "Prediction ID (for update/resolve)"},
					"limit":           map[string]interface{}{"type": "integer", "description": "Max items to return (for list, default: 50)"},
					"filter": map[string]interface{}{
						"type":        "string",
						"description": "Filter predictions (all, unresolved, resolved)",
						"enum":        []string{"all", "unresolved", "resolved"},
					},
				},
				"required": []string{"action"},
			},
		},
	}
}

type PredictionRecord struct {
	ID             string  `json:"id"`
	Question       string  `json:"question"`
	Probability    float64 `json:"probability"`
	ResolutionDate string  `json:"resolution_date,omitempty"`
	Actual         *int    `json:"actual,omitempty"`
	CreatedAt      string  `json:"created_at"`
	ResolvedAt     string  `json:"resolved_at,omitempty"`
}

func (t *PredictionTrackerTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	storagePath, _ := args["storage_path"].(string)
	if storagePath == "" {
		storagePath = "predictions.json"
	}

	var records []PredictionRecord

	if _, err := os.Stat(storagePath); err == nil {
		data, err := os.ReadFile(storagePath)
		if err == nil {
			_ = json.Unmarshal(data, &records)
		}
	}

	switch action {
	case "add":
		question, _ := args["question"].(string)
		if question == "" {
			return "", fmt.Errorf("question required for add")
		}
		prob, _ := args["probability"].(float64)
		if prob < 0 || prob > 1 {
			return "", fmt.Errorf("probability must be between 0 and 1")
		}
		resolutionDate, _ := args["resolution_date"].(string)

		id := fmt.Sprintf("pred_%d", time.Now().UnixNano())
		record := PredictionRecord{
			ID:             id,
			Question:       question,
			Probability:    prob,
			ResolutionDate: resolutionDate,
			CreatedAt:      time.Now().Format(time.RFC3339),
		}
		records = append(records, record)

		if data, err := json.MarshalIndent(records, "", "  "); err == nil {
			_ = os.WriteFile(storagePath, data, 0644)
		}

		return fmt.Sprintf("Prediction added!\nID: %s\nQuestion: %s\nProbability: %.0f%%",
			id, question, prob*100), nil

	case "list":
		filter, _ := args["filter"].(string)
		limit := 50
		if v, ok := args["limit"].(float64); ok {
			limit = int(v)
		}

		var filtered []PredictionRecord
		for _, r := range records {
			switch filter {
			case "resolved":
				if r.Actual != nil {
					filtered = append(filtered, r)
				}
			case "unresolved":
				if r.Actual == nil {
					filtered = append(filtered, r)
				}
			default:
				filtered = append(filtered, r)
			}
		}

		if len(filtered) > limit {
			filtered = filtered[:limit]
		}

		var result strings.Builder
		result.WriteString(fmt.Sprintf("=== Predictions (%d total, %d shown) ===\n\n", len(records), len(filtered)))

		resolvedCount := 0
		for _, r := range records {
			if r.Actual != nil {
				resolvedCount++
			}
		}
		result.WriteString(fmt.Sprintf("Resolved: %d, Unresolved: %d\n\n", resolvedCount, len(records)-resolvedCount))

		for i, r := range filtered {
			result.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, r.ID, r.Question))
			result.WriteString(fmt.Sprintf("   Probability: %.0f%%", r.Probability*100))
			if r.Actual != nil {
				result.WriteString(fmt.Sprintf(", Actual: %d", *r.Actual))
				score := math.Pow(r.Probability-float64(*r.Actual), 2)
				result.WriteString(fmt.Sprintf(", Brier: %.4f", score))
			}
			if r.ResolutionDate != "" {
				result.WriteString(fmt.Sprintf("\n   Resolve by: %s", r.ResolutionDate))
			}
			result.WriteString("\n\n")
		}

		return result.String(), nil

	case "resolve":
		id, _ := args["id"].(string)
		if id == "" {
			return "", fmt.Errorf("id required for resolve")
		}
		actualRaw, ok := args["actual"].(float64)
		if !ok {
			return "", fmt.Errorf("actual required for resolve (0 or 1)")
		}
		actual := int(actualRaw)
		if actual != 0 && actual != 1 {
			return "", fmt.Errorf("actual must be 0 or 1")
		}

		found := false
		for i, r := range records {
			if r.ID == id {
				records[i].Actual = &actual
				now := time.Now().Format(time.RFC3339)
				records[i].ResolvedAt = now
				found = true
				break
			}
		}

		if !found {
			return "", fmt.Errorf("prediction not found: %s", id)
		}

		if data, err := json.MarshalIndent(records, "", "  "); err == nil {
			_ = os.WriteFile(storagePath, data, 0644)
		}

		return fmt.Sprintf("Prediction resolved!\nID: %s\nActual: %d", id, actual), nil

	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}
