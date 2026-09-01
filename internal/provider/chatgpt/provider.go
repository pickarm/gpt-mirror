// Package chatgpt defines the application-facing ChatGPT provider boundary.
//
// Implementations own protocol, transport, authentication/session resolution,
// and streaming details. Services and handlers must only depend on the types
// and interfaces in this package, never on raw HTTP/SSE response objects.
package chatgpt

import "context"

// Provider is the stable boundary between application services and ChatGPT.
// AccountRef intentionally contains no raw credentials; concrete providers are
// responsible for resolving credentials through their own dependencies.
type Provider interface {
	Health(ctx context.Context, account AccountRef) (AccountStatus, error)
	Models(ctx context.Context, account AccountRef) ([]Model, error)

	ListConversations(ctx context.Context, account AccountRef, page PageRequest) (ConversationPage, error)
	GetConversation(ctx context.Context, account AccountRef, conversationID string) (Conversation, error)

	CreateConversation(ctx context.Context, account AccountRef, req SendMessageRequest) (Stream, error)
	ContinueConversation(ctx context.Context, account AccountRef, conversationID string, req SendMessageRequest) (Stream, error)

	RenameConversation(ctx context.Context, account AccountRef, conversationID string, title string) error
	ArchiveConversation(ctx context.Context, account AccountRef, conversationID string, archived bool) error
	DeleteConversation(ctx context.Context, account AccountRef, conversationID string) error
}

// Stream is a provider-owned streaming abstraction. Implementations may use
// SSE, chunked HTTP, WebSocket, or another protocol internally without leaking
// those details to services or handlers.
type Stream interface {
	Recv(ctx context.Context) (StreamEvent, error)
	Close() error
}
