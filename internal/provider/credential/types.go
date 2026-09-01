package credential

import (
	"context"
	"errors"
	"time"
)

type Representation string

const (
	RepresentationLegacyFields Representation = "legacy_fields"
	RepresentationToken        Representation = "token"
	RepresentationCookie       Representation = "cookie"
	RepresentationReference    Representation = "reference"
)

type State string

const (
	StateHealthy State = "healthy"
	StateExpired State = "expired"
	StateBlocked State = "blocked"
	StateUnknown State = "unknown"
)

var (
	ErrCredentialNotFound   = errors.New("credential not found")
	ErrEncryptionKeyMissing = errors.New("credential encryption key is not configured")
	ErrExpired              = errors.New("credential expired")
	ErrBlocked              = errors.New("credential blocked")
	ErrInvalid              = errors.New("credential invalid")
)

// Secret is the internal credential representation. It intentionally supports
// multiple forms so services do not need to know whether a future provider uses
// tokens, cookies, an external/browser-profile reference, or authenticated
// proxy credentials. Every field in Secret is encrypted at rest by Provider.
type Secret struct {
	Representation Representation `json:"representation"`
	Password       string         `json:"password,omitempty"`
	SessionToken   string         `json:"session_token,omitempty"`
	AccessToken    string         `json:"access_token,omitempty"`
	RefreshToken   string         `json:"refresh_token,omitempty"`
	SessionKey     string         `json:"session_key,omitempty"`
	Cookie         string         `json:"cookie,omitempty"`
	Reference      string         `json:"reference,omitempty"`
	ProxyURL       string         `json:"proxy_url,omitempty"`
}

func (s Secret) Empty() bool {
	return s.Password == "" &&
		s.SessionToken == "" &&
		s.AccessToken == "" &&
		s.RefreshToken == "" &&
		s.SessionKey == "" &&
		s.Cookie == "" &&
		s.Reference == "" &&
		s.ProxyURL == ""
}

func (s Secret) Kind() string {
	if s.Representation != "" {
		return string(s.Representation)
	}
	switch {
	case s.Reference != "":
		return string(RepresentationReference)
	case s.Cookie != "":
		return string(RepresentationCookie)
	case s.AccessToken != "" || s.RefreshToken != "" || s.SessionToken != "" || s.SessionKey != "":
		return string(RepresentationToken)
	default:
		return string(RepresentationLegacyFields)
	}
}

type Health struct {
	State     State     `json:"state"`
	Message   string    `json:"message,omitempty"`
	CheckedAt time.Time `json:"checkedAt,omitempty"`
}

type Status struct {
	HasCredential bool
	State         State
	Message       string
	CheckedAt     *time.Time
}

type Record struct {
	AccountID  uint
	Kind       string
	Ciphertext string
	State      State
	Message    string
	CheckedAt  *time.Time
}

type Store interface {
	Get(ctx context.Context, accountID uint) (*Record, error)
	Upsert(ctx context.Context, record *Record) error
	Delete(ctx context.Context, accountID uint) error
	UpdateHealth(ctx context.Context, accountID uint, health Health) error
}

type Validator interface {
	Validate(ctx context.Context, accountID uint, secret Secret) (Health, error)
}

type Provider interface {
	Resolve(ctx context.Context, accountID uint) (Secret, error)
	Status(ctx context.Context, accountID uint) (Status, error)
	Put(ctx context.Context, accountID uint, secret Secret) error
	Delete(ctx context.Context, accountID uint) error
	Validate(ctx context.Context, accountID uint) (Health, error)
	CanPersist() bool
}

func HealthFromError(err error) Health {
	switch {
	case errors.Is(err, ErrExpired), errors.Is(err, ErrInvalid):
		return Health{State: StateExpired, Message: "credential expired or invalid"}
	case errors.Is(err, ErrBlocked):
		return Health{State: StateBlocked, Message: "credential or account is blocked"}
	default:
		return Health{State: StateUnknown, Message: "credential state could not be determined"}
	}
}
