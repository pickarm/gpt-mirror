package chatgpt

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

const maxSSELineSize = 8 << 20

type sseStream struct {
	mu             sync.Mutex
	body           io.ReadCloser
	scanner        *bufio.Scanner
	operation      string
	pendingData    []string
	queue          []StreamEvent
	lastText       map[string]string
	completed      map[string]bool
	seenConversation map[string]bool
	done           bool
	closed         bool
}

func newSSEStream(body io.ReadCloser, operation string) Stream {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), maxSSELineSize)
	return &sseStream{
		body:             body,
		scanner:          scanner,
		operation:        operation,
		lastText:         make(map[string]string),
		completed:        make(map[string]bool),
		seenConversation: make(map[string]bool),
	}
}

func (s *sseStream) Recv(ctx context.Context) (StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return StreamEvent{}, io.EOF
	}
	if len(s.queue) > 0 {
		return s.pop(), nil
	}
	if s.done {
		return StreamEvent{}, io.EOF
	}

	for {
		select {
		case <-ctx.Done():
			return StreamEvent{}, ctx.Err()
		default:
		}

		if !s.scanner.Scan() {
			if len(s.pendingData) > 0 {
				s.flushEvent()
				if len(s.queue) > 0 {
					return s.pop(), nil
				}
			}
			if err := s.scanner.Err(); err != nil {
				return StreamEvent{}, &Error{Kind: ErrorKindProtocol, Operation: s.operation, Err: fmt.Errorf("read SSE stream: %w", err)}
			}
			s.done = true
			return StreamEvent{}, io.EOF
		}

		line := strings.TrimSuffix(s.scanner.Text(), "\r")
		if line == "" {
			s.flushEvent()
			if len(s.queue) > 0 {
				return s.pop(), nil
			}
			if s.done {
				return StreamEvent{}, io.EOF
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			s.pendingData = append(s.pendingData, data)
		}
	}
}

func (s *sseStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.done = true
	if s.body != nil {
		return s.body.Close()
	}
	return nil
}

func (s *sseStream) flushEvent() {
	if len(s.pendingData) == 0 {
		return
	}
	data := strings.Join(s.pendingData, "\n")
	s.pendingData = s.pendingData[:0]
	if strings.TrimSpace(data) == "[DONE]" {
		s.queue = append(s.queue, StreamEvent{Type: StreamEventDone})
		s.done = true
		return
	}

	var payload struct {
		ConversationID string      `json:"conversation_id"`
		Message        *webMessage `json:"message"`
		Error          interface{} `json:"error"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		s.queue = append(s.queue, StreamEvent{
			Type:  StreamEventDone,
			Delta: "",
			Message: nil,
		})
		s.done = true
		return
	}
	if payload.Error != nil {
		s.done = true
		return
	}

	if payload.ConversationID != "" && !s.seenConversation[payload.ConversationID] {
		s.seenConversation[payload.ConversationID] = true
		s.queue = append(s.queue, StreamEvent{
			Type:           StreamEventConversation,
			ConversationID: payload.ConversationID,
		})
	}
	if payload.Message == nil {
		return
	}

	role, ok := normalizeRole(payload.Message.Author.Role)
	if !ok {
		return
	}
	messageID := payload.Message.ID
	text := extractWebContent(payload.Message.Content)
	if role == RoleAssistant && text != "" {
		previous := s.lastText[messageID]
		delta := ""
		if strings.HasPrefix(text, previous) {
			delta = text[len(previous):]
		} else if text != previous {
			// Upstream occasionally rewrites an in-progress block. The stream
			// abstraction cannot retract already-emitted text, so expose the new
			// snapshot as a delta and rely on MessageCompleted as authoritative.
			delta = text
		}
		if delta != "" {
			s.queue = append(s.queue, StreamEvent{
				Type:           StreamEventMessageDelta,
				ConversationID: payload.ConversationID,
				MessageID:      messageID,
				Delta:          delta,
			})
		}
		s.lastText[messageID] = text
	}

	if isCompletedWebMessage(payload.Message) && !s.completed[messageID] {
		s.completed[messageID] = true
		message := &Message{
			ID:        messageID,
			Role:      role,
			Content:   text,
			CreatedAt: parseUpstreamTime(payload.Message.CreateTime),
		}
		s.queue = append(s.queue, StreamEvent{
			Type:           StreamEventMessageCompleted,
			ConversationID: payload.ConversationID,
			MessageID:      messageID,
			Message:        message,
		})
	}
}

func isCompletedWebMessage(message *webMessage) bool {
	if message == nil {
		return false
	}
	return message.EndTurn || strings.EqualFold(message.Status, "finished_successfully") || strings.EqualFold(message.Status, "finished")
}

func (s *sseStream) pop() StreamEvent {
	event := s.queue[0]
	s.queue = s.queue[1:]
	return event
}

var _ Stream = (*sseStream)(nil)
var _ = errors.Is
