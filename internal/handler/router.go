package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/zhouzirui/z-tavern/backend/internal/handler/chat"
	"github.com/zhouzirui/z-tavern/backend/internal/handler/persona"
	"github.com/zhouzirui/z-tavern/backend/internal/handler/speech"
	"github.com/zhouzirui/z-tavern/backend/internal/handler/stream"
	middlewarePkg "github.com/zhouzirui/z-tavern/backend/internal/middleware"
	personaModel "github.com/zhouzirui/z-tavern/backend/internal/model/persona"
	aiService "github.com/zhouzirui/z-tavern/backend/internal/service/ai"
	chatService "github.com/zhouzirui/z-tavern/backend/internal/service/chat"
	speechService "github.com/zhouzirui/z-tavern/backend/internal/service/speech"
	"github.com/zhouzirui/z-tavern/backend/pkg/utils"
)

// NewRouter wires HTTP routes to core services.
func NewRouter(personas personaModel.Store, chatSvc *chatService.Service, aiSvc *aiService.Service, speechSvc *speechService.Service) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middlewarePkg.CORS)

	// Create handlers
	personaHandler := persona.New(personas)
	chatHandler := chat.New(chatSvc, personas)

	// Create stream handler for AI responses if AI service is available
	var streamHandler *stream.Handler
	if aiSvc != nil {
		streamHandler = stream.New(aiSvc, chatSvc, personas)
	}

	r.Route("/api", func(api chi.Router) {
		// Register persona routes
		personaHandler.RegisterRoutes(api)

		// Register chat routes
		chatHandler.RegisterRoutes(api)

		// Enhanced streaming endpoint with AI integration
		api.Get("/stream/{sessionID}", func(w http.ResponseWriter, r *http.Request) {
			sessionID := chi.URLParam(r, "sessionID")
			userMessage := r.URL.Query().Get("message")

			if streamHandler == nil {
				utils.RespondError(w, http.StatusServiceUnavailable, "ai streaming unavailable")
				return
			}
			if userMessage == "" {
				utils.RespondError(w, http.StatusBadRequest, "message query parameter is required")
				return
			}

			// Handle AI-powered streaming response
			if err := streamHandler.HandleStreamRequest(r.Context(), w, sessionID, userMessage); err != nil {
				log.Printf("[stream] error handling request: %v", err)
				utils.RespondError(w, http.StatusInternalServerError, "streaming failed")
			}
		})

		// Register speech routes if speech service is available
		if speechSvc != nil {
			speechHandler := speech.New(speechSvc)
			speechHandler.RegisterRoutes(api, aiSvc, chatSvc, personas)
		}
	})

	return r
}

// handleHeartbeatStream provides the original heartbeat functionality as fallback
func handleHeartbeatStream(w http.ResponseWriter, r *http.Request, sessionID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		utils.RespondError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	utils.SetupSSEHeaders(w)

	ctx := r.Context()
	log.Printf("[sse] opening heartbeat stream for session=%s", sessionID)

	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	utils.SendSSEChunk(w, flusher, map[string]any{
		"event":   "status",
		"message": "stream established",
	})

	for {
		select {
		case <-ctx.Done():
			log.Printf("[sse] closing heartbeat stream for session=%s", sessionID)
			return
		case t := <-ticker.C:
			utils.SendSSEChunk(w, flusher, map[string]any{
				"event":   "heartbeat",
				"message": "awaiting llm response",
				"time":    t.UTC().Format(time.RFC3339),
			})
		}
	}
}
