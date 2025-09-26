package chat

import "time"

// Message persists individual turns for audit/debug.
type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	Emotion   string    `json:"emotion,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}