package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ManagerOnly 部门经理权限中间件：允许 manager 角色访问（admin 自行使用 AdminOnly 路径）。
// 必须在 JWTAuth 中间件之后使用。
func ManagerOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := GetUserRoleFromContext(c)
		if !ok {
			AbortWithError(c, 401, "UNAUTHORIZED", "User not found in context")
			return
		}
		if role != service.RoleManager {
			AbortWithError(c, 403, "FORBIDDEN", "Department manager access required")
			return
		}
		c.Next()
	}
}
