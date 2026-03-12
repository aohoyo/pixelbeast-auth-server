// Package service 上传服务
package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
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

// UploadToken 上传凭证 - 根据存储类型返回不同字段
type UploadToken struct {
	// 通用字段
	StorageType string `json:"storageType"` // aliyun/tencent/qiniu/minio
	Key         string `json:"key"`         // 存储键名
	URL         string `json:"url"`         // 上传URL

	// 阿里云OSS字段
	AccessKeyId string `json:"accessKeyId,omitempty"` // AccessKey ID
	Policy      string `json:"policy,omitempty"`      // Policy
	Signature   string `json:"signature,omitempty"`   // 签名

	// 腾讯云COS字段
	SignAlgorithm string `json:"signAlgorithm,omitempty"` // 签名算法
	KeyTime       string `json:"keyTime,omitempty"`       // 密钥有效期

	// 七牛云字段
	Token  string `json:"token,omitempty"`  // 上传Token
	Domain string `json:"domain,omitempty"` // 域名

	// MinIO字段
	ContentType string `json:"contentType,omitempty"` // 内容类型

	ExpireAt int64 `json:"expireAt"` // 过期时间戳
}

// GetUploadToken 获取上传凭证
func (s *UploadService) GetUploadToken(ctx context.Context, tenantID uint64, key string, storageType string) (*UploadToken, error) {
	// 获取存储配置
	config, err := s.storageService.GetDecryptedConfig(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("获取存储配置失败: %w", err)
	}

	if config == nil {
		return nil, fmt.Errorf("请先配置存储")
	}

	// 根据存储类型生成不同的上传凭证
	switch config.Type {
	case "aliyun":
		return s.getAliyunUploadToken(config, key)
	case "tencent":
		return s.getTencentUploadToken(config, key)
	case "qiniu":
		return s.getQiniuUploadToken(config, key)
	case "minio":
		return s.getMinioUploadToken(config, key)
	default:
		return nil, fmt.Errorf("不支持的存储类型: %s", config.Type)
	}
}

// getAliyunUploadToken 获取阿里云OSS上传凭证
func (s *UploadService) getAliyunUploadToken(config *DecryptedStorageConfig, key string) (*UploadToken, error) {
	// 构建Policy
	expireTime := time.Now().Add(time.Hour)
	policyMap := map[string]interface{}{
		"expiration": expireTime.Format("2006-01-02T12:00:00.000Z"),
		"conditions": [][]interface{}{
			{"content-length-range", 0, 104857600}, // 最大100MB
		},
	}
	policyJSON, _ := json.Marshal(policyMap)
	policy := base64.StdEncoding.EncodeToString(policyJSON)

	// 计算签名
	signature := base64.StdEncoding.EncodeToString(
		hmacSHA1([]byte(config.SecretKey), []byte(policy)),
	)

	return &UploadToken{
		StorageType: "aliyun",
		Key:         key,
		URL:         fmt.Sprintf("https://%s.%s", config.Bucket, config.Endpoint),
		AccessKeyId: config.AccessKey,
		Policy:      policy,
		Signature:   signature,
		ExpireAt:    expireTime.Unix(),
	}, nil
}

// getTencentUploadToken 获取腾讯云COS上传凭证
func (s *UploadService) getTencentUploadToken(config *DecryptedStorageConfig, key string) (*UploadToken, error) {
	// 构建KeyTime
	now := time.Now()
	expireTime := now.Add(time.Hour)
	keyTime := fmt.Sprintf("%d;%d", now.Unix(), expireTime.Unix())

	// 构建Policy
	policyMap := map[string]interface{}{
		"expiration": expireTime.Format("2006-01-02T12:00:00Z"),
		"conditions": [][]interface{}{
			{"content-length-range", 0, 104857600},
		},
	}
	policyJSON, _ := json.Marshal(policyMap)
	policy := base64.StdEncoding.EncodeToString(policyJSON)

	// 计算签名
	signKey := hmacSHA1([]byte(config.SecretKey), []byte(keyTime))
	signature := base64.StdEncoding.EncodeToString(
		hmacSHA1(signKey, []byte(policy)),
	)

	return &UploadToken{
		StorageType:   "tencent",
		Key:           key,
		URL:           fmt.Sprintf("https://%s.cos.%s.myqcloud.com", config.Bucket, config.Region),
		AccessKeyId:   config.AccessKey,
		Policy:        policy,
		Signature:     signature,
		SignAlgorithm: "sha1",
		KeyTime:       keyTime,
		ExpireAt:      expireTime.Unix(),
	}, nil
}

// getQiniuUploadToken 获取七牛云上传凭证
func (s *UploadService) getQiniuUploadToken(config *DecryptedStorageConfig, key string) (*UploadToken, error) {
	// 构建上传策略
	putPolicy := map[string]interface{}{
		"scope":      fmt.Sprintf("%s:%s", config.Bucket, key),
		"deadline":   time.Now().Add(time.Hour).Unix(),
		"returnBody": `{"key":"$(key)","hash":"$(etag)","fsize":$(fsize)}`,
	}
	putPolicyJSON, _ := json.Marshal(putPolicy)
	encodedPutPolicy := base64.URLEncoding.EncodeToString(putPolicyJSON)

	// 计算签名
	sign := hmacSHA1([]byte(config.SecretKey), []byte(encodedPutPolicy))
	encodedSign := base64.URLEncoding.EncodeToString(sign)

	// Token
	token := fmt.Sprintf("%s:%s:%s", config.AccessKey, encodedSign, encodedPutPolicy)

	return &UploadToken{
		StorageType: "qiniu",
		Key:         key,
		URL:         fmt.Sprintf("https://upload-%s.qiniup.com", config.Region),
		Token:       token,
		Domain:      config.Domain,
		ExpireAt:    time.Now().Add(time.Hour).Unix(),
	}, nil
}

// getMinioUploadToken 获取MinIO上传凭证
func (s *UploadService) getMinioUploadToken(config *DecryptedStorageConfig, key string) (*UploadToken, error) {
	// MinIO 使用预签名URL
	expireTime := time.Now().Add(time.Hour)

	// 构建预签名URL
	url := fmt.Sprintf("%s/%s/%s", config.Domain, config.Bucket, key)

	return &UploadToken{
		StorageType: "minio",
		Key:         key,
		URL:         url,
		ExpireAt:    expireTime.Unix(),
	}, nil
}

// hmacSHA1 计算HMAC-SHA1
func hmacSHA1(key []byte, data []byte) []byte {
	h := hmac.New(sha1.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// UploadFile 上传文件（后端直传备用方案）
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
			BasePath: config.Endpoint,
			BaseURL:  config.Domain,
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
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
