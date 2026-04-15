package api

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/penzhan8451/fangclaw-go/internal/uploadregistry"
)

type Attachment struct {
	FileID      string `json:"file_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
}

func ResolveAttachments(attachments []Attachment, text string) string {
	if len(attachments) == 0 {
		return text
	}

	var parts []string
	if text != "" {
		parts = append(parts, text)
	}

	for _, att := range attachments {
		meta, ok := uploadregistry.Get(att.FileID)
		if !ok {
			continue
		}

		content, err := os.ReadFile(meta.FilePath)
		if err != nil {
			continue
		}

		if strings.HasPrefix(meta.ContentType, "image/") {
			b64 := base64.StdEncoding.EncodeToString(content)
			parts = append(parts, fmt.Sprintf("\n--- Image: %s ---\n[data:image;base64,%s]\n", meta.Filename, b64))
		} else {
			parts = append(parts, fmt.Sprintf("\n--- File: %s ---\n%s\n", meta.Filename, string(content)))
		}
	}

	return strings.Join(parts, "")
}
