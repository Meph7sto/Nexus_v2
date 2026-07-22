package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func allowAllAdminPermissionsForRouteTest() middleware.AdminPermissionMiddleware {
	return func(service.AdminPermissionResource, service.AdminPermissionAction) gin.HandlerFunc {
		return func(c *gin.Context) { c.Next() }
	}
}
