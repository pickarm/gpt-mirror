package model

import "time"

// AccountCredential stores encrypted account authentication material outside of
// the account table. Secret values are never exposed through JSON.
type AccountCredential struct {
	AccountID  uint       `json:"-" gorm:"primaryKey;column:account_id"`
	Kind       string     `json:"-" gorm:"column:kind;default:legacy_fields"`
	Ciphertext string     `json:"-" gorm:"column:ciphertext;type:text;not null"`
	State      string     `json:"-" gorm:"column:state;default:unknown"`
	Message    string     `json:"-" gorm:"column:message;type:text"`
	CheckedAt  *time.Time `json:"-" gorm:"column:checked_at"`
	CreateTime *LocalTime `json:"-" gorm:"autoCreateTime;column:create_time"`
	UpdateTime *LocalTime `json:"-" gorm:"autoUpdateTime;column:update_time"`
}

func (m *AccountCredential) TableName() string {
	return "account_credential"
}
