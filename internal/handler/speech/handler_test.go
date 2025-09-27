package speech

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
	speechmodel "github.com/zhouzirui/z-tavern/backend/internal/model/speech"
	"github.com/zhouzirui/z-tavern/backend/internal/service/ai"
	chatservice "github.com/zhouzirui/z-tavern/backend/internal/service/chat"
	emotionservice "github.com/zhouzirui/z-tavern/backend/internal/service/emotion"
	speechsvc "github.com/zhouzirui/z-tavern/backend/internal/service/speech"
)

type fakeSpeechService struct {
	transcribeSession string
	synthSession      string
	synthVoice        string
}

func (f *fakeSpeechService) TranscribeAudio(ctx context.Context, req *speechmodel.ASRRequest) (*speechmodel.ASRResponse, error) {
	f.transcribeSession = req.SessionID
	return &speechmodel.ASRResponse{SessionID: req.SessionID, Text: "ok"}, nil
}

func (f *fakeSpeechService) SynthesizeSpeech(ctx context.Context, req *speechmodel.TTSRequest) (*speechmodel.TTSResponse, error) {
	f.synthSession = req.SessionID
	f.synthVoice = req.Voice
	return &speechmodel.TTSResponse{SessionID: req.SessionID, Format: "mp3"}, nil
}

func (f *fakeSpeechService) TranscribeBuffer(ctx context.Context, sessionID string, audioData []byte, format, language string) (*speechmodel.ASRResponse, error) {
	f.transcribeSession = sessionID
	return &speechmodel.ASRResponse{SessionID: sessionID, Text: "ok"}, nil
}

func (f *fakeSpeechService) SynthesizeToBuffer(ctx context.Context, req *speechmodel.TTSRequest) (*speechmodel.TTSResponse, error) {
	f.synthSession = req.SessionID
	f.synthVoice = req.Voice
	return &speechmodel.TTSResponse{SessionID: req.SessionID, AudioData: []byte("audio"), Format: "mp3"}, nil
}

func TestProcessTranscribeOverridesSession(t *testing.T) {
	fakeSvc := &fakeSpeechService{}
	handler := New(fakeSvc, nil, nil)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("audio", "sample.wav")
	if err != nil {
		t.Fatalf("CreateFormFile err: %v", err)
	}
	if _, err := part.Write([]byte("audio")); err != nil {
		t.Fatalf("write audio err: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close err: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/speech/transcribe/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	handler.processTranscribe(rr, req, "session-override")

	if fakeSvc.transcribeSession != "session-override" {
		t.Fatalf("expected override session, got %s", fakeSvc.transcribeSession)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rr.Code)
	}
}

func TestProcessSynthesizeOverridesSession(t *testing.T) {
	fakeSvc := &fakeSpeechService{}
	chatSvc := chatservice.NewService()
	personaStore := persona.NewMemoryStore([]persona.Persona{{
		ID:      "wizard",
		VoiceID: "hogwarts-young-hero",
	}})
	session, err := chatSvc.CreateSession(context.Background(), "wizard")
	if err != nil {
		t.Fatalf("CreateSession err: %v", err)
	}

	handler := New(fakeSvc, chatSvc, personaStore)

	payload := map[string]any{"text": "hello"}
	buf, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal err: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/speech/synthesize/test", bytes.NewReader(buf))
	rr := httptest.NewRecorder()

	handler.processSynthesize(rr, req, session.ID)

	if fakeSvc.synthSession != session.ID {
		t.Fatalf("expected override session, got %s", fakeSvc.synthSession)
	}

	expectedVoice := speechsvc.NormalizeVoiceAlias("hogwarts-young-hero")
	if fakeSvc.synthVoice != expectedVoice {
		t.Fatalf("expected voice %s, got %s", expectedVoice, fakeSvc.synthVoice)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rr.Code)
	}
}

func TestWebSocketFallbackWhenUnavailable(t *testing.T) {
	handler := New(nil, nil, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/speech/ws/abc", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 status, got %d", rr.Code)
	}
}

func TestWebSocketRegisteredWhenServicesPresent(t *testing.T) {
	fakeSvc := &fakeSpeechService{}
	chatSvc := chatservice.NewService()
	personaStore := persona.NewMemoryStore(nil)
	handler := New(fakeSvc, chatSvc, personaStore)
	r := chi.NewRouter()
	aiSvc := &ai.Service{}
	emotionSvc := (*emotionservice.Service)(nil)

	handler.RegisterRoutes(r, aiSvc, emotionSvc, chatSvc, personaStore)

	req := httptest.NewRequest(http.MethodGet, "/speech/ws/abc", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code == http.StatusNotImplemented {
		t.Fatalf("websocket route should not fallback when services present")
	}
}
