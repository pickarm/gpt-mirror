package chatgpt_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	provider "PandoraHelper/internal/provider/chatgpt"
	credentialprovider "PandoraHelper/internal/provider/credential"
	"PandoraHelper/internal/model"
	"PandoraHelper/internal/repository"
	apptransport "PandoraHelper/internal/transport"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type testCredentialProvider struct {
	secret credentialprovider.Secret
}

func (p *testCredentialProvider) Resolve(context.Context, uint) (credentialprovider.Secret, error) {
	return p.secret, nil
}
func (p *testCredentialProvider) Status(context.Context, uint) (credentialprovider.Status, error) {
	return credentialprovider.Status{HasCredential: !p.secret.Empty(), State: credentialprovider.StateUnknown}, nil
}
func (*testCredentialProvider) Put(context.Context, uint, credentialprovider.Secret) error { return nil }
func (*testCredentialProvider) Delete(context.Context, uint) error                         { return nil }
func (*testCredentialProvider) Validate(context.Context, uint) (credentialprovider.Health, error) {
	return credentialprovider.Health{State: credentialprovider.StateUnknown}, nil
}
func (*testCredentialProvider) RecordHealth(context.Context, uint, credentialprovider.Health) error {
	return nil
}
func (*testCredentialProvider) CanPersist() bool { return true }

type testAccountRepository struct {
	account *model.Account
}

func (r *testAccountRepository) GetAccount(context.Context, int64) (*model.Account, error) {
	if r.account == nil {
		return &model.Account{ID: 1}, nil
	}
	copyAccount := *r.account
	return &copyAccount, nil
}
func (*testAccountRepository) Update(context.Context, *model.Account) error { return nil }
func (*testAccountRepository) Create(context.Context, *model.Account) error { return nil }
func (*testAccountRepository) SearchAccount(context.Context, string, string) ([]*model.Account, error) {
	return nil, nil
}
func (*testAccountRepository) DeleteAccount(context.Context, int64) error { return nil }
func (*testAccountRepository) GetShareAccountList(*gin.Context) ([]*model.Account, error) {
	return nil, nil
}

var _ repository.AccountRepository = (*testAccountRepository)(nil)
var _ credentialprovider.Provider = (*testCredentialProvider)(nil)

func newWebProvider(t *testing.T, baseURL string) provider.Provider {
	t.Helper()
	factory, err := apptransport.NewFactory(apptransport.DefaultSettings())
	if err != nil {
		t.Fatalf("NewFactory: %v", err)
	}
	conf := viper.New()
	conf.Set("chatgpt.base_url", baseURL)
	conf.Set("chatgpt.conversation_path", "/backend-api/f/conversation")
	result, err := provider.NewWebProvider(
		&testCredentialProvider{secret: credentialprovider.Secret{
			Representation: credentialprovider.RepresentationToken,
			AccessToken:    "test-access-token",
		}},
		&testAccountRepository{account: &model.Account{ID: 1}},
		factory,
		conf,
	)
	if err != nil {
		t.Fatalf("NewWebProvider: %v", err)
	}
	return result
}

func requireBearer(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestWebProviderPaginationPreservesUpstreamIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		if r.URL.Path != "/backend-api/conversations" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("offset") {
		case "", "0":
			_, _ = io.WriteString(w, `{"items":[{"id":"conv-1","title":"One","create_time":1788269000,"update_time":1788269100},{"id":"conv-2","title":"Two","create_time":1788269200,"update_time":1788269300}],"total":3,"offset":0,"limit":2}`)
		case "2":
			_, _ = io.WriteString(w, `{"items":[{"id":"conv-3","title":"Three","create_time":1788269400,"update_time":1788269500}],"total":3,"offset":2,"limit":2}`)
		default:
			t.Fatalf("unexpected offset: %s", r.URL.Query().Get("offset"))
		}
	}))
	defer server.Close()

	p := newWebProvider(t, server.URL)
	first, err := p.ListConversations(context.Background(), provider.AccountRef{ID: 1}, provider.PageRequest{Limit: 2})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first.Items) != 2 || first.Items[0].ID != "conv-1" || first.Items[1].ID != "conv-2" {
		t.Fatalf("unexpected first page: %#v", first)
	}
	if !first.HasMore || first.NextCursor != "offset:2" {
		t.Fatalf("unexpected first cursor: %#v", first)
	}

	second, err := p.ListConversations(context.Background(), provider.AccountRef{ID: 1}, provider.PageRequest{Limit: 2, Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second.Items) != 1 || second.Items[0].ID != "conv-3" || second.HasMore || second.NextCursor != "" {
		t.Fatalf("unexpected second page: %#v", second)
	}
}

func TestWebProviderConversationUsesCurrentNodeBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		if r.URL.Path != "/backend-api/conversation/conv-branch" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"conversation_id":"conv-branch",
			"title":"Branch test",
			"current_node":"assistant-current",
			"mapping":{
				"hidden-root":{"id":"hidden-root","parent":null,"message":{"id":"hidden-msg","author":{"role":"system"},"content":{"content_type":"text","parts":["hidden"]},"metadata":{"is_visually_hidden_from_conversation":true}}},
				"user-current":{"id":"user-current","parent":"hidden-root","message":{"id":"user-msg","author":{"role":"user"},"content":{"content_type":"text","parts":["question"]},"create_time":1788269000}},
				"assistant-old":{"id":"assistant-old","parent":"user-current","message":{"id":"old-msg","author":{"role":"assistant"},"content":{"content_type":"text","parts":["old branch"]}}},
				"assistant-current":{"id":"assistant-current","parent":"user-current","message":{"id":"assistant-msg","author":{"role":"assistant"},"content":{"content_type":"text","parts":["current answer"]},"create_time":1788269010,"status":"finished_successfully","end_turn":true}}
			}
		}`)
	}))
	defer server.Close()

	p := newWebProvider(t, server.URL)
	conversation, err := p.GetConversation(context.Background(), provider.AccountRef{ID: 1}, "conv-branch")
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if conversation.ID != "conv-branch" || len(conversation.Messages) != 2 {
		t.Fatalf("unexpected conversation: %#v", conversation)
	}
	if conversation.Messages[0].ID != "user-msg" || conversation.Messages[0].Content != "question" {
		t.Fatalf("unexpected user message: %#v", conversation.Messages[0])
	}
	if conversation.Messages[1].ID != "assistant-msg" || conversation.Messages[1].Content != "current answer" {
		t.Fatalf("unexpected assistant message: %#v", conversation.Messages[1])
	}
	for _, message := range conversation.Messages {
		if strings.Contains(message.Content, "old branch") || strings.Contains(message.Content, "hidden") {
			t.Fatalf("non-current/hidden content leaked into active branch: %#v", conversation.Messages)
		}
	}
}

func TestWebProviderSSEFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/conversation_stream.sse")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		switch r.URL.Path {
		case "/backend-api/sentinel/chat-requirements":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"token":"requirements-token","arkose":{"required":false},"proofofwork":{"required":false},"turnstile":{"required":false}}`)
		case "/backend-api/f/conversation":
			if got := r.Header.Get("Openai-Sentinel-Chat-Requirements-Token"); got != "requirements-token" {
				t.Fatalf("requirements token = %q", got)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write(fixture)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	p := newWebProvider(t, server.URL)
	stream, err := p.CreateConversation(context.Background(), provider.AccountRef{ID: 1}, provider.SendMessageRequest{
		Model: "auto",
		Message: provider.InputMessage{Role: provider.RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
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
	if len(events) != 5 {
		t.Fatalf("event count = %d, events=%#v", len(events), events)
	}
	if events[0].Type != provider.StreamEventConversation || events[0].ConversationID != "conv-1" {
		t.Fatalf("unexpected conversation event: %#v", events[0])
	}
	if events[1].Type != provider.StreamEventMessageDelta || events[1].Delta != "Hel" {
		t.Fatalf("unexpected first delta: %#v", events[1])
	}
	if events[2].Type != provider.StreamEventMessageDelta || events[2].Delta != "lo" {
		t.Fatalf("unexpected second delta: %#v", events[2])
	}
	if events[3].Type != provider.StreamEventMessageCompleted || events[3].Message == nil || events[3].Message.Content != "Hello" {
		t.Fatalf("unexpected completion: %#v", events[3])
	}
	if events[4].Type != provider.StreamEventDone {
		t.Fatalf("unexpected terminal event: %#v", events[4])
	}
}

func TestWebProviderChallengeRequiresBrowserExecutor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		if r.URL.Path != "/backend-api/sentinel/chat-requirements" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"token":"requirements-token","proofofwork":{"required":true}}`)
	}))
	defer server.Close()

	p := newWebProvider(t, server.URL)
	_, err := p.CreateConversation(context.Background(), provider.AccountRef{ID: 1}, provider.SendMessageRequest{
		Message: provider.InputMessage{Role: provider.RoleUser, Content: "hello"},
	})
	if !provider.IsKind(err, provider.ErrorKindUnavailable) {
		t.Fatalf("expected unavailable browser-executor error, got %v", err)
	}
}

func TestWebProviderErrorKinds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		switch r.URL.Path {
		case "/backend-api/models":
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case "/backend-api/conversations":
			w.Header().Set("Retry-After", "2")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
		case "/backend-api/conversation/missing":
			http.Error(w, "missing", http.StatusNotFound)
		case "/backend-api/conversation/bad-json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	p := newWebProvider(t, server.URL)
	_, err := p.Models(context.Background(), provider.AccountRef{ID: 1})
	if !provider.IsKind(err, provider.ErrorKindAuth) {
		t.Fatalf("models error kind = %v", err)
	}
	_, err = p.ListConversations(context.Background(), provider.AccountRef{ID: 1}, provider.PageRequest{})
	if !provider.IsKind(err, provider.ErrorKindRateLimit) {
		t.Fatalf("list error kind = %v", err)
	}
	var rateErr *provider.Error
	if !errors.As(err, &rateErr) || rateErr.RetryAfter != 2*time.Second {
		t.Fatalf("retry-after not normalized: %v", err)
	}
	_, err = p.GetConversation(context.Background(), provider.AccountRef{ID: 1}, "missing")
	if !provider.IsKind(err, provider.ErrorKindNotFound) {
		t.Fatalf("not-found error kind = %v", err)
	}
	_, err = p.GetConversation(context.Background(), provider.AccountRef{ID: 1}, "bad-json")
	if !provider.IsKind(err, provider.ErrorKindProtocol) {
		t.Fatalf("protocol error kind = %v", err)
	}
}

func TestWebProviderTransportErrorKind(t *testing.T) {
	p := newWebProvider(t, "http://127.0.0.1:1")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := p.Models(ctx, provider.AccountRef{ID: 1})
	if !provider.IsKind(err, provider.ErrorKindTransport) {
		t.Fatalf("transport error kind = %v", err)
	}
}
