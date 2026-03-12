// Package service 上传服务
package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"gorm.io/gorm"

	"license-server/internal/storage"
)

// UploadService 上传服务
type UploadService struct {
	db             *gorm.DB
	storageService *StorageService
}

// NewUploadService 创建上传服务
func NewUploadService(db *gorm.DB, storageService *StorageService) *UploadService {
	return &UploadService{
		db:             db,
		storageService: storageService,
	}
}

// UploadToken 上传凭证
type UploadToken struct {
	Key         string            `json:"key"`          // 存储键名
	URL         string            `json:"url"`          // 上传URL（直传用）
	Headers     map[string]string `json:"headers"`      // 请求头
	Params      map[string]string `json:"params"`       // 表单参数
	ContentType string            `json:"content_type"` // 内容类型
	ExpireAt    int64             `json:"expire_at"`    // 过期时间戳
}

// GetUploadToken 获取上传凭证
// 根据用户配置的存储类型返回对应的直传参数
func (s *UploadService) GetUploadToken(ctx context.Context, tenantID uint64, key string, storageType string) (*UploadToken, error) {
	// 获取存储配置
	config, err := s.storageService.GetStorageConfig(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("获取存储配置失败: %w", err)
	}

	if !config.HasConfig {
		return nil, fmt.Errorf("请先配置存储")
	}

	// 根据存储类型生成不同的上传凭证
	switch config.Type {
	case "aliyun":
		return s.getAliyunUploadToken(ctx, tenantID, key)
	case "tencent":
		return s.getTencentUploadToken(ctx, tenantID, key)
	case "qiniu":
		return s.getQiniuUploadToken(ctx, tenantID, key)
	case "minio":
		return s.getMinioUploadToken(ctx, tenantID, key)
	default:
		return nil, fmt.Errorf("不支持的存储类型: %s", config.Type)
	}
}

// UploadFile 上传文件
func (s *UploadService) UploadFile(ctx context.Context, tenantID uint64, key string, reader io.Reader, size int64, contentType string) (string, error) {
	// 获取存储提供者
	provider, err := s.getStorageProvider(ctx, tenantID)
	if err != nil {
		return "", err
	}

	// 上传文件
	if err := provider.Upload(ctx, key, reader, size, contentType); err != nil {
		return "", fmt.Errorf("上传失败: %w", err)
	}

	// 获取文件URL
	url := provider.GetPublicURL(key)

	return url, nil
}

// DeleteFile 删除文件
func (s *UploadService) DeleteFile(ctx context.Context, tenantID uint64, fileURL string) error {
	// 获取存储提供者
	provider, err := s.getStorageProvider(ctx, tenantID)
	if err != nil {
		return err
	}

	// 从URL中提取key
	key := s.extractKeyFromURL(fileURL)
	if key == "" {
		return fmt.Errorf("无效的文件URL")
	}

	// 删除文件
	if err := provider.Delete(ctx, key); err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}

	return nil
}

// getStorageProvider 获取存储提供者
func (s *UploadService) getStorageProvider(ctx context.Context, tenantID uint64) (storage.Provider, error) {
	// 获取存储配置
	config, err := s.storageService.GetDecryptedConfig(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("获取存储配置失败: %w", err)
	}

	if config == nil {
		return nil, fmt.Errorf("未配置存储")
	}

	// 转换为 storage.Config
	storageConfig := &storage.Config{
		Type: config.Type,
	}

	switch config.Type {
	case "aliyun":
		storageConfig.Aliyun = storage.AliConfig{
			Endpoint:        config.Endpoint,
			AccessKeyID:     config.AccessKey,
			AccessKeySecret: config.SecretKey,
			Bucket:          config.Bucket,
		}
	case "tencent":
		storageConfig.Tencent = storage.TencentConfig{
			Region:    config.Region,
			SecretID:  config.AccessKey,
			SecretKey: config.SecretKey,
			Bucket:    config.Bucket,
		}
	case "qiniu":
		storageConfig.Qiniu = storage.QiniuConfig{
			AccessKey: config.AccessKey,
			SecretKey: config.SecretKey,
			Bucket:    config.Bucket,
			Region:    config.Region,
			Domain:    config.Domain,
		}
	case "minio":
		storageConfig.Local = storage.LocalConfig{
			BasePath: config.BasePath,
			BaseURL:  config.BaseURL,
		}
	}

	// 创建存储提供者
	provider, err := storage.NewProvider(storageConfig)
	if err != nil {
		return nil, fmt.Errorf("创建存储提供者失败: %w", err)
	}

	return provider, nil
}

// extractKeyFromURL 从URL中提取存储key
func (s *UploadService) extractKeyFromURL(url string) string {
	// 简单实现：提取最后一个/后面的部分
	// 实际应该根据存储配置解析
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// getAliyunUploadToken 获取阿里云OSS上传凭证
func (s *UploadService) getAliyunUploadToken(ctx context.Context, tenantID uint64, key string) (*UploadToken, error) {
	// 阿里云使用POST直传，需要生成签名
	// 这里简化处理，返回基础信息
	config, err := s.storageService.GetStorageConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	expireAt := time.Now().Add(time.Hour).Unix()

	return &UploadToken{
		Key:         key,
		URL:         fmt.Sprintf("https://%s.%s", config.Bucket, config.Endpoint),
		Headers:     map[string]string{},
		Params:      map[string]string{},
		ContentType: "",
		ExpireAt:    expireAt,
	}, nil
}

// getTencentUploadToken 获取腾讯云COS上传凭证
func (s *UploadService) getTencentUploadToken(ctx context.Context, tenantID uint64, key string) (*UploadToken, error) {
	config, err := s.storageService.GetStorageConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	expireAt := time.Now().Add(time.Hour).Unix()

	return &UploadToken{
		Key:         key,
		URL:         fmt.Sprintf("https://%s.cos.%s.myqcloud.com", config.Bucket, config.Region),
		Headers:     map[string]string{},
		Params:      map[string]string{},
		ContentType: "",
		ExpireAt:    expireAt,
	}, nil
}

// getQiniuUploadToken 获取七牛云上传凭证
func (s *UploadService) getQiniuUploadToken(ctx context.Context, tenantID uint64, key string) (*UploadToken, error) {
	config, err := s.storageService.GetStorageConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	expireAt := time.Now().Add(time.Hour).Unix()

	return &UploadToken{
		Key:         key,
		URL:         fmt.Sprintf("https://upload-%s.qiniup.com", config.Region),
		Headers:     map[string]string{},
		Params:      map[string]string{},
		ContentType: "",
		ExpireAt:    expireAt,
	}, nil
}

// getMinioUploadToken 获取MinIO上传凭证
func (s *UploadService) getMinioUploadToken(ctx context.Context, tenantID uint64, key string) (*UploadToken, error) {
	config, err := s.storageService.GetStorageConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	expireAt := time.Now().Add(time.Hour).Unix()

	// 从 Domain 字段获取 BaseURL
	baseURL := config.Domain
	if baseURL == "" {
		baseURL = config.Endpoint
	}

	return &UploadToken{
		Key:         key,
		URL:         baseURL,
		Headers:     map[string]string{},
		Params:      map[string]string{},
		ContentType: "",
		ExpireAt:    expireAt,
	}, nil
}
