package service

import (
	chatgptprovider "PandoraHelper/internal/provider/chatgpt"
	"context"
	"errors"
)

type ConversationService interface {
	Health(ctx context.Context, accountID uint) (chatgptprovider.AccountStatus, error)
	Models(ctx context.Context, accountID uint) ([]chatgptprovider.Model, error)
	List(ctx context.Context, accountID uint, page chatgptprovider.PageRequest) (chatgptprovider.ConversationPage, error)
	Get(ctx context.Context, accountID uint, conversationID string) (chatgptprovider.Conversation, error)
	Create(ctx context.Context, accountID uint, req chatgptprovider.SendMessageRequest) (chatgptprovider.Stream, error)
	Continue(ctx context.Context, accountID uint, conversationID string, req chatgptprovider.SendMessageRequest) (chatgptprovider.Stream, error)
	Rename(ctx context.Context, accountID uint, conversationID, title string) error
	Archive(ctx context.Context, accountID uint, conversationID string, archived bool) error
	Delete(ctx context.Context, accountID uint, conversationID string) error
}

type conversationService struct {
	*Service
}

func NewConversationService(service *Service) ConversationService {
	return &conversationService{Service: service}
}

func (s *conversationService) accountRef(accountID uint) (chatgptprovider.AccountRef, error) {
	if accountID == 0 {
		return chatgptprovider.AccountRef{}, errors.New("account id is required")
	}
	return chatgptprovider.AccountRef{ID: accountID}, nil
}

func (s *conversationService) Health(ctx context.Context, accountID uint) (chatgptprovider.AccountStatus, error) {
	account, err := s.accountRef(accountID)
	if err != nil {
		return chatgptprovider.AccountStatus{}, err
	}
	return s.chatgptProvider.Health(ctx, account)
}

func (s *conversationService) Models(ctx context.Context, accountID uint) ([]chatgptprovider.Model, error) {
	account, err := s.accountRef(accountID)
	if err != nil {
		return nil, err
	}
	return s.chatgptProvider.Models(ctx, account)
}

func (s *conversationService) List(ctx context.Context, accountID uint, page chatgptprovider.PageRequest) (chatgptprovider.ConversationPage, error) {
	account, err := s.accountRef(accountID)
	if err != nil {
		return chatgptprovider.ConversationPage{}, err
	}
	return s.chatgptProvider.ListConversations(ctx, account, page)
}

func (s *conversationService) Get(ctx context.Context, accountID uint, conversationID string) (chatgptprovider.Conversation, error) {
	account, err := s.accountRef(accountID)
	if err != nil {
		return chatgptprovider.Conversation{}, err
	}
	return s.chatgptProvider.GetConversation(ctx, account, conversationID)
}

func (s *conversationService) Create(ctx context.Context, accountID uint, req chatgptprovider.SendMessageRequest) (chatgptprovider.Stream, error) {
	account, err := s.accountRef(accountID)
	if err != nil {
		return nil, err
	}
	return s.chatgptProvider.CreateConversation(ctx, account, req)
}

func (s *conversationService) Continue(ctx context.Context, accountID uint, conversationID string, req chatgptprovider.SendMessageRequest) (chatgptprovider.Stream, error) {
	account, err := s.accountRef(accountID)
	if err != nil {
		return nil, err
	}
	return s.chatgptProvider.ContinueConversation(ctx, account, conversationID, req)
}

func (s *conversationService) Rename(ctx context.Context, accountID uint, conversationID, title string) error {
	account, err := s.accountRef(accountID)
	if err != nil {
		return err
	}
	return s.chatgptProvider.RenameConversation(ctx, account, conversationID, title)
}

func (s *conversationService) Archive(ctx context.Context, accountID uint, conversationID string, archived bool) error {
	account, err := s.accountRef(accountID)
	if err != nil {
		return err
	}
	return s.chatgptProvider.ArchiveConversation(ctx, account, conversationID, archived)
}

func (s *conversationService) Delete(ctx context.Context, accountID uint, conversationID string) error {
	account, err := s.accountRef(accountID)
	if err != nil {
		return err
	}
	return s.chatgptProvider.DeleteConversation(ctx, account, conversationID)
}
