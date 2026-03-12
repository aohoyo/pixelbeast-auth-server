// Package service 文件管理服务
package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"license-server/internal/model"
)

// FileService 文件服务
type FileService struct {
	db             *gorm.DB
	storageService *StorageService
}

// NewFileService 创建文件服务
func NewFileService(db *gorm.DB, storageService *StorageService) *FileService {
	return &FileService{
		db:             db,
		storageService: storageService,
	}
}

// CreateFolderRequest 创建文件夹请求
type CreateFolderRequest struct {
	Name     string `json:"name" binding:"required"`
	ParentID uint64 `json:"parent_id"`
}

// FileResponse 文件响应
type FileResponse struct {
	ID            uint64 `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	FileType      string `json:"file_type"`
	Size          int64  `json:"size"`
	URL           string `json:"url"`
	ChildrenCount int64  `json:"childrenCount,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// ListFiles 获取文件列表
func (s *FileService) ListFiles(ctx context.Context, tenantID uint64, parentID uint64) ([]FileResponse, error) {
	var files []model.File
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND parent_id = ?", tenantID, parentID).
		Order("type DESC, name ASC").
		Find(&files).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	result := make([]FileResponse, 0, len(files))
	for _, file := range files {
		resp := FileResponse{
			ID:        file.ID,
			Name:      file.Name,
			Type:      file.Type,
			FileType:  file.FileType,
			Size:      file.Size,
			URL:       file.URL,
			CreatedAt: file.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: file.UpdatedAt.Format("2006-01-02 15:04:05"),
		}

		// 如果是文件夹，统计子文件数量
		if file.IsFolder() {
			var count int64
			s.db.WithContext(ctx).Model(&model.File{}).
				Where("parent_id = ?", file.ID).
				Count(&count)
			resp.ChildrenCount = count
		}

		result = append(result, resp)
	}

	return result, nil
}

// CreateFolder 创建文件夹
func (s *FileService) CreateFolder(ctx context.Context, tenantID uint64, req *CreateFolderRequest) (*model.File, error) {
	// 检查同目录下是否有同名文件夹
	var existing model.File
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND parent_id = ? AND name = ? AND type = ?",
			tenantID, req.ParentID, req.Name, "folder").
		First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("folder already exists")
	}

	folder := &model.File{
		TenantID: tenantID,
		ParentID: req.ParentID,
		Name:     req.Name,
		Type:     "folder",
		FileType: "folder",
	}

	if err := s.db.WithContext(ctx).Create(folder).Error; err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	return folder, nil
}

// UploadFile 上传文件
func (s *FileService) UploadFile(ctx context.Context, tenantID uint64, parentID uint64, filename string, reader io.Reader, size int64) (*model.File, error) {
	// 获取存储提供者
	provider, err := s.storageService.GetStorageProvider(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(filename))
	fileType := model.GetFileTypeByExt(ext)

	// 检查同目录下是否有同名文件 - 如果存在则删除旧文件（覆盖逻辑）
	var existing model.File
	err = s.db.WithContext(ctx).
		Where("tenant_id = ? AND parent_id = ? AND name = ? AND type = ?",
			tenantID, parentID, filename, "file").
		First(&existing).Error
	if err == nil {
		// 删除旧文件记录和存储
		_ = provider.Delete(ctx, existing.Path)
		s.db.WithContext(ctx).Delete(&existing)
	}

	// 生成存储路径（保持原文件名）
	storageKey := fmt.Sprintf("files/%d/%s", tenantID, filename)

	// 上传文件
	contentType := getContentTypeByExt(ext)
	if err := provider.Upload(ctx, storageKey, reader, size, contentType); err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	// 获取文件URL
	url := provider.GetPublicURL(storageKey)

	// 创建文件记录
	file := &model.File{
		TenantID: tenantID,
		ParentID: parentID,
		Name:     filename,
		Type:     "file",
		FileType: fileType,
		Size:     size,
		Path:     storageKey,
		URL:      url,
	}

	if err := s.db.WithContext(ctx).Create(file).Error; err != nil {
		// 删除已上传的文件
		_ = provider.Delete(ctx, storageKey)
		return nil, fmt.Errorf("failed to create file record: %w", err)
	}

	return file, nil
}

// DeleteFile 删除文件/文件夹
func (s *FileService) DeleteFile(ctx context.Context, tenantID uint64, fileID uint64) error {
	var file model.File
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", fileID, tenantID).
		First(&file).Error; err != nil {
		return fmt.Errorf("file not found")
	}

	// 如果是文件夹，递归删除子文件
	if file.IsFolder() {
		var children []model.File
		s.db.WithContext(ctx).Where("parent_id = ?", fileID).Find(&children)
		for _, child := range children {
			if err := s.DeleteFile(ctx, tenantID, child.ID); err != nil {
				return err
			}
		}
	} else {
		// 删除存储中的文件
		provider, err := s.storageService.GetStorageProvider(ctx, tenantID)
		if err == nil {
			_ = provider.Delete(ctx, file.Path)
		}
	}

	// 删除数据库记录
	return s.db.WithContext(ctx).Delete(&file).Error
}

// BatchDeleteFiles 批量删除文件
func (s *FileService) BatchDeleteFiles(ctx context.Context, tenantID uint64, fileIDs []uint64) error {
	for _, id := range fileIDs {
		if err := s.DeleteFile(ctx, tenantID, id); err != nil {
			return err
		}
	}
	return nil
}

// RenameFile 重命名文件
func (s *FileService) RenameFile(ctx context.Context, tenantID uint64, fileID uint64, newName string) error {
	var file model.File
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", fileID, tenantID).
		First(&file).Error; err != nil {
		return fmt.Errorf("file not found")
	}

	// 检查同目录下是否有同名文件
	var existing model.File
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND parent_id = ? AND name = ? AND id != ?",
			tenantID, file.ParentID, newName, fileID).
		First(&existing).Error
	if err == nil {
		return fmt.Errorf("file with same name already exists")
	}

	file.Name = newName
	return s.db.WithContext(ctx).Save(&file).Error
}

// GetDownloadURL 获取下载链接
func (s *FileService) GetDownloadURL(ctx context.Context, tenantID uint64, fileID uint64) (string, error) {
	var file model.File
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", fileID, tenantID).
		First(&file).Error; err != nil {
		return "", fmt.Errorf("file not found")
	}

	if file.IsFolder() {
		return "", fmt.Errorf("cannot download folder")
	}

	// 直接使用公开URL（存储桶是公开的）
	return file.URL, nil
}

// getContentTypeByExt 根据扩展名获取Content-Type
func getContentTypeByExt(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}
