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
)

type fakeSpeechService struct {
	transcribeSession string
	synthSession      string
}

func (f *fakeSpeechService) TranscribeAudio(ctx context.Context, req *speechmodel.ASRRequest) (*speechmodel.ASRResponse, error) {
	f.transcribeSession = req.SessionID
	return &speechmodel.ASRResponse{SessionID: req.SessionID, Text: "ok"}, nil
}

func (f *fakeSpeechService) SynthesizeSpeech(ctx context.Context, req *speechmodel.TTSRequest) (*speechmodel.TTSResponse, error) {
	f.synthSession = req.SessionID
	return &speechmodel.TTSResponse{SessionID: req.SessionID, Format: "mp3"}, nil
}

func (f *fakeSpeechService) TranscribeBuffer(ctx context.Context, sessionID string, audioData []byte, format, language string) (*speechmodel.ASRResponse, error) {
	f.transcribeSession = sessionID
	return &speechmodel.ASRResponse{SessionID: sessionID, Text: "ok"}, nil
}

func (f *fakeSpeechService) SynthesizeToBuffer(ctx context.Context, sessionID, text, voice, language string) (*speechmodel.TTSResponse, error) {
	f.synthSession = sessionID
	return &speechmodel.TTSResponse{SessionID: sessionID, AudioData: []byte("audio"), Format: "mp3"}, nil
}

func TestProcessTranscribeOverridesSession(t *testing.T) {
	fakeSvc := &fakeSpeechService{}
	handler := New(fakeSvc)

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
	handler := New(fakeSvc)

	payload := map[string]any{"text": "hello"}
	buf, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal err: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/speech/synthesize/test", bytes.NewReader(buf))
	rr := httptest.NewRecorder()

	handler.processSynthesize(rr, req, "session-override")

	if fakeSvc.synthSession != "session-override" {
		t.Fatalf("expected override session, got %s", fakeSvc.synthSession)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rr.Code)
	}
}

func TestWebSocketFallbackWhenUnavailable(t *testing.T) {
	handler := New(nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/speech/ws/abc", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 status, got %d", rr.Code)
	}
}

func TestWebSocketRegisteredWhenServicesPresent(t *testing.T) {
	fakeSvc := &fakeSpeechService{}
	handler := New(fakeSvc)
	r := chi.NewRouter()
	aiSvc := &ai.Service{}
	chatSvc := chatservice.NewService()
	personaStore := persona.NewMemoryStore(nil)

	handler.RegisterRoutes(r, aiSvc, chatSvc, personaStore)

	req := httptest.NewRequest(http.MethodGet, "/speech/ws/abc", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code == http.StatusNotImplemented {
		t.Fatalf("websocket route should not fallback when services present")
	}
}
