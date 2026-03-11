// Package service 用量统计服务
// 预留功能：下载统计、用量分析
package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"license-server/internal/model"
)

// UsageService 用量统计服务
type UsageService struct {
	db *gorm.DB
}

// NewUsageService 创建用量统计服务
func NewUsageService(db *gorm.DB) *UsageService {
	return &UsageService{db: db}
}

// RecordDownloadRequest 记录下载请求
type RecordDownloadRequest struct {
	TenantID  uint64
	VersionID uint64
	ClientIP  string
	UserAgent string
	DeviceID  string
}

// StatsQueryRequest 统计查询请求
type StatsQueryRequest struct {
	TenantID   uint64
	SoftwareID uint64
	VersionID  uint64
	StartDate  time.Time
	EndDate    time.Time
}

// StatsResponse 统计响应
type StatsResponse struct {
	Date          string `json:"date"`
	DownloadCount int64  `json:"download_count"`
	UniqueDevices int64  `json:"unique_devices"`
}

// RecordDownload 记录下载日志
// 用于用量统计预留功能
func (s *UsageService) RecordDownload(ctx context.Context, req *RecordDownloadRequest) error {
	log := &model.DownloadLog{
		TenantID:     req.TenantID,
		VersionID:    req.VersionID,
		ClientIP:     req.ClientIP,
		UserAgent:    req.UserAgent,
		DeviceID:     req.DeviceID,
		DownloadedAt: time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("failed to record download log: %w", err)
	}

	// 异步更新统计表
	go s.updateUsageStats(req.TenantID, req.VersionID, req.DeviceID)

	return nil
}

// updateUsageStats 更新用量统计
// 内部方法，异步执行
func (s *UsageService) updateUsageStats(tenantID uint64, versionID uint64, deviceID string) {
	// TODO: 实现按天统计逻辑
	// 1. 查询或创建当天的统计记录
	// 2. 增加下载计数
	// 3. 如果是新设备，增加独立设备计数
}

// GetDownloadLogs 获取下载日志列表
func (s *UsageService) GetDownloadLogs(ctx context.Context, versionID uint64, page, pageSize int) ([]model.DownloadLog, int64, error) {
	var logs []model.DownloadLog
	var total int64

	query := s.db.WithContext(ctx).Model(&model.DownloadLog{})

	if versionID > 0 {
		query = query.Where("version_id = ?", versionID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	if err := query.Order("downloaded_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list logs: %w", err)
	}

	return logs, total, nil
}

// GetStats 获取用量统计
func (s *UsageService) GetStats(ctx context.Context, req *StatsQueryRequest) ([]StatsResponse, error) {
	var stats []model.UsageStats

	query := s.db.WithContext(ctx).Model(&model.UsageStats{})

	if req.TenantID > 0 {
		query = query.Where("tenant_id = ?", req.TenantID)
	}
	if req.SoftwareID > 0 {
		query = query.Where("software_id = ?", req.SoftwareID)
	}
	if req.VersionID > 0 {
		query = query.Where("version_id = ?", req.VersionID)
	}
	if !req.StartDate.IsZero() {
		query = query.Where("stats_date >= ?", req.StartDate)
	}
	if !req.EndDate.IsZero() {
		query = query.Where("stats_date <= ?", req.EndDate)
	}

	if err := query.Order("stats_date DESC").Find(&stats).Error; err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	var resp []StatsResponse
	for _, stat := range stats {
		resp = append(resp, StatsResponse{
			Date:          stat.StatsDate.Format("2006-01-02"),
			DownloadCount: stat.DownloadCount,
			UniqueDevices: stat.UniqueDevices,
		})
	}

	return resp, nil
}

// GetTotalDownloads 获取总下载次数
func (s *UsageService) GetTotalDownloads(ctx context.Context, softwareID uint64) (int64, error) {
	var count int64

	query := s.db.WithContext(ctx).Model(&model.DownloadLog{}).
		Joins("JOIN version ON version.id = download_log.version_id").
		Where("version.software_id = ?", softwareID)

	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count downloads: %w", err)
	}

	return count, nil
}

// GetVersionDownloads 获取版本下载统计
func (s *UsageService) GetVersionDownloads(ctx context.Context, versionID uint64) (int64, error) {
	var count int64

	if err := s.db.WithContext(ctx).Model(&model.DownloadLog{}).
		Where("version_id = ?", versionID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count version downloads: %w", err)
	}

	return count, nil
}

// AggregateDailyStats 聚合每日统计
// 定时任务调用，汇总前一天的下载数据
func (s *UsageService) AggregateDailyStats(ctx context.Context, date time.Time) error {
	// TODO: 实现每日统计聚合逻辑
	// 1. 查询指定日期的下载日志
	// 2. 按软件、版本分组统计
	// 3. 更新或创建usage_stats记录
	return nil
}
