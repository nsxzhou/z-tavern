package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/cloudwego/eino/schema"
	analysis "github.com/zhouzirui/z-tavern/backend/internal/analysis/emotion"
	"github.com/zhouzirui/z-tavern/backend/internal/model/chat"
	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
	aiService "github.com/zhouzirui/z-tavern/backend/internal/service/ai"
	chatService "github.com/zhouzirui/z-tavern/backend/internal/service/chat"
	emotionservice "github.com/zhouzirui/z-tavern/backend/internal/service/emotion"
)

// Handler manages streaming AI responses via Server-Sent Events
type Handler struct {
	aiService  *aiService.Service
	emotionSvc *emotionservice.Service
	chatSvc    *chatService.Service
	personas   persona.Store
}

// New creates a new stream handler
func New(aiSvc *aiService.Service, emotionSvc *emotionservice.Service, chatSvc *chatService.Service, personas persona.Store) *Handler {
	return &Handler{
		aiService:  aiSvc,
		emotionSvc: emotionSvc,
		chatSvc:    chatSvc,
		personas:   personas,
	}
}

// StreamResponse represents a streaming response chunk
type StreamResponse struct {
	Event     string `json:"event"`
	Content   string `json:"content,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Finished  bool   `json:"finished,omitempty"`
	Error     string `json:"error,omitempty"`
}

// HandleStreamRequest processes streaming AI responses for a chat session
func (h *Handler) HandleStreamRequest(ctx context.Context, w http.ResponseWriter, sessionID string, userMessage string) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming unsupported")
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Resolve session and persona context
	session, persona, err := h.getSessionPersona(ctx, sessionID)
	if err != nil {
		h.sendSSEError(w, flusher, fmt.Sprintf("failed to get session persona: %v", err))
		return err
	}

	// Load conversation history
	messages, err := h.chatSvc.LoadTranscript(ctx, session.ID)
	if err != nil {
		h.sendSSEError(w, flusher, fmt.Sprintf("failed to load conversation: %v", err))
		return err
	}

	// Save user message. When the client already persisted the message via REST, avoid duplicating it.
	if !hasMatchingUserMessage(messages, sessionID, userMessage) {
		userMsg := chat.Message{
			SessionID: sessionID,
			Sender:    "user",
			Content:   userMessage,
		}
		if err := h.chatSvc.SaveMessage(ctx, userMsg); err != nil {
			log.Printf("failed to save user message: %v", err)
		} else {
			messages = append(messages, userMsg)
		}
	}

	var promptGuidance *emotionservice.Guidance
	if h.emotionSvc != nil && h.emotionSvc.Enabled() {
		guidance := h.emotionSvc.Analyze(ctx, persona, messages, userMessage, "")
		promptGuidance = &guidance
	}

	// Send initial response
	h.sendSSE(w, flusher, StreamResponse{
		Event:     "start",
		SessionID: sessionID,
		Content:   fmt.Sprintf("%s的回复:", persona.Name),
	})

	response, err := h.dispatchAIResponse(ctx, w, flusher, sessionID, persona, messages, userMessage, promptGuidance)
	if err != nil {
		h.sendSSEError(w, flusher, fmt.Sprintf("AI generation failed: %v", err))
		return err
	}

	// Save assistant message
	var finalGuidance emotionservice.Guidance
	if h.emotionSvc != nil {
		finalGuidance = h.emotionSvc.Analyze(ctx, persona, append(messages, chat.Message{
			SessionID: sessionID,
			Sender:    "assistant",
			Content:   response.Content,
		}), userMessage, response.Content)
	} else if promptGuidance != nil {
		finalGuidance = *promptGuidance
	} else {
		fallbackDecision := analysis.Analyze(userMessage, response.Content)
		finalGuidance = emotionservice.Guidance{Decision: fallbackDecision}
	}

	assistantMsg := chat.Message{
		SessionID: sessionID,
		Sender:    "assistant",
		Content:   response.Content,
		Emotion:   string(finalGuidance.Decision.Emotion),
	}
	if err := h.chatSvc.SaveMessage(ctx, assistantMsg); err != nil {
		log.Printf("failed to save assistant message: %v", err)
	}

	emotionPayload, err := json.Marshal(map[string]any{
		"emotion":    finalGuidance.Decision.Emotion,
		"scale":      finalGuidance.Decision.Scale,
		"confidence": finalGuidance.Confidence,
	})
	if err == nil {
		h.sendSSE(w, flusher, StreamResponse{
			Event:     "emotion",
			SessionID: sessionID,
			Content:   string(emotionPayload),
		})
	}

	// Send completion signal
	h.sendSSE(w, flusher, StreamResponse{
		Event:     "end",
		SessionID: sessionID,
		Finished:  true,
	})

	log.Printf("[stream] completed response for session=%s, persona=%s", sessionID, persona.ID)
	return nil
}

// generateStreamingResponse creates an AI response using the enhanced prompt system
func (h *Handler) dispatchAIResponse(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, sessionID string, persona *persona.Persona, messages []chat.Message, userMessage string, guidance *emotionservice.Guidance) (*schema.Message, error) {
	if h.aiService.StreamingEnabled() {
		return h.streamAIResponse(ctx, w, flusher, sessionID, persona, messages, userMessage, guidance)
	}

	response, err := h.aiService.GenerateResponse(ctx, sessionID, persona, messages, userMessage, guidance)
	if err != nil {
		return nil, err
	}

	h.sendSSE(w, flusher, StreamResponse{
		Event:     "message",
		SessionID: sessionID,
		Content:   response.Content,
	})

	return response, nil
}

// getSessionPersona retrieves session and associated persona information
func (h *Handler) getSessionPersona(ctx context.Context, sessionID string) (*chat.Session, *persona.Persona, error) {
	session, err := h.chatSvc.GetSession(ctx, sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("session not found: %w", err)
	}

	persona, ok := h.personas.FindByID(session.PersonaID)
	if !ok {
		return nil, nil, fmt.Errorf("persona %s not found", session.PersonaID)
	}

	return &session, &persona, nil
}

func hasMatchingUserMessage(messages []chat.Message, sessionID, content string) bool {
	if len(messages) == 0 {
		return false
	}

	last := messages[len(messages)-1]
	if last.SessionID != sessionID {
		return false
	}

	if last.Sender != "user" {
		return false
	}

	return last.Content == content
}

// sendSSE sends a Server-Sent Event
func (h *Handler) sendSSE(w http.ResponseWriter, flusher http.Flusher, response StreamResponse) {
	data, err := json.Marshal(response)
	if err != nil {
		log.Printf("failed to marshal SSE response: %v", err)
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// sendSSEError sends an error via Server-Sent Events
func (h *Handler) sendSSEError(w http.ResponseWriter, flusher http.Flusher, errorMsg string) {
	h.sendSSE(w, flusher, StreamResponse{
		Event: "error",
		Error: errorMsg,
	})
}

func (h *Handler) streamAIResponse(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, sessionID string, persona *persona.Persona, messages []chat.Message, userMessage string, guidance *emotionservice.Guidance) (*schema.Message, error) {
	stream, err := h.aiService.StreamResponse(ctx, persona, messages, userMessage, guidance)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	chunks := make([]*schema.Message, 0, 8)

	for {
		chunk, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return nil, recvErr
		}
		if chunk == nil {
			continue
		}

		chunks = append(chunks, chunk)
		if chunk.Content != "" {
			h.sendSSE(w, flusher, StreamResponse{
				Event:     "delta",
				SessionID: sessionID,
				Content:   chunk.Content,
			})
		}
	}

	response, err := schema.ConcatMessages(chunks)
	if err != nil {
		return nil, err
	}

	h.sendSSE(w, flusher, StreamResponse{
		Event:     "message",
		SessionID: sessionID,
		Content:   response.Content,
	})

	return response, nil
}
