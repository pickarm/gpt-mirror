package chatgpt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// browserWriteClient preserves the normal HTTP client for all reads while
// routing browser-sensitive write operations through the isolated Playwright
// worker. The existing lifecycle/SSE parser therefore remains unchanged.
func (p *WebProvider) browserWriteClient(base *http.Client, cookie, proxyURL string) *http.Client {
	if base == nil || p.browser == nil || !p.browser.Enabled() || strings.TrimSpace(cookie) == "" {
		return base
	}
	clone := *base
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clone.Transport = &browserWriteRoundTripper{
		base:      transport,
		browser:   p.browser,
		cookie:    cookie,
		proxyURL:  proxyURL,
		writePath: p.conversationPath,
	}
	return &clone
}

type browserWriteRoundTripper struct {
	base      http.RoundTripper
	browser   BrowserExecutor
	cookie    string
	proxyURL  string
	writePath string
}

func (t *browserWriteRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, errors.New("nil request")
	}

	// When browser-backed writes are enabled, the official browser application
	// owns Sentinel/PoW/Turnstile negotiation. Returning a challenge-free local
	// requirements response prevents the HTTP lifecycle from attempting to
	// reproduce those browser-only mechanisms.
	if req.Method == http.MethodPost && req.URL.Path == "/backend-api/sentinel/chat-requirements" {
		return syntheticJSONResponse(req, http.StatusOK, `{"token":"","arkose":{"required":false},"proofofwork":{"required":false},"turnstile":{"required":false}}`), nil
	}

	if req.Method == http.MethodPost && req.URL.Path == t.writePath {
		input, err := decodeBrowserWriteRequest(req)
		if err != nil {
			return syntheticJSONResponse(req, http.StatusBadRequest, fmt.Sprintf(`{"error":%q}`, err.Error())), nil
		}
		input.Cookie = t.cookie
		input.ProxyURL = t.proxyURL
		return t.browserResponse(req.Context(), req, input)
	}

	return t.base.RoundTrip(req)
}

func decodeBrowserWriteRequest(req *http.Request) (BrowserSendRequest, error) {
	if req.Body == nil {
		return BrowserSendRequest{}, errors.New("conversation request body is empty")
	}
	defer req.Body.Close()
	limited, err := io.ReadAll(io.LimitReader(req.Body, 2<<20))
	if err != nil {
		return BrowserSendRequest{}, fmt.Errorf("read conversation request: %w", err)
	}

	var payload struct {
		ConversationID string `json:"conversation_id"`
		Model          string `json:"model"`
		Temporary      bool   `json:"history_and_training_disabled"`
		Messages       []struct {
			Content struct {
				Parts []string `json:"parts"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(limited, &payload); err != nil {
		return BrowserSendRequest{}, fmt.Errorf("decode conversation request: %w", err)
	}
	if len(payload.Messages) == 0 || len(payload.Messages[0].Content.Parts) == 0 {
		return BrowserSendRequest{}, errors.New("conversation request has no text message")
	}
	message := strings.TrimSpace(payload.Messages[0].Content.Parts[0])
	if message == "" {
		return BrowserSendRequest{}, errors.New("conversation message is empty")
	}
	return BrowserSendRequest{
		ConversationID: strings.TrimSpace(payload.ConversationID),
		Model:          strings.TrimSpace(payload.Model),
		Message:        message,
		Temporary:      payload.Temporary,
	}, nil
}

func (t *browserWriteRoundTripper) browserResponse(ctx context.Context, req *http.Request, input BrowserSendRequest) (*http.Response, error) {
	stream, err := t.browser.Send(ctx, input)
	if err != nil {
		return providerErrorResponse(req, err), nil
	}

	first, err := stream.Recv(ctx)
	if err != nil {
		_ = stream.Close()
		return providerErrorResponse(req, err), nil
	}

	reader, writer := io.Pipe()
	go pumpBrowserStream(ctx, stream, writer, first)

	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header: http.Header{
			"Content-Type":  []string{"text/event-stream; charset=utf-8"},
			"Cache-Control": []string{"no-cache, no-transform"},
		},
		Body:    reader,
		Request: req,
	}, nil
}

func pumpBrowserStream(ctx context.Context, stream Stream, writer *io.PipeWriter, first StreamEvent) {
	defer stream.Close()
	defer writer.Close()

	messageID := fmt.Sprintf("browser-assistant-%d", time.Now().UnixNano())
	accumulated := ""
	writeEvent := func(event StreamEvent) error {
		if event.MessageID != "" {
			messageID = event.MessageID
		}
		switch event.Type {
		case StreamEventConversation:
			return writeUpstreamSSE(writer, map[string]interface{}{
				"conversation_id": event.ConversationID,
			})
		case StreamEventMessageDelta:
			accumulated += event.Delta
			return writeUpstreamSSE(writer, map[string]interface{}{
				"conversation_id": event.ConversationID,
				"message": map[string]interface{}{
					"id":     messageID,
					"author": map[string]string{"role": "assistant"},
					"content": map[string]interface{}{
						"content_type": "text",
						"parts":        []string{accumulated},
					},
					"status":   "in_progress",
					"end_turn": false,
				},
			})
		case StreamEventMessageCompleted:
			content := accumulated
			if event.Message != nil && event.Message.Content != "" {
				content = event.Message.Content
			}
			return writeUpstreamSSE(writer, map[string]interface{}{
				"conversation_id": event.ConversationID,
				"message": map[string]interface{}{
					"id":     messageID,
					"author": map[string]string{"role": "assistant"},
					"content": map[string]interface{}{
						"content_type": "text",
						"parts":        []string{content},
					},
					"status":   "finished_successfully",
					"end_turn": true,
				},
			})
		case StreamEventDone:
			_, err := io.WriteString(writer, "data: [DONE]\n\n")
			return err
		default:
			return nil
		}
	}

	if err := writeEvent(first); err != nil {
		_ = writer.CloseWithError(err)
		return
	}
	if first.Type == StreamEventDone {
		return
	}

	for {
		event, err := stream.Recv(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return
			}
			_ = writer.CloseWithError(err)
			return
		}
		if err := writeEvent(event); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		if event.Type == StreamEventDone {
			return
		}
	}
}

func writeUpstreamSSE(writer io.Writer, payload interface{}) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "data: %s\n\n", encoded)
	return err
}

func providerErrorResponse(req *http.Request, err error) *http.Response {
	status := http.StatusServiceUnavailable
	var providerErr *Error
	if errors.As(err, &providerErr) {
		switch providerErr.Kind {
		case ErrorKindAuth:
			status = http.StatusUnauthorized
		case ErrorKindInvalidRequest:
			status = http.StatusBadRequest
		case ErrorKindRateLimit:
			status = http.StatusTooManyRequests
		case ErrorKindTransport, ErrorKindProtocol:
			status = http.StatusBadGateway
		case ErrorKindNotFound:
			status = http.StatusNotFound
		case ErrorKindUnavailable:
			status = http.StatusServiceUnavailable
		}
	}
	message := "browser-backed ChatGPT write failed"
	if err != nil {
		message = err.Error()
	}
	encoded, _ := json.Marshal(map[string]string{"error": message})
	return syntheticJSONResponse(req, status, string(encoded))
}

func syntheticJSONResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Request:    req,
	}
}

var _ http.RoundTripper = (*browserWriteRoundTripper)(nil)
