package service

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const usageInteractionResponseCaptureContextKey = "usage_interaction_response_capture"

// UsageInteractionCaptureMiddleware only retains bytes while interaction
// recording is enabled. It wraps the response written to the client, rather
// than relying on an upstream buffer that a protocol adapter may discard.
func (s *UsageInteractionService) UsageInteractionCaptureMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c == nil {
			return
		}
		if c.Request == nil || c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		enabled, err := s.RecordingEnabled(c.Request.Context())
		if err == nil && enabled {
			beginUsageInteractionResponseCapture(c)
		}
		c.Next()
	}
}

// UsageInteractionCaptureMiddleware installs the response tee used by every
// HTTP gateway protocol. The service is injected after construction so direct
// service tests keep their existing constructor shape.
func (s *GatewayService) UsageInteractionCaptureMiddleware() gin.HandlerFunc {
	if s == nil || s.usageInteractionService == nil {
		return passthroughUsageInteractionMiddleware
	}
	return s.usageInteractionService.UsageInteractionCaptureMiddleware()
}

func passthroughUsageInteractionMiddleware(c *gin.Context) {
	if c == nil {
		return
	}
	c.Next()
}

type usageInteractionResponseCapture struct {
	gin.ResponseWriter
	body      []byte
	truncated bool
}

func beginUsageInteractionResponseCapture(c *gin.Context) *usageInteractionResponseCapture {
	if c == nil {
		return nil
	}
	if existing, ok := c.Get(usageInteractionResponseCaptureContextKey); ok {
		if capture, ok := existing.(*usageInteractionResponseCapture); ok {
			return capture
		}
	}
	capture := &usageInteractionResponseCapture{ResponseWriter: c.Writer}
	c.Writer = capture
	c.Set(usageInteractionResponseCaptureContextKey, capture)
	return capture
}

func (w *usageInteractionResponseCapture) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if n > 0 {
		if n > len(data) {
			n = len(data)
		}
		w.capture(data[:n])
	}
	return n, err
}

func (w *usageInteractionResponseCapture) WriteString(value string) (int, error) {
	n, err := w.ResponseWriter.WriteString(value)
	if n > 0 {
		if n > len(value) {
			n = len(value)
		}
		w.capture([]byte(value[:n]))
	}
	return n, err
}

func (w *usageInteractionResponseCapture) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *usageInteractionResponseCapture) capture(data []byte) {
	if w == nil || len(data) == 0 {
		return
	}
	remaining := maxUsageInteractionPayloadBytes - len(w.body)
	if remaining <= 0 {
		w.truncated = true
		return
	}
	if len(data) > remaining {
		w.body = append(w.body, data[:remaining]...)
		w.truncated = true
		return
	}
	w.body = append(w.body, data...)
}

func (w *usageInteractionResponseCapture) snapshot() ([]byte, bool) {
	if w == nil {
		return nil, false
	}
	return append([]byte(nil), w.body...), w.truncated
}

// BuildUsageInteractionCaptureFromContext records the exact downstream body.
// This fixes the source implementation's missing-output defect: forwarding
// success no longer depends on a separate ForwardResult buffer being filled.
func BuildUsageInteractionCaptureFromContext(c *gin.Context, requestBody []byte, requestParameters map[string]any) *UsageInteractionCapture {
	if c == nil {
		return nil
	}
	value, ok := c.Get(usageInteractionResponseCaptureContextKey)
	if !ok {
		return nil
	}
	capture, ok := value.(*usageInteractionResponseCapture)
	if !ok || capture == nil {
		return nil
	}
	responseBody, responseTruncated := capture.snapshot()
	return buildUsageInteractionCapture(requestBody, responseBody, false, responseTruncated, requestParameters)
}
