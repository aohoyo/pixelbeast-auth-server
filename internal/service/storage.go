// Package service 存储配置服务
package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"gorm.io/gorm"

	"license-server/internal/model"
	"license-server/internal/storage"
)

// StorageService 存储配置服务
type StorageService struct {
	db        *gorm.DB
	secretKey string // AES加密密钥
}

// NewStorageService 创建存储配置服务
func NewStorageService(db *gorm.DB, secretKey string) *StorageService {
	if len(secretKey) != 32 {
		// 如果密钥长度不是32字节，进行填充或截断
		if len(secretKey) < 32 {
			secretKey = secretKey + string(make([]byte, 32-len(secretKey)))
		} else {
			secretKey = secretKey[:32]
		}
	}
	return &StorageService{
		db:        db,
		secretKey: secretKey,
	}
}

// StorageConfigRequest 存储配置请求
type StorageConfigRequest struct {
	Type      string `json:"type" binding:"required"` // aliyun/tencent/qiniu/minio
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`  // 阿里云/腾讯云需要
	Domain    string `json:"domain"`  // CDN域名
	Endpoint  string `json:"endpoint"` // MinIO需要
	UseSSL    bool   `json:"use_ssl"`  // MinIO SSL
}

// StorageConfigResponse 存储配置响应
type StorageConfigResponse struct {
	ID        uint64 `json:"id"`
	Type      string `json:"type"`
	Status    int    `json:"status"`
	HasConfig bool   `json:"has_config"`
	// 配置详情（已解密）
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	Region    string `json:"region,omitempty"`
	Domain    string `json:"domain,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
}

// StorageTypeInfo 存储类型信息
type StorageTypeInfo struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Fields      []Field `json:"fields"`
}

// Field 表单字段定义
type Field struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"` // text/password
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder"`
}

// GetStorageTypes 获取支持的存储类型
func GetStorageTypes() []StorageTypeInfo {
	return []StorageTypeInfo{
		{
			Type:        "aliyun",
			Name:        "阿里云OSS",
			Description: "国内首选，稳定可靠",
			Website:     "https://oss.aliyun.com",
			Fields: []Field{
				{Name: "access_key", Label: "AccessKey ID", Type: "text", Required: true, Placeholder: "LTAI..."},
				{Name: "secret_key", Label: "AccessKey Secret", Type: "password", Required: true, Placeholder: ""},
				{Name: "bucket", Label: "Bucket名称", Type: "text", Required: true, Placeholder: "my-bucket"},
				{Name: "region", Label: "Region", Type: "text", Required: true, Placeholder: "oss-cn-hangzhou"},
				{Name: "domain", Label: "CDN域名(可选)", Type: "text", Required: false, Placeholder: "https://cdn.example.com"},
			},
		},
		{
			Type:        "tencent",
			Name:        "腾讯云COS",
			Description: "性价比高，CDN加速",
			Website:     "https://cos.tencent.com",
			Fields: []Field{
				{Name: "access_key", Label: "SecretId", Type: "text", Required: true, Placeholder: ""},
				{Name: "secret_key", Label: "SecretKey", Type: "password", Required: true, Placeholder: ""},
				{Name: "bucket", Label: "Bucket名称", Type: "text", Required: true, Placeholder: "my-bucket-123456"},
				{Name: "region", Label: "Region", Type: "text", Required: true, Placeholder: "ap-guangzhou"},
				{Name: "domain", Label: "CDN域名(可选)", Type: "text", Required: false, Placeholder: "https://cdn.example.com"},
			},
		},
		{
			Type:        "qiniu",
			Name:        "七牛云Kodo",
			Description: "免费额度多，价格实惠",
			Website:     "https://qiniu.com",
			Fields: []Field{
				{Name: "access_key", Label: "AccessKey", Type: "text", Required: true, Placeholder: ""},
				{Name: "secret_key", Label: "SecretKey", Type: "password", Required: true, Placeholder: ""},
				{Name: "bucket", Label: "Bucket名称", Type: "text", Required: true, Placeholder: "my-bucket"},
				{Name: "domain", Label: "CDN域名", Type: "text", Required: true, Placeholder: "https://xxx.qiniudn.com"},
			},
		},
		{
			Type:        "minio",
			Name:        "MinIO(自建)",
			Description: "开源对象存储，完全自主控制",
			Website:     "https://min.io",
			Fields: []Field{
				{Name: "endpoint", Label: "服务地址", Type: "text", Required: true, Placeholder: "localhost:9000"},
				{Name: "access_key", Label: "AccessKey", Type: "text", Required: true, Placeholder: "minioadmin"},
				{Name: "secret_key", Label: "SecretKey", Type: "password", Required: true, Placeholder: "minioadmin"},
				{Name: "bucket", Label: "Bucket名称", Type: "text", Required: true, Placeholder: "pixelbeast"},
				{Name: "domain", Label: "公开访问域名", Type: "text", Required: false, Placeholder: "http://localhost:9000/pixelbeast"},
				{Name: "use_ssl", Label: "使用SSL", Type: "checkbox", Required: false, Placeholder: ""},
			},
		},
	}
}

// GetStorageConfig 获取租户存储配置
func (s *StorageService) GetStorageConfig(ctx context.Context, tenantID uint64) (*StorageConfigResponse, error) {
	var config model.TenantStorage
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&config).Error

	if err == gorm.ErrRecordNotFound {
		return &StorageConfigResponse{
			HasConfig: false,
			Status:    0,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get storage config: %w", err)
	}

	// 解密配置 JSON
	decryptedConfig, err := s.decrypt(config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(decryptedConfig), &configMap); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &StorageConfigResponse{
		ID:        config.ID,
		Type:      config.Type,
		Status:    config.Status,
		HasConfig: true,
		// 返回配置详情（解密后）
		AccessKey: getStringFromMap(configMap, "access_key"),
		SecretKey: getStringFromMap(configMap, "secret_key"),
		Bucket:    getStringFromMap(configMap, "bucket"),
		Region:    getStringFromMap(configMap, "region"),
		Domain:    getStringFromMap(configMap, "domain"),
		Endpoint:  getStringFromMap(configMap, "endpoint"),
	}, nil
}

// getStringFromMap 从 map 中获取字符串
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// SaveStorageConfig 保存存储配置
func (s *StorageService) SaveStorageConfig(ctx context.Context, tenantID uint64, req *StorageConfigRequest) error {
	// 构建配置JSON
	configMap := map[string]interface{}{
		"access_key": req.AccessKey,
		"secret_key": req.SecretKey,
		"bucket":     req.Bucket,
	}
	if req.Region != "" {
		configMap["region"] = req.Region
	}
	if req.Domain != "" {
		configMap["domain"] = req.Domain
	}
	if req.Endpoint != "" {
		configMap["endpoint"] = req.Endpoint
	}
	if req.Type == "minio" {
		configMap["use_ssl"] = req.UseSSL
	}

	configJSON, err := json.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 加密配置
	encryptedConfig, err := s.encrypt(string(configJSON))
	if err != nil {
		return fmt.Errorf("failed to encrypt config: %w", err)
	}

	// 查找现有配置
	var existing model.TenantStorage
	err = s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&existing).Error

	now := time.Now()
	if err == gorm.ErrRecordNotFound {
		// 创建新配置
		config := &model.TenantStorage{
			TenantID: tenantID,
			Type:     req.Type,
			Config:   encryptedConfig,
			Status:   1, // 已配置
		}
		return s.db.WithContext(ctx).Create(config).Error
	}
	if err != nil {
		return fmt.Errorf("failed to check existing config: %w", err)
	}

	// 更新现有配置
	return s.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
		"type":       req.Type,
		"config":     encryptedConfig,
		"status":     1,
		"updated_at": now,
	}).Error
}

// TestStorageConfig 测试存储配置
func (s *StorageService) TestStorageConfig(ctx context.Context, req *StorageConfigRequest) error {
	// 创建临时存储提供者进行测试
	provider, err := s.createProviderFromConfig(req)
	if err != nil {
		return err
	}

	// 测试上传一个小文件
	testKey := fmt.Sprintf("test/%d/test.txt", time.Now().Unix())
	testContent := "PixelBeast Storage Test"

	err = provider.Upload(ctx, testKey, strings.NewReader(testContent), int64(len(testContent)), "text/plain")
	if err != nil {
		return fmt.Errorf("storage test failed: %w", err)
	}

	// 清理测试文件
	_ = provider.Delete(ctx, testKey)

	return nil
}

// GetStorageProvider 获取租户的存储提供者
func (s *StorageService) GetStorageProvider(ctx context.Context, tenantID uint64) (storage.Provider, error) {
	var config model.TenantStorage
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&config).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("storage not configured, please configure storage first")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get storage config: %w", err)
	}

	if config.Status != 1 {
		return nil, fmt.Errorf("storage not configured or verification failed")
	}

	// 解密配置
	decryptedConfig, err := s.decrypt(config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	// 解析配置
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(decryptedConfig), &configMap); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 构建存储配置请求
	req := &StorageConfigRequest{
		Type: config.Type,
	}
	if v, ok := configMap["access_key"].(string); ok {
		req.AccessKey = v
	}
	if v, ok := configMap["secret_key"].(string); ok {
		req.SecretKey = v
	}
	if v, ok := configMap["bucket"].(string); ok {
		req.Bucket = v
	}
	if v, ok := configMap["region"].(string); ok {
		req.Region = v
	}
	if v, ok := configMap["domain"].(string); ok {
		req.Domain = v
	}
	if v, ok := configMap["endpoint"].(string); ok {
		req.Endpoint = v
	}
	if v, ok := configMap["use_ssl"].(bool); ok {
		req.UseSSL = v
	}

	return s.createProviderFromConfig(req)
}

// createProviderFromConfig 从配置创建存储提供者
func (s *StorageService) createProviderFromConfig(req *StorageConfigRequest) (storage.Provider, error) {
	switch req.Type {
	case "aliyun":
		return storage.NewAliyunProvider(&storage.AliConfig{
			Endpoint:        fmt.Sprintf("%s.aliyuncs.com", req.Region),
			AccessKeyID:     req.AccessKey,
			AccessKeySecret: req.SecretKey,
			Bucket:          req.Bucket,
		})
	case "tencent":
		return storage.NewTencentProvider(&storage.TencentConfig{
			Region:    req.Region,
			SecretID:  req.AccessKey,
			SecretKey: req.SecretKey,
			Bucket:    req.Bucket,
		})
	case "qiniu":
		return storage.NewQiniuProvider(&storage.QiniuConfig{
			AccessKey: req.AccessKey,
			SecretKey: req.SecretKey,
			Bucket:    req.Bucket,
			Region:    req.Region,
			Domain:    req.Domain,
		})
	case "minio":
		return storage.NewMinioProvider(&storage.MinioConfig{
			Endpoint:  req.Endpoint,
			AccessKey: req.AccessKey,
			SecretKey: req.SecretKey,
			Bucket:    req.Bucket,
			Domain:    req.Domain,
			UseSSL:    req.UseSSL,
		})
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", req.Type)
	}
}

// encrypt AES加密
func (s *StorageService) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher([]byte(s.secretKey))
	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(plaintext))

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt AES解密
func (s *StorageService) decrypt(ciphertext string) (string, error) {
	block, err := aes.NewCipher([]byte(s.secretKey))
	if err != nil {
		return "", err
	}

	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	if len(decoded) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := decoded[:aes.BlockSize]
	decoded = decoded[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(decoded, decoded)

	return string(decoded), nil
}

// MarkStorageFailed 标记存储验证失败
func (s *StorageService) MarkStorageFailed(ctx context.Context, tenantID uint64) error {
	return s.db.WithContext(ctx).Model(&model.TenantStorage{}).
		Where("tenant_id = ?", tenantID).
		Update("status", 2).Error
}

// HasStorageConfig 检查租户是否已配置存储
func (s *StorageService) HasStorageConfig(ctx context.Context, tenantID uint64) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.TenantStorage{}).
		Where("tenant_id = ? AND status = 1", tenantID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DecryptedStorageConfig 解密后的存储配置
type DecryptedStorageConfig struct {
	Type      string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	Domain    string
	Endpoint  string
	BasePath  string
	BaseURL   string
}

// GetDecryptedConfig 获取解密后的存储配置
func (s *StorageService) GetDecryptedConfig(ctx context.Context, tenantID uint64) (*DecryptedStorageConfig, error) {
	var config model.TenantStorage
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND status = 1", tenantID).First(&config).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("storage not configured")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get storage config: %w", err)
	}

	// 解密配置
	decryptedConfig, err := s.decrypt(config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(decryptedConfig), &configMap); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	result := &DecryptedStorageConfig{
		Type: config.Type,
	}

	if v, ok := configMap["access_key"].(string); ok {
		result.AccessKey = v
	}
	if v, ok := configMap["secret_key"].(string); ok {
		result.SecretKey = v
	}
	if v, ok := configMap["bucket"].(string); ok {
		result.Bucket = v
	}
	if v, ok := configMap["region"].(string); ok {
		result.Region = v
	}
	if v, ok := configMap["domain"].(string); ok {
		result.Domain = v
	}
	if v, ok := configMap["endpoint"].(string); ok {
		result.Endpoint = v
	}

	return result, nil
}