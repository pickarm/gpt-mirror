package chatgpt

import "context"

// UnavailableProvider is the safe default until a concrete ChatGPT provider is
// configured. It guarantees that wiring the provider boundary never causes
// implicit network access.
type UnavailableProvider struct{}

func NewUnavailableProvider() Provider {
	return &UnavailableProvider{}
}

func (*UnavailableProvider) Health(context.Context, AccountRef) (AccountStatus, error) {
	return AccountStatus{State: AccountStateUnknown}, unavailableError("health")
}

func (*UnavailableProvider) Models(context.Context, AccountRef) ([]Model, error) {
	return nil, unavailableError("models")
}

func (*UnavailableProvider) ListConversations(context.Context, AccountRef, PageRequest) (ConversationPage, error) {
	return ConversationPage{}, unavailableError("list_conversations")
}

func (*UnavailableProvider) GetConversation(context.Context, AccountRef, string) (Conversation, error) {
	return Conversation{}, unavailableError("get_conversation")
}

func (*UnavailableProvider) CreateConversation(context.Context, AccountRef, SendMessageRequest) (Stream, error) {
	return nil, unavailableError("create_conversation")
}

func (*UnavailableProvider) ContinueConversation(context.Context, AccountRef, string, SendMessageRequest) (Stream, error) {
	return nil, unavailableError("continue_conversation")
}

func (*UnavailableProvider) RenameConversation(context.Context, AccountRef, string, string) error {
	return unavailableError("rename_conversation")
}

func (*UnavailableProvider) ArchiveConversation(context.Context, AccountRef, string, bool) error {
	return unavailableError("archive_conversation")
}

func (*UnavailableProvider) DeleteConversation(context.Context, AccountRef, string) error {
	return unavailableError("delete_conversation")
}
