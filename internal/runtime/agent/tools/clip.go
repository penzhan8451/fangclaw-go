package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/mediaprocessing"
	"github.com/penzhan8451/fangclaw-go/internal/uploadregistry"
)

// ============ Video Editor Tool (Clip) ============

type VideoEditorTool struct{}

func NewVideoEditorTool() *VideoEditorTool { return &VideoEditorTool{} }

func (t *VideoEditorTool) Name() string { return "video_editor" }
func (t *VideoEditorTool) Description() string {
	return "Edit videos by trimming, cutting, merging, and applying basic effects"
}

func (t *VideoEditorTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "video_editor",
			"description": "Edit videos by performing operations like trim, cut, merge, add text overlays, and apply transitions.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input_file": map[string]interface{}{"type": "string", "description": "Path to input video file"},
					"operations": map[string]interface{}{
						"type":        "array",
						"description": "List of editing operations to perform",
						"items":       map[string]interface{}{"type": "object"},
					},
					"output_file": map[string]interface{}{"type": "string", "description": "Path for output video file"},
					"format":      map[string]interface{}{"type": "string", "description": "Output format (mp4, avi, mov, etc.)", "default": "mp4"},
				},
				"required": []string{"input_file", "operations"},
			},
		},
	}
}

func (t *VideoEditorTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	inputFile, _ := args["input_file"].(string)
	if inputFile == "" {
		return "", fmt.Errorf("input_file required")
	}

	opsRaw, ok := args["operations"].([]interface{})
	if !ok {
		return "", fmt.Errorf("operations must be an array")
	}

	outputFile, _ := args["output_file"].(string)
	format := "mp4"
	if f, ok := args["format"].(string); ok && f != "" {
		format = f
	}

	if outputFile == "" {
		outputFile = fmt.Sprintf("edited_%d.%s", time.Now().Unix(), format)
	}

	// First, try to find the file path from upload registry
	var filePath string
	if meta, ok := uploadregistry.Get(inputFile); ok {
		filePath = meta.FilePath
	} else if meta, ok := uploadregistry.FindByFilename(inputFile); ok {
		filePath = meta.FilePath
	} else if meta, ok := uploadregistry.FindByBasename(inputFile); ok {
		filePath = meta.FilePath
	} else if _, err := os.Stat(inputFile); err == nil {
		filePath = inputFile
	} else {
		return "", fmt.Errorf("file not found: %s", inputFile)
	}

	// Parse operations
	var operations []mediaprocessing.VideoOperation
	for _, opRaw := range opsRaw {
		opMap, ok := opRaw.(map[string]interface{})
		if !ok {
			continue
		}

		op := mediaprocessing.VideoOperation{
			Type: opMap["type"].(string),
		}

		if start, ok := opMap["start_time"].(string); ok {
			op.StartTime = start
		}
		if end, ok := opMap["end_time"].(string); ok {
			op.EndTime = end
		}
		if text, ok := opMap["text"].(string); ok {
			op.Text = text
		}
		if position, ok := opMap["position"].(string); ok {
			op.Position = position
		}

		operations = append(operations, op)
	}

	// Call the real video editing using FFmpeg
	result, err := mediaprocessing.EditVideo(ctx, filePath, operations, outputFile, format)
	if err != nil {
		return "", err
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Video Editing Report\n"))
	output.WriteString(fmt.Sprintf("====================\n"))
	output.WriteString(fmt.Sprintf("Input File: %s\n", filePath))
	output.WriteString(fmt.Sprintf("Output File: %s\n", result.OutputFile))
	output.WriteString(fmt.Sprintf("Format: %s\n", format))
	output.WriteString(fmt.Sprintf("Duration: %s\n", result.Duration))
	output.WriteString(fmt.Sprintf("Size: %s\n", result.Size))
	output.WriteString(fmt.Sprintf("Operations Performed: %d\n\n", len(operations)))

	for i, op := range operations {
		output.WriteString(fmt.Sprintf("%d. Operation: %s\n", i+1, op.Type))
		if op.StartTime != "" {
			output.WriteString(fmt.Sprintf("   Start Time: %s\n", op.StartTime))
		}
		if op.EndTime != "" {
			output.WriteString(fmt.Sprintf("   End Time: %s\n", op.EndTime))
		}
		if op.Text != "" {
			output.WriteString(fmt.Sprintf("   Text: %s\n", op.Text))
		}
		if op.Position != "" {
			output.WriteString(fmt.Sprintf("   Position: %s\n", op.Position))
		}
		output.WriteString("\n")
	}

	output.WriteString(fmt.Sprintf("✅ %s\n", result.Message))

	return output.String(), nil
}

// ============ Transcribe Tool (Clip) ============

type TranscribeTool struct{}

func NewTranscribeTool() *TranscribeTool { return &TranscribeTool{} }

func (t *TranscribeTool) Name() string { return "transcribe" }
func (t *TranscribeTool) Description() string {
	return "Transcribe audio/video content to text with timestamp support"
}

func (t *TranscribeTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "transcribe",
			"description": "Convert audio/video speech to text. Supports multiple languages and provides timestamps.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"media_file":          map[string]interface{}{"type": "string", "description": "Path to audio/video file"},
					"language":            map[string]interface{}{"type": "string", "description": "Language code (en, zh, es, etc.)", "default": "en"},
					"model":               map[string]interface{}{"type": "string", "description": "Speech recognition model", "default": "whisper-base"},
					"timestamps":          map[string]interface{}{"type": "boolean", "description": "Include timestamps in output", "default": true},
					"speaker_diarization": map[string]interface{}{"type": "boolean", "description": "Identify different speakers", "default": false},
				},
				"required": []string{"media_file"},
			},
		},
	}
}

func (t *TranscribeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	mediaFile, _ := args["media_file"].(string)
	if mediaFile == "" {
		return "", fmt.Errorf("media_file required")
	}

	// First, try to find the file path from upload registry
	var filePath string
	if meta, ok := uploadregistry.Get(mediaFile); ok {
		filePath = meta.FilePath
	} else if meta, ok := uploadregistry.FindByFilename(mediaFile); ok {
		filePath = meta.FilePath
	} else if meta, ok := uploadregistry.FindByBasename(mediaFile); ok {
		filePath = meta.FilePath
	} else if _, err := os.Stat(mediaFile); err == nil {
		filePath = mediaFile
	} else {
		return "", fmt.Errorf("file not found: %s", mediaFile)
	}

	// Call the real transcription using OpenAI/Groq Whisper API
	text, err := mediaprocessing.TranscribeAudio(ctx, filePath)
	if err != nil {
		return "", err
	}

	return text, nil
}

// ============ Highlight Detection Tool (Clip) ============

type HighlightDetectionTool struct{}

func NewHighlightDetectionTool() *HighlightDetectionTool { return &HighlightDetectionTool{} }

func (t *HighlightDetectionTool) Name() string { return "highlight_detection" }
func (t *HighlightDetectionTool) Description() string {
	return "Detect and extract highlight moments from videos based on audio/activity cues"
}

func (t *HighlightDetectionTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "highlight_detection",
			"description": "Automatically detect highlight moments in videos using audio energy, motion detection, and scene analysis.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_file": map[string]interface{}{"type": "string", "description": "Path to input video file"},
					"detection_method": map[string]interface{}{
						"type":        "string",
						"description": "Method to use (audio_energy, motion_detection, scene_analysis)",
						"default":     "audio_energy",
					},
					"sensitivity":  map[string]interface{}{"type": "number", "description": "Detection sensitivity (0.0-1.0)", "default": 0.7},
					"min_duration": map[string]interface{}{"type": "number", "description": "Minimum highlight duration in seconds", "default": 5.0},
					"max_clips":    map[string]interface{}{"type": "integer", "description": "Maximum number of highlights to extract", "default": 10},
				},
				"required": []string{"video_file"},
			},
		},
	}
}

func (t *HighlightDetectionTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	videoFile, _ := args["video_file"].(string)
	if videoFile == "" {
		return "", fmt.Errorf("video_file required")
	}

	method := "audio_energy"
	if m, ok := args["detection_method"].(string); ok && m != "" {
		method = m
	}

	sensitivity := 0.7
	if s, ok := args["sensitivity"].(float64); ok {
		sensitivity = s
	}

	minDuration := 5.0
	if md, ok := args["min_duration"].(float64); ok {
		minDuration = md
	}

	maxClips := 10
	if mc, ok := args["max_clips"].(float64); ok {
		maxClips = int(mc)
	}

	// First, try to find the file path from upload registry
	var filePath string
	if meta, ok := uploadregistry.Get(videoFile); ok {
		filePath = meta.FilePath
	} else if meta, ok := uploadregistry.FindByFilename(videoFile); ok {
		filePath = meta.FilePath
	} else if meta, ok := uploadregistry.FindByBasename(videoFile); ok {
		filePath = meta.FilePath
	} else if _, err := os.Stat(videoFile); err == nil {
		filePath = videoFile
	} else {
		return "", fmt.Errorf("file not found: %s", videoFile)
	}

	// Call the real highlight detection using FFmpeg
	result, err := mediaprocessing.DetectHighlights(ctx, filePath, method, sensitivity, minDuration, maxClips)
	if err != nil {
		return "", err
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Highlight Detection Report\n"))
	output.WriteString(fmt.Sprintf("==========================\n"))
	output.WriteString(fmt.Sprintf("Video File: %s\n", filePath))
	output.WriteString(fmt.Sprintf("Detection Method: %s\n", method))
	output.WriteString(fmt.Sprintf("Sensitivity: %.2f\n", sensitivity))
	output.WriteString(fmt.Sprintf("Min Duration: %.1f sec\n", minDuration))
	output.WriteString(fmt.Sprintf("Max Clips: %d\n", maxClips))
	output.WriteString(fmt.Sprintf("Highlights Found: %d\n\n", len(result.Highlights)))

	for i, highlight := range result.Highlights {
		output.WriteString(fmt.Sprintf("Highlight #%d:\n", i+1))
		output.WriteString(fmt.Sprintf("  Start: %s\n", highlight.Start))
		output.WriteString(fmt.Sprintf("  End:   %s\n", highlight.End))
		output.WriteString(fmt.Sprintf("  Confidence: %.0f%%\n", highlight.Confidence*100))
		output.WriteString(fmt.Sprintf("  Description: %s\n\n", highlight.Description))
	}

	output.WriteString(fmt.Sprintf("✅ %s\n", result.Message))

	return output.String(), nil
}

// ============ Caption Generator Tool (Clip) ============

type CaptionGeneratorTool struct{}

func NewCaptionGeneratorTool() *CaptionGeneratorTool { return &CaptionGeneratorTool{} }

func (t *CaptionGeneratorTool) Name() string { return "caption_generator" }
func (t *CaptionGeneratorTool) Description() string {
	return "Generate captions/subtitles for videos with formatting and timing options"
}

func (t *CaptionGeneratorTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "caption_generator",
			"description": "Create captions/subtitles for videos with customizable formatting, positioning, and timing.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_file": map[string]interface{}{"type": "string", "description": "Path to input video file"},
					"transcript": map[string]interface{}{"type": "string", "description": "Transcript text with timestamps"},
					"format": map[string]interface{}{
						"type":        "string",
						"description": "Caption format (srt, vtt, ass)",
						"default":     "srt",
					},
					"style": map[string]interface{}{
						"type":        "object",
						"description": "Caption styling options",
						"properties": map[string]interface{}{
							"font_size":        map[string]interface{}{"type": "integer", "default": 16},
							"font_color":       map[string]interface{}{"type": "string", "default": "white"},
							"background_color": map[string]interface{}{"type": "string", "default": "black"},
							"position":         map[string]interface{}{"type": "string", "default": "bottom"},
						},
					},
					"burn_in": map[string]interface{}{"type": "boolean", "description": "Burn captions into video", "default": false},
				},
				"required": []string{"video_file", "transcript"},
			},
		},
	}
}

func (t *CaptionGeneratorTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	videoFile, _ := args["video_file"].(string)
	if videoFile == "" {
		return "", fmt.Errorf("video_file required")
	}

	transcript, _ := args["transcript"].(string)
	if transcript == "" {
		return "", fmt.Errorf("transcript required")
	}

	format := "srt"
	if f, ok := args["format"].(string); ok && f != "" {
		format = f
	}

	burnIn := false
	if bi, ok := args["burn_in"].(bool); ok {
		burnIn = bi
	}

	// Parse style if provided
	var style mediaprocessing.CaptionStyle
	if styleRaw, ok := args["style"].(map[string]interface{}); ok {
		if fs, ok := styleRaw["font_size"].(float64); ok {
			style.FontSize = int(fs)
		}
		if fc, ok := styleRaw["font_color"].(string); ok {
			style.FontColor = fc
		}
		if bc, ok := styleRaw["background_color"].(string); ok {
			style.BackgroundColor = bc
		}
		if pos, ok := styleRaw["position"].(string); ok {
			style.Position = pos
		}
	}

	// First, try to find the file path from upload registry
	var filePath string
	if meta, ok := uploadregistry.Get(videoFile); ok {
		filePath = meta.FilePath
	} else if meta, ok := uploadregistry.FindByFilename(videoFile); ok {
		filePath = meta.FilePath
	} else if meta, ok := uploadregistry.FindByBasename(videoFile); ok {
		filePath = meta.FilePath
	} else if _, err := os.Stat(videoFile); err == nil {
		filePath = videoFile
	} else {
		return "", fmt.Errorf("file not found: %s", videoFile)
	}

	// Call the real caption generation
	result, err := mediaprocessing.GenerateCaptions(ctx, filePath, transcript, format, style, burnIn)
	if err != nil {
		return "", err
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Caption Generation Report\n"))
	output.WriteString(fmt.Sprintf("=========================\n"))
	output.WriteString(fmt.Sprintf("Video File: %s\n", filePath))
	output.WriteString(fmt.Sprintf("Caption File: %s\n", result.CaptionFile))
	output.WriteString(fmt.Sprintf("Format: %s\n", result.Format))
	output.WriteString(fmt.Sprintf("Captions Generated: %d\n", result.CaptionsCount))
	output.WriteString(fmt.Sprintf("Burn-in: %v\n\n", burnIn))

	if style.FontSize > 0 {
		output.WriteString(fmt.Sprintf("Style: Font Size=%d, Color=%s, BG Color=%s, Position=%s\n",
			style.FontSize, style.FontColor, style.BackgroundColor, style.Position))
	}

	output.WriteString(fmt.Sprintf("\n✅ %s\n", result.Message))

	return output.String(), nil
}
