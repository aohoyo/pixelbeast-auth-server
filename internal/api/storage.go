// Package api 存储配置API
// 纯平台模式：每个租户必须自配存储
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"license-server/internal/middleware"
	"license-server/internal/service"
)

// StorageHandler 存储配置处理器
type StorageHandler struct {
	storageService *service.StorageService
}

// NewStorageHandler 创建存储配置处理器
func NewStorageHandler(storageService *service.StorageService) *StorageHandler {
	return &StorageHandler{
		storageService: storageService,
	}
}

// RegisterRoutes 注册路由
func (h *StorageHandler) RegisterRoutes(r *gin.RouterGroup, jwtSecret string) {
	// 所有存储配置接口都需要认证
	auth := r.Group("")
	auth.Use(middleware.AuthMiddleware(jwtSecret))
	{
		storage := auth.Group("/storage")
		{
			storage.GET("/types", h.GetStorageTypes)
			storage.GET("/config", h.GetStorageConfig)
			storage.POST("/config", h.SaveStorageConfig)
			storage.POST("/test", h.TestStorageConfig)
		}
	}
}

// GetStorageTypes 获取支持的存储类型
// @Summary 获取支持的存储类型
// @Description 返回所有支持的存储类型及其配置字段
// @Tags 存储配置
// @Produce json
// @Success 200 {object} Response{data=[]service.StorageTypeInfo}
// @Router /storage/types [get]
func (h *StorageHandler) GetStorageTypes(c *gin.Context) {
	types := service.GetStorageTypes()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": types,
	})
}

// GetStorageConfig 获取存储配置
// @Summary 获取当前租户的存储配置
// @Description 返回存储配置状态（不返回敏感信息）
// @Tags 存储配置
// @Produce json
// @Success 200 {object} Response{data=service.StorageConfigResponse}
// @Router /storage/config [get]
func (h *StorageHandler) GetStorageConfig(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		// 如果没有租户ID，使用用户ID作为租户ID（单租户模式）
		tenantID = middleware.GetUserID(c)
	}

	config, err := h.storageService.GetStorageConfig(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": config,
	})
}

// SaveStorageConfig 保存存储配置
// @Summary 保存存储配置
// @Description 保存租户的存储配置（SecretKey加密存储）
// @Tags 存储配置
// @Accept json
// @Produce json
// @Param body body service.StorageConfigRequest true "存储配置"
// @Success 200 {object} Response
// @Router /storage/config [post]
func (h *StorageHandler) SaveStorageConfig(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		// 如果没有租户ID，使用用户ID作为租户ID（单租户模式）
		tenantID = middleware.GetUserID(c)
	}

	var req service.StorageConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	// 验证必填字段
	if req.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "storage type is required",
		})
		return
	}
	if req.AccessKey == "" || req.SecretKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "access_key and secret_key are required",
		})
		return
	}
	if req.Bucket == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "bucket is required",
		})
		return
	}

	// 保存配置
	if err := h.storageService.SaveStorageConfig(c.Request.Context(), tenantID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "保存成功",
	})
}

// TestStorageConfig 测试存储配置
// @Summary 测试存储配置
// @Description 测试存储配置是否可用
// @Tags 存储配置
// @Accept json
// @Produce json
// @Param body body service.StorageConfigRequest true "存储配置"
// @Success 200 {object} Response
// @Router /storage/test [post]
func (h *StorageHandler) TestStorageConfig(c *gin.Context) {
	var req service.StorageConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	// 测试配置
	if err := h.storageService.TestStorageConfig(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "存储测试失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "连接成功",
	})
}