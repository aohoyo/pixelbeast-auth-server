// Package api 文件上传API
package api

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"license-server/internal/middleware"
	"license-server/internal/service"
)

// UploadHandler 上传处理器
type UploadHandler struct {
	uploadService *service.UploadService
}

// NewUploadHandler 创建上传处理器
func NewUploadHandler(uploadService *service.UploadService) *UploadHandler {
	return &UploadHandler{
		uploadService: uploadService,
	}
}

// RegisterRoutes 注册路由
func (h *UploadHandler) RegisterRoutes(r *gin.RouterGroup, jwtSecret string) {
	// 所有上传接口都需要认证
	auth := r.Group("")
	auth.Use(middleware.AuthMiddleware(jwtSecret))
	{
		upload := auth.Group("/upload")
		{
			upload.POST("/token", h.GetUploadToken)
			upload.POST("", h.UploadFile)
			upload.POST("/delete", h.DeleteFile)
		}
	}
}

// GetUploadTokenRequest 获取上传凭证请求
type GetUploadTokenRequest struct {
	Filename    string `json:"filename" binding:"required"`
	PathPrefix  string `json:"path_prefix"` // 路径前缀，如：software/icons
	StorageType string `json:"storage_type"` // user(用户配置) / system(系统配置)
}

// GetUploadToken 获取上传凭证
// @Summary 获取上传凭证
// @Description 获取直传到用户存储的凭证
// @Tags 上传
// @Accept json
// @Produce json
// @Param body body GetUploadTokenRequest true "上传参数"
// @Success 200 {object} Response{data=service.UploadToken}
// @Router /upload/token [post]
func (h *UploadHandler) GetUploadToken(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	var req GetUploadTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	// 默认使用用户存储
	if req.StorageType == "" {
		req.StorageType = "user"
	}

	// 生成存储路径
	key := generateStorageKey(req.PathPrefix, req.Filename)

	// 获取上传凭证
	token, err := h.uploadService.GetUploadToken(c.Request.Context(), tenantID, key, req.StorageType)
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
		"data": token,
	})
}

// UploadFile 上传文件
// @Summary 上传文件
// @Description 直传文件到用户配置的存储
// @Tags 上传
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "文件"
// @Param path_prefix formData string false "路径前缀"
// @Success 200 {object} Response{data=UploadResponse}
// @Router /upload [post]
func (h *UploadHandler) UploadFile(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "请上传文件",
		})
		return
	}
	defer file.Close()

	// 获取路径前缀
	pathPrefix := c.PostForm("path_prefix")
	if pathPrefix == "" {
		pathPrefix = "uploads"
	}

	// 生成存储路径
	key := generateStorageKey(pathPrefix, header.Filename)

	// 上传文件
	url, err := h.uploadService.UploadFile(c.Request.Context(), tenantID, key, file, header.Size, header.Header.Get("Content-Type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "上传成功",
		"data": gin.H{
			"url":      url,
			"filename": header.Filename,
			"size":     header.Size,
			"key":      key,
		},
	})
}

// DeleteFileRequest 删除文件请求
type DeleteFileRequest struct {
	URL string `json:"url" binding:"required"`
}

// DeleteFile 删除文件
// @Summary 删除文件
// @Description 从用户存储删除文件
// @Tags 上传
// @Accept json
// @Produce json
// @Param body body DeleteFileRequest true "文件URL"
// @Success 200 {object} Response
// @Router /upload/delete [post]
func (h *UploadHandler) DeleteFile(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	var req DeleteFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	// 删除文件
	if err := h.uploadService.DeleteFile(c.Request.Context(), tenantID, req.URL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "删除成功",
	})
}

// generateStorageKey 生成存储键名
// 格式：{path_prefix}/{timestamp}_{filename}
func generateStorageKey(pathPrefix, filename string) string {
	timestamp := time.Now().Unix()
	// 清理文件名，移除路径分隔符
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")
	// 添加时间戳避免重名
	name := fmt.Sprintf("%d_%s", timestamp, filename)
	return filepath.Join(pathPrefix, name)
}
