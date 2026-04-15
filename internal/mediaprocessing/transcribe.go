package mediaprocessing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// TranscribeAudioResult represents the result of audio transcription.
type TranscribeAudioResult struct {
	Text string `json:"text"`
}

// TranscribeAudio transcribes audio using OpenAI/Groq Whisper API.
func TranscribeAudio(ctx context.Context, filePath string) (string, error) {
	var apiKey string
	var baseURL string
	var model string

	if os.Getenv("GROQ_API_KEY") != "" {
		apiKey = os.Getenv("GROQ_API_KEY")
		baseURL = "https://api.groq.com/openai/v1/audio/transcriptions"
		model = "whisper-large-v3"
	} else if os.Getenv("OPENAI_API_KEY") != "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		baseURL = "https://api.openai.com/v1/audio/transcriptions"
		model = "whisper-1"
	} else {
		return "", fmt.Errorf("no API key configured: set GROQ_API_KEY or OPENAI_API_KEY")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	if err := writer.WriteField("model", model); err != nil {
		return "", fmt.Errorf("failed to write model field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL, &body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var result TranscribeAudioResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Text, nil
}
