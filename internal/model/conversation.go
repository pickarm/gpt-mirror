package model

import "time"

// Conversation stores routing/audit metadata only. ChatGPT remains the source
// of truth for conversation content and message history.
type Conversation struct {
	ID                     uint      `gorm:"primaryKey"`
	AccountID              uint      `gorm:"column:account_id;index;not null"`
	UpstreamConversationID string    `gorm:"column:upstream_conversation_id;index;not null"`
	UpstreamMessageID      string    `gorm:"column:upstream_message_id;index"`
	Model                  string    `gorm:"column:model"`
	Operation              string    `gorm:"column:operation;index"`
	Timestamp              time.Time `gorm:"column:timestamp;index"`
}

func (m *Conversation) TableName() string {
	return "conversation_metadata"
}
