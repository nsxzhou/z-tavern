package chat

import "time"

// Session captures a transient anonymous conversation.
type Session struct {
	ID        string    `json:"id"`
	PersonaID string    `json:"personaId"`
	CreatedAt time.Time `json:"createdAt"`
}