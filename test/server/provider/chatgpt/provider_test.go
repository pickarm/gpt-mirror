package chatgpt_test

import (
	"context"
	"errors"
	"io"
	"testing"

	provider "PandoraHelper/internal/provider/chatgpt"
)

func TestFakeProviderModels(t *testing.T) {
	fake := &provider.Fake{
		ModelsFunc: func(_ context.Context, account provider.AccountRef) ([]provider.Model, error) {
			if account.ID != 42 {
				t.Fatalf("unexpected account id: %d", account.ID)
			}
			return []provider.Model{{ID: "model-1", DisplayName: "Test Model"}}, nil
		},
	}

	models, err := fake.Models(context.Background(), provider.AccountRef{ID: 42})
	if err != nil {
		t.Fatalf("Models returned error: %v", err)
	}
	if len(models) != 1 || models[0].ID != "model-1" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestFakeProviderUnsetHookFailsWithoutNetwork(t *testing.T) {
	fake := &provider.Fake{}
	_, err := fake.GetConversation(context.Background(), provider.AccountRef{ID: 1}, "conversation-1")
	if err == nil {
		t.Fatal("expected unavailable error")
	}
	if !provider.IsKind(err, provider.ErrorKindUnavailable) {
		t.Fatalf("unexpected error kind: %v", err)
	}
}

func TestSliceStream(t *testing.T) {
	terminalErr := &provider.Error{Kind: provider.ErrorKindRateLimit, Operation: "stream"}
	stream := provider.NewSliceStream([]provider.StreamEvent{
		{Type: provider.StreamEventMessageDelta, Delta: "hel"},
		{Type: provider.StreamEventMessageDelta, Delta: "lo"},
		{Type: provider.StreamEventDone},
	}, terminalErr)

	ctx := context.Background()
	for i, want := range []provider.StreamEventType{
		provider.StreamEventMessageDelta,
		provider.StreamEventMessageDelta,
		provider.StreamEventDone,
	} {
		event, err := stream.Recv(ctx)
		if err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
		if event.Type != want {
			t.Fatalf("event %d type = %s, want %s", i, event.Type, want)
		}
	}

	_, err := stream.Recv(ctx)
	if !provider.IsKind(err, provider.ErrorKindRateLimit) {
		t.Fatalf("expected rate-limit terminal error, got %v", err)
	}

	_, err = stream.Recv(ctx)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF after terminal error, got %v", err)
	}
}

func TestUnavailableProvider(t *testing.T) {
	p := provider.NewUnavailableProvider()
	status, err := p.Health(context.Background(), provider.AccountRef{ID: 7})
	if status.State != provider.AccountStateUnknown {
		t.Fatalf("unexpected state: %s", status.State)
	}
	if !provider.IsKind(err, provider.ErrorKindUnavailable) {
		t.Fatalf("unexpected error: %v", err)
	}
}
