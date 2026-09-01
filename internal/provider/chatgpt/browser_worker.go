package chatgpt

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type BrowserSendRequest struct {
	ConversationID string `json:"conversationId,omitempty"`
	Model          string `json:"model,omitempty"`
	Message        string `json:"message"`
	Temporary      bool   `json:"temporary,omitempty"`
	Cookie         string `json:"cookie"`
	ProxyURL       string `json:"proxyUrl,omitempty"`
}

// BrowserExecutor is an optional challenge-safe write adapter. It drives the
// official ChatGPT browser application and returns provider-native stream
// events without exposing browser/DOM details to services or handlers.
type BrowserExecutor interface {
	Enabled() bool
	Send(ctx context.Context, req BrowserSendRequest) (Stream, error)
}

type browserWorkerExecutor struct {
	enabled    bool
	socketPath string
	client     *http.Client
}

func NewBrowserWorkerExecutor(conf *viper.Viper) BrowserExecutor {
	socketPath := strings.TrimSpace(conf.GetString("chatgpt.browser.socket_path"))
	if socketPath == "" {
		socketPath = "/run/gpt-mirror/browser.sock"
	}

	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DisableCompression: true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	return &browserWorkerExecutor{
		enabled:    conf.GetBool("chatgpt.browser.enabled"),
		socketPath: socketPath,
		client:     &http.Client{Transport: transport},
	}
}

func (e *browserWorkerExecutor) Enabled() bool {
	return e != nil && e.enabled && e.socketPath != ""
}

func (e *browserWorkerExecutor) Send(ctx context.Context, input BrowserSendRequest) (Stream, error) {
	const operation = "browser_send"
	if !e.Enabled() {
		return nil, &Error{Kind: ErrorKindUnavailable, Operation: operation, Err: errors.New("browser-backed write executor is disabled")}
	}
	if strings.TrimSpace(input.Cookie) == "" {
		return nil, &Error{Kind: ErrorKindUnavailable, Operation: operation, Err: errors.New("browser-backed writes require a browser session cookie")}
	}
	if strings.TrimSpace(input.Message) == "" {
		return nil, &Error{Kind: ErrorKindInvalidRequest, Operation: operation, Err: errors.New("message content is required")}
	}

	body, err := json.Marshal(input)
	if err != nil {
		return nil, &Error{Kind: ErrorKindProtocol, Operation: operation, Err: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://browser-worker/v1/send", bytes.NewReader(body))
	if err != nil {
		return nil, &Error{Kind: ErrorKindProtocol, Operation: operation, Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/x-ndjson")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, &Error{Kind: ErrorKindUnavailable, Operation: operation, Err: fmt.Errorf("browser worker unavailable: %w", err)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		var payload struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload)
		message := strings.TrimSpace(payload.Error)
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		kind := ErrorKindUnavailable
		if resp.StatusCode == http.StatusBadRequest {
			kind = ErrorKindInvalidRequest
		}
		return nil, &Error{Kind: kind, Operation: operation, StatusCode: resp.StatusCode, Err: errors.New(message)}
	}

	return newBrowserWorkerStream(resp.Body), nil
}

type browserWorkerEvent struct {
	Type           string `json:"type"`
	Kind           string `json:"kind,omitempty"`
	MessageText    string `json:"message,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	MessageID      string `json:"messageId,omitempty"`
	Delta          string `json:"delta,omitempty"`
	Message        *struct {
		ID      string `json:"id,omitempty"`
		Role    string `json:"role,omitempty"`
		Content string `json:"content,omitempty"`
	} `json:"message,omitempty"`
}

type browserWorkerStream struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
	closed  bool
}

func newBrowserWorkerStream(body io.ReadCloser) Stream {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	return &browserWorkerStream{body: body, scanner: scanner}
}

func (s *browserWorkerStream) Recv(ctx context.Context) (StreamEvent, error) {
	if s == nil || s.closed {
		return StreamEvent{}, io.EOF
	}
	select {
	case <-ctx.Done():
		_ = s.Close()
		return StreamEvent{}, ctx.Err()
	default:
	}

	for s.scanner.Scan() {
		line := bytes.TrimSpace(s.scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event browserWorkerEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return StreamEvent{}, &Error{Kind: ErrorKindProtocol, Operation: "browser_stream", Err: fmt.Errorf("decode browser worker event: %w", err)}
		}
		if event.Type == "error" {
			return StreamEvent{}, browserWorkerError(event.Kind, event.MessageText)
		}

		out := StreamEvent{
			Type:           StreamEventType(event.Type),
			ConversationID: event.ConversationID,
			MessageID:      event.MessageID,
			Delta:          event.Delta,
		}
		if event.Message != nil {
			role := Role(event.Message.Role)
			if role == "" {
				role = RoleAssistant
			}
			out.Message = &Message{
				ID:      event.Message.ID,
				Role:    role,
				Content: event.Message.Content,
			}
		}
		return out, nil
	}

	if err := s.scanner.Err(); err != nil {
		return StreamEvent{}, &Error{Kind: ErrorKindTransport, Operation: "browser_stream", Err: err}
	}
	return StreamEvent{}, io.EOF
}

func (s *browserWorkerStream) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	return s.body.Close()
}

func browserWorkerError(kind, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "browser worker failed"
	}
	providerKind := ErrorKindUnavailable
	switch kind {
	case "auth":
		providerKind = ErrorKindAuth
	case "transport":
		providerKind = ErrorKindTransport
	case "invalid_request":
		providerKind = ErrorKindInvalidRequest
	case "protocol":
		providerKind = ErrorKindProtocol
	}
	return &Error{Kind: providerKind, Operation: "browser_stream", Err: errors.New(message)}
}
