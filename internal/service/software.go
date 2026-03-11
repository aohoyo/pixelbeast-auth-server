// Package service 软件管理服务
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"

	"license-server/internal/model"
)

// SoftwareService 软件服务
type SoftwareService struct {
	db *gorm.DB
}

// NewSoftwareService 创建软件服务
func NewSoftwareService(db *gorm.DB) *SoftwareService {
	return &SoftwareService{db: db}
}

// CreateSoftwareRequest 创建软件请求
type CreateSoftwareRequest struct {
	TenantID    uint64 `json:"tenant_id"` // 多租户预留
	Name        string `json:"name" binding:"required"`
	Slug        string `json:"slug"`  // 可选，为空时自动生成
	APIKey      string `json:"api_key"` // 可选，为空时自动生成
	Description string `json:"description"`
	Platform    string `json:"platform" binding:"required"` // windows/macos/linux
}

// generateAPIKey 生成随机 API Key（32位）
func generateAPIKey() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// UpdateSoftwareRequest 更新软件请求
type UpdateSoftwareRequest struct {
	Name        string `json:"name"`
	APIKey      string `json:"api_key"`
	Description string `json:"description"`
	Platform    string `json:"platform"`
	Status      *int   `json:"status"`
}

// SoftwareResponse 软件响应
type SoftwareResponse struct {
	ID          uint64 `json:"id"`
	TenantID    uint64 `json:"tenant_id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Platform    string `json:"platform"`
	Status      int    `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Create 创建软件
func (s *SoftwareService) Create(ctx context.Context, req *CreateSoftwareRequest) (*model.Software, error) {
	// 自动生成 slug（如果为空）
	if req.Slug == "" {
		req.Slug = fmt.Sprintf("app_%d", time.Now().Unix())
	}

	// 检查slug是否已存在
	var existing model.Software
	if err := s.db.WithContext(ctx).Where("slug = ?", req.Slug).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("software with slug '%s' already exists", req.Slug)
	}

	// 如果前端传了 api_key，使用前端传入的，否则自动生成
	apiKey := req.APIKey
	if apiKey == "" {
		apiKey = generateAPIKey()
	}
	
	software := &model.Software{
		TenantID:    req.TenantID,
		Name:        req.Name,
		Slug:        req.Slug,
		APIKey:      apiKey,
		Description: req.Description,
		Platform:    req.Platform,
		Status:      1, // 默认启用
	}

	if err := s.db.WithContext(ctx).Create(software).Error; err != nil {
		return nil, fmt.Errorf("failed to create software: %w", err)
	}

	return software, nil
}

// GetByID 根据ID获取软件
func (s *SoftwareService) GetByID(ctx context.Context, id uint64) (*model.Software, error) {
	var software model.Software
	if err := s.db.WithContext(ctx).Preload("Versions").First(&software, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("software not found")
		}
		return nil, fmt.Errorf("failed to get software: %w", err)
	}
	return &software, nil
}

// GetBySlug 根据Slug获取软件
func (s *SoftwareService) GetBySlug(ctx context.Context, slug string) (*model.Software, error) {
	var software model.Software
	if err := s.db.WithContext(ctx).Where("slug = ?", slug).First(&software).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("software not found")
		}
		return nil, fmt.Errorf("failed to get software: %w", err)
	}
	return &software, nil
}

// List 获取软件列表
func (s *SoftwareService) List(ctx context.Context, tenantID uint64, page, pageSize int) ([]model.Software, int64, error) {
	var softwares []model.Software
	var total int64

	query := s.db.WithContext(ctx).Model(&model.Software{})

	// 多租户过滤
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count software: %w", err)
	}

	if err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&softwares).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list software: %w", err)
	}

	return softwares, total, nil
}

// Update 更新软件
func (s *SoftwareService) Update(ctx context.Context, id uint64, req *UpdateSoftwareRequest) (*model.Software, error) {
	software, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.APIKey != "" {
		updates["api_key"] = req.APIKey
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Platform != "" {
		updates["platform"] = req.Platform
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) == 0 {
		return software, nil
	}

	if err := s.db.WithContext(ctx).Model(software).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update software: %w", err)
	}

	return software, nil
}

// Delete 删除软件（软删除）
func (s *SoftwareService) Delete(ctx context.Context, id uint64) error {
	if err := s.db.WithContext(ctx).Delete(&model.Software{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete software: %w", err)
	}
	return nil
}

// GetLatestVersion 获取软件最新版本
func (s *SoftwareService) GetLatestVersion(ctx context.Context, softwareID uint64) (*model.Version, error) {
	var version model.Version
	if err := s.db.WithContext(ctx).
		Where("software_id = ? AND status = ?", softwareID, 1).
		Order("version_code DESC").
		First(&version).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no published version found")
		}
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}
	return &version, nil
}

// GetByAPIKey 根据 API Key 获取软件
func (s *SoftwareService) GetByAPIKey(ctx context.Context, apiKey string) (*model.Software, error) {
	var software model.Software
	if err := s.db.WithContext(ctx).Where("api_key = ?", apiKey).First(&software).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("software not found")
		}
		return nil, fmt.Errorf("failed to get software: %w", err)
	}
	return &software, nil
}

// UpdateAPIKey 更新软件的 API Key
func (s *SoftwareService) UpdateAPIKey(ctx context.Context, id uint64, apiKey string) error {
	if err := s.db.WithContext(ctx).Model(&model.Software{}).Where("id = ?", id).Update("api_key", apiKey).Error; err != nil {
		return fmt.Errorf("failed to update api key: %w", err)
	}
	return nil
}
