package middleware

import (
	"net/http"
	"strings"

	"maoyan-service/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// AuthRequired JWT 鉴权中间件：
//   从 Authorization header 提取 Bearer Token → 调用 AuthService.ValidateToken
//   → 将解析出的 userID 注入 gin.Context（c.Set("user_id", ...)）
//   → 后续 handler 通过 c.GetString("user_id") 获取
//   验证失败直接返回 401，不进入 handler
func AuthRequired(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"code": 401, "msg": "未登录"})
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		userID, err := auth.ValidateToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"code": 401, "msg": "token无效或已过期"})
			return
		}

		c.Set("user_id", userID.String())
		c.Next()
	}
}
