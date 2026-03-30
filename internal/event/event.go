package event

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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

func NewClipboardEvent(content, deviceID string, createdAt time.Time) ClipboardEvent {
	return ClipboardEvent{
		ID:             NewEventID(),
		Content:        content,
		ContentHash:    HashContent(content),
		SourceDeviceID: deviceID,
		CreatedAt:      createdAt.UTC(),
	}
}

func HashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func NewEventID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}

	return "evt_" + hex.EncodeToString(buf[:])
}
