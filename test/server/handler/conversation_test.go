package handler_test

import (
	"PandoraHelper/internal/handler"
	chatgptprovider "PandoraHelper/internal/provider/chatgpt"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type conversationServiceStub struct{}

func (conversationServiceStub) Health(context.Context, uint) (chatgptprovider.AccountStatus, error) {
	return chatgptprovider.AccountStatus{}, nil
}
func (conversationServiceStub) Models(context.Context, uint) ([]chatgptprovider.Model, error) {
	return nil, nil
}
func (conversationServiceStub) List(context.Context, uint, chatgptprovider.PageRequest) (chatgptprovider.ConversationPage, error) {
	return chatgptprovider.ConversationPage{}, nil
}
func (conversationServiceStub) Get(context.Context, uint, string) (chatgptprovider.Conversation, error) {
	return chatgptprovider.Conversation{}, nil
}
func (conversationServiceStub) Create(context.Context, uint, chatgptprovider.SendMessageRequest) (chatgptprovider.Stream, error) {
	return chatgptprovider.NewSliceStream([]chatgptprovider.StreamEvent{
		{Type: chatgptprovider.StreamEventConversation, ConversationID: "conv-1"},
		{Type: chatgptprovider.StreamEventMessageDelta, ConversationID: "conv-1", MessageID: "msg-1", Delta: "hello"},
		{Type: chatgptprovider.StreamEventDone, ConversationID: "conv-1", MessageID: "msg-1"},
	}, nil), nil
}
func (conversationServiceStub) Continue(context.Context, uint, string, chatgptprovider.SendMessageRequest) (chatgptprovider.Stream, error) {
	return chatgptprovider.NewSliceStream(nil, nil), nil
}
func (conversationServiceStub) Rename(context.Context, uint, string, string) error { return nil }
func (conversationServiceStub) Archive(context.Context, uint, string, bool) error { return nil }
func (conversationServiceStub) Delete(context.Context, uint, string) error { return nil }

func TestConversationCreateStreamsSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/chatgpt/conversations/create", strings.NewReader(`{"accountId":1,"model":"auto","message":"hello"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	h := handler.NewConversationHandler(handler.NewHandler(nil), conversationServiceStub{})
	h.Create(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	body := recorder.Body.String()
	require.Contains(t, body, "event: message")
	require.Contains(t, body, `"type":"message_delta"`)
	require.Contains(t, body, `"conversationId":"conv-1"`)
	require.Contains(t, body, `"delta":"hello"`)
	require.Contains(t, body, `"type":"done"`)
}
