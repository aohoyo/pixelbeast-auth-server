// Package storage 阿里云OSS存储实现
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// AliyunProvider 阿里云OSS存储提供者
type AliyunProvider struct {
	client *oss.Client
	bucket *oss.Bucket
	config *AliConfig
}

// NewAliyunProvider 创建阿里云OSS存储提供者
func NewAliyunProvider(cfg *AliConfig) (*AliyunProvider, error) {
	// 创建OSS客户端
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create oss client: %w", err)
	}

	// 获取bucket
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket: %w", err)
	}

	return &AliyunProvider{
		client: client,
		bucket: bucket,
		config: cfg,
	}, nil
}

// Upload 上传文件到OSS
func (p *AliyunProvider) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	options := []oss.Option{}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
	}

	err := p.bucket.PutObject(key, reader, options...)
	if err != nil {
		return fmt.Errorf("failed to upload to oss: %w", err)
	}

	return nil
}

// Download 从OSS下载文件
func (p *AliyunProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	body, err := p.bucket.GetObject(key)
	if err != nil {
		return nil, fmt.Errorf("failed to download from oss: %w", err)
	}

	return body, nil
}

// Delete 从OSS删除文件
func (p *AliyunProvider) Delete(ctx context.Context, key string) error {
	err := p.bucket.DeleteObject(key)
	if err != nil {
		return fmt.Errorf("failed to delete from oss: %w", err)
	}

	return nil
}

// Exists 检查OSS文件是否存在
func (p *AliyunProvider) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := p.bucket.IsObjectExist(key)
	if err != nil {
		return false, fmt.Errorf("failed to check object exist: %w", err)
	}

	return exists, nil
}

// GetURL 获取OSS文件签名URL
func (p *AliyunProvider) GetURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	// 生成签名URL
	signedURL, err := p.bucket.SignURL(key, oss.HTTPGet, int64(expire.Seconds()))
	if err != nil {
		return "", fmt.Errorf("failed to sign url: %w", err)
	}

	return signedURL, nil
}

// GetPublicURL 获取OSS文件公开URL
func (p *AliyunProvider) GetPublicURL(key string) string {
	// 构建公开URL
	return fmt.Sprintf("https://%s.%s/%s", p.config.Bucket, p.config.Endpoint, key)
}

// List 列出OSS文件
func (p *AliyunProvider) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	// 列出文件
	result, err := p.bucket.ListObjects(oss.Prefix(prefix))
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	files := make([]FileInfo, 0)
	for _, obj := range result.Objects {
		files = append(files, FileInfo{
			Name: obj.Key,
			Type: "file",
			Size: obj.Size,
			URL:  p.GetPublicURL(obj.Key),
			Time: obj.LastModified,
		})
	}

	return files, nil
}
