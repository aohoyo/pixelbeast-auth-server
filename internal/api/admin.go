// Package api 管理后台API
// 包含软件管理、版本管理、用量统计等接口
package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"license-server/internal/middleware"
	"license-server/internal/service"
)

// AdminHandler 管理后台处理器
type AdminHandler struct {
	softwareService *service.SoftwareService
	updateService   *service.UpdateService
	usageService    *service.UsageService
	jwtSecret       string
}

// NewAdminHandler 创建管理后台处理器
func NewAdminHandler(
	softwareService *service.SoftwareService,
	updateService *service.UpdateService,
	usageService *service.UsageService,
	jwtSecret string,
) *AdminHandler {
	return &AdminHandler{
		softwareService: softwareService,
		updateService:   updateService,
		usageService:    usageService,
		jwtSecret:       jwtSecret,
	}
}

// RegisterRoutes 注册路由
// 新路由结构：
// - /software/* - 软件管理
// - /version/* - 版本管理
// - /stats/* - 统计
func (h *AdminHandler) RegisterRoutes(r *gin.RouterGroup) {
	// 公开接口：登录
	r.POST("/admin/login", h.Login)

	// 所有管理接口都需要认证
	auth := r.Group("")
	auth.Use(middleware.AuthMiddleware(h.jwtSecret))
	{
		// 软件管理
		software := auth.Group("/software")
		{
			software.POST("", h.CreateSoftware)
			software.GET("", h.ListSoftware)
			software.GET("/:id", h.GetSoftware)
			software.PUT("/:id", h.UpdateSoftware)
			software.DELETE("/:id", h.DeleteSoftware)
			software.POST("/:id/reset-key", h.ResetAPIKey)
		}

		// 版本管理
		version := auth.Group("/version")
		{
			version.POST("", h.CreateVersion)
			version.GET("", h.ListVersions)
			version.GET("/:id", h.GetVersion)
			version.PUT("/:id", h.UpdateVersion)
			version.DELETE("/:id", h.DeleteVersion)
			version.POST("/:id/publish", h.PublishVersion)
			version.POST("/:id/upload", h.UploadPackage)
		}

		// 统计
		stats := auth.Group("/stats")
		{
			stats.GET("/downloads", h.GetDownloadStats)
			stats.GET("/software/:id/downloads", h.GetSoftwareDownloads)
		}
	}
}

// CreateSoftware 创建软件
// @Summary 创建软件
// @Description 创建新的软件项目
// @Tags 软件管理
// @Accept json
// @Produce json
// @Param body body service.CreateSoftwareRequest true "软件信息"
// @Success 200 {object} Response{data=model.Software}
// @Router /software [post]
func (h *AdminHandler) CreateSoftware(c *gin.Context) {
	var req service.CreateSoftwareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	// 获取租户ID（多租户预留）
	req.TenantID = middleware.GetTenantID(c)

	software, err := h.softwareService.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": software})
}

// ListSoftware 获取软件列表
// @Summary 获取软件列表
// @Description 分页获取软件列表
// @Tags 软件管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} Response{data=ListResponse}
// @Router /software [get]
func (h *AdminHandler) ListSoftware(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	tenantID := middleware.GetTenantID(c)

	softwares, total, err := h.softwareService.List(c.Request.Context(), tenantID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"list":  softwares,
			"total": total,
			"page":  page,
		},
	})
}

// GetSoftware 获取软件详情
// @Summary 获取软件详情
// @Description 根据ID获取软件详细信息
// @Tags 软件管理
// @Produce json
// @Param id path int true "软件ID"
// @Success 200 {object} Response{data=model.Software}
// @Router /software/{id} [get]
func (h *AdminHandler) GetSoftware(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	software, err := h.softwareService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": software})
}

// UpdateSoftware 更新软件
// @Summary 更新软件
// @Description 更新软件信息
// @Tags 软件管理
// @Accept json
// @Produce json
// @Param id path int true "软件ID"
// @Param body body service.UpdateSoftwareRequest true "软件信息"
// @Success 200 {object} Response{data=model.Software}
// @Router /software/{id} [put]
func (h *AdminHandler) UpdateSoftware(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	var req service.UpdateSoftwareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	software, err := h.softwareService.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": software})
}

// DeleteSoftware 删除软件
// @Summary 删除软件
// @Description 软删除软件
// @Tags 软件管理
// @Produce json
// @Param id path int true "软件ID"
// @Success 200 {object} Response
// @Router /software/{id} [delete]
func (h *AdminHandler) DeleteSoftware(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	if err := h.softwareService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

// CreateVersion 创建版本
// @Summary 创建版本
// @Description 创建新的软件版本
// @Tags 版本管理
// @Accept json
// @Produce json
// @Param body body service.CreateVersionRequest true "版本信息"
// @Success 200 {object} Response{data=model.Version}
// @Router /version [post]
func (h *AdminHandler) CreateVersion(c *gin.Context) {
	var req service.CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	// 获取租户ID（多租户预留）
	req.TenantID = middleware.GetTenantID(c)

	version, err := h.updateService.CreateVersion(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": version})
}

// ListVersions 获取版本列表
// @Summary 获取版本列表
// @Description 分页获取版本列表
// @Tags 版本管理
// @Produce json
// @Param software_id query int false "软件ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} Response{data=ListResponse}
// @Router /version [get]
func (h *AdminHandler) ListVersions(c *gin.Context) {
	softwareID, _ := strconv.ParseUint(c.Query("software_id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	tenantID := middleware.GetTenantID(c)

	versions, total, err := h.updateService.ListVersions(c.Request.Context(), softwareID, tenantID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"list":  versions,
			"total": total,
			"page":  page,
		},
	})
}

// GetVersion 获取版本详情
// @Summary 获取版本详情
// @Description 根据ID获取版本详细信息
// @Tags 版本管理
// @Produce json
// @Param id path int true "版本ID"
// @Success 200 {object} Response{data=model.Version}
// @Router /version/{id} [get]
func (h *AdminHandler) GetVersion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	version, err := h.updateService.GetVersionByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": version})
}

// UpdateVersion 更新版本
// @Summary 更新版本
// @Description 更新版本信息
// @Tags 版本管理
// @Accept json
// @Produce json
// @Param id path int true "版本ID"
// @Param body body service.UpdateVersionRequest true "版本信息"
// @Success 200 {object} Response{data=model.Version}
// @Router /version/{id} [put]
func (h *AdminHandler) UpdateVersion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	var req service.UpdateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	version, err := h.updateService.UpdateVersion(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": version})
}

// DeleteVersion 删除版本
// @Summary 删除版本
// @Description 删除版本及其包文件
// @Tags 版本管理
// @Produce json
// @Param id path int true "版本ID"
// @Success 200 {object} Response
// @Router /version/{id} [delete]
func (h *AdminHandler) DeleteVersion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	if err := h.updateService.DeleteVersion(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

// PublishVersion 发布版本
// @Summary 发布版本
// @Description 将版本状态改为已发布
// @Tags 版本管理
// @Produce json
// @Param id path int true "版本ID"
// @Success 200 {object} Response{data=model.Version}
// @Router /version/{id}/publish [post]
func (h *AdminHandler) PublishVersion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	req := service.UpdateVersionRequest{
		Status: func() *int { i := 1; return &i }(),
	}

	version, err := h.updateService.UpdateVersion(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": version})
}

// UploadPackage 上传版本包
// @Summary 上传版本包
// @Description 上传版本安装包
// @Tags 版本管理
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "版本ID"
// @Param file formData file true "安装包文件"
// @Success 200 {object} Response{data=model.Version}
// @Router /version/{id}/upload [post]
func (h *AdminHandler) UploadPackage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "file required"})
		return
	}
	defer file.Close()

	if err := h.updateService.UploadPackage(c.Request.Context(), id, file, header.Size, header.Filename, header.Header.Get("Content-Type")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	version, err := h.updateService.GetVersionByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": version})
}

// GetDownloadStats 获取下载统计
// @Summary 获取下载统计
// @Description 获取下载统计数据
// @Tags 用量统计
// @Produce json
// @Param start_date query string false "开始日期"
// @Param end_date query string false "结束日期"
// @Success 200 {object} Response{data=[]service.StatsResponse}
// @Router /stats/downloads [get]
func (h *AdminHandler) GetDownloadStats(c *gin.Context) {
	// TODO: 实现统计查询
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []interface{}{}})
}

// GetSoftwareDownloads 获取软件下载统计
// @Summary 获取软件下载统计
// @Description 获取指定软件的下载统计
// @Tags 用量统计
// @Produce json
// @Param id path int true "软件ID"
// @Success 200 {object} Response{data=int64}
// @Router /stats/software/{id}/downloads [get]
func (h *AdminHandler) GetSoftwareDownloads(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	count, err := h.usageService.GetTotalDownloads(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": count})
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

// Login 管理员登录
// @Summary 管理员登录
// @Description 使用用户名密码登录获取JWT令牌
// @Tags 认证
// @Accept json
// @Produce json
// @Param body body LoginRequest true "登录信息"
// @Success 200 {object} Response{data=LoginResponse}
// @Router /admin/login [post]
func (h *AdminHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	// TODO: 实现真正的用户验证
	// 这里使用硬编码的测试账户
	if req.Username != "admin" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "用户名不存在"})
		return
	}
	if req.Password != "admin123" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "密码错误"})
		return
	}

	// 生成JWT令牌
	token, err := middleware.GenerateToken(1, req.Username, "admin", &middleware.JWTConfig{
		Secret:      h.jwtSecret,
		ExpireHours: 24,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": LoginResponse{
			Token:     token,
			ExpiresIn: 86400,
		},
	})
}

// ResetAPIKey 重置软件 API Key
// @Summary 重置API Key
// @Description 生成新的 API Key 并更新
// @Tags 软件管理
// @Produce json
// @Param id path int true "软件ID"
// @Success 200 {object} Response{data=map[string]string}
// @Router /software/{id}/reset-key [post]
func (h *AdminHandler) ResetAPIKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	// 生成新的 API Key（调用 service 中的方法）
	newKey := generateAPIKey()

	// 更新数据库
	if err := h.softwareService.UpdateAPIKey(c.Request.Context(), id, newKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": gin.H{"api_key": newKey}})
}

// generateAPIKey 生成随机 API Key（32位）
func generateAPIKey() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Response 统一响应结构
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// ListResponse 列表响应结构
type ListResponse struct {
	List  interface{} `json:"list"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
}