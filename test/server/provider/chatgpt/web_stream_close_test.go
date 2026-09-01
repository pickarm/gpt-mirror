package chatgpt_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	provider "PandoraHelper/internal/provider/chatgpt"
)

func TestWebProviderStreamCloseInterruptsBlockedRecv(t *testing.T) {
	streamStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		switch r.URL.Path {
		case "/backend-api/sentinel/chat-requirements":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"token":"requirements-token","arkose":{"required":false},"proofofwork":{"required":false},"turnstile":{"required":false}}`)
		case "/backend-api/f/conversation":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			close(streamStarted)
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.CloseClientConnections()
	defer server.Close()

	p := newWebProvider(t, server.URL)
	stream, err := p.CreateConversation(context.Background(), provider.AccountRef{ID: 1}, provider.SendMessageRequest{
		Message: provider.InputMessage{Role: provider.RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	<-streamStarted

	recvStarted := make(chan struct{})
	recvDone := make(chan error, 1)
	go func() {
		close(recvStarted)
		_, recvErr := stream.Recv(context.Background())
		recvDone <- recvErr
	}()
	<-recvStarted
	// Give Recv enough time to enter Scanner.Scan on the open response body.
	time.Sleep(50 * time.Millisecond)

	closeDone := make(chan error, 1)
	go func() { closeDone <- stream.Close() }()

	select {
	case closeErr := <-closeDone:
		if closeErr != nil {
			t.Fatalf("Close: %v", closeErr)
		}
	case <-time.After(time.Second):
		t.Fatal("Close blocked behind Recv instead of interrupting the SSE body")
	}

	select {
	case recvErr := <-recvDone:
		if !errors.Is(recvErr, io.EOF) {
			t.Fatalf("Recv after Close = %v, want EOF", recvErr)
		}
	case <-time.After(time.Second):
		t.Fatal("Recv remained blocked after Close")
	}
}
