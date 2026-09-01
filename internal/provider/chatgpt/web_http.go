package chatgpt

import (
	"PandoraHelper/internal/repository"
	credentialprovider "PandoraHelper/internal/provider/credential"
	apptransport "PandoraHelper/internal/transport"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const defaultChatGPTBaseURL = "https://chatgpt.com"
const defaultConversationPath = "/backend-api/f/conversation"
const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

type WebProvider struct {
	credentials      credentialprovider.Provider
	accounts         repository.AccountRepository
	transportFactory *apptransport.Factory
	baseURL          *url.URL
	conversationPath string
	userAgent        string
}

type webSession struct {
	client    *http.Client
	token     string
	cookie    string
	deviceID  string
	userAgent string
}

func NewWebProvider(
	credentials credentialprovider.Provider,
	accounts repository.AccountRepository,
	transportFactory *apptransport.Factory,
	conf *viper.Viper,
) (Provider, error) {
	base := strings.TrimSpace(conf.GetString("chatgpt.base_url"))
	if base == "" {
		base = defaultChatGPTBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid chatgpt.base_url %q", base)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")

	conversationPath := strings.TrimSpace(conf.GetString("chatgpt.conversation_path"))
	if conversationPath == "" {
		conversationPath = defaultConversationPath
	}
	if !strings.HasPrefix(conversationPath, "/backend-api/") {
		return nil, fmt.Errorf("chatgpt.conversation_path must stay under /backend-api/")
	}
	userAgent := strings.TrimSpace(conf.GetString("chatgpt.user_agent"))
	if userAgent == "" {
		userAgent = defaultUserAgent
	}

	return &WebProvider{
		credentials:      credentials,
		accounts:         accounts,
		transportFactory: transportFactory,
		baseURL:          parsed,
		conversationPath: conversationPath,
		userAgent:        userAgent,
	}, nil
}

func (p *WebProvider) session(ctx context.Context, account AccountRef, operation string) (*webSession, error) {
	if account.ID == 0 {
		return nil, &Error{Kind: ErrorKindInvalidRequest, Operation: operation, Err: errors.New("account id is required")}
	}
	modelAccount, err := p.accounts.GetAccount(ctx, int64(account.ID))
	if err != nil {
		return nil, normalizeProviderError(operation, err)
	}
	secret, err := p.credentials.Resolve(ctx, account.ID)
	if err != nil {
		kind := ErrorKindAuth
		if errors.Is(err, credentialprovider.ErrEncryptionKeyMissing) {
			kind = ErrorKindUnavailable
		}
		return nil, &Error{Kind: kind, Operation: operation, Err: errors.New("account credential is unavailable")}
	}
	proxyURL := modelAccount.ProxyURL
	if secret.ProxyURL != "" {
		proxyURL = secret.ProxyURL
	}
	client, _, err := p.transportFactory.Client(proxyURL)
	if err != nil {
		return nil, &Error{Kind: ErrorKindTransport, Operation: operation, Err: err}
	}

	cookie := strings.TrimSpace(secret.Cookie)
	if cookie == "" && secret.SessionToken != "" {
		cookie = "__Secure-next-auth.session-token=" + secret.SessionToken
	}
	token := strings.TrimSpace(secret.AccessToken)
	if token == "" {
		if cookie == "" {
			return nil, &Error{Kind: ErrorKindAuth, Operation: operation, Err: errors.New("credential has no access token or browser session cookie")}
		}
		token, err = p.exchangeAccessToken(ctx, client, cookie, operation)
		if err != nil {
			return nil, err
		}
	}

	return &webSession{
		client:    client,
		token:     token,
		cookie:    cookie,
		deviceID:  cookieValue(cookie, "oai-did"),
		userAgent: p.userAgent,
	}, nil
}

func (p *WebProvider) exchangeAccessToken(ctx context.Context, client *http.Client, cookie, operation string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.resolveURL("/api/auth/session"), nil)
	if err != nil {
		return "", &Error{Kind: ErrorKindProtocol, Operation: operation, Err: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", &Error{Kind: ErrorKindTransport, Operation: operation, Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", providerHTTPError(operation, resp)
	}
	var payload struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil {
		return "", &Error{Kind: ErrorKindProtocol, Operation: operation, StatusCode: resp.StatusCode, Err: fmt.Errorf("decode auth session: %w", err)}
	}
	if payload.AccessToken == "" {
		return "", &Error{Kind: ErrorKindAuth, Operation: operation, StatusCode: resp.StatusCode, Err: errors.New("browser session did not yield an access token")}
	}
	return payload.AccessToken, nil
}

func (p *WebProvider) newRequest(ctx context.Context, session *webSession, method, path string, body interface{}, operation string) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, &Error{Kind: ErrorKindProtocol, Operation: operation, Err: fmt.Errorf("encode request: %w", err)}
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.resolveURL(path), reader)
	if err != nil {
		return nil, &Error{Kind: ErrorKindProtocol, Operation: operation, Err: err}
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+session.token)
	req.Header.Set("User-Agent", session.userAgent)
	req.Header.Set("Oai-Language", "en-US")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if session.cookie != "" {
		req.Header.Set("Cookie", session.cookie)
	}
	if session.deviceID != "" {
		req.Header.Set("Oai-Device-Id", session.deviceID)
	}
	return req, nil
}

func (p *WebProvider) doJSON(ctx context.Context, session *webSession, method, path string, body interface{}, output interface{}, operation string) error {
	req, err := p.newRequest(ctx, session, method, path, body, operation)
	if err != nil {
		return err
	}
	resp, err := session.client.Do(req)
	if err != nil {
		return &Error{Kind: ErrorKindTransport, Operation: operation, Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerHTTPError(operation, resp)
	}
	if output == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(output); err != nil {
		return &Error{Kind: ErrorKindProtocol, Operation: operation, StatusCode: resp.StatusCode, Err: fmt.Errorf("decode upstream JSON: %w", err)}
	}
	return nil
}

func (p *WebProvider) resolveURL(path string) string {
	base := *p.baseURL
	pathPart, rawQuery, _ := strings.Cut(path, "?")
	base.Path = strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(pathPart, "/")
	base.RawQuery = rawQuery
	base.Fragment = ""
	return base.String()
}

func providerHTTPError(operation string, resp *http.Response) error {
	kind := ErrorKindProtocol
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		kind = ErrorKindAuth
	case http.StatusNotFound:
		kind = ErrorKindNotFound
	case http.StatusTooManyRequests:
		kind = ErrorKindRateLimit
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		kind = ErrorKindInvalidRequest
	default:
		if resp.StatusCode >= 500 {
			kind = ErrorKindTransport
		}
	}
	providerErr := &Error{Kind: kind, Operation: operation, StatusCode: resp.StatusCode, Err: errors.New(http.StatusText(resp.StatusCode))}
	if value := resp.Header.Get("Retry-After"); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
			providerErr.RetryAfter = time.Duration(seconds) * time.Second
		}
	}
	return providerErr
}

func normalizeProviderError(operation string, err error) error {
	var providerErr *Error
	if errors.As(err, &providerErr) {
		return err
	}
	return &Error{Kind: ErrorKindTransport, Operation: operation, Err: err}
}

func cookieValue(header, name string) string {
	for _, part := range strings.Split(header, ";") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) == 2 && pair[0] == name {
			return pair[1]
		}
	}
	return ""
}
