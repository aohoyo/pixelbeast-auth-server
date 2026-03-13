// Package storage 腾讯云COS存储实现
package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// TencentProvider 腾讯云COS存储提供者
type TencentProvider struct {
	client *cos.Client
	config *TencentConfig
}

// NewTencentProvider 创建腾讯云COS存储提供者
func NewTencentProvider(cfg *TencentConfig) (*TencentProvider, error) {
	// 构建COS URL
	u, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cfg.Bucket, cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to parse cos url: %w", err)
	}

	// 创建COS客户端
	baseURL := &cos.BaseURL{BucketURL: u}
	client := cos.NewClient(baseURL, &http.Client{
		Timeout: 60 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})

	return &TencentProvider{
		client: client,
		config: cfg,
	}, nil
}

// Upload 上传文件到COS
func (p *TencentProvider) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	opt := &cos.ObjectPutOptions{}
	if contentType != "" {
		opt.ContentType = contentType
	}

	_, err := p.client.Object.Put(ctx, key, reader, opt)
	if err != nil {
		return fmt.Errorf("failed to upload to cos: %w", err)
	}

	return nil
}

// Download 从COS下载文件
func (p *TencentProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := p.client.Object.Get(ctx, key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to download from cos: %w", err)
	}

	return resp.Body, nil
}

// Delete 从COS删除文件
func (p *TencentProvider) Delete(ctx context.Context, key string) error {
	_, err := p.client.Object.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete from cos: %w", err)
	}

	return nil
}

// Move 移动COS文件（复制+删除）
func (p *TencentProvider) Move(ctx context.Context, srcKey, dstKey string) error {
	// 源文件路径格式：bucket/key
	src := fmt.Sprintf("%s/%s", p.config.Bucket, srcKey)
	
	// 复制文件
	opt := &cos.ObjectCopyOptions{}
	_, _, err := p.client.Object.Copy(ctx, dstKey, src, opt)
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	// 删除原文件
	_, err = p.client.Object.Delete(ctx, srcKey)
	if err != nil {
		// 尝试回滚
		_, _ = p.client.Object.Delete(ctx, dstKey)
		return fmt.Errorf("failed to delete source object: %w", err)
	}

	return nil
}

// Copy 复制COS文件
func (p *TencentProvider) Copy(ctx context.Context, srcKey, dstKey string) error {
	src := fmt.Sprintf("%s/%s", p.config.Bucket, srcKey)
	opt := &cos.ObjectCopyOptions{}
	_, _, err := p.client.Object.Copy(ctx, dstKey, src, opt)
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	return nil
}

// Exists 检查COS文件是否存在
func (p *TencentProvider) Exists(ctx context.Context, key string) (bool, error) {
	_, err := p.client.Object.Head(ctx, key, nil)
	if err != nil {
		if cos.IsNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object exist: %w", err)
	}
	return true, nil
}

// GetURL 获取COS文件签名URL
func (p *TencentProvider) GetURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	// 生成预签名URL
	presignedURL, err := p.client.Object.GetPresignedURL(ctx, http.MethodGet, key, p.config.SecretID, p.config.SecretKey, expire, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get presigned url: %w", err)
	}

	return presignedURL.String(), nil
}

// GetPublicURL 获取COS文件公开URL
func (p *TencentProvider) GetPublicURL(key string) string {
	return fmt.Sprintf("https://%s.cos.%s.myqcloud.com/%s", p.config.Bucket, p.config.Region, key)
}

// List 列出COS文件
func (p *TencentProvider) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	// 使用普通列表接口
	result, _, err := p.client.Bucket.Get(ctx, &cos.BucketGetOptions{
		Prefix: prefix,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	files := make([]FileInfo, 0)
	for _, obj := range result.Contents {
		// 解析时间字符串
		t, _ := time.Parse(time.RFC3339, obj.LastModified)
		files = append(files, FileInfo{
			Name: obj.Key,
			Type: "file",
			Size: obj.Size,
			URL:  p.GetPublicURL(obj.Key),
			Time: t,
		})
	}

	return files, nil
}
