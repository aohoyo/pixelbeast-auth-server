// Package api 文件管理API
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"license-server/internal/middleware"
	"license-server/internal/service"
)

// FileHandler 文件处理器
type FileHandler struct {
	fileService *service.FileService
}

// NewFileHandler 创建文件处理器
func NewFileHandler(fileService *service.FileService) *FileHandler {
	return &FileHandler{
		fileService: fileService,
	}
}

// RegisterRoutes 注册路由
func (h *FileHandler) RegisterRoutes(r *gin.RouterGroup, jwtSecret string) {
	auth := r.Group("/files")
	auth.Use(middleware.AuthMiddleware(jwtSecret))
	{
		auth.GET("", h.ListFiles)
		auth.POST("/folder", h.CreateFolder)
		auth.POST("", h.UploadFile)
		auth.DELETE("/:id", h.DeleteFile)
		auth.DELETE("/batch", h.BatchDelete)
		auth.PUT("/:id/rename", h.RenameFile)
		auth.GET("/:id/download", h.GetDownloadURL)
	}
}

// ListFiles 获取文件列表
func (h *FileHandler) ListFiles(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	parentID, _ := strconv.ParseUint(c.Query("parent_id"), 10, 64)

	files, err := h.fileService.ListFiles(c.Request.Context(), tenantID, parentID)
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
		"data": files,
	})
}

// CreateFolder 创建文件夹
func (h *FileHandler) CreateFolder(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	var req service.CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	folder, err := h.fileService.CreateFolder(c.Request.Context(), tenantID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "创建成功",
		"data": folder,
	})
}

// UploadFile 上传文件
func (h *FileHandler) UploadFile(c *gin.Context) {
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

	parentID, _ := strconv.ParseUint(c.PostForm("parent_id"), 10, 64)

	// 上传文件
	result, err := h.fileService.UploadFile(c.Request.Context(), tenantID, parentID, header.Filename, file, header.Size)
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
			"id":   result.ID,
			"name": result.Name,
			"url":  result.URL,
			"size": result.Size,
		},
	})
}

// DeleteFile 删除文件
func (h *FileHandler) DeleteFile(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	fileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "invalid file id",
		})
		return
	}

	if err := h.fileService.DeleteFile(c.Request.Context(), tenantID, fileID); err != nil {
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

// BatchDelete 批量删除
func (h *FileHandler) BatchDelete(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	var req struct {
		IDs []uint64 `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	if err := h.fileService.BatchDeleteFiles(c.Request.Context(), tenantID, req.IDs); err != nil {
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

// RenameFile 重命名文件
func (h *FileHandler) RenameFile(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	fileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "invalid file id",
		})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	if err := h.fileService.RenameFile(c.Request.Context(), tenantID, fileID, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "重命名成功",
	})
}

// GetDownloadURL 获取下载链接
func (h *FileHandler) GetDownloadURL(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	fileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "invalid file id",
		})
		return
	}

	url, err := h.fileService.GetDownloadURL(c.Request.Context(), tenantID, fileID)
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
		"data": gin.H{
			"url": url,
		},
	})
}
