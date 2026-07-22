package handler

import "github.com/gin-gonic/gin"

// UsageInteractionCaptureMiddleware keeps interaction capture at the gateway
// boundary so it can observe the exact bytes written to the client.
func (h *GatewayHandler) UsageInteractionCaptureMiddleware() gin.HandlerFunc {
	if h == nil || h.gatewayService == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return h.gatewayService.UsageInteractionCaptureMiddleware()
}
