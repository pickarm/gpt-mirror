package v1

import (
	chatgptprovider "PandoraHelper/internal/provider/chatgpt"
	"time"
)

type ChatGPTAccountRequest struct {
	AccountID uint `json:"accountId" binding:"required"`
}

type ChatGPTConversationListRequest struct {
	AccountID uint   `json:"accountId" binding:"required"`
	Cursor    string `json:"cursor"`
	Limit     int    `json:"limit"`
}

type ChatGPTConversationRequest struct {
	AccountID      uint   `json:"accountId" binding:"required"`
	ConversationID string `json:"conversationId" binding:"required"`
}

type ChatGPTSendRequest struct {
	AccountID       uint   `json:"accountId" binding:"required"`
	ConversationID  string `json:"conversationId"`
	Model           string `json:"model"`
	ParentMessageID string `json:"parentMessageId"`
	Message         string `json:"message" binding:"required"`
	Temporary       bool   `json:"temporary"`
}

type ChatGPTRenameRequest struct {
	AccountID      uint   `json:"accountId" binding:"required"`
	ConversationID string `json:"conversationId" binding:"required"`
	Title          string `json:"title" binding:"required"`
}

type ChatGPTArchiveRequest struct {
	AccountID      uint   `json:"accountId" binding:"required"`
	ConversationID string `json:"conversationId" binding:"required"`
	Archived       bool   `json:"archived"`
}

type ChatGPTAccountStatus struct {
	State     string    `json:"state"`
	Label     string    `json:"label"`
	CheckedAt time.Time `json:"checkedAt"`
}

type ChatGPTModel struct {
	ID           string   `json:"id"`
	Slug         string   `json:"slug"`
	DisplayName  string   `json:"displayName"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type ChatGPTConversationSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ChatGPTConversationPage struct {
	Items      []ChatGPTConversationSummary `json:"items"`
	NextCursor string                       `json:"nextCursor,omitempty"`
	HasMore    bool                         `json:"hasMore"`
}

type ChatGPTMessage struct {
	ID        string    `json:"id"`
	ParentID  string    `json:"parentId,omitempty"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type ChatGPTConversation struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Messages  []ChatGPTMessage `json:"messages"`
	Archived  bool             `json:"archived"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
}

type ChatGPTStreamEvent struct {
	Type           string          `json:"type"`
	ConversationID string          `json:"conversationId,omitempty"`
	MessageID      string          `json:"messageId,omitempty"`
	Delta          string          `json:"delta,omitempty"`
	Message        *ChatGPTMessage `json:"message,omitempty"`
}

func NewChatGPTAccountStatus(status chatgptprovider.AccountStatus) ChatGPTAccountStatus {
	return ChatGPTAccountStatus{State: string(status.State), Label: status.Label, CheckedAt: status.CheckedAt}
}

func NewChatGPTModels(models []chatgptprovider.Model) []ChatGPTModel {
	out := make([]ChatGPTModel, 0, len(models))
	for _, model := range models {
		out = append(out, ChatGPTModel{ID: model.ID, Slug: model.Slug, DisplayName: model.DisplayName, Capabilities: model.Capabilities})
	}
	return out
}

func NewChatGPTConversationPage(page chatgptprovider.ConversationPage) ChatGPTConversationPage {
	items := make([]ChatGPTConversationSummary, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, ChatGPTConversationSummary{ID: item.ID, Title: item.Title, Archived: item.Archived, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt})
	}
	return ChatGPTConversationPage{Items: items, NextCursor: page.NextCursor, HasMore: page.HasMore}
}

func NewChatGPTConversation(conversation chatgptprovider.Conversation) ChatGPTConversation {
	messages := make([]ChatGPTMessage, 0, len(conversation.Messages))
	for _, message := range conversation.Messages {
		messages = append(messages, newChatGPTMessage(message))
	}
	return ChatGPTConversation{ID: conversation.ID, Title: conversation.Title, Messages: messages, Archived: conversation.Archived, CreatedAt: conversation.CreatedAt, UpdatedAt: conversation.UpdatedAt}
}

func NewChatGPTStreamEvent(event chatgptprovider.StreamEvent) ChatGPTStreamEvent {
	out := ChatGPTStreamEvent{Type: string(event.Type), ConversationID: event.ConversationID, MessageID: event.MessageID, Delta: event.Delta}
	if event.Message != nil {
		message := newChatGPTMessage(*event.Message)
		out.Message = &message
	}
	return out
}

func newChatGPTMessage(message chatgptprovider.Message) ChatGPTMessage {
	return ChatGPTMessage{ID: message.ID, ParentID: message.ParentID, Role: string(message.Role), Content: message.Content, CreatedAt: message.CreatedAt}
}
