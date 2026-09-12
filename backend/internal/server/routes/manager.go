package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterManagerRoutes 注册部门经理路由（普通 JWT + ManagerOnly，独立于 /admin）。
func RegisterManagerRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	managerOnly middleware.ManagerOnlyMiddleware,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	manager := v1.Group("/manager")
	manager.Use(gin.HandlerFunc(jwtAuth))
	manager.Use(gin.HandlerFunc(managerOnly))
	manager.Use(panelRateLimiter.Global())
	{
		manager.GET("/members", h.Manager.Members)
		manager.GET("/subscriptions/:id/progress", h.Manager.SubscriptionProgress)
		manager.POST("/subscriptions/:id/reset-quota", h.Manager.ResetQuota)
	}
}
