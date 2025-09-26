package speech

import (
	"testing"

	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
)

func boolPtr(v bool) *bool { return &v }

func TestApplyConfigUpdatesState(t *testing.T) {
	seeds := persona.Seed()
	personas := persona.NewMemoryStore(seeds)
	state := newConnectionState("session", &seeds[0])
	handler := &WebSocketHandler{personaStore: personas}

	cfg := ConfigMessage{
		PersonaID:  "socrates",
		Language:   "en-US",
		Voice:      "new-voice",
		ASREnabled: boolPtr(false),
		TTSEnabled: boolPtr(true),
		StreamMode: boolPtr(false),
	}

	handler.applyConfig(state, cfg)

	if state.language != "en-US" {
		t.Fatalf("expected language en-US, got %s", state.language)
	}
	if state.voice != "new-voice" {
		t.Fatalf("expected voice new-voice, got %s", state.voice)
	}
	if state.persona == nil || state.persona.ID != "socrates" {
		t.Fatalf("expected persona socrates")
	}
	if state.asrEnabled {
		t.Fatalf("expected ASR disabled")
	}
	if !state.ttsEnabled {
		t.Fatalf("expected TTS enabled")
	}
	if state.streamMode {
		t.Fatalf("expected stream mode disabled")
	}
}
