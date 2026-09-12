package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

// JWTAuthMiddleware JWT 认证中间件类型
type JWTAuthMiddleware gin.HandlerFunc

// OptionalJWTAuthMiddleware 可选 JWT 认证中间件类型：匿名放行，带 token 严格校验
type OptionalJWTAuthMiddleware gin.HandlerFunc

// AdminAuthMiddleware 管理员认证中间件类型
type AdminAuthMiddleware gin.HandlerFunc

// ManagerOnlyMiddleware 部门经理角色中间件类型（挂在 JWTAuth 之后）
type ManagerOnlyMiddleware gin.HandlerFunc

// APIKeyAuthMiddleware API Key 认证中间件类型
type APIKeyAuthMiddleware gin.HandlerFunc

// ProviderSet 中间件层的依赖注入
var ProviderSet = wire.NewSet(
	NewJWTAuthMiddleware,
	NewOptionalJWTAuthMiddleware,
	NewAdminAuthMiddleware,
	NewManagerOnlyMiddleware,
	NewAPIKeyAuthMiddleware,
	NewAuditLogMiddleware,
	NewStepUpAuthMiddleware,
)

// NewManagerOnlyMiddleware 创建部门经理角色中间件
func NewManagerOnlyMiddleware() ManagerOnlyMiddleware {
	return ManagerOnlyMiddleware(ManagerOnly())
}
