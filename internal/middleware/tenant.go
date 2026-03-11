// Package middleware 多租户中间件
// 预留多租户支持，用于租户隔离和权限控制
package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TenantContext 租户上下文键
type TenantContext struct {
	TenantID uint64
	ApiKey   string
	Status   int
}

// TenantMiddleware 多租户中间件
// 从请求中提取租户信息并注入到context
// 预留功能：支持API Key和JWT两种方式识别租户
func TenantMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 尝试从context获取租户ID（可能已由认证中间件设置）
		if tenantID, exists := c.Get("tenantID"); exists {
			c.Set("tenant_id", tenantID)
			c.Next()
			return
		}

		// 尝试从API Key解析租户
		apiKey, exists := c.Get("apiKey")
		if exists && apiKey != "" {
			// TODO: 查询数据库验证API Key并获取租户信息
			// 这里预留，后续实现租户查询逻辑
			// var tenant model.Tenant
			// if err := db.Where("api_key = ?", apiKey).First(&tenant).Error; err != nil {
			//     c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "invalid api key"})
			//     c.Abort()
			//     return
			// }
			// c.Set("tenant_id", tenant.ID)
			// c.Set("tenant_status", tenant.Status)
		}

		// 默认租户ID为0（系统级）
		c.Set("tenant_id", uint64(0))
		c.Next()
	}
}

// RequireTenant 要求必须有租户信息
// 用于需要租户隔离的接口
func RequireTenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, exists := c.Get("tenant_id")
		if !exists || tenantID == uint64(0) {
			c.JSON(http.StatusForbidden, gin.H{
				"code": 403,
				"msg":  "tenant information required",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// TenantScope 租户作用域
// 用于GORM查询时自动添加租户过滤条件
func TenantScope(tenantID uint64) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if tenantID > 0 {
			return db.Where("tenant_id = ?", tenantID)
		}
		return db
	}
}

// WithTenant 包装函数，自动注入租户ID到context
func WithTenant(c *gin.Context, tenantID uint64) {
	c.Set("tenant_id", tenantID)
}

// TenantStatsMiddleware 租户用量统计中间件
// 预留功能：记录API调用次数
func TenantStatsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录开始时间
		c.Set("request_start", time.Now())

		c.Next()

		// TODO: 异步记录用量统计
		// tenantID := GetTenantID(c)
		// if tenantID > 0 {
		//     // 记录API调用
		//     go recordAPICall(tenantID, c.Request.URL.Path, time.Since(start))
		// }
	}
}
