package chatgpt

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportBrowserFake struct {
	enabled bool
	request BrowserSendRequest
	stream  Stream
	err     error
}

func (f *transportBrowserFake) Enabled() bool { return f.enabled }
func (f *transportBrowserFake) Send(_ context.Context, req BrowserSendRequest) (Stream, error) {
	f.request = req
	return f.stream, f.err
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestBrowserWriteRoundTripperConvertsWorkerStreamToProviderSSE(t *testing.T) {
	browser := &transportBrowserFake{
		enabled: true,
		stream: NewSliceStream([]StreamEvent{
			{Type: StreamEventConversation, ConversationID: "conv-browser"},
			{Type: StreamEventMessageDelta, ConversationID: "conv-browser", Delta: "Hel"},
			{Type: StreamEventMessageDelta, ConversationID: "conv-browser", Delta: "lo"},
			{Type: StreamEventMessageCompleted, ConversationID: "conv-browser", Message: &Message{Role: RoleAssistant, Content: "Hello"}},
			{Type: StreamEventDone, ConversationID: "conv-browser"},
		}, nil),
	}
	baseCalled := false
	transport := &browserWriteRoundTripper{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			baseCalled = true
			return nil, errors.New("base transport must not handle browser write")
		}),
		browser:   browser,
		cookie:    "oai-did=device-test; session=test-session",
		proxyURL:  "socks5h://127.0.0.1:1080",
		writePath: "/backend-api/f/conversation",
	}

	body := `{"conversation_id":"conv-browser","model":"gpt-test","messages":[{"content":{"parts":["hello"]}}]}`
	req, err := http.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/f/conversation", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()
	if baseCalled {
		t.Fatal("browser write unexpectedly used base transport")
	}
	if browser.request.Message != "hello" || browser.request.Model != "gpt-test" || browser.request.ConversationID != "conv-browser" {
		t.Fatalf("unexpected browser request: %#v", browser.request)
	}
	if browser.request.Cookie == "" || browser.request.ProxyURL == "" {
		t.Fatalf("browser request lost cookie/proxy: %#v", browser.request)
	}

	stream := newSSEStream(resp.Body, "test_browser_write")
	var events []StreamEvent
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
	if len(events) != 5 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Type != StreamEventConversation || events[0].ConversationID != "conv-browser" {
		t.Fatalf("conversation event = %#v", events[0])
	}
	if events[1].Type != StreamEventMessageDelta || events[1].Delta != "Hel" {
		t.Fatalf("first delta = %#v", events[1])
	}
	if events[2].Type != StreamEventMessageDelta || events[2].Delta != "lo" {
		t.Fatalf("second delta = %#v", events[2])
	}
	if events[3].Type != StreamEventMessageCompleted || events[3].Message == nil || events[3].Message.Content != "Hello" {
		t.Fatalf("completed event = %#v", events[3])
	}
	if events[4].Type != StreamEventDone {
		t.Fatalf("done event = %#v", events[4])
	}
}

func TestBrowserWriteRoundTripperSynthesizesChallengeRequirements(t *testing.T) {
	browser := &transportBrowserFake{enabled: true}
	baseCalled := false
	transport := &browserWriteRoundTripper{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			baseCalled = true
			return nil, errors.New("base transport must not handle requirements")
		}),
		browser:   browser,
		cookie:    "session=test-session",
		writePath: "/backend-api/f/conversation",
	}
	req, _ := http.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/sentinel/chat-requirements", strings.NewReader(`{}`))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()
	if baseCalled {
		t.Fatal("requirements request unexpectedly used base transport")
	}
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(raw), `"required":false`) {
		t.Fatalf("requirements response: status=%d body=%s", resp.StatusCode, raw)
	}
}

func TestBrowserWriteRoundTripperPassesReadsToBaseTransport(t *testing.T) {
	baseCalled := false
	transport := &browserWriteRoundTripper{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			baseCalled = true
			return syntheticJSONResponse(req, http.StatusOK, `{"ok":true}`), nil
		}),
		browser:   &transportBrowserFake{enabled: true},
		cookie:    "session=test-session",
		writePath: "/backend-api/f/conversation",
	}
	req, _ := http.NewRequest(http.MethodGet, "https://chatgpt.com/backend-api/conversations", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()
	if !baseCalled {
		t.Fatal("read request did not use base transport")
	}
}
