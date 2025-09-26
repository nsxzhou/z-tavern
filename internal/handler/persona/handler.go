package persona

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
)

// Handler persona服务的HTTP处理器
type Handler struct {
	personas persona.Store
}

// New 创建persona处理器
func New(personas persona.Store) *Handler {
	return &Handler{
		personas: personas,
	}
}

// RegisterRoutes 注册persona相关的路由
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/personas", h.handleListPersonas)
}

// handleListPersonas 列出所有persona
func (h *Handler) handleListPersonas(w http.ResponseWriter, r *http.Request) {
	personas := h.personas.List()
	h.respondJSON(w, http.StatusOK, personas)
}

// respondJSON 发送JSON响应
func (h *Handler) respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}