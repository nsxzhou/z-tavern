package chat

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zhouzirui/z-tavern/backend/internal/model/chat"
)

var (
	ErrPersonaRequired = errors.New("persona id is required")
	ErrSessionNotFound = errors.New("session not found")
)

// Service encapsulates conversation state management.
type Service struct {
	mu       sync.RWMutex
	sessions map[string]chat.Session
	messages map[string][]chat.Message
}

// NewService bootstraps the in-memory chat service suitable for early iterations.
func NewService() *Service {
	return &Service{
		sessions: make(map[string]chat.Session),
		messages: make(map[string][]chat.Message),
	}
}

// CreateSession provisions an anonymous session bound to a persona.
func (s *Service) CreateSession(_ context.Context, personaID string) (chat.Session, error) {
	if personaID == "" {
		return chat.Session{}, ErrPersonaRequired
	}

	session := chat.Session{
		ID:        uuid.NewString(),
		PersonaID: personaID,
		CreatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.messages[session.ID] = make([]chat.Message, 0, 16)
	s.mu.Unlock()

	return session, nil
}

// SaveMessage appends a message to the session history.
func (s *Service) SaveMessage(_ context.Context, message chat.Message) error {
	if message.SessionID == "" {
		return ErrSessionNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[message.SessionID]; !ok {
		return ErrSessionNotFound
	}

	message.ID = uuid.NewString()
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	}

	s.messages[message.SessionID] = append(s.messages[message.SessionID], message)
	return nil
}

// GetSession retrieves a session by identifier.
func (s *Service) GetSession(_ context.Context, sessionID string) (chat.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return chat.Session{}, ErrSessionNotFound
	}
	return session, nil
}

// LoadTranscript returns stored messages for the provided session.
func (s *Service) LoadTranscript(_ context.Context, sessionID string) ([]chat.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages, ok := s.messages[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}

	copied := make([]chat.Message, len(messages))
	copy(copied, messages)
	return copied, nil
}
