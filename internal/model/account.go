package model

type Account struct {
	ID              uint       `json:"id" gorm:"primaryKey" gorm:"column:id"`
	Email           string     `json:"email" gorm:"column:email"`
	Password        string     `json:"-" gorm:"column:password"`
	SessionToken    string     `json:"-" gorm:"column:session_token"`
	AccessToken     string     `json:"-" gorm:"column:access_token"`
	CreateTime      *LocalTime `json:"createTime" gorm:"autoCreateTime" gorm:"column:create_time"`
	UpdateTime      *LocalTime `json:"updateTime" gorm:"autoUpdateTime" gorm:"column:update_time"`
	Shared          int        `json:"shared" gorm:"column:shared"`
	RefreshToken    string     `json:"-" gorm:"column:refresh_token"`
	AccountType     string     `json:"accountType" gorm:"column:account_type" gorm:"default:chatgpt"`
	SessionKey      string     `json:"-" gorm:"column:session_key"`
	OneApiChannelId string     `json:"oneApiChannelId" gorm:"column:one_api_channel_id;default:''"`
	ProxyURL        string     `json:"-" gorm:"column:proxy_url;default:''"`
	Shares          []Share    `json:"-" gorm:"foreignKey:AccountID;constraint:OnDelete:CASCADE"`
}

func (m *Account) TableName() string {
	return "account"
}
