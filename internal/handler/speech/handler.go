package speech

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
	"github.com/zhouzirui/z-tavern/backend/internal/model/speech"
	"github.com/zhouzirui/z-tavern/backend/internal/service/ai"
	chatservice "github.com/zhouzirui/z-tavern/backend/internal/service/chat"
	emotionservice "github.com/zhouzirui/z-tavern/backend/internal/service/emotion"
	speechsvc "github.com/zhouzirui/z-tavern/backend/internal/service/speech"
)

// SpeechService 抽象语音业务，便于测试与替换实现
type SpeechService interface {
	TranscribeAudio(rCtx context.Context, req *speech.ASRRequest) (*speech.ASRResponse, error)
	SynthesizeSpeech(rCtx context.Context, req *speech.TTSRequest) (*speech.TTSResponse, error)
	TranscribeBuffer(rCtx context.Context, sessionID string, audioData []byte, format, language string) (*speech.ASRResponse, error)
	SynthesizeToBuffer(rCtx context.Context, req *speech.TTSRequest) (*speech.TTSResponse, error)
}

// Handler 语音服务的HTTP处理器
type Handler struct {
	speechSvc    SpeechService
	chatSvc      *chatservice.Service
	personaStore persona.Store
}

// New 创建语音处理器
func New(speechSvc SpeechService, chatSvc *chatservice.Service, personaStore persona.Store) *Handler {
	return &Handler{
		speechSvc:    speechSvc,
		chatSvc:      chatSvc,
		personaStore: personaStore,
	}
}

// RegisterRoutes 注册语音相关的路由
func (h *Handler) RegisterRoutes(r chi.Router, aiSvc *ai.Service, emotionSvc *emotionservice.Service, chatSvc *chatservice.Service, personaStore persona.Store) {
	r.Route("/speech", func(speechRouter chi.Router) {
		// ASR 端点
		speechRouter.Post("/transcribe", h.handleTranscribe)
		speechRouter.Post("/transcribe/{sessionID}", h.handleTranscribeWithSession)

		// TTS 端点
		speechRouter.Post("/synthesize", h.handleSynthesize)
		speechRouter.Post("/synthesize/{sessionID}", h.handleSynthesizeWithSession)

		// 健康检查
		speechRouter.Get("/health", h.handleHealth)

		// WebSocket端点 (如果实时语音链路可用)
		if h.websocketAvailable(aiSvc, chatSvc, personaStore) {
			wsHandler := NewWebSocketHandler(h.speechSvc, aiSvc, emotionSvc, chatSvc, personaStore)
			wsHandler.RegisterWebSocketRoutes(speechRouter)
		} else {
			speechRouter.Get("/ws/{sessionID}", func(w http.ResponseWriter, _ *http.Request) {
				h.respondError(w, http.StatusNotImplemented, "speech websocket not available")
			})
		}
	})
}

func (h *Handler) websocketAvailable(aiSvc *ai.Service, chatSvc *chatservice.Service, personaStore persona.Store) bool {
	if h.speechSvc == nil || aiSvc == nil || chatSvc == nil || personaStore == nil {
		return false
	}
	return true
}

// handleTranscribe 处理语音转文本请求
func (h *Handler) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	h.processTranscribe(w, r, "")
}

// handleTranscribeWithSession 处理带会话ID的语音转文本请求
func (h *Handler) handleTranscribeWithSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		h.respondError(w, http.StatusBadRequest, "sessionID is required")
		return
	}

	h.processTranscribe(w, r, sessionID)
}

// handleSynthesize 处理文本转语音请求
func (h *Handler) handleSynthesize(w http.ResponseWriter, r *http.Request) {
	h.processSynthesize(w, r, "")
}

// handleSynthesizeWithSession 处理带会话ID的文本转语音请求
func (h *Handler) handleSynthesizeWithSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		h.respondError(w, http.StatusBadRequest, "sessionID is required")
		return
	}

	h.processSynthesize(w, r, sessionID)
}

func (h *Handler) processTranscribe(w http.ResponseWriter, r *http.Request, overrideSessionID string) {
	err := r.ParseMultipartForm(32 << 20) // 32MB max
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
		return
	}

	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, header, err := r.FormFile("audio")
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "audio file is required")
		return
	}
	defer file.Close()

	sessionID := overrideSessionID
	if sessionID == "" {
		sessionID = r.FormValue("sessionId")
	}
	if sessionID == "" {
		sessionID = "default"
	}

	language := r.FormValue("language")
	if language == "" {
		language = "zh-CN"
	}

	format := h.inferAudioFormat(header.Filename)

	asrReq := &speech.ASRRequest{
		SessionID: sessionID,
		AudioData: file,
		Format:    format,
		Language:  language,
	}

	resp, err := h.speechSvc.TranscribeAudio(r.Context(), asrReq)
	if err != nil {
		log.Printf("[speech] ASR error: %v", err)
		h.respondError(w, http.StatusInternalServerError, "speech recognition failed")
		return
	}

	h.respondJSON(w, http.StatusOK, resp)
}

func (h *Handler) processSynthesize(w http.ResponseWriter, r *http.Request, overrideSessionID string) {
	var req speech.TTSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if overrideSessionID != "" {
		req.SessionID = overrideSessionID
	}

	if strings.TrimSpace(req.Text) == "" {
		h.respondError(w, http.StatusBadRequest, "text is required")
		return
	}

	if req.SessionID == "" {
		req.SessionID = "default"
	}

	if strings.TrimSpace(req.Voice) == "" {
		if resolved := h.resolveVoiceFromContext(r.Context(), req.SessionID); resolved != "" {
			req.Voice = resolved
		}
	}

	resp, err := h.speechSvc.SynthesizeSpeech(r.Context(), &req)
	if err != nil {
		log.Printf("[speech] TTS error: %v", err)
		h.respondError(w, http.StatusInternalServerError, "speech synthesis failed")
		return
	}

	if len(resp.AudioData) > 0 {
		format := resp.Format
		if format == "" {
			format = "octet-stream"
		}
		w.Header().Set("Content-Type", "audio/"+format)
		w.Header().Set("Content-Length", strconv.Itoa(len(resp.AudioData)))
		w.Header().Set("Content-Disposition", "attachment; filename=speech."+format)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(resp.AudioData); err != nil {
			log.Printf("failed to write audio response: %v", err)
		}
	} else {
		h.respondJSON(w, http.StatusOK, resp)
	}
}

func (h *Handler) resolveVoiceFromContext(ctx context.Context, sessionID string) string {
	if h.chatSvc == nil || h.personaStore == nil {
		return ""
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}

	session, err := h.chatSvc.GetSession(ctx, sessionID)
	if err != nil {
		return ""
	}

	personaID := strings.TrimSpace(session.PersonaID)
	if personaID == "" {
		return ""
	}

	personaObj, ok := h.personaStore.FindByID(personaID)
	if !ok {
		return ""
	}

	return speechsvc.NormalizeVoiceAlias(personaObj.VoiceID)
}

// handleHealth 健康检查端点
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	h.respondJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "speech",
	})
}

// inferAudioFormat 从文件名推断音频格式
func (h *Handler) inferAudioFormat(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mp3":
		return "mp3"
	case ".wav":
		return "wav"
	case ".webm":
		return "webm"
	case ".m4a":
		return "m4a"
	case ".aac":
		return "aac"
	default:
		return "wav"
	}
}

// respondJSON 发送JSON响应
func (h *Handler) respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

// respondError 发送错误响应
func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{"error": message})
}
