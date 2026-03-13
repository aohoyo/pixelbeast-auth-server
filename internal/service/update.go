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
	Channel       string `json:"channel"`
	ChangeLog     string `json:"changelog"`
	IsForced      bool   `json:"is_forced"`
	IsIncremental bool   `json:"is_incremental"`
	MinVersion    string `json:"min_version"`
}

// UpdateVersionRequest 更新版本请求
type UpdateVersionRequest struct {
	ChangeLog     string `json:"changelog"`
	Channel       string `json:"channel"`
	IsForced      *bool  `json:"is_forced"`
	IsIncremental *bool  `json:"is_incremental"`
	MinVersion    string `json:"min_version"`
	Status        *int   `json:"status"`
	
	// 灰度发布配置
	GrayEnabled   *bool  `json:"gray_enabled"`
	GrayPercent   *int   `json:"gray_percent"`
	GrayStatus    *int   `json:"gray_status"`
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
	HasUpdate       bool   `json:"has_update"`
	Version         string `json:"version,omitempty"`
	VersionCode     int    `json:"version_code,omitempty"`
	ChangeLog       string `json:"changelog,omitempty"`
	DownloadURL     string `json:"download_url,omitempty"`
	PackageSize     int64  `json:"package_size,omitempty"`
	PackageHash     string `json:"package_hash,omitempty"`
	PackageHashAlgo string `json:"package_hash_algo,omitempty"`
	IsForced        bool   `json:"is_forced,omitempty"`
	IsIncremental   bool   `json:"is_incremental,omitempty"`
	MinVersion      string `json:"min_version,omitempty"`
	PublishedAt     string `json:"published_at,omitempty"`
}

// VersionResponse 版本响应
type VersionResponse struct {
	ID              uint64 `json:"id"`
	TenantID        uint64 `json:"tenant_id"`
	SoftwareID      uint64 `json:"software_id"`
	Version         string `json:"version"`
	VersionCode     int    `json:"version_code"`
	ChangeLog       string `json:"changelog"`
	PackageURL      string `json:"package_url"`
	PackageSize     int64  `json:"package_size"`
	PackageHash     string `json:"package_hash"`
	PackageHashAlgo string `json:"package_hash_algo"`
	IsForced        bool   `json:"is_forced"`
	IsIncremental   bool   `json:"is_incremental"`
	MinVersion      string `json:"min_version"`
	Status          int    `json:"status"`
	PublishedAt     string `json:"published_at"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
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
		Channel:       req.Channel,
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
	if req.Channel != "" {
		updates["channel"] = req.Channel
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
	
	// 灰度配置更新
	if req.GrayEnabled != nil {
		updates["gray_enabled"] = *req.GrayEnabled
	}
	if req.GrayPercent != nil {
		if *req.GrayPercent < 0 {
			*req.GrayPercent = 0
		}
		if *req.GrayPercent > 100 {
			*req.GrayPercent = 100
		}
		updates["gray_percent"] = *req.GrayPercent
	}
	if req.GrayStatus != nil {
		updates["gray_status"] = *req.GrayStatus
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
		"package_url":       packageURL,
		"package_size":      size,
		"package_hash":      packageHash,
		"package_hash_algo": "sha256",
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

	// 灰度发布检查
	if latestVersion.GrayEnabled && latestVersion.GrayPercent > 0 {
		// 灰度进行中才参与灰度判断
		if latestVersion.GrayStatus == 1 {
			// 根据 DeviceID 决定是否返回灰度版本
			// 使用 DeviceID 的 hash 值取模来判断
			if req.DeviceID != "" {
				hash := sha256.Sum256([]byte(req.DeviceID))
				hashInt := int(hash[0])<<24 | int(hash[1])<<16 | int(hash[2])<<8 | int(hash[3])
				percent := hashInt % 100
				
				// 如果不在灰度范围内，查找上一个稳定版本
				if percent >= latestVersion.GrayPercent {
					var stableVersion model.Version
					if err := s.db.WithContext(ctx).
						Where("software_id = ? AND status = ? AND version_code > ? AND (gray_enabled = ? OR gray_status = ?)", 
							software.ID, 1, currentVersionCode, false, 2).
						Order("version_code DESC").
						First(&stableVersion).Error; err == nil {
						latestVersion = stableVersion
					} else {
						// 没有稳定版本，不返回更新
						return &CheckUpdateResponse{HasUpdate: false}, nil
					}
				}
			}
		} else if latestVersion.GrayStatus == 0 || latestVersion.GrayStatus == 3 {
			// 灰度未开始或已暂停，不返回灰度版本
			var stableVersion model.Version
			if err := s.db.WithContext(ctx).
				Where("software_id = ? AND status = ? AND version_code > ? AND (gray_enabled = ? OR gray_status = ?)", 
					software.ID, 1, currentVersionCode, false, 2).
				Order("version_code DESC").
				First(&stableVersion).Error; err == nil {
				latestVersion = stableVersion
			} else {
				return &CheckUpdateResponse{HasUpdate: false}, nil
			}
		}
		// GrayStatus == 2 已完成，正常返回灰度版本
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
		HasUpdate:       true,
		Version:         latestVersion.Version,
		VersionCode:     latestVersion.VersionCode,
		ChangeLog:       latestVersion.ChangeLog,
		DownloadURL:     latestVersion.PackageURL,
		PackageSize:     latestVersion.PackageSize,
		PackageHash:     latestVersion.PackageHash,
		PackageHashAlgo: latestVersion.PackageHashAlgo,
		IsForced:        latestVersion.IsForced,
		IsIncremental:   latestVersion.IsIncremental,
		MinVersion:      latestVersion.MinVersion,
		PublishedAt:     latestVersion.PublishedAt.Format("2006-01-02T00:00:00Z"),
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

// ========== 增量更新相关 ==========

// FileInfo 文件信息
type FileInfo struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	Hash        string `json:"hash"`
	HashAlgo    string `json:"hash_algo"`
	DownloadURL string `json:"download_url"`
}

// DeltaCheckRequest 增量检查请求
type DeltaCheckRequest struct {
	SoftwareSlug   string            `json:"software_slug" binding:"required"`
	CurrentVersion string            `json:"current_version" binding:"required"`
	LocalFiles     map[string]string `json:"local_files"` // path -> hash
}

// DeltaCheckResponse 增量检查响应
type DeltaCheckResponse struct {
	HasUpdate    bool       `json:"has_update"`
	Version      string     `json:"version,omitempty"`
	ChangedFiles []FileInfo `json:"changed_files,omitempty"`
	NewFiles     []FileInfo `json:"new_files,omitempty"`
	DeletedFiles []string   `json:"deleted_files,omitempty"`
	DeltaSize    int64      `json:"delta_size,omitempty"`
	FullSize     int64      `json:"full_size,omitempty"`
	FullURL      string     `json:"full_url,omitempty"`
	FullHash     string     `json:"full_hash,omitempty"`
}

// CheckDeltaUpdate 检查增量更新
func (s *UpdateService) CheckDeltaUpdate(ctx context.Context, req *DeltaCheckRequest) (*DeltaCheckResponse, error) {
	// 获取软件信息
	var software model.Software
	if err := s.db.WithContext(ctx).Where("slug = ?", req.SoftwareSlug).First(&software).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("software not found")
		}
		return nil, fmt.Errorf("failed to get software: %w", err)
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
			return &DeltaCheckResponse{HasUpdate: false}, nil
		}
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}

	// 获取最新版本的文件清单
	var versionFiles []model.VersionFile
	if err := s.db.WithContext(ctx).
		Where("version_id = ?", latestVersion.ID).
		Find(&versionFiles).Error; err != nil {
		return nil, fmt.Errorf("failed to get version files: %w", err)
	}

	// 如果没有文件清单，返回全量更新信息
	if len(versionFiles) == 0 {
		return &DeltaCheckResponse{
			HasUpdate: true,
			Version:   latestVersion.Version,
			FullSize:  latestVersion.PackageSize,
			FullURL:   latestVersion.PackageURL,
			FullHash:  latestVersion.PackageHash,
		}, nil
	}

	// 计算增量
	response := &DeltaCheckResponse{
		HasUpdate:    true,
		Version:      latestVersion.Version,
		ChangedFiles: []FileInfo{},
		NewFiles:     []FileInfo{},
		DeletedFiles: []string{},
		DeltaSize:    0,
		FullSize:     latestVersion.PackageSize,
		FullURL:      latestVersion.PackageURL,
		FullHash:     latestVersion.PackageHash,
	}

	// 新版本文件映射
	newFileMap := make(map[string]model.VersionFile)
	for _, f := range versionFiles {
		newFileMap[f.Path] = f
	}

	// 找出变化和新增的文件
	for path, newFile := range newFileMap {
		localHash, exists := req.LocalFiles[path]

		fileInfo := FileInfo{
			Path:        newFile.Path,
			Size:        newFile.Size,
			Hash:        newFile.Hash,
			HashAlgo:    newFile.HashAlgo,
			DownloadURL: newFile.DownloadURL,
		}

		if !exists {
			// 新增文件
			response.NewFiles = append(response.NewFiles, fileInfo)
			response.DeltaSize += newFile.Size
		} else if localHash != newFile.Hash {
			// 文件变化
			response.ChangedFiles = append(response.ChangedFiles, fileInfo)
			response.DeltaSize += newFile.Size
		}
	}

	// 找出删除的文件
	for path := range req.LocalFiles {
		if _, exists := newFileMap[path]; !exists {
			response.DeletedFiles = append(response.DeletedFiles, path)
		}
	}

	return response, nil
}

// GetVersionFiles 获取版本文件清单
func (s *UpdateService) GetVersionFiles(ctx context.Context, versionID uint64) ([]model.VersionFile, error) {
	var files []model.VersionFile
	if err := s.db.WithContext(ctx).
		Where("version_id = ?", versionID).
		Order("path ASC").
		Find(&files).Error; err != nil {
		return nil, fmt.Errorf("failed to get version files: %w", err)
	}
	return files, nil
}

// SaveVersionFile 保存版本文件
func (s *UpdateService) SaveVersionFile(ctx context.Context, file *model.VersionFile) error {
	if err := s.db.WithContext(ctx).Create(file).Error; err != nil {
		return fmt.Errorf("failed to save version file: %w", err)
	}
	return nil
}

// DeleteVersionFiles 删除版本文件清单
func (s *UpdateService) DeleteVersionFiles(ctx context.Context, versionID uint64) error {
	if err := s.db.WithContext(ctx).
		Where("version_id = ?", versionID).
		Delete(&model.VersionFile{}).Error; err != nil {
		return fmt.Errorf("failed to delete version files: %w", err)
	}
	return nil
}
