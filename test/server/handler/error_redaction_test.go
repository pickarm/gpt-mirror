package handler_test

import (
	v1 "PandoraHelper/api/v1"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleErrorRedactsSecretsAndUsesInternalCodeForUnmappedError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	v1.HandleError(ctx, http.StatusInternalServerError, errors.New(
		"Authorization=Bearer response-secret password=hunter2 proxy=socks5://alice:proxy-pass@127.0.0.1:1080",
	), nil)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("HTTP status = %d", recorder.Code)
	}
	for _, forbidden := range []string{"response-secret", "hunter2", "alice", "proxy-pass"} {
		if strings.Contains(recorder.Body.String(), forbidden) {
			t.Fatalf("public error leaked %q: %s", forbidden, recorder.Body.String())
		}
	}
	var response v1.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != 500 {
		t.Fatalf("response status code = %d, want 500", response.Code)
	}
}
