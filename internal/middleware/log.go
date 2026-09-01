package middleware

import (
	"PandoraHelper/pkg/log"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/duke-git/lancet/v2/cryptor"
	"github.com/duke-git/lancet/v2/random"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"io"
	"strings"
	"time"
	"unicode"
)

const redactedLogValue = "[REDACTED]"

func RequestLogMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// The configuration is initialized once per request
		uuid, err := random.UUIdV4()
		if err != nil {
			return
		}
		trace := cryptor.Md5String(uuid)
		logger.WithValue(ctx, zap.String("trace", trace))
		logger.WithValue(ctx, zap.String("request_method", ctx.Request.Method))
		// Request headers are intentionally not logged because they can carry
		// Authorization/cookie credentials.
		logger.WithValue(ctx, zap.String("request_url", ctx.Request.URL.String()))
		if ctx.Request.Body != nil {
			bodyBytes, _ := ctx.GetRawData()
			ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			logger.WithValue(ctx, zap.String("request_params", RedactRequestBodyForLog(bodyBytes)))
		}
		logger.WithContext(ctx).Info("Request")
		ctx.Next()
	}
}

// RedactRequestBodyForLog returns a log-safe representation of a JSON request.
// Sensitive values are removed recursively. Non-JSON request bodies are never
// emitted verbatim because form/multipart payloads may also contain secrets.
func RedactRequestBodyForLog(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var value interface{}
	if err := json.Unmarshal(body, &value); err != nil {
		return fmt.Sprintf("<non-json body: %d bytes>", len(body))
	}
	redactJSONValue(value)
	redacted, err := json.Marshal(value)
	if err != nil {
		return "<json body redaction failed>"
	}
	return string(redacted)
}

func redactJSONValue(value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if isSensitiveLogKey(key) {
				typed[key] = redactedLogValue
				continue
			}
			redactJSONValue(child)
		}
	case []interface{}:
		for _, child := range typed {
			redactJSONValue(child)
		}
	}
}

func isSensitiveLogKey(key string) bool {
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, key)

	for _, marker := range []string{
		"password",
		"token",
		"secret",
		"sessionkey",
		"cookie",
		"credential",
		"authorization",
		"apikey",
		"proxyurl",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func ResponseLogMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: ctx.Writer}
		ctx.Writer = blw
		startTime := time.Now()
		ctx.Next()
		duration := time.Since(startTime).String()
		logger.WithContext(ctx).Info("Response", zap.Any("time", duration))
	}
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
