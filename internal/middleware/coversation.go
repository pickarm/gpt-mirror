package middleware

import (
	"PandoraHelper/internal/repository"
	"PandoraHelper/pkg/log"

	"github.com/gin-gonic/gin"
)

// ConversationLoggerMiddleware is retained as a compatibility shell for old
// wiring. Conversation content must not be mirrored into a local source of
// truth. M5 records routing/audit metadata only at layers that possess real
// account and upstream conversation/message identifiers.
type ConversationLoggerMiddleware struct {
	logger     *log.Logger
	repository repository.ConversationRepository
}

func NewConversationLoggerMiddleware(logger *log.Logger, repo repository.ConversationRepository) *ConversationLoggerMiddleware {
	return &ConversationLoggerMiddleware{
		logger:     logger,
		repository: repo,
	}
}

// ClaudeLogConversation intentionally performs no request/response capture.
// The legacy implementation copied full prompts and completions into the local
// database, which conflicts with upstream-source-of-truth semantics.
func (m *ConversationLoggerMiddleware) ClaudeLogConversation() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// ChatGptLogConversation intentionally performs no request/response capture.
// ChatGPT conversation content is read from the upstream Provider when needed.
func (m *ConversationLoggerMiddleware) ChatGptLogConversation() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
