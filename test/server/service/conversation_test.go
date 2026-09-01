package service_test

import (
	chatgptprovider "PandoraHelper/internal/provider/chatgpt"
	"PandoraHelper/internal/service"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConversationServiceDelegatesToProvider(t *testing.T) {
	fake := &chatgptprovider.Fake{
		ModelsFunc: func(_ context.Context, account chatgptprovider.AccountRef) ([]chatgptprovider.Model, error) {
			require.Equal(t, uint(42), account.ID)
			return []chatgptprovider.Model{{ID: "model-1", Slug: "model-1", DisplayName: "Model 1"}}, nil
		},
		ListConversationsFunc: func(_ context.Context, account chatgptprovider.AccountRef, page chatgptprovider.PageRequest) (chatgptprovider.ConversationPage, error) {
			require.Equal(t, uint(42), account.ID)
			require.Equal(t, 25, page.Limit)
			return chatgptprovider.ConversationPage{Items: []chatgptprovider.ConversationSummary{{ID: "conv-1", Title: "Cloud chat"}}}, nil
		},
	}

	base := service.NewService(nil, nil, nil, nil, fake)
	conversations := service.NewConversationService(base)

	models, err := conversations.Models(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "model-1", models[0].ID)

	page, err := conversations.List(context.Background(), 42, chatgptprovider.PageRequest{Limit: 25})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, "conv-1", page.Items[0].ID)
}

func TestConversationServiceRejectsMissingAccount(t *testing.T) {
	base := service.NewService(nil, nil, nil, nil, &chatgptprovider.Fake{})
	conversations := service.NewConversationService(base)

	_, err := conversations.Models(context.Background(), 0)
	require.ErrorContains(t, err, "account id is required")
}
