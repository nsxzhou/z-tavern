package stream

import (
	"context"
	"testing"

	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
	chatservice "github.com/zhouzirui/z-tavern/backend/internal/service/chat"
)

func TestGetSessionPersonaReturnsBoundPersona(t *testing.T) {
	chatSvc := chatservice.NewService()
	store := persona.NewMemoryStore(persona.Seed())
	handler := New(nil, nil, chatSvc, store)

	ctx := context.Background()
	session, err := chatSvc.CreateSession(ctx, "socrates")
	if err != nil {
		t.Fatalf("CreateSession err: %v", err)
	}

	_, gotPersona, err := handler.getSessionPersona(ctx, session.ID)
	if err != nil {
		t.Fatalf("getSessionPersona err: %v", err)
	}

	if gotPersona.ID != "socrates" {
		t.Fatalf("expected persona socrates, got %s", gotPersona.ID)
	}
}

func TestGetSessionPersonaMissingPersona(t *testing.T) {
	chatSvc := chatservice.NewService()
	store := persona.NewMemoryStore(nil)
	handler := New(nil, nil, chatSvc, store)

	ctx := context.Background()
	session, err := chatSvc.CreateSession(ctx, "unknown")
	if err != nil {
		t.Fatalf("CreateSession err: %v", err)
	}

	if _, _, err := handler.getSessionPersona(ctx, session.ID); err == nil {
		t.Fatal("expected error when persona not found")
	}
}
