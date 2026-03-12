// Package service 升级服务
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"license-server/internal/model"
	"license-server/internal/storage"
)

// UpdateService 升级服务
type UpdateService struct {
	db              *gorm.DB
	storageService  *StorageService
}

// NewUpdateService 创建升级服务
func NewUpdateService(db *gorm.DB, storageService *StorageService) *UpdateService {
	return &UpdateService{
		db:             db,
		storageService: storageService,
	}
}

// CreateVersionRequest 创建版本请求
type CreateVersionRequest struct {
	TenantID      uint64 `json:"tenant_id"`
	SoftwareID    uint64 `json:"software_id" binding:"required"`
	Version       string `json:"version" binding:"required"`
	ChangeLog     string `json:"changelog"`
	IsForced      bool   `json:"is_forced"`
	IsIncremental bool   `json:"is_incremental"`
	MinVersion    string `json:"min_version"`
}

// UpdateVersionRequest 更新版本请求
type UpdateVersionRequest struct {
	ChangeLog     string `json:"changelog"`
	IsForced      *bool  `json:"is_forced"`
	IsIncremental *bool  `json:"is_incremental"`
	MinVersion    string `json:"min_version"`
	Status        *int   `json:"status"`
}

// CheckUpdateRequest 检查更新请求
type CheckUpdateRequest struct {
	SoftwareSlug string `json:"software_slug" form:"software_slug" binding:"required"`
	CurrentVersion string `json:"current_version" form:"current_version" binding:"required"`
	Platform     string `json:"platform" form:"platform"`
	DeviceID     string `json:"device_id" form:"device_id"`
}

// CheckUpdateResponse 检查更新响应
type CheckUpdateResponse struct {
	HasUpdate     bool   `json:"has_update"`
	Version       string `json:"version,omitempty"`
	VersionCode   int    `json:"version_code,omitempty"`
	ChangeLog     string `json:"changelog,omitempty"`
	DownloadURL   string `json:"download_url,omitempty"`
	PackageSize   int64  `json:"package_size,omitempty"`
	PackageHash   string `json:"package_hash,omitempty"`
	IsForced      bool   `json:"is_forced,omitempty"`
	IsIncremental bool   `json:"is_incremental,omitempty"`
	MinVersion    string `json:"min_version,omitempty"`
	PublishedAt   string `json:"published_at,omitempty"`
}

// VersionResponse 版本响应
type VersionResponse struct {
	ID            uint64 `json:"id"`
	TenantID      uint64 `json:"tenant_id"`
	SoftwareID    uint64 `json:"software_id"`
	Version       string `json:"version"`
	VersionCode   int    `json:"version_code"`
	ChangeLog     string `json:"changelog"`
	PackageURL    string `json:"package_url"`
	PackageSize   int64  `json:"package_size"`
	PackageHash   string `json:"package_hash"`
	IsForced      bool   `json:"is_forced"`
	IsIncremental bool   `json:"is_incremental"`
	MinVersion    string `json:"min_version"`
	Status        int    `json:"status"`
	PublishedAt   string `json:"published_at"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// CreateVersion 创建版本
func (s *UpdateService) CreateVersion(ctx context.Context, req *CreateVersionRequest) (*model.Version, error) {
	// 检查软件是否存在
	var software model.Software
	if err := s.db.WithContext(ctx).First(&software, req.SoftwareID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("software not found")
		}
		return nil, fmt.Errorf("failed to get software: %w", err)
	}

	// 解析版本号
	versionCode, err := parseVersionCode(req.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid version format: %w", err)
	}

	// 检查版本是否已存在
	var existing model.Version
	if err := s.db.WithContext(ctx).
		Where("software_id = ? AND version = ?", req.SoftwareID, req.Version).
		First(&existing).Error; err == nil {
		return nil, fmt.Errorf("version '%s' already exists", req.Version)
	}

	version := &model.Version{
		TenantID:      req.TenantID,
		SoftwareID:    req.SoftwareID,
		Version:       req.Version,
		VersionCode:   versionCode,
		ChangeLog:     req.ChangeLog,
		IsForced:      req.IsForced,
		IsIncremental: req.IsIncremental,
		MinVersion:    req.MinVersion,
		Status:        0, // 草稿状态
	}

	if err := s.db.WithContext(ctx).Create(version).Error; err != nil {
		return nil, fmt.Errorf("failed to create version: %w", err)
	}

	return version, nil
}

// GetVersionByID 根据ID获取版本
func (s *UpdateService) GetVersionByID(ctx context.Context, id uint64) (*model.Version, error) {
	var version model.Version
	if err := s.db.WithContext(ctx).Preload("Software").First(&version, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("version not found")
		}
		return nil, fmt.Errorf("failed to get version: %w", err)
	}
	return &version, nil
}

// ListVersions 获取版本列表
func (s *UpdateService) ListVersions(ctx context.Context, softwareID uint64, tenantID uint64, page, pageSize int) ([]model.Version, int64, error) {
	var versions []model.Version
	var total int64

	query := s.db.WithContext(ctx).Model(&model.Version{})

	if softwareID > 0 {
		query = query.Where("software_id = ?", softwareID)
	}
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count versions: %w", err)
	}

	if err := query.Order("version_code DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&versions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list versions: %w", err)
	}

	return versions, total, nil
}

// UpdateVersion 更新版本
func (s *UpdateService) UpdateVersion(ctx context.Context, id uint64, req *UpdateVersionRequest) (*model.Version, error) {
	version, err := s.GetVersionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})

	if req.ChangeLog != "" {
		updates["changelog"] = req.ChangeLog
	}
	if req.IsForced != nil {
		updates["is_forced"] = *req.IsForced
	}
	if req.IsIncremental != nil {
		updates["is_incremental"] = *req.IsIncremental
	}
	if req.MinVersion != "" {
		updates["min_version"] = req.MinVersion
	}
	if req.Status != nil {
		updates["status"] = *req.Status
		if *req.Status == 1 {
			updates["published_at"] = gorm.Expr("NOW()")
		}
	}

	if len(updates) == 0 {
		return version, nil
	}

	if err := s.db.WithContext(ctx).Model(version).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update version: %w", err)
	}

	return version, nil
}

// DeleteVersion 删除版本
func (s *UpdateService) DeleteVersion(ctx context.Context, id uint64) error {
	// 获取版本信息
	version, err := s.GetVersionByID(ctx, id)
	if err != nil {
		return err
	}

	// 如果已上传包，删除存储
	if version.PackageURL != "" {
		// 获取租户的存储提供者
		provider, err := s.storageService.GetStorageProvider(ctx, version.TenantID)
		if err == nil {
			key := storage.GenerateKey(version.SoftwareID, version.Version, "package")
			_ = provider.Delete(ctx, key)
		}
	}

	if err := s.db.WithContext(ctx).Delete(&model.Version{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete version: %w", err)
	}

	return nil
}

// UploadPackage 上传版本包
func (s *UpdateService) UploadPackage(ctx context.Context, versionID uint64, reader io.Reader, size int64, filename string, contentType string) error {
	version, err := s.GetVersionByID(ctx, versionID)
	if err != nil {
		return err
	}

	// 获取租户的存储提供者
	provider, err := s.storageService.GetStorageProvider(ctx, version.TenantID)
	if err != nil {
		return fmt.Errorf("storage not configured: %w", err)
	}

	// 生成存储key
	key := storage.GenerateKey(version.SoftwareID, version.Version, filename)

	// 计算哈希
	hash := sha256.New()
	teeReader := io.TeeReader(reader, hash)

	// 上传到存储
	if err := provider.Upload(ctx, key, teeReader, size, contentType); err != nil {
		return fmt.Errorf("failed to upload package: %w", err)
	}

	packageHash := hex.EncodeToString(hash.Sum(nil))
	packageURL := provider.GetPublicURL(key)

	// 更新版本信息
	updates := map[string]interface{}{
		"package_url":  packageURL,
		"package_size": size,
		"package_hash": packageHash,
	}

	if err := s.db.WithContext(ctx).Model(version).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update version package info: %w", err)
	}

	return nil
}

// CheckUpdate 检查更新
func (s *UpdateService) CheckUpdate(ctx context.Context, req *CheckUpdateRequest) (*CheckUpdateResponse, error) {
	// 获取软件信息
	var software model.Software
	if err := s.db.WithContext(ctx).Where("slug = ?", req.SoftwareSlug).First(&software).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("software not found")
		}
		return nil, fmt.Errorf("failed to get software: %w", err)
	}

	// 检查软件状态
	if software.Status != 1 {
		return nil, fmt.Errorf("software is disabled")
	}

	// 解析当前版本
	currentVersionCode, err := parseVersionCode(req.CurrentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version: %w", err)
	}

	// 获取最新发布版本
	var latestVersion model.Version
	if err := s.db.WithContext(ctx).
		Where("software_id = ? AND status = ? AND version_code > ?", software.ID, 1, currentVersionCode).
		Order("version_code DESC").
		First(&latestVersion).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 没有更新
			return &CheckUpdateResponse{HasUpdate: false}, nil
		}
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}

	// 检查增量更新条件
	if latestVersion.IsIncremental && latestVersion.MinVersion != "" {
		minVersionCode, _ := parseVersionCode(latestVersion.MinVersion)
		if currentVersionCode < minVersionCode {
			// 当前版本低于最低支持版本，需要全量更新
			// 查找全量包
			var fullVersion model.Version
			if err := s.db.WithContext(ctx).
				Where("software_id = ? AND status = ? AND is_incremental = ? AND version_code >= ?",
					software.ID, 1, false, latestVersion.VersionCode).
				Order("version_code DESC").
				First(&fullVersion).Error; err == nil {
				latestVersion = fullVersion
			}
		}
	}

	return &CheckUpdateResponse{
		HasUpdate:     true,
		Version:       latestVersion.Version,
		VersionCode:   latestVersion.VersionCode,
		ChangeLog:     latestVersion.ChangeLog,
		DownloadURL:   latestVersion.PackageURL,
		PackageSize:   latestVersion.PackageSize,
		PackageHash:   latestVersion.PackageHash,
		IsForced:      latestVersion.IsForced,
		IsIncremental: latestVersion.IsIncremental,
		MinVersion:    latestVersion.MinVersion,
		PublishedAt:   latestVersion.PublishedAt.Format("2006-01-02T00:00:00Z"),
	}, nil
}

// GetDownloadURL 获取下载URL
func (s *UpdateService) GetDownloadURL(ctx context.Context, versionID uint64) (string, error) {
	version, err := s.GetVersionByID(ctx, versionID)
	if err != nil {
		return "", err
	}

	if version.PackageURL == "" {
		return "", fmt.Errorf("package not uploaded")
	}

	return version.PackageURL, nil
}

// parseVersionCode 解析版本号为数字
// 支持格式: 1.2.3 -> 10203, 1.2.3.4 -> 1020304
func parseVersionCode(version string) (int, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return 0, fmt.Errorf("version must have 2-4 parts")
	}

	code := 0
	multiplier := 1000000

	for i, part := range parts {
		if i >= 3 {
			break
		}
		num, err := strconv.Atoi(part)
		if err != nil {
			return 0, fmt.Errorf("invalid version part: %s", part)
		}
		if num > 999 {
			return 0, fmt.Errorf("version part too large: %d", num)
		}
		code += num * multiplier
		multiplier /= 1000
	}

	return code, nil
}
