package handler

import (
	v1 "PandoraHelper/api/v1"
	chatgptprovider "PandoraHelper/internal/provider/chatgpt"
	appsecurity "PandoraHelper/internal/security"
	"PandoraHelper/internal/service"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type ConversationHandler struct {
	*Handler
	conversationService service.ConversationService
}

func NewConversationHandler(handler *Handler, conversationService service.ConversationService) *ConversationHandler {
	return &ConversationHandler{Handler: handler, conversationService: conversationService}
}

func (h *ConversationHandler) Health(ctx *gin.Context) {
	req := new(v1.ChatGPTAccountRequest)
	if !bindChatGPTRequest(ctx, req) {
		return
	}
	status, err := h.conversationService.Health(ctx.Request.Context(), req.AccountID)
	if err != nil {
		handleChatGPTError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, v1.NewChatGPTAccountStatus(status))
}

func (h *ConversationHandler) Models(ctx *gin.Context) {
	req := new(v1.ChatGPTAccountRequest)
	if !bindChatGPTRequest(ctx, req) {
		return
	}
	models, err := h.conversationService.Models(ctx.Request.Context(), req.AccountID)
	if err != nil {
		handleChatGPTError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, v1.NewChatGPTModels(models))
}

func (h *ConversationHandler) List(ctx *gin.Context) {
	req := new(v1.ChatGPTConversationListRequest)
	if !bindChatGPTRequest(ctx, req) {
		return
	}
	page, err := h.conversationService.List(ctx.Request.Context(), req.AccountID, chatgptprovider.PageRequest{Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		handleChatGPTError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, v1.NewChatGPTConversationPage(page))
}

func (h *ConversationHandler) Get(ctx *gin.Context) {
	req := new(v1.ChatGPTConversationRequest)
	if !bindChatGPTRequest(ctx, req) {
		return
	}
	conversation, err := h.conversationService.Get(ctx.Request.Context(), req.AccountID, req.ConversationID)
	if err != nil {
		handleChatGPTError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, v1.NewChatGPTConversation(conversation))
}

func (h *ConversationHandler) Create(ctx *gin.Context) {
	req := new(v1.ChatGPTSendRequest)
	if !bindChatGPTRequest(ctx, req) {
		return
	}
	stream, err := h.conversationService.Create(ctx.Request.Context(), req.AccountID, sendMessageRequest(req))
	if err != nil {
		handleChatGPTError(ctx, err)
		return
	}
	h.writeStream(ctx, stream)
}

func (h *ConversationHandler) Continue(ctx *gin.Context) {
	req := new(v1.ChatGPTSendRequest)
	if !bindChatGPTRequest(ctx, req) {
		return
	}
	if strings.TrimSpace(req.ConversationID) == "" {
		v1.HandleError(ctx, http.StatusBadRequest, errors.New("conversationId is required"), nil)
		return
	}
	stream, err := h.conversationService.Continue(ctx.Request.Context(), req.AccountID, req.ConversationID, sendMessageRequest(req))
	if err != nil {
		handleChatGPTError(ctx, err)
		return
	}
	h.writeStream(ctx, stream)
}

func (h *ConversationHandler) Rename(ctx *gin.Context) {
	req := new(v1.ChatGPTRenameRequest)
	if !bindChatGPTRequest(ctx, req) {
		return
	}
	if err := h.conversationService.Rename(ctx.Request.Context(), req.AccountID, req.ConversationID, req.Title); err != nil {
		handleChatGPTError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

func (h *ConversationHandler) Archive(ctx *gin.Context) {
	req := new(v1.ChatGPTArchiveRequest)
	if !bindChatGPTRequest(ctx, req) {
		return
	}
	if err := h.conversationService.Archive(ctx.Request.Context(), req.AccountID, req.ConversationID, req.Archived); err != nil {
		handleChatGPTError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

func (h *ConversationHandler) Delete(ctx *gin.Context) {
	req := new(v1.ChatGPTConversationRequest)
	if !bindChatGPTRequest(ctx, req) {
		return
	}
	if err := h.conversationService.Delete(ctx.Request.Context(), req.AccountID, req.ConversationID); err != nil {
		handleChatGPTError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

func (h *ConversationHandler) writeStream(ctx *gin.Context, stream chatgptprovider.Stream) {
	defer stream.Close()
	ctx.Header("Content-Type", "text/event-stream; charset=utf-8")
	ctx.Header("Cache-Control", "no-cache, no-transform")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")
	ctx.Status(http.StatusOK)
	ctx.Writer.Flush()

	for {
		event, err := stream.Recv(ctx.Request.Context())
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, ctx.Request.Context().Err()) {
				return
			}
			writeSSE(ctx, "error", map[string]string{"message": appsecurity.RedactText(err.Error())})
			return
		}
		if err := writeSSE(ctx, "message", v1.NewChatGPTStreamEvent(event)); err != nil {
			return
		}
		if event.Type == chatgptprovider.StreamEventDone {
			return
		}
	}
}

func writeSSE(ctx *gin.Context, eventName string, payload interface{}) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(ctx.Writer, "event: %s\ndata: %s\n\n", eventName, encoded); err != nil {
		return err
	}
	ctx.Writer.Flush()
	return nil
}

func sendMessageRequest(req *v1.ChatGPTSendRequest) chatgptprovider.SendMessageRequest {
	return chatgptprovider.SendMessageRequest{
		Model:           req.Model,
		ParentMessageID: req.ParentMessageID,
		Message: chatgptprovider.InputMessage{
			Role:    chatgptprovider.RoleUser,
			Content: req.Message,
		},
		Temporary: req.Temporary,
	}
}

func bindChatGPTRequest(ctx *gin.Context, req interface{}) bool {
	if err := ctx.ShouldBindJSON(req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return false
	}
	return true
}

func handleChatGPTError(ctx *gin.Context, err error) {
	status := http.StatusInternalServerError
	var providerErr *chatgptprovider.Error
	if errors.As(err, &providerErr) {
		switch providerErr.Kind {
		case chatgptprovider.ErrorKindInvalidRequest:
			status = http.StatusBadRequest
		case chatgptprovider.ErrorKindAuth:
			if providerErr.StatusCode == http.StatusForbidden {
				status = http.StatusForbidden
			} else {
				status = http.StatusUnauthorized
			}
		case chatgptprovider.ErrorKindNotFound:
			status = http.StatusNotFound
		case chatgptprovider.ErrorKindRateLimit:
			status = http.StatusTooManyRequests
		case chatgptprovider.ErrorKindUnavailable:
			status = http.StatusServiceUnavailable
		case chatgptprovider.ErrorKindTransport, chatgptprovider.ErrorKindProtocol:
			status = http.StatusBadGateway
		}
	}
	v1.HandleError(ctx, status, err, nil)
}
