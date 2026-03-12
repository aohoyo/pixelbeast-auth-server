// Package storage MinIO存储实现
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioConfig MinIO存储配置
type MinioConfig struct {
	Endpoint  string `json:"endpoint" yaml:"endpoint"`   // 例如: localhost:9000
	AccessKey string `json:"access_key" yaml:"access_key"`
	SecretKey string `json:"secret_key" yaml:"secret_key"`
	Bucket    string `json:"bucket" yaml:"bucket"`
	Domain    string `json:"domain" yaml:"domain"`       // 公开访问域名
	UseSSL    bool   `json:"use_ssl" yaml:"use_ssl"`     // 是否使用SSL
}

// MinioProvider MinIO存储提供者
type MinioProvider struct {
	client *minio.Client
	bucket string
	domain string
}

// NewMinioProvider 创建MinIO存储提供者
func NewMinioProvider(cfg *MinioConfig) (*MinioProvider, error) {
	// 创建MinIO客户端
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	// 检查bucket是否存在，不存在则创建
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		// 设置bucket策略为公开读
		policy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Principal": {"AWS": "*"},
				"Action": ["s3:GetObject"],
				"Resource": ["arn:aws:s3:::%s/*"]
			}]
		}`, cfg.Bucket)
		if err := client.SetBucketPolicy(ctx, cfg.Bucket, policy); err != nil {
			// 策略设置失败不影响使用，只记录日志
			fmt.Printf("[WARN] failed to set bucket policy: %v\n", err)
		}
	}

	domain := cfg.Domain
	if domain == "" {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		domain = fmt.Sprintf("%s://%s/%s", scheme, cfg.Endpoint, cfg.Bucket)
	}

	return &MinioProvider{
		client: client,
		bucket: cfg.Bucket,
		domain: domain,
	}, nil
}

// Upload 上传文件到MinIO
func (p *MinioProvider) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := p.client.PutObject(ctx, p.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("failed to upload to minio: %w", err)
	}

	return nil
}

// Download 从MinIO下载文件
func (p *MinioProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := p.client.GetObject(ctx, p.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to download from minio: %w", err)
	}

	return obj, nil
}

// Delete 从MinIO删除文件
func (p *MinioProvider) Delete(ctx context.Context, key string) error {
	err := p.client.RemoveObject(ctx, p.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete from minio: %w", err)
	}

	return nil
}

// Exists 检查MinIO文件是否存在
func (p *MinioProvider) Exists(ctx context.Context, key string) (bool, error) {
	_, err := p.client.StatObject(ctx, p.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		// 检查是否是对象不存在的错误
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object exist: %w", err)
	}

	return true, nil
}

// GetURL 获取MinIO文件签名URL
func (p *MinioProvider) GetURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	url, err := p.client.PresignedGetObject(ctx, p.bucket, key, expire, nil)
	if err != nil {
		return "", fmt.Errorf("failed to sign url: %w", err)
	}

	return url.String(), nil
}

// GetPublicURL 获取MinIO文件公开URL
func (p *MinioProvider) GetPublicURL(key string) string {
	return fmt.Sprintf("%s/%s", p.domain, key)
}