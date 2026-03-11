// Package api 升级服务API
// 包含检查更新、下载版本包等客户端接口
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"license-server/internal/middleware"
	"license-server/internal/service"
)

// UpdateHandler 升级服务处理器
type UpdateHandler struct {
	updateService   *service.UpdateService
	usageService    *service.UsageService
	softwareService *service.SoftwareService
}

// NewUpdateHandler 创建升级服务处理器
func NewUpdateHandler(updateService *service.UpdateService, usageService *service.UsageService, softwareService *service.SoftwareService) *UpdateHandler {
	return &UpdateHandler{
		updateService:   updateService,
		usageService:    usageService,
		softwareService: softwareService,
	}
}

// RegisterRoutes 注册路由
func (h *UpdateHandler) RegisterRoutes(r *gin.RouterGroup) {
	update := r.Group("/update")
	{
		// 检查更新 - API Key 验证在 handler 内部完成
		update.GET("/check", h.CheckUpdate)
		update.POST("/check", h.CheckUpdate)

		// 下载版本包 - 需要API Key
		update.GET("/download/:id", middleware.APIKeyAuth(), h.DownloadPackage)
	}
}

// CheckUpdate 检查更新
// @Summary 检查更新
// @Description 客户端检查是否有新版本可用
// @Tags 升级服务
// @Accept json
// @Produce json
// @Param software_slug query string true "软件标识"
// @Param current_version query string true "当前版本号"
// @Param platform query string false "平台"
// @Param device_id query string false "设备ID"
// @Param api_key query string false "API密钥"
// @Success 200 {object} Response{data=service.CheckUpdateResponse}
// @Failure 400 {object} Response
// @Router /update/check [get]
func (h *UpdateHandler) CheckUpdate(c *gin.Context) {
	// 从 Header 或 Query 获取 API Key
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		apiKey = c.Query("api_key")
	}

	// 验证 API Key
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "missing api key"})
		return
	}

	// 验证 API Key 是否有效
	software, err := h.softwareService.GetByAPIKey(c.Request.Context(), apiKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "invalid api key"})
		return
	}

	var req service.CheckUpdateRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	// 优先从JSON body获取参数
	if c.Request.Method == "POST" {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
	}

	// 使用 API Key 关联的软件标识
	if req.SoftwareSlug == "" {
		req.SoftwareSlug = software.Slug
	}

	resp, err := h.updateService.CheckUpdate(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": resp})
}

// DownloadPackage 下载版本包
// @Summary 下载版本包
// @Description 下载指定版本的安装包
// @Tags 升级服务
// @Produce octet-stream
// @Param id path int true "版本ID"
// @Param api_key query string false "API密钥"
// @Success 200 {file} binary
// @Failure 404 {object} Response
// @Router /update/download/{id} [get]
func (h *UpdateHandler) DownloadPackage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	// 获取版本信息
	version, err := h.updateService.GetVersionByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	}

	// 记录下载日志（异步）
	go func() {
		_ = h.usageService.RecordDownload(c.Request.Context(), &service.RecordDownloadRequest{
			VersionID: id,
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			DeviceID:  c.Query("device_id"),
		})
	}()

	// 重定向到下载URL
	c.Redirect(http.StatusFound, version.PackageURL)
}

// DownloadPackageDirect 直接下载版本包（流式传输）
// @Summary 直接下载版本包
// @Description 通过服务器中转下载安装包
// @Tags 升级服务
// @Produce octet-stream
// @Param id path int true "版本ID"
// @Param api_key query string false "API密钥"
// @Success 200 {file} binary
// @Failure 404 {object} Response
// @Router /update/download/{id}/direct [get]
func (h *UpdateHandler) DownloadPackageDirect(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid id"})
		return
	}

	// 获取版本信息
	version, err := h.updateService.GetVersionByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	}

	// 设置响应头
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=\""+version.Version+".pkg\"")
	c.Header("Content-Length", strconv.FormatInt(version.PackageSize, 10))

	// TODO: 从存储服务获取文件流并返回
	// 这里需要根据存储类型实现不同的下载逻辑

	// 记录下载日志
	go func() {
		_ = h.usageService.RecordDownload(c.Request.Context(), &service.RecordDownloadRequest{
			VersionID: id,
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			DeviceID:  c.Query("device_id"),
		})
	}()

	c.Status(http.StatusOK)
}

// GetLatestVersion 获取最新版本信息
// @Summary 获取最新版本
// @Description 获取软件的最新发布版本信息
// @Tags 升级服务
// @Produce json
// @Param software_slug query string true "软件标识"
// @Success 200 {object} Response{data=model.Version}
// @Router /update/latest [get]
func (h *UpdateHandler) GetLatestVersion(c *gin.Context) {
	slug := c.Query("software_slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "software_slug required"})
		return
	}

	// TODO: 实现获取最新版本逻辑
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": nil})
}
