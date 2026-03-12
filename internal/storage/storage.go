// Package storage 存储接口定义
// 支持插件化存储，可切换本地/阿里云OSS/腾讯云COS/七牛云
package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// FileInfo 文件信息
type FileInfo struct {
	Name string
	Type string // file 或 folder
	Size int64
	URL  string
	Time time.Time
}

// Provider 存储提供者接口
type Provider interface {
	// Upload 上传文件
	// key: 存储键名
	// reader: 文件内容读取器
	// size: 文件大小
	// contentType: MIME类型
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error

	// Download 下载文件
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete 删除文件
	Delete(ctx context.Context, key string) error

	// Exists 检查文件是否存在
	Exists(ctx context.Context, key string) (bool, error)

	// GetURL 获取文件访问URL
	// expire: URL过期时间(0表示永久)
	GetURL(ctx context.Context, key string, expire time.Duration) (string, error)

	// GetPublicURL 获取公开访问URL
	GetPublicURL(key string) string

	// List 列出文件
	List(ctx context.Context, prefix string) ([]FileInfo, error)
}

// Config 存储配置
type Config struct {
	Type   string      `yaml:"type"`
	Local  LocalConfig `yaml:"local"`
	Aliyun AliConfig   `yaml:"aliyun"`
	Tencent TencentConfig `yaml:"tencent"`
	Qiniu  QiniuConfig `yaml:"qiniu"`
}

// LocalConfig 本地存储配置
type LocalConfig struct {
	BasePath string `yaml:"base_path"`
	BaseURL  string `yaml:"base_url"`
}

// AliConfig 阿里云OSS配置
type AliConfig struct {
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	Bucket          string `yaml:"bucket"`
}

// TencentConfig 腾讯云COS配置
type TencentConfig struct {
	Region    string `yaml:"region"`
	SecretID  string `yaml:"secret_id"`
	SecretKey string `yaml:"secret_key"`
	Bucket    string `yaml:"bucket"`
}

// QiniuConfig 七牛云配置
type QiniuConfig struct {
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"` // z0:华东 z1:华北 z2:华南 na0:北美 as0:东南亚
	Domain    string `yaml:"domain"`
}

// NewProvider 创建存储提供者
// 根据配置类型返回对应的存储实现
func NewProvider(cfg *Config) (Provider, error) {
	switch cfg.Type {
	case "local":
		return NewLocalProvider(&cfg.Local)
	case "aliyun":
		return NewAliyunProvider(&cfg.Aliyun)
	case "tencent":
		return NewTencentProvider(&cfg.Tencent)
	case "qiniu":
		return NewQiniuProvider(&cfg.Qiniu)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}

// GenerateKey 生成存储键名
// format: software/{software_id}/versions/{version}/{filename}
func GenerateKey(softwareID uint64, version string, filename string) string {
	return fmt.Sprintf("software/%d/versions/%s/%s", softwareID, version, filename)
}
