package chat_test

import (
	"context"
	"testing"

	chat "github.com/zhouzirui/z-tavern/backend/internal/service/chat"
)

func TestServiceGetSession(t *testing.T) {
	svc := chat.NewService()
	ctx := context.Background()

	session, err := svc.CreateSession(ctx, "iron-man")
	if err != nil {
		t.Fatalf("CreateSession err: %v", err)
	}

	got, err := svc.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession err: %v", err)
	}

	if got.ID != session.ID {
		t.Fatalf("unexpected session ID: got %s want %s", got.ID, session.ID)
	}
	if got.PersonaID != "iron-man" {
		t.Fatalf("unexpected persona ID: got %s", got.PersonaID)
	}
}

func TestServiceGetSessionNotFound(t *testing.T) {
	svc := chat.NewService()
	ctx := context.Background()

	if _, err := svc.GetSession(ctx, "missing"); err == nil {
		t.Fatal("expected error for missing session")
	}
}
