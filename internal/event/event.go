package event

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

type ClipboardEvent struct {
	ID             string    `json:"id"`
	Content        string    `json:"content"`
	ContentHash    string    `json:"content_hash"`
	SourceDeviceID string    `json:"source_device_id"`
	CreatedAt      time.Time `json:"created_at"`
}

func NewClipboardEvent(content, deviceID string, detectedAt time.Time) ClipboardEvent {
	return ClipboardEvent{
		ID:             NewEventID(),
		Content:        content,
		ContentHash:    HashContent(content),
		SourceDeviceID: deviceID,
		CreatedAt:      detectedAt.UTC(),
	}
}

func HashContent(content string) string {
	h := sha256.New()
	h.Write([]byte(normalizeHashInput(content)))
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

func normalizeHashInput(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	content = norm.NFC.String(content)
	return content
}

func NewEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}
