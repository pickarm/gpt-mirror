package handler_test

import (
	"PandoraHelper/internal/handler"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHealthAndReadinessEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := handler.NewHealthCheckHandler()
	router := gin.New()
	router.GET("/health", h.GetHealthCheck)
	router.GET("/readiness", h.ReadinessHandler)

	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "health", path: "/health", body: `{"status":"healthy"}`},
		{name: "readiness", path: "/readiness", body: `{"status":"ready"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.JSONEq(t, tt.body, recorder.Body.String())
		})
	}
}
