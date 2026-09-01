package v1

import (
	"PandoraHelper/internal/model"
	"time"
)

type SearchAccountRequest struct {
	Email       string `json:"email" example:"1234@gmail.com"`
	AccountType string `json:"accountType" example:"chatgpt"`
}

// AccountWriteRequest is the only HTTP DTO that accepts account credential
// material. model.Account itself never serializes secrets.
type AccountWriteRequest struct {
	ID              uint   `json:"id"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	SessionToken    string `json:"sessionToken"`
	AccessToken     string `json:"accessToken"`
	Shared          int    `json:"shared"`
	RefreshToken    string `json:"refreshToken"`
	AccountType     string `json:"accountType"`
	SessionKey      string `json:"sessionKey"`
	OneApiChannelId string `json:"oneApiChannelId"`
	ProxyURL        string `json:"proxyUrl"`
}

func (r *AccountWriteRequest) ToModel() *model.Account {
	return &model.Account{
		ID:              r.ID,
		Email:           r.Email,
		Password:        r.Password,
		SessionToken:    r.SessionToken,
		AccessToken:     r.AccessToken,
		Shared:          r.Shared,
		RefreshToken:    r.RefreshToken,
		AccountType:     r.AccountType,
		SessionKey:      r.SessionKey,
		OneApiChannelId: r.OneApiChannelId,
		ProxyURL:        r.ProxyURL,
	}
}

type AccountSummary struct {
	ID                uint             `json:"id"`
	Email             string           `json:"email"`
	AccountType       string           `json:"accountType"`
	CreateTime        *model.LocalTime `json:"createTime,omitempty"`
	UpdateTime        *model.LocalTime `json:"updateTime,omitempty"`
	Shared            int              `json:"shared"`
	OneApiChannelId   string           `json:"oneApiChannelId,omitempty"`
	HasCredential     bool             `json:"hasCredential"`
	CredentialState   string           `json:"credentialState"`
	CredentialMessage string           `json:"credentialMessage,omitempty"`
	CredentialChecked *time.Time       `json:"credentialCheckedAt,omitempty"`
	ProxyConfigured   bool             `json:"proxyConfigured"`
	ProxyDisplay      string           `json:"proxyDisplay,omitempty"`
}

type DeleteAccountRequest struct {
	Id int64 `json:"id" binding:"required"`
}

type RefreshAccountRequest struct {
	Id int64 `json:"id" binding:"required"`
}

type LoginShareAccountRequest struct {
	Id         int64  `json:"id"`
	UniqueName string `json:"uniqueName"`
	SelectType string `json:"selectType"`
}

type SearchAccountResponseData struct {
	Response
	Data []*AccountSummary `json:"data"`
}

type AccountResponseData struct {
	Response
}

type ShareAccountResponseData struct {
	Response
	Accounts []*model.Account `json:"accounts"`
	Custom   bool             `json:"custom"`
	Random   bool             `json:"random"`
}
