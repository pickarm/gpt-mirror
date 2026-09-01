package chatgpt

import (
	"context"
	"io"
	"sync"
)

// Fake is a hook-based Provider implementation for service tests. Any unset
// hook fails with ErrorKindUnavailable instead of making a network request.
type Fake struct {
	HealthFunc               func(context.Context, AccountRef) (AccountStatus, error)
	ModelsFunc               func(context.Context, AccountRef) ([]Model, error)
	ListConversationsFunc    func(context.Context, AccountRef, PageRequest) (ConversationPage, error)
	GetConversationFunc      func(context.Context, AccountRef, string) (Conversation, error)
	CreateConversationFunc   func(context.Context, AccountRef, SendMessageRequest) (Stream, error)
	ContinueConversationFunc func(context.Context, AccountRef, string, SendMessageRequest) (Stream, error)
	RenameConversationFunc   func(context.Context, AccountRef, string, string) error
	ArchiveConversationFunc  func(context.Context, AccountRef, string, bool) error
	DeleteConversationFunc   func(context.Context, AccountRef, string) error
}

func (f *Fake) Health(ctx context.Context, account AccountRef) (AccountStatus, error) {
	if f != nil && f.HealthFunc != nil {
		return f.HealthFunc(ctx, account)
	}
	return AccountStatus{}, unavailableError("health")
}

func (f *Fake) Models(ctx context.Context, account AccountRef) ([]Model, error) {
	if f != nil && f.ModelsFunc != nil {
		return f.ModelsFunc(ctx, account)
	}
	return nil, unavailableError("models")
}

func (f *Fake) ListConversations(ctx context.Context, account AccountRef, page PageRequest) (ConversationPage, error) {
	if f != nil && f.ListConversationsFunc != nil {
		return f.ListConversationsFunc(ctx, account, page)
	}
	return ConversationPage{}, unavailableError("list_conversations")
}

func (f *Fake) GetConversation(ctx context.Context, account AccountRef, conversationID string) (Conversation, error) {
	if f != nil && f.GetConversationFunc != nil {
		return f.GetConversationFunc(ctx, account, conversationID)
	}
	return Conversation{}, unavailableError("get_conversation")
}

func (f *Fake) CreateConversation(ctx context.Context, account AccountRef, req SendMessageRequest) (Stream, error) {
	if f != nil && f.CreateConversationFunc != nil {
		return f.CreateConversationFunc(ctx, account, req)
	}
	return nil, unavailableError("create_conversation")
}

func (f *Fake) ContinueConversation(ctx context.Context, account AccountRef, conversationID string, req SendMessageRequest) (Stream, error) {
	if f != nil && f.ContinueConversationFunc != nil {
		return f.ContinueConversationFunc(ctx, account, conversationID, req)
	}
	return nil, unavailableError("continue_conversation")
}

func (f *Fake) RenameConversation(ctx context.Context, account AccountRef, conversationID string, title string) error {
	if f != nil && f.RenameConversationFunc != nil {
		return f.RenameConversationFunc(ctx, account, conversationID, title)
	}
	return unavailableError("rename_conversation")
}

func (f *Fake) ArchiveConversation(ctx context.Context, account AccountRef, conversationID string, archived bool) error {
	if f != nil && f.ArchiveConversationFunc != nil {
		return f.ArchiveConversationFunc(ctx, account, conversationID, archived)
	}
	return unavailableError("archive_conversation")
}

func (f *Fake) DeleteConversation(ctx context.Context, account AccountRef, conversationID string) error {
	if f != nil && f.DeleteConversationFunc != nil {
		return f.DeleteConversationFunc(ctx, account, conversationID)
	}
	return unavailableError("delete_conversation")
}

// SliceStream is a deterministic in-memory Stream for tests.
type SliceStream struct {
	mu          sync.Mutex
	events      []StreamEvent
	terminalErr error
	index       int
	closed      bool
	errReturned bool
}

func NewSliceStream(events []StreamEvent, terminalErr error) *SliceStream {
	copied := append([]StreamEvent(nil), events...)
	return &SliceStream{events: copied, terminalErr: terminalErr}
}

func (s *SliceStream) Recv(ctx context.Context) (StreamEvent, error) {
	select {
	case <-ctx.Done():
		return StreamEvent{}, ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return StreamEvent{}, io.EOF
	}
	if s.index < len(s.events) {
		event := s.events[s.index]
		s.index++
		return event, nil
	}
	if s.terminalErr != nil && !s.errReturned {
		s.errReturned = true
		return StreamEvent{}, s.terminalErr
	}
	return StreamEvent{}, io.EOF
}

func (s *SliceStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}
