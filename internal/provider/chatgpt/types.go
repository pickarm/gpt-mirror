package chatgpt

import "time"

// AccountRef identifies the application account whose provider session should
// be used. It deliberately excludes cookies, access tokens, refresh tokens, and
// other credential material.
type AccountRef struct {
	ID uint
}

type AccountState string

const (
	AccountStateHealthy AccountState = "healthy"
	AccountStateExpired AccountState = "expired"
	AccountStateBlocked AccountState = "blocked"
	AccountStateUnknown AccountState = "unknown"
)

type AccountStatus struct {
	State     AccountState
	Label     string
	CheckedAt time.Time
}

type Model struct {
	ID           string
	Slug         string
	DisplayName  string
	Capabilities []string
}

type PageRequest struct {
	Cursor string
	Limit  int
}

type ConversationSummary struct {
	ID        string
	Title     string
	Archived  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ConversationPage struct {
	Items      []ConversationSummary
	NextCursor string
	HasMore    bool
}

type Conversation struct {
	ID        string
	Title     string
	Messages  []Message
	Archived  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

type Message struct {
	ID        string
	ParentID  string
	Role      Role
	Content   string
	CreatedAt time.Time
}

type InputMessage struct {
	Role    Role
	Content string
}

type SendMessageRequest struct {
	Model           string
	ParentMessageID string
	Message         InputMessage
	Temporary       bool
}

type StreamEventType string

const (
	StreamEventConversation     StreamEventType = "conversation"
	StreamEventMessageDelta     StreamEventType = "message_delta"
	StreamEventMessageCompleted StreamEventType = "message_completed"
	StreamEventDone             StreamEventType = "done"
)

type StreamEvent struct {
	Type           StreamEventType
	ConversationID string
	MessageID      string
	Delta          string
	Message        *Message
}
