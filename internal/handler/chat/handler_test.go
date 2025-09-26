package chat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
	chatservice "github.com/zhouzirui/z-tavern/backend/internal/service/chat"
)

func setupRouter() (*chi.Mux, *chatservice.Service, persona.Store) {
	chatSvc := chatservice.NewService()
	store := persona.NewMemoryStore(persona.Seed())
	handler := New(chatSvc, store)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r, chatSvc, store
}

func TestCreateSessionValidPersona(t *testing.T) {
	r, _, store := setupRouter()
	personas := store.List()
	body := map[string]string{"personaId": personas[0].ID}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.Code)
	}
}

func TestCreateSessionInvalidPersona(t *testing.T) {
	r, _, _ := setupRouter()
	body := map[string]string{"personaId": "non-existent"}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestCreateSessionMissingPersonaID(t *testing.T) {
	r, _, _ := setupRouter()
	payload := []byte(`{}`)

	req := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}
