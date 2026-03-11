// Package api 租户管理API
// 多租户预留接口（需要管理员权限）
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"license-server/internal/middleware"
	"license-server/internal/model"
)

// TenantHandler 租户处理器
type TenantHandler struct {
	db        *gorm.DB
	jwtSecret string
}

// NewTenantHandler 创建租户处理器
func NewTenantHandler(db *gorm.DB, jwtSecret string) *TenantHandler {
	return &TenantHandler{
		db:        db,
		jwtSecret: jwtSecret,
	}
}

// RegisterRoutes 注册路由
func (h *TenantHandler) RegisterRoutes(r *gin.RouterGroup) {
	// 需要管理员权限
	tenant := r.Group("/tenant")
	tenant.Use(middleware.AuthMiddleware(h.jwtSecret))
	tenant.Use(middleware.AdminMiddleware())
	{
		tenant.POST("", h.CreateTenant)
		tenant.GET("", h.ListTenants)
		tenant.GET("/:id", h.GetTenant)
		tenant.PUT("/:id", h.UpdateTenant)
		tenant.DELETE("/:id", h.DeleteTenant)
		tenant.POST("/:id/reset-key", h.ResetAPIKey)
	}
}

// CreateTenantRequest 创建租户请求
type CreateTenantRequest struct {
	Name string `json:"name" binding:"required"`
}

// UpdateTenantRequest 更新租户请求
type UpdateTenantRequest struct {
	Name   string `json:"name"`
	Status *int   `json:"status"`
}

// TenantResponse 租户响应
type TenantResponse struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	ApiKey    string `json:"api_key"`
	Status    int    `json:"status"`
	ExpiredAt string `json:"expired_at,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreateTenant 创建租户
// @Summary 创建租户
// @Description 创建新的租户（多租户预留）
// @Tags 租户管理
// @Accept json
// @Produce json
// @Param body body CreateTenantRequest true "租户信息"
// @Success 200 {object} Response{data=TenantResponse}
// @Router /tenant [post]
func (h *TenantHandler) CreateTenant(c *gin.Context) {
	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	tenant := &model.Tenant{
		Name:   req.Name,
		Status: 1,
	}

	if err := h.db.Create(tenant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": tenant,
	})
}

// ListTenants 获取租户列表
// @Summary 获取租户列表
// @Description 分页获取租户列表（多租户预留）
// @Tags 租户管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} Response{data=ListResponse}
// @Router /tenant [get]
func (h *TenantHandler) ListTenants(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	var tenants []model.Tenant
	var total int64

	if err := h.db.Model(&model.Tenant{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	if err := h.db.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&tenants).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"list":  tenants,
			"total": total,
			"page":  page,
		},
	})
}

// GetTenant 获取租户详情
// @Summary 获取租户详情
// @Description 根据ID获取租户详细信息（多租户预留）
// @Tags 租户管理
// @Produce json
// @Param id path int true "租户ID"
// @Success 200 {object} Response{data=model.Tenant}
// @Router /tenant/{id} [get]
func (h *TenantHandler) GetTenant(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	var tenant model.Tenant
	if err := h.db.First(&tenant, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "tenant not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": tenant})
}

// UpdateTenant 更新租户
// @Summary 更新租户
// @Description 更新租户信息（多租户预留）
// @Tags 租户管理
// @Accept json
// @Produce json
// @Param id path int true "租户ID"
// @Param body body UpdateTenantRequest true "租户信息"
// @Success 200 {object} Response{data=model.Tenant}
// @Router /tenant/{id} [put]
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	var req UpdateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "no fields to update"})
		return
	}

	if err := h.db.Model(&model.Tenant{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	var tenant model.Tenant
	h.db.First(&tenant, id)

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": tenant})
}

// DeleteTenant 删除租户
// @Summary 删除租户
// @Description 删除租户（多租户预留）
// @Tags 租户管理
// @Produce json
// @Param id path int true "租户ID"
// @Success 200 {object} Response
// @Router /tenant/{id} [delete]
func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	if err := h.db.Delete(&model.Tenant{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

// ResetAPIKey 重置API Key
// @Summary 重置API Key
// @Description 重置租户的API Key（多租户预留）
// @Tags 租户管理
// @Produce json
// @Param id path int true "租户ID"
// @Success 200 {object} Response{data=map[string]string}
// @Router /tenant/{id}/reset-key [post]
func (h *TenantHandler) ResetAPIKey(c *gin.Context) {
	_, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	// TODO: 生成新的API Key
	// newKey := generateAPIKey()
	// if err := h.db.Model(&model.Tenant{}).Where("id = ?", id).Update("api_key", newKey).Error; err != nil {
	//     c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
	//     return
	// }

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"api_key": "new-api-key-placeholder",
		},
	})
}