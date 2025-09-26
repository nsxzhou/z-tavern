package speech

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/zhouzirui/z-tavern/backend/internal/model/chat"
	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
	"github.com/zhouzirui/z-tavern/backend/internal/service/ai"
	chatservice "github.com/zhouzirui/z-tavern/backend/internal/service/chat"
)

// WebSocketHandler WebSocket语音处理器
type WebSocketHandler struct {
	speechSvc    SpeechService
	aiSvc        *ai.Service
	chatSvc      *chatservice.Service
	personaStore persona.Store
	upgrader     websocket.Upgrader
}

// NewWebSocketHandler 创建WebSocket处理器
func NewWebSocketHandler(speechSvc SpeechService, aiSvc *ai.Service, chatSvc *chatservice.Service, personaStore persona.Store) *WebSocketHandler {
	return &WebSocketHandler{
		speechSvc:    speechSvc,
		aiSvc:        aiSvc,
		chatSvc:      chatSvc,
		personaStore: personaStore,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// RegisterWebSocketRoutes 注册WebSocket路由
func (h *WebSocketHandler) RegisterWebSocketRoutes(r chi.Router) {
	r.Get("/ws/{sessionID}", h.handleWebSocket)
}

type inboundMessage struct {
	Type      string          `json:"type"`
	SessionID string          `json:"sessionId"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"`
}

// AudioMessage 音频消息
type AudioMessage struct {
	AudioData  []byte `json:"audioData"`
	Format     string `json:"format"`
	Language   string `json:"language"`
	IsFinal    bool   `json:"isFinal"`
	ChunkIndex int    `json:"chunkIndex"`
}

// TextMessage 文本消息
type TextMessage struct {
	Text        string  `json:"text"`
	IsFinal     bool    `json:"isFinal"`
	Confidence  float64 `json:"confidence,omitempty"`
	MessageType string  `json:"messageType"`
}

// ConfigMessage 配置消息
type ConfigMessage struct {
	PersonaID  string `json:"personaId"`
	Language   string `json:"language"`
	ASREnabled *bool  `json:"asrEnabled,omitempty"`
	TTSEnabled *bool  `json:"ttsEnabled,omitempty"`
	StreamMode *bool  `json:"streamMode,omitempty"`
	Voice      string `json:"voice"`
}

type outgoingMessage struct {
	Type      string      `json:"type"`
	SessionID string      `json:"sessionId,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

type connectionState struct {
	sessionID   string
	persona     *persona.Persona
	language    string
	voice       string
	asrEnabled  bool
	ttsEnabled  bool
	streamMode  bool
	audioFormat string
	buffer      bytes.Buffer
}

func newConnectionState(sessionID string, persona *persona.Persona) *connectionState {
	state := &connectionState{
		sessionID:  sessionID,
		persona:    persona,
		language:   "zh-CN",
		voice:      persona.VoiceID,
		asrEnabled: true,
		ttsEnabled: true,
		streamMode: true,
	}
	return state
}

// handleWebSocket 处理WebSocket连接
func (h *WebSocketHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		http.Error(w, "sessionID is required", http.StatusBadRequest)
		return
	}

	if h.chatSvc == nil {
		http.Error(w, "chat service unavailable", http.StatusServiceUnavailable)
		return
	}

	session, err := h.chatSvc.GetSession(r.Context(), sessionID)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	persona, ok := h.personaStore.FindByID(session.PersonaID)
	if !ok {
		http.Error(w, "persona not found", http.StatusBadRequest)
		return
	}

	state := newConnectionState(sessionID, &persona)

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[websocket] upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("[websocket] new connection for session: %s", sessionID)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	go h.pingLoop(ctx, conn)

	h.sendInfo(conn, sessionID, map[string]any{
		"type":     "connected",
		"persona":  persona.ID,
		"language": state.language,
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
			var msg inboundMessage
			if err := conn.ReadJSON(&msg); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("[websocket] read error: %v", err)
				}
				return
			}

			conn.SetReadDeadline(time.Now().Add(60 * time.Second))

			if msg.SessionID != "" && msg.SessionID != sessionID {
				h.sendError(conn, "session mismatch")
				continue
			}

			h.handleMessage(ctx, conn, state, &msg)
		}
	}
}

func (h *WebSocketHandler) handleMessage(ctx context.Context, conn *websocket.Conn, state *connectionState, msg *inboundMessage) {
	switch msg.Type {
	case "audio":
		h.handleAudioMessage(ctx, conn, state, msg.Data)
	case "text":
		h.handleTextMessage(ctx, conn, state, msg.Data)
	case "config":
		h.handleConfigMessage(conn, state, msg.Data)
	default:
		h.sendError(conn, "unsupported message type: "+msg.Type)
	}
}

func (h *WebSocketHandler) handleAudioMessage(ctx context.Context, conn *websocket.Conn, state *connectionState, raw json.RawMessage) {
	if !state.asrEnabled {
		h.sendInfo(conn, state.sessionID, map[string]any{"type": "asr", "enabled": false})
		return
	}

	var audio AudioMessage
	if err := json.Unmarshal(raw, &audio); err != nil {
		h.sendError(conn, "invalid audio payload")
		return
	}

	if len(audio.AudioData) > 0 {
		written, _ := state.buffer.Write(audio.AudioData)
		log.Printf("[websocket] buffered audio chunk session=%s size=%d total=%d", state.sessionID, written, state.buffer.Len())
	}
	if audio.Format != "" {
		state.audioFormat = audio.Format
	}
	if audio.Language != "" {
		state.language = audio.Language
	}

	if audio.IsFinal || !state.streamMode {
		h.processBufferedAudio(ctx, conn, state)
	}
}

func (h *WebSocketHandler) processBufferedAudio(ctx context.Context, conn *websocket.Conn, state *connectionState) {
	audioBytes := state.buffer.Bytes()
	state.buffer.Reset()

	if len(audioBytes) == 0 {
		return
	}

	format := state.audioFormat
	if format == "" {
		format = "wav"
	}

	h.dumpAudioDebug(state.sessionID, format, audioBytes)
	log.Printf("[websocket] processing ASR audio session=%s format=%s bytes=%d", state.sessionID, format, len(audioBytes))

	asrResp, err := h.speechSvc.TranscribeBuffer(ctx, state.sessionID, audioBytes, format, state.language)
	if err != nil {
		h.sendError(conn, fmt.Sprintf("ASR failed: %v", err))
		return
	}

	h.sendInfo(conn, state.sessionID, map[string]any{
		"type":       "asr",
		"text":       asrResp.Text,
		"confidence": asrResp.Confidence,
		"isFinal":    true,
	})

	if asrResp.Text == "" {
		return
	}

	if err := h.processUserText(ctx, conn, state, asrResp.Text); err != nil {
		h.sendError(conn, err.Error())
	}
}

func (h *WebSocketHandler) dumpAudioDebug(sessionID, format string, data []byte) {
	if len(data) == 0 {
		return
	}

	fileName := fmt.Sprintf("asr-%s-%d.%s", sessionID, time.Now().UnixNano(), format)
	path := filepath.Join(os.TempDir(), fileName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		log.Printf("[websocket] failed to write debug audio: %v", err)
		return
	}
	log.Printf("[websocket] wrote ASR debug audio to %s", path)
}

func (h *WebSocketHandler) handleTextMessage(ctx context.Context, conn *websocket.Conn, state *connectionState, raw json.RawMessage) {
	var text TextMessage
	if err := json.Unmarshal(raw, &text); err != nil {
		h.sendError(conn, "invalid text payload")
		return
	}
	if text.Text == "" {
		return
	}

	if err := h.processUserText(ctx, conn, state, text.Text); err != nil {
		h.sendError(conn, err.Error())
	}
}

func (h *WebSocketHandler) processUserText(ctx context.Context, conn *websocket.Conn, state *connectionState, userText string) error {
	if h.chatSvc == nil {
		return errors.New("chat service unavailable")
	}

	messages, err := h.chatSvc.LoadTranscript(ctx, state.sessionID)
	if err != nil {
		return fmt.Errorf("load transcript failed: %w", err)
	}

	userMsg := chat.Message{SessionID: state.sessionID, Sender: "user", Content: userText}
	if err := h.chatSvc.SaveMessage(ctx, userMsg); err != nil {
		return fmt.Errorf("save user message failed: %w", err)
	}

	h.sendInfo(conn, state.sessionID, map[string]any{
		"type": "user",
		"text": userText,
	})

	if h.aiSvc == nil {
		return errors.New("ai service unavailable")
	}

	responseText, err := h.generateAIResponse(ctx, conn, state, messages, userText)
	if err != nil {
		return err
	}

	assistantMsg := chat.Message{SessionID: state.sessionID, Sender: "assistant", Content: responseText}
	if err := h.chatSvc.SaveMessage(ctx, assistantMsg); err != nil {
		log.Printf("[websocket] save assistant message failed: %v", err)
	}

	if state.ttsEnabled && responseText != "" {
		h.sendTTS(ctx, conn, state, responseText)
	}

	return nil
}

func (h *WebSocketHandler) generateAIResponse(ctx context.Context, conn *websocket.Conn, state *connectionState, history []chat.Message, userText string) (string, error) {
	if !h.aiSvc.StreamingEnabled() {
		resp, err := h.aiSvc.GenerateResponse(ctx, state.sessionID, state.persona, history, userText)
		if err != nil {
			return "", fmt.Errorf("ai generation failed: %w", err)
		}
		text := resp.Content
		h.sendInfo(conn, state.sessionID, map[string]any{
			"type":    "ai",
			"text":    text,
			"isFinal": true,
		})
		return text, nil
	}

	stream, err := h.aiSvc.StreamResponse(ctx, state.persona, history, userText)
	if err != nil {
		return "", fmt.Errorf("ai streaming failed: %w", err)
	}
	defer stream.Close()

	var chunks []*schema.Message
	for {
		chunk, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return "", fmt.Errorf("ai stream recv failed: %w", recvErr)
		}
		if chunk == nil {
			continue
		}
		chunks = append(chunks, chunk)
		if chunk.Content != "" {
			h.sendInfo(conn, state.sessionID, map[string]any{
				"type": "ai_delta",
				"text": chunk.Content,
			})
		}
	}

	merged, err := schema.ConcatMessages(chunks)
	if err != nil {
		return "", fmt.Errorf("concat ai chunks failed: %w", err)
	}

	text := merged.Content
	h.sendInfo(conn, state.sessionID, map[string]any{
		"type":    "ai",
		"text":    text,
		"isFinal": true,
	})

	return text, nil
}

func (h *WebSocketHandler) sendTTS(ctx context.Context, conn *websocket.Conn, state *connectionState, text string) {
	ttsResp, err := h.speechSvc.SynthesizeToBuffer(ctx, state.sessionID, text, state.voice, state.language)
	if err != nil {
		log.Printf("[websocket] TTS failed: %v", err)
		h.sendInfo(conn, state.sessionID, map[string]any{
			"type":  "tts",
			"error": "synthesis failed",
		})
		return
	}

	if len(ttsResp.AudioData) == 0 {
		log.Printf("[websocket] TTS returned empty audio session=%s", state.sessionID)
		return
	}

	log.Printf("[websocket] TTS sending audio session=%s bytes=%d format=%s", state.sessionID, len(ttsResp.AudioData), ttsResp.Format)
	audioB64 := base64.StdEncoding.EncodeToString(ttsResp.AudioData)
	h.sendInfo(conn, state.sessionID, map[string]any{
		"type":      "tts",
		"audioData": audioB64,
		"format":    ttsResp.Format,
		"isFinal":   true,
	})
}

func (h *WebSocketHandler) handleConfigMessage(conn *websocket.Conn, state *connectionState, raw json.RawMessage) {
	var cfg ConfigMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		h.sendError(conn, "invalid config payload")
		return
	}

	h.applyConfig(state, cfg)

	log.Printf("[websocket] config applied session=%s persona=%s voice=%s language=%s", state.sessionID, cfg.PersonaID, state.voice, state.language)

	personaID := ""
	if state.persona != nil {
		personaID = state.persona.ID
	}

	h.sendInfo(conn, state.sessionID, map[string]any{
		"type":       "config",
		"persona":    personaID,
		"language":   state.language,
		"voice":      state.voice,
		"asr":        state.asrEnabled,
		"tts":        state.ttsEnabled,
		"streamMode": state.streamMode,
	})
}

func (h *WebSocketHandler) applyConfig(state *connectionState, cfg ConfigMessage) {
	if cfg.Language != "" {
		state.language = cfg.Language
	}
	if cfg.Voice != "" {
		state.voice = cfg.Voice
	}
	if cfg.PersonaID != "" && state.persona != nil && cfg.PersonaID != state.persona.ID {
		if persona, ok := h.personaStore.FindByID(cfg.PersonaID); ok {
			state.persona = &persona
		}
	}
	if cfg.ASREnabled != nil {
		state.asrEnabled = *cfg.ASREnabled
	}
	if cfg.TTSEnabled != nil {
		state.ttsEnabled = *cfg.TTSEnabled
	}
	if cfg.StreamMode != nil {
		state.streamMode = *cfg.StreamMode
	}
}

func (h *WebSocketHandler) sendInfo(conn *websocket.Conn, sessionID string, data map[string]any) {
	msg := outgoingMessage{
		Type:      "result",
		SessionID: sessionID,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("[websocket] write info failed: %v", err)
	}
}

func (h *WebSocketHandler) sendError(conn *websocket.Conn, message string) {
	msg := outgoingMessage{
		Type:      "error",
		Data:      map[string]string{"message": message},
		Timestamp: time.Now().Unix(),
	}
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("[websocket] write error failed: %v", err)
	}
}

// pingLoop 定期发送ping消息
func (h *WebSocketHandler) pingLoop(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(54 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
