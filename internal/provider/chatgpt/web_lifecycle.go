package chatgpt

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultConversationPageSize = 28
	maxConversationPageSize     = 100
)

type webConversationListResponse struct {
	Items      []webConversationSummary `json:"items"`
	Total      int                      `json:"total"`
	Offset     int                      `json:"offset"`
	Limit      int                      `json:"limit"`
	NextCursor string                   `json:"next_cursor"`
	HasMore    *bool                    `json:"has_more"`
}

type webConversationSummary struct {
	ID         string      `json:"id"`
	Title      string      `json:"title"`
	CreateTime interface{} `json:"create_time"`
	UpdateTime interface{} `json:"update_time"`
	IsArchived bool        `json:"is_archived"`
}

type webConversationDetail struct {
	ID             string                     `json:"id"`
	ConversationID string                     `json:"conversation_id"`
	Title          string                     `json:"title"`
	CreateTime     interface{}                `json:"create_time"`
	UpdateTime     interface{}                `json:"update_time"`
	IsArchived     bool                       `json:"is_archived"`
	CurrentNode    string                     `json:"current_node"`
	Mapping        map[string]webMappingNode  `json:"mapping"`
}

type webMappingNode struct {
	ID       string      `json:"id"`
	Parent   *string     `json:"parent"`
	Message  *webMessage `json:"message"`
}

type webMessage struct {
	ID         string                 `json:"id"`
	Author     webAuthor              `json:"author"`
	Content    webContent             `json:"content"`
	CreateTime interface{}            `json:"create_time"`
	Status     string                 `json:"status"`
	EndTurn    bool                   `json:"end_turn"`
	Metadata   map[string]interface{} `json:"metadata"`
}

type webAuthor struct {
	Role string `json:"role"`
}

type webContent struct {
	ContentType string        `json:"content_type"`
	Parts       []interface{} `json:"parts"`
	Text        string        `json:"text"`
}

type webModelListResponse struct {
	Models []struct {
		ID           string   `json:"id"`
		Slug         string   `json:"slug"`
		Title        string   `json:"title"`
		DisplayName  string   `json:"display_name"`
		Capabilities []string `json:"capabilities"`
	} `json:"models"`
}

type chatRequirementsResponse struct {
	Token string `json:"token"`
	Arkose struct {
		Required bool `json:"required"`
	} `json:"arkose"`
	ProofOfWork struct {
		Required bool `json:"required"`
	} `json:"proofofwork"`
	Turnstile struct {
		Required bool `json:"required"`
	} `json:"turnstile"`
}

func (p *WebProvider) Health(ctx context.Context, account AccountRef) (AccountStatus, error) {
	checkedAt := time.Now()
	_, err := p.Models(ctx, account)
	if err == nil {
		return AccountStatus{State: AccountStateHealthy, Label: "authenticated", CheckedAt: checkedAt}, nil
	}

	status := AccountStatus{State: AccountStateUnknown, Label: "provider check failed", CheckedAt: checkedAt}
	var providerErr *Error
	if errors.As(err, &providerErr) && providerErr.Kind == ErrorKindAuth {
		if providerErr.StatusCode == http.StatusForbidden {
			status.State = AccountStateBlocked
			status.Label = "account session rejected"
		} else {
			status.State = AccountStateExpired
			status.Label = "account session expired"
		}
	}
	return status, err
}

func (p *WebProvider) Models(ctx context.Context, account AccountRef) ([]Model, error) {
	const operation = "models"
	session, err := p.session(ctx, account, operation)
	if err != nil {
		return nil, err
	}

	var payload webModelListResponse
	if err := p.doJSON(ctx, session, http.MethodGet, "/backend-api/models", nil, &payload, operation); err != nil {
		return nil, err
	}

	models := make([]Model, 0, len(payload.Models))
	for _, item := range payload.Models {
		id := firstNonEmpty(item.ID, item.Slug)
		slug := firstNonEmpty(item.Slug, item.ID)
		if id == "" && slug == "" {
			continue
		}
		display := firstNonEmpty(item.DisplayName, item.Title, slug, id)
		models = append(models, Model{
			ID:           id,
			Slug:         slug,
			DisplayName:  display,
			Capabilities: append([]string(nil), item.Capabilities...),
		})
	}
	return models, nil
}

func (p *WebProvider) ListConversations(ctx context.Context, account AccountRef, page PageRequest) (ConversationPage, error) {
	const operation = "list_conversations"
	session, err := p.session(ctx, account, operation)
	if err != nil {
		return ConversationPage{}, err
	}

	limit := page.Limit
	if limit <= 0 {
		limit = defaultConversationPageSize
	}
	if limit > maxConversationPageSize {
		limit = maxConversationPageSize
	}

	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("order", "updated")
	query.Set("is_archived", "false")
	query.Set("is_starred", "false")

	offset := 0
	if page.Cursor != "" {
		if parsedOffset, ok := decodeOffsetCursor(page.Cursor); ok {
			offset = parsedOffset
			query.Set("offset", strconv.Itoa(offset))
		} else {
			query.Set("cursor", page.Cursor)
		}
	} else {
		query.Set("offset", "0")
	}

	var payload webConversationListResponse
	path := "/backend-api/conversations?" + query.Encode()
	if err := p.doJSON(ctx, session, http.MethodGet, path, nil, &payload, operation); err != nil {
		return ConversationPage{}, err
	}

	items := make([]ConversationSummary, 0, len(payload.Items))
	for _, item := range payload.Items {
		if item.ID == "" {
			continue
		}
		items = append(items, ConversationSummary{
			ID:        item.ID,
			Title:     item.Title,
			Archived:  item.IsArchived,
			CreatedAt: parseUpstreamTime(item.CreateTime),
			UpdatedAt: parseUpstreamTime(item.UpdateTime),
		})
	}

	hasMore := false
	if payload.HasMore != nil {
		hasMore = *payload.HasMore
	} else if payload.Total > 0 {
		responseOffset := payload.Offset
		if responseOffset == 0 && offset > 0 {
			responseOffset = offset
		}
		hasMore = responseOffset+len(payload.Items) < payload.Total
	} else {
		hasMore = len(payload.Items) >= limit
	}

	nextCursor := payload.NextCursor
	if hasMore && nextCursor == "" {
		responseOffset := payload.Offset
		if responseOffset == 0 && offset > 0 {
			responseOffset = offset
		}
		nextCursor = encodeOffsetCursor(responseOffset + len(payload.Items))
	}

	return ConversationPage{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (p *WebProvider) GetConversation(ctx context.Context, account AccountRef, conversationID string) (Conversation, error) {
	const operation = "get_conversation"
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return Conversation{}, &Error{Kind: ErrorKindInvalidRequest, Operation: operation, Err: errors.New("conversation id is required")}
	}

	session, err := p.session(ctx, account, operation)
	if err != nil {
		return Conversation{}, err
	}
	var payload webConversationDetail
	path := "/backend-api/conversation/" + url.PathEscape(conversationID)
	if err := p.doJSON(ctx, session, http.MethodGet, path, nil, &payload, operation); err != nil {
		return Conversation{}, err
	}
	return normalizeConversation(payload, conversationID), nil
}

func (p *WebProvider) CreateConversation(ctx context.Context, account AccountRef, req SendMessageRequest) (Stream, error) {
	return p.sendConversation(ctx, account, "", req, "create_conversation")
}

func (p *WebProvider) ContinueConversation(ctx context.Context, account AccountRef, conversationID string, req SendMessageRequest) (Stream, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, &Error{Kind: ErrorKindInvalidRequest, Operation: "continue_conversation", Err: errors.New("conversation id is required")}
	}
	return p.sendConversation(ctx, account, conversationID, req, "continue_conversation")
}

func (p *WebProvider) RenameConversation(ctx context.Context, account AccountRef, conversationID string, title string) error {
	const operation = "rename_conversation"
	conversationID = strings.TrimSpace(conversationID)
	title = strings.TrimSpace(title)
	if conversationID == "" || title == "" {
		return &Error{Kind: ErrorKindInvalidRequest, Operation: operation, Err: errors.New("conversation id and title are required")}
	}
	session, err := p.session(ctx, account, operation)
	if err != nil {
		return err
	}
	return p.doJSON(ctx, session, http.MethodPatch, "/backend-api/conversation/"+url.PathEscape(conversationID), map[string]interface{}{"title": title}, nil, operation)
}

func (p *WebProvider) ArchiveConversation(ctx context.Context, account AccountRef, conversationID string, archived bool) error {
	const operation = "archive_conversation"
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return &Error{Kind: ErrorKindInvalidRequest, Operation: operation, Err: errors.New("conversation id is required")}
	}
	session, err := p.session(ctx, account, operation)
	if err != nil {
		return err
	}
	return p.doJSON(ctx, session, http.MethodPatch, "/backend-api/conversation/"+url.PathEscape(conversationID), map[string]interface{}{"is_archived": archived}, nil, operation)
}

func (p *WebProvider) DeleteConversation(ctx context.Context, account AccountRef, conversationID string) error {
	const operation = "delete_conversation"
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return &Error{Kind: ErrorKindInvalidRequest, Operation: operation, Err: errors.New("conversation id is required")}
	}
	session, err := p.session(ctx, account, operation)
	if err != nil {
		return err
	}
	return p.doJSON(ctx, session, http.MethodPatch, "/backend-api/conversation/"+url.PathEscape(conversationID), map[string]interface{}{"is_visible": false}, nil, operation)
}

func (p *WebProvider) sendConversation(ctx context.Context, account AccountRef, conversationID string, input SendMessageRequest, operation string) (Stream, error) {
	content := strings.TrimSpace(input.Message.Content)
	if content == "" {
		return nil, &Error{Kind: ErrorKindInvalidRequest, Operation: operation, Err: errors.New("message content is required")}
	}

	session, err := p.session(ctx, account, operation)
	if err != nil {
		return nil, err
	}
	requirements, err := p.chatRequirements(ctx, session, operation)
	if err != nil {
		return nil, err
	}
	if requirements.Arkose.Required || requirements.ProofOfWork.Required || requirements.Turnstile.Required {
		return nil, &Error{
			Kind:      ErrorKindUnavailable,
			Operation: operation,
			Err:       errors.New("upstream requires a browser challenge executor for this session"),
		}
	}

	messageID, err := randomUUID()
	if err != nil {
		return nil, &Error{Kind: ErrorKindProtocol, Operation: operation, Err: fmt.Errorf("create message id: %w", err)}
	}
	parentID := strings.TrimSpace(input.ParentMessageID)
	if parentID == "" {
		parentID, err = randomUUID()
		if err != nil {
			return nil, &Error{Kind: ErrorKindProtocol, Operation: operation, Err: fmt.Errorf("create parent message id: %w", err)}
		}
	}

	role := input.Message.Role
	if role == "" {
		role = RoleUser
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = "auto"
	}

	body := map[string]interface{}{
		"action": "next",
		"messages": []interface{}{
			map[string]interface{}{
				"id": messageID,
				"author": map[string]interface{}{"role": string(role)},
				"content": map[string]interface{}{
					"content_type": "text",
					"parts":        []string{content},
				},
				"metadata": map[string]interface{}{},
			},
		},
		"parent_message_id":             parentID,
		"model":                         model,
		"timezone_offset_min":           0,
		"history_and_training_disabled": input.Temporary,
		"conversation_mode":             map[string]interface{}{"kind": "primary_assistant"},
		"suggestions":                   []interface{}{},
		"force_paragen":                 false,
		"force_rate_limit":              false,
	}
	if conversationID != "" {
		body["conversation_id"] = conversationID
	}

	req, err := p.newRequest(ctx, session, http.MethodPost, p.conversationPath, body, operation)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	if requirements.Token != "" {
		req.Header.Set("Openai-Sentinel-Chat-Requirements-Token", requirements.Token)
	}

	resp, err := session.client.Do(req)
	if err != nil {
		return nil, &Error{Kind: ErrorKindTransport, Operation: operation, Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, providerHTTPError(operation, resp)
	}
	return newSSEStream(resp.Body, operation), nil
}

func (p *WebProvider) chatRequirements(ctx context.Context, session *webSession, operation string) (chatRequirementsResponse, error) {
	var payload chatRequirementsResponse
	if err := p.doJSON(ctx, session, http.MethodPost, "/backend-api/sentinel/chat-requirements", map[string]interface{}{}, &payload, operation); err != nil {
		return chatRequirementsResponse{}, err
	}
	return payload, nil
}

func normalizeConversation(payload webConversationDetail, fallbackID string) Conversation {
	conversationID := firstNonEmpty(payload.ConversationID, payload.ID, fallbackID)
	conversation := Conversation{
		ID:        conversationID,
		Title:     payload.Title,
		Archived:  payload.IsArchived,
		CreatedAt: parseUpstreamTime(payload.CreateTime),
		UpdatedAt: parseUpstreamTime(payload.UpdateTime),
	}
	if payload.CurrentNode == "" || len(payload.Mapping) == 0 {
		return conversation
	}

	nodeIDs := make([]string, 0, 32)
	seen := make(map[string]struct{})
	current := payload.CurrentNode
	for current != "" {
		if _, exists := seen[current]; exists {
			break
		}
		seen[current] = struct{}{}
		node, ok := payload.Mapping[current]
		if !ok {
			break
		}
		nodeIDs = append(nodeIDs, current)
		if node.Parent == nil {
			break
		}
		current = *node.Parent
	}

	messages := make([]Message, 0, len(nodeIDs))
	for i := len(nodeIDs) - 1; i >= 0; i-- {
		node := payload.Mapping[nodeIDs[i]]
		if node.Message == nil || visuallyHidden(node.Message.Metadata) {
			continue
		}
		role, ok := normalizeRole(node.Message.Author.Role)
		if !ok {
			continue
		}
		messageID := firstNonEmpty(node.Message.ID, node.ID, nodeIDs[i])
		parentID := ""
		if node.Parent != nil {
			if parentNode, ok := payload.Mapping[*node.Parent]; ok && parentNode.Message != nil {
				parentID = firstNonEmpty(parentNode.Message.ID, parentNode.ID, *node.Parent)
			} else {
				parentID = *node.Parent
			}
		}
		messages = append(messages, Message{
			ID:        messageID,
			ParentID:  parentID,
			Role:      role,
			Content:   extractWebContent(node.Message.Content),
			CreatedAt: parseUpstreamTime(node.Message.CreateTime),
		})
	}
	conversation.Messages = messages
	return conversation
}

func normalizeRole(raw string) (Role, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "user":
		return RoleUser, true
	case "assistant":
		return RoleAssistant, true
	case "system", "developer":
		return RoleSystem, true
	case "tool":
		return RoleTool, true
	default:
		return "", false
	}
}

func visuallyHidden(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	value, ok := metadata["is_visually_hidden_from_conversation"]
	if !ok {
		return false
	}
	hidden, _ := value.(bool)
	return hidden
}

func extractWebContent(content webContent) string {
	texts := make([]string, 0, len(content.Parts)+1)
	for _, part := range content.Parts {
		switch value := part.(type) {
		case string:
			if value != "" {
				texts = append(texts, value)
			}
		case map[string]interface{}:
			if text, ok := value["text"].(string); ok && text != "" {
				texts = append(texts, text)
			}
		}
	}
	if len(texts) == 0 && content.Text != "" {
		texts = append(texts, content.Text)
	}
	return strings.Join(texts, "\n")
}

func parseUpstreamTime(value interface{}) time.Time {
	switch typed := value.(type) {
	case float64:
		seconds := int64(typed)
		nanos := int64((typed - float64(seconds)) * float64(time.Second))
		return time.Unix(seconds, nanos).UTC()
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parseUpstreamTime(parsed)
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return time.Time{}
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return parseUpstreamTime(parsed)
		}
		if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func encodeOffsetCursor(offset int) string {
	if offset < 0 {
		offset = 0
	}
	return "offset:" + strconv.Itoa(offset)
}

func decodeOffsetCursor(cursor string) (int, bool) {
	if !strings.HasPrefix(cursor, "offset:") {
		return 0, false
	}
	offset, err := strconv.Atoi(strings.TrimPrefix(cursor, "offset:"))
	if err != nil || offset < 0 {
		return 0, false
	}
	return offset, true
}

func randomUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
