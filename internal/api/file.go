// Package api 文件管理API
package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"license-server/internal/middleware"
	"license-server/internal/model"
	"license-server/internal/service"
)

// FileHandler 文件处理器
type FileHandler struct {
	fileService *service.FileService
	db          *gorm.DB
}

// NewFileHandler 创建文件处理器
func NewFileHandler(fileService *service.FileService, db *gorm.DB) *FileHandler {
	return &FileHandler{
		fileService: fileService,
		db:          db,
	}
}

// RegisterRoutes 注册路由
func (h *FileHandler) RegisterRoutes(r *gin.RouterGroup, jwtSecret string) {
	auth := r.Group("/files")
	auth.Use(middleware.AuthMiddleware(jwtSecret))
	{
		auth.GET("", h.ListFiles)
		auth.GET("/stats", h.GetStorageStats)
		auth.POST("/folder", h.CreateFolder)
		auth.POST("", h.UploadFile)
		auth.POST("/record", h.CreateFileRecord)
		auth.POST("/move", h.MoveFiles)
		auth.POST("/copy", h.CopyFiles)
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
	
	// 搜索、排序、筛选参数
	search := c.Query("search")
	sort := c.Query("sort")     // name, size, updated_at
	order := c.Query("order")   // asc, desc
	fileType := c.Query("type") // image, video, audio, document, other

	// 默认排序
	if sort == "" {
		sort = "updated_at"
	}
	if order == "" {
		order = "desc"
	}

	files, err := h.fileService.ListFilesWithFilter(c.Request.Context(), tenantID, parentID, search, sort, order, fileType)
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

// CreateFileRecordRequest 创建文件记录请求
type CreateFileRecordRequest struct {
	Name     string `json:"name" binding:"required"`
	URL      string `json:"url" binding:"required"`
	Size     int64  `json:"size"`
	ParentID uint64 `json:"parent_id"`
	Source   string `json:"source"`   // 来源：software/version/file
	SourceID uint64 `json:"source_id"` // 关联的业务ID
}

// CreateFileRecord 创建文件记录（直传后调用）
func (h *FileHandler) CreateFileRecord(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	var req CreateFileRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	// 创建文件记录
	file := &model.File{
		TenantID: tenantID,
		ParentID: req.ParentID,
		Name:     req.Name,
		Type:     "file",
		FileType: model.GetFileTypeByExt(getFileExt(req.Name)),
		Size:     req.Size,
		URL:      req.URL,
		Path:     req.URL, // 使用URL作为路径
		Source:   req.Source,
		SourceID: req.SourceID,
	}

	if err := h.db.WithContext(c.Request.Context()).Create(file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "创建文件记录失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "创建成功",
		"data": file,
	})
}

// getFileExt 获取文件扩展名
func getFileExt(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
		return "." + parts[len(parts)-1]
	}
	return ""
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

// GetStorageStats 获取存储统计
func (h *FileHandler) GetStorageStats(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	stats, err := h.fileService.GetStorageStats(c.Request.Context(), tenantID)
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
		"data": stats,
	})
}

// MoveFilesRequest 移动文件请求
type MoveFilesRequest struct {
	FileIDs  []uint64 `json:"file_ids" binding:"required"`
	TargetID uint64   `json:"target_id"` // 目标文件夹ID，0表示根目录
}

// MoveFiles 移动文件
func (h *FileHandler) MoveFiles(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	var req MoveFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	if err := h.fileService.MoveFiles(c.Request.Context(), tenantID, req.FileIDs, req.TargetID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "移动成功",
	})
}

// CopyFilesRequest 复制文件请求
type CopyFilesRequest struct {
	FileIDs  []uint64 `json:"file_ids" binding:"required"`
	TargetID uint64   `json:"target_id"` // 目标文件夹ID，0表示根目录
}

// CopyFiles 复制文件
func (h *FileHandler) CopyFiles(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == 0 {
		tenantID = middleware.GetUserID(c)
	}

	var req CopyFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	if err := h.fileService.CopyFiles(c.Request.Context(), tenantID, req.FileIDs, req.TargetID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "复制成功",
	})
}
