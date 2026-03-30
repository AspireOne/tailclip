package event

import (
	"crypto/sha256"
	"fmt"
	"time"
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
	h.Write([]byte(content))
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

func NewEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}
