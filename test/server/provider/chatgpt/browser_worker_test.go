package chatgpt_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	provider "PandoraHelper/internal/provider/chatgpt"
	"github.com/spf13/viper"
)

func startUnixWorker(t *testing.T, handler http.Handler) string {
	t.Helper()
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "browser.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		_ = listener.Close()
		_ = os.Remove(socketPath)
	})
	return socketPath
}

func browserExecutorForSocket(socketPath string) provider.BrowserExecutor {
	conf := viper.New()
	conf.Set("chatgpt.browser.enabled", true)
	conf.Set("chatgpt.browser.socket_path", socketPath)
	return provider.NewBrowserWorkerExecutor(conf)
}

func TestBrowserWorkerExecutorStreamsProviderEvents(t *testing.T) {
	var received provider.BrowserSendRequest
	socketPath := startUnixWorker(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/send" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode worker request: %v", err)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, "{\"type\":\"conversation\",\"conversationId\":\"conv-browser\"}\n")
		_, _ = io.WriteString(w, "{\"type\":\"message_delta\",\"conversationId\":\"conv-browser\",\"delta\":\"Hel\"}\n")
		_, _ = io.WriteString(w, "{\"type\":\"message_completed\",\"conversationId\":\"conv-browser\",\"message\":{\"id\":\"assistant-1\",\"role\":\"assistant\",\"content\":\"Hello\"}}\n")
		_, _ = io.WriteString(w, "{\"type\":\"done\",\"conversationId\":\"conv-browser\"}\n")
	}))

	executor := browserExecutorForSocket(socketPath)
	stream, err := executor.Send(context.Background(), provider.BrowserSendRequest{
		ConversationID: "conv-browser",
		Model:          "gpt-test",
		Message:        "hello",
		Cookie:         "oai-did=device-test; session=test-session",
		ProxyURL:       "socks5h://user:pass@127.0.0.1:1080",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	defer stream.Close()

	var events []provider.StreamEvent
	for {
		event, recvErr := stream.Recv(context.Background())
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			t.Fatalf("Recv: %v", recvErr)
		}
		events = append(events, event)
	}

	if received.Message != "hello" || received.Model != "gpt-test" || received.ConversationID != "conv-browser" {
		t.Fatalf("unexpected worker request: %#v", received)
	}
	if received.Cookie == "" || received.ProxyURL == "" {
		t.Fatalf("cookie/proxy were not passed to worker")
	}
	if len(events) != 4 {
		t.Fatalf("event count = %d, events=%#v", len(events), events)
	}
	if events[0].Type != provider.StreamEventConversation || events[0].ConversationID != "conv-browser" {
		t.Fatalf("conversation event = %#v", events[0])
	}
	if events[1].Type != provider.StreamEventMessageDelta || events[1].Delta != "Hel" {
		t.Fatalf("delta event = %#v", events[1])
	}
	if events[2].Type != provider.StreamEventMessageCompleted || events[2].Message == nil || events[2].Message.Content != "Hello" {
		t.Fatalf("completed event = %#v", events[2])
	}
	if events[3].Type != provider.StreamEventDone {
		t.Fatalf("done event = %#v", events[3])
	}
}

func TestBrowserWorkerExecutorMapsStreamError(t *testing.T) {
	socketPath := startUnixWorker(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, "{\"type\":\"error\",\"kind\":\"auth\",\"message\":\"browser session expired\"}\n")
	}))

	stream, err := browserExecutorForSocket(socketPath).Send(context.Background(), provider.BrowserSendRequest{
		Message: "hello",
		Cookie:  "session=test-session",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	defer stream.Close()

	_, err = stream.Recv(context.Background())
	if !provider.IsKind(err, provider.ErrorKindAuth) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestBrowserWorkerExecutorDisabled(t *testing.T) {
	conf := viper.New()
	conf.Set("chatgpt.browser.enabled", false)
	executor := provider.NewBrowserWorkerExecutor(conf)
	_, err := executor.Send(context.Background(), provider.BrowserSendRequest{
		Message: "hello",
		Cookie:  "session=test-session",
	})
	if !provider.IsKind(err, provider.ErrorKindUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}
