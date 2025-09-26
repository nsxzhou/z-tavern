package chat

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/zhouzirui/z-tavern/backend/internal/model/chat"
	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
	chatService "github.com/zhouzirui/z-tavern/backend/internal/service/chat"
)

// Handler 聊天服务的HTTP处理器
type Handler struct {
	chatSvc      *chatService.Service
	personaStore persona.Store
}

// New 创建聊天处理器
func New(chatSvc *chatService.Service, personaStore persona.Store) *Handler {
	return &Handler{
		chatSvc:      chatSvc,
		personaStore: personaStore,
	}
}

// RegisterRoutes 注册聊天相关的路由
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/session", h.handleCreateSession)
	r.Post("/messages", h.handleSaveMessage)
}

// handleCreateSession 创建会话
func (h *Handler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		PersonaID string `json:"personaId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if payload.PersonaID == "" {
		respondError(w, http.StatusBadRequest, "personaId is required")
		return
	}

	if _, ok := h.personaStore.FindByID(payload.PersonaID); !ok {
		respondError(w, http.StatusBadRequest, "persona not found")
		return
	}

	session, err := h.chatSvc.CreateSession(r.Context(), payload.PersonaID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, session)
}

// handleSaveMessage 保存消息
func (h *Handler) handleSaveMessage(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		SessionID string `json:"sessionId"`
		Sender    string `json:"sender"`
		Content   string `json:"content"`
		Emotion   string `json:"emotion"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	message := chat.Message{
		SessionID: payload.SessionID,
		Sender:    payload.Sender,
		Content:   payload.Content,
		Emotion:   payload.Emotion,
	}

	if err := h.chatSvc.SaveMessage(r.Context(), message); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "session not found" {
			status = http.StatusNotFound
		}
		respondError(w, status, err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

// respondJSON 发送JSON响应
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

// respondError 发送错误响应
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
