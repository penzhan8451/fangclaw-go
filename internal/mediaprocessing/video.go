package mediaprocessing

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// VideoOperation represents a single video editing operation
type VideoOperation struct {
	Type      string
	StartTime string
	EndTime   string
	Text      string
	Position  string
}

// EditVideoResult represents the result of video editing
type EditVideoResult struct {
	OutputFile string
	Duration   string
	Size       string
	Message    string
}

// EditVideo performs video editing operations using FFmpeg
func EditVideo(ctx context.Context, inputFile string, operations []VideoOperation, outputFile string, format string) (*EditVideoResult, error) {
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file not found: %s", inputFile)
	}

	if outputFile == "" {
		outputFile = fmt.Sprintf("edited_%d.%s", time.Now().Unix(), format)
	}

	currentFile := inputFile
	var err error

	for i, op := range operations {
		switch op.Type {
		case "trim":
			currentFile, err = trimVideo(ctx, currentFile, op.StartTime, op.EndTime, fmt.Sprintf("trim_%d_%s", i, filepath.Base(currentFile)))
		case "crop":
			currentFile, err = cropVideo(ctx, currentFile, fmt.Sprintf("crop_%d_%s", i, filepath.Base(currentFile)))
		case "add_text":
			currentFile, err = addTextOverlay(ctx, currentFile, op.Text, op.Position, fmt.Sprintf("text_%d_%s", i, filepath.Base(currentFile)))
		}
		if err != nil {
			return nil, err
		}
	}

	if currentFile != inputFile {
		if err := os.Rename(currentFile, outputFile); err != nil {
			return nil, fmt.Errorf("failed to rename output file: %w", err)
		}
	}

	info, err := getVideoInfo(ctx, outputFile)
	if err != nil {
		return &EditVideoResult{
			OutputFile: outputFile,
			Message:    "Video edited successfully, but failed to get file info",
		}, nil
	}

	return &EditVideoResult{
		OutputFile: outputFile,
		Duration:   info.Duration,
		Size:       info.Size,
		Message:    "Video edited successfully",
	}, nil
}

// trimVideo trims a video from start time to end time
func trimVideo(ctx context.Context, inputFile, startTime, endTime, outputFile string) (string, error) {
	args := []string{"-y"}

	if startTime != "" {
		args = append(args, "-ss", startTime)
	}

	args = append(args, "-i", inputFile)

	if endTime != "" {
		if startTime != "" {
			args = append(args, "-to", endTime)
		} else {
			args = append(args, "-t", endTime)
		}
	}

	args = append(args, "-c:v", "libx264", "-c:a", "aac", "-preset", "fast", "-crf", "23", "-movflags", "+faststart", outputFile)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg trim failed: %w - %s", err, stderr.String())
	}

	return outputFile, nil
}

// cropVideo crops a video to vertical 9:16 aspect ratio
func cropVideo(ctx context.Context, inputFile, outputFile string) (string, error) {
	args := []string{
		"-y", "-i", inputFile,
		"-vf", "crop=ih*9/16:ih:(iw-ih*9/16)/2:0,scale=1080:1920",
		"-c:a", "copy",
		outputFile,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg crop failed: %w - %s", err, stderr.String())
	}

	return outputFile, nil
}

// addTextOverlay adds a text overlay to the video
func addTextOverlay(ctx context.Context, inputFile, text, position, outputFile string) (string, error) {
	x := "(w-text_w)/2"
	y := "(h-text_h)/2"

	switch position {
	case "top":
		y = "10"
	case "bottom":
		y = "h-text_h-10"
	case "left":
		x = "10"
	case "right":
		x = "w-text_w-10"
	case "top_left":
		x = "10"
		y = "10"
	case "top_right":
		x = "w-text_w-10"
		y = "10"
	case "bottom_left":
		x = "10"
		y = "h-text_h-10"
	case "bottom_right":
		x = "w-text_w-10"
		y = "h-text_h-10"
	}

	escapedText := strings.ReplaceAll(text, ":", "\\:")
	escapedText = strings.ReplaceAll(escapedText, "'", "\\'")

	args := []string{
		"-y", "-i", inputFile,
		"-vf", fmt.Sprintf("drawtext=text='%s':fontsize=48:fontcolor=white:borderw=3:bordercolor=black:x=%s:y=%s", escapedText, x, y),
		"-c:a", "copy",
		outputFile,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg text overlay failed: %w - %s", err, stderr.String())
	}

	return outputFile, nil
}

// videoInfo holds basic video metadata
type videoInfo struct {
	Duration string
	Size     string
}

// getVideoInfo retrieves basic information about a video file
func getVideoInfo(ctx context.Context, filePath string) (*videoInfo, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	sizeStr := formatFileSize(fileInfo.Size())

	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", filePath)
	output, err := cmd.Output()
	if err != nil {
		return &videoInfo{
			Duration: "unknown",
			Size:     sizeStr,
		}, nil
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err == nil {
		minutes := int(duration) / 60
		seconds := int(duration) % 60
		durationStr = fmt.Sprintf("%d:%02d", minutes, seconds)
	}

	return &videoInfo{
		Duration: durationStr,
		Size:     sizeStr,
	}, nil
}

// formatFileSize converts bytes to a human-readable format
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Highlight represents a detected highlight moment
type Highlight struct {
	Start       string
	End         string
	Confidence  float64
	Description string
}

// DetectHighlightsResult represents the result of highlight detection
type DetectHighlightsResult struct {
	VideoFile  string
	Method     string
	Highlights []Highlight
	Message    string
}

// DetectHighlights detects highlight moments in a video using FFmpeg
func DetectHighlights(ctx context.Context, videoFile, method string, sensitivity, minDuration float64, maxClips int) (*DetectHighlightsResult, error) {
	if _, err := os.Stat(videoFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("video file not found: %s", videoFile)
	}

	var highlights []Highlight
	var err error

	switch method {
	case "audio_energy":
		highlights, err = detectSilenceAndHighlights(ctx, videoFile, sensitivity, minDuration)
	case "motion_detection", "scene_analysis":
		highlights, err = detectSceneChanges(ctx, videoFile, sensitivity, minDuration)
	default:
		highlights, err = detectSilenceAndHighlights(ctx, videoFile, sensitivity, minDuration)
	}

	if err != nil {
		return nil, err
	}

	if len(highlights) > maxClips {
		highlights = highlights[:maxClips]
	}

	return &DetectHighlightsResult{
		VideoFile:  videoFile,
		Method:     method,
		Highlights: highlights,
		Message:    "Highlight detection completed successfully",
	}, nil
}

// detectSilenceAndHighlights detects highlights by finding non-silent segments
func detectSilenceAndHighlights(ctx context.Context, videoFile string, sensitivity, minDuration float64) ([]Highlight, error) {
	noiseDb := fmt.Sprintf("%.1f", -30.0-(sensitivity*20.0))
	durationSec := fmt.Sprintf("%.1f", 1.5)

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", videoFile,
		"-af", fmt.Sprintf("silencedetect=noise=%sdB:d=%s", noiseDb, durationSec),
		"-f", "null", "-")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg silence detection failed: %w - %s", err, string(output))
	}

	var silenceStarts, silenceEnds []float64
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "silence_start") {
			parts := strings.Split(line, "silence_start: ")
			if len(parts) > 1 {
				if ts, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
					silenceStarts = append(silenceStarts, ts)
				}
			}
		} else if strings.Contains(line, "silence_end") {
			parts := strings.Split(line, "silence_end: ")
			if len(parts) > 1 {
				if ts, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
					silenceEnds = append(silenceEnds, ts)
				}
			}
		}
	}

	var highlights []Highlight
	for i := 0; i < len(silenceEnds) && i < len(silenceStarts); i++ {
		if i+1 < len(silenceStarts) {
			start := silenceEnds[i]
			end := silenceStarts[i+1]
			duration := end - start
			if duration >= minDuration && duration <= 120 {
				highlights = append(highlights, Highlight{
					Start:       formatTimestamp(start),
					End:         formatTimestamp(end),
					Confidence:  0.85,
					Description: "Audio activity detected",
				})
			}
		}
	}

	if len(highlights) == 0 {
		durationSec := 30.0
		highlights = append(highlights, Highlight{
			Start:       "00:00:00",
			End:         formatTimestamp(durationSec),
			Confidence:  0.5,
			Description: "Default clip (no highlights detected)",
		})
	}

	return highlights, nil
}

// detectSceneChanges detects highlights by finding scene changes
func detectSceneChanges(ctx context.Context, videoFile string, sensitivity, minDuration float64) ([]Highlight, error) {
	sceneThreshold := fmt.Sprintf("%.2f", 0.3-(sensitivity*0.2))

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", videoFile,
		"-filter:v", fmt.Sprintf("select='gt(scene,%s)',showinfo", sceneThreshold),
		"-f", "null", "-")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg scene detection failed: %w - %s", err, string(output))
	}

	var sceneTimes []float64
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "pts_time:") {
			parts := strings.Split(line, "pts_time:")
			if len(parts) > 1 {
				nextPart := strings.Split(parts[1], " ")[0]
				if ts, err := strconv.ParseFloat(strings.TrimSpace(nextPart), 64); err == nil {
					sceneTimes = append(sceneTimes, ts)
				}
			}
		}
	}

	var highlights []Highlight
	for i := 0; i < len(sceneTimes)-1; i++ {
		start := sceneTimes[i]
		end := sceneTimes[i+1]
		duration := end - start
		if duration >= minDuration && duration <= 120 {
			highlights = append(highlights, Highlight{
				Start:       formatTimestamp(start),
				End:         formatTimestamp(end),
				Confidence:  0.80,
				Description: "Scene change detected",
			})
		}
	}

	if len(highlights) == 0 {
		highlights, _ = detectSilenceAndHighlights(ctx, videoFile, sensitivity, minDuration)
	}

	return highlights, nil
}

// formatTimestamp converts seconds to HH:MM:SS format
func formatTimestamp(seconds float64) string {
	hrs := int(seconds) / 3600
	mins := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hrs, mins, secs)
}

// CaptionStyle represents styling options for captions
type CaptionStyle struct {
	FontSize        int
	FontColor       string
	BackgroundColor string
	Position        string
}

// Caption represents a single caption entry
type Caption struct {
	Index     int
	StartTime string
	EndTime   string
	Text      string
}

// GenerateCaptionsResult represents the result of caption generation
type GenerateCaptionsResult struct {
	VideoFile     string
	CaptionFile   string
	Format        string
	CaptionsCount int
	Message       string
}

// GenerateCaptions generates subtitle files in SRT or VTT format
func GenerateCaptions(ctx context.Context, videoFile, transcript, format string, style CaptionStyle, burnIn bool) (*GenerateCaptionsResult, error) {
	if _, err := os.Stat(videoFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("video file not found: %s", videoFile)
	}

	captions, err := parseTranscript(transcript)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transcript: %w", err)
	}

	captionFile := fmt.Sprintf("captions_%d.%s", time.Now().Unix(), format)
	var content string

	switch format {
	case "srt":
		content = generateSRT(captions)
	case "vtt":
		content = generateVTT(captions)
	default:
		content = generateSRT(captions)
		captionFile = fmt.Sprintf("captions_%d.srt", time.Now().Unix())
	}

	if err := os.WriteFile(captionFile, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write caption file: %w", err)
	}

	if burnIn {
		burnedFile := fmt.Sprintf("burned_%s", filepath.Base(videoFile))
		if err := burnCaptions(ctx, videoFile, captionFile, style, burnedFile); err != nil {
			return &GenerateCaptionsResult{
				VideoFile:     videoFile,
				CaptionFile:   captionFile,
				Format:        format,
				CaptionsCount: len(captions),
				Message:       fmt.Sprintf("Captions generated, but failed to burn into video: %v", err),
			}, nil
		}
		return &GenerateCaptionsResult{
			VideoFile:     burnedFile,
			CaptionFile:   captionFile,
			Format:        format,
			CaptionsCount: len(captions),
			Message:       "Captions generated and burned into video successfully",
		}, nil
	}

	return &GenerateCaptionsResult{
		VideoFile:     videoFile,
		CaptionFile:   captionFile,
		Format:        format,
		CaptionsCount: len(captions),
		Message:       "Captions generated successfully",
	}, nil
}

// parseTranscript parses transcript text with timestamps into Caption objects
func parseTranscript(transcript string) ([]Caption, error) {
	var captions []Caption
	lines := strings.Split(transcript, "\n")
	index := 1

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var startTime, endTime string
		text := line

		if strings.Contains(line, "-->") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) >= 3 {
				startTime = strings.Trim(parts[0], "[]() ")
				endTime = strings.Trim(parts[2], "[]() ")
				text = strings.Join(parts[3:], " ")
				if text == "" && i+1 < len(lines) {
					text = strings.TrimSpace(lines[i+1])
					i++
				}
			}
		} else {
			startTime = formatTimestamp(float64(index * 5))
			endTime = formatTimestamp(float64(index*5 + 5))
		}

		if text != "" {
			captions = append(captions, Caption{
				Index:     index,
				StartTime: startTime,
				EndTime:   endTime,
				Text:      text,
			})
			index++
		}
	}

	if len(captions) == 0 {
		captions = append(captions, Caption{
			Index:     1,
			StartTime: "00:00:00",
			EndTime:   "00:00:10",
			Text:      transcript,
		})
	}

	return captions, nil
}

// generateSRT generates SubRip (SRT) format captions
func generateSRT(captions []Caption) string {
	var buf bytes.Buffer
	for _, cap := range captions {
		startSRT := convertToSRTTime(cap.StartTime)
		endSRT := convertToSRTTime(cap.EndTime)
		buf.WriteString(fmt.Sprintf("%d\n", cap.Index))
		buf.WriteString(fmt.Sprintf("%s --> %s\n", startSRT, endSRT))
		buf.WriteString(fmt.Sprintf("%s\n\n", cap.Text))
	}
	return buf.String()
}

// convertToSRTTime converts time format to SRT format (HH:MM:SS,mmm)
func convertToSRTTime(timeStr string) string {
	if !strings.Contains(timeStr, ",") && !strings.Contains(timeStr, ".") {
		return timeStr + ",000"
	}
	return strings.Replace(timeStr, ".", ",", 1)
}

// generateVTT generates WebVTT format captions
func generateVTT(captions []Caption) string {
	var buf bytes.Buffer
	buf.WriteString("WEBVTT\n\n")
	for _, cap := range captions {
		startVTT := convertToVTTTime(cap.StartTime)
		endVTT := convertToVTTTime(cap.EndTime)
		buf.WriteString(fmt.Sprintf("%d\n", cap.Index))
		buf.WriteString(fmt.Sprintf("%s --> %s\n", startVTT, endVTT))
		buf.WriteString(fmt.Sprintf("%s\n\n", cap.Text))
	}
	return buf.String()
}

// convertToVTTTime converts time format to VTT format (HH:MM:SS.mmm)
func convertToVTTTime(timeStr string) string {
	if !strings.Contains(timeStr, ".") && !strings.Contains(timeStr, ",") {
		return timeStr + ".000"
	}
	return strings.Replace(timeStr, ",", ".", 1)
}

// burnCaptions burns captions into video using FFmpeg
func burnCaptions(ctx context.Context, videoFile, captionFile string, style CaptionStyle, outputFile string) error {
	fontSize := 24
	if style.FontSize > 0 {
		fontSize = style.FontSize
	}

	fontColor := "white"
	if style.FontColor != "" {
		fontColor = style.FontColor
	}

	bgColor := "black"
	if style.BackgroundColor != "" {
		bgColor = style.BackgroundColor
	}

	args := []string{
		"-y", "-i", videoFile,
		"-vf", fmt.Sprintf("subtitles=%s:force_style='FontSize=%d,PrimaryColour=&H%x&,BackColour=&H%x&,Alignment=2,MarginV=20'",
			captionFile, fontSize, colorToHex(fontColor), colorToHex(bgColor)),
		"-c:a", "copy",
		outputFile,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg burn captions failed: %w - %s", err, stderr.String())
	}

	return nil
}

// colorToHex converts common color names to ASS hex format (BBGGRR)
func colorToHex(color string) string {
	colorMap := map[string]string{
		"white":   "FFFFFF",
		"black":   "000000",
		"red":     "0000FF",
		"green":   "00FF00",
		"blue":    "FF0000",
		"yellow":  "00FFFF",
		"cyan":    "FFFF00",
		"magenta": "FF00FF",
	}

	if hex, ok := colorMap[strings.ToLower(color)]; ok {
		return hex
	}

	return "FFFFFF"
}
