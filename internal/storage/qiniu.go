// Package storage 七牛云存储实现
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

// QiniuProvider 七牛云存储提供者
type QiniuProvider struct {
	mac      *qbox.Mac
	bucket   string
	domain   string
	upToken  string
}

// NewQiniuProvider 创建七牛云存储提供者
func NewQiniuProvider(cfg *QiniuConfig) (*QiniuProvider, error) {
	mac := qbox.NewMac(cfg.AccessKey, cfg.SecretKey)

	// 生成上传凭证
	putPolicy := storage.PutPolicy{
		Scope: cfg.Bucket,
	}
	upToken := putPolicy.UploadToken(mac)

	return &QiniuProvider{
		mac:     mac,
		bucket:  cfg.Bucket,
		domain:  cfg.Domain,
		upToken: upToken,
	}, nil
}

// getZone 根据区域代码获取七牛云Zone
func getZone(region string) *storage.Zone {
	switch region {
	case "z0", "": // 华东（默认）
		return &storage.ZoneHuadong
	case "z1": // 华北
		return &storage.ZoneHuabei
	case "z2": // 华南
		return &storage.ZoneHuanan
	case "na0": // 北美
		return &storage.ZoneBeimei
	case "as0": // 东南亚
		return &storage.ZoneXinjiapo
	default:
		return &storage.ZoneHuadong
	}
}

// Upload 上传文件到七牛云
func (p *QiniuProvider) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	// 这里需要从配置中获取region，暂时使用华东
	cfg := storage.Config{
		Zone:          &storage.ZoneHuadong,
		UseHTTPS:      true,
		UseCdnDomains: false,
	}

	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}

	putExtra := storage.PutExtra{}
	if contentType != "" {
		putExtra.MimeType = contentType
	}

	err := formUploader.Put(ctx, &ret, p.upToken, key, reader, size, &putExtra)
	if err != nil {
		return fmt.Errorf("failed to upload to qiniu: %w", err)
	}

	return nil
}

// Download 从七牛云下载文件
func (p *QiniuProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	// 七牛云SDK没有直接提供下载方法，使用HTTP方式
	url := p.GetPublicURL(key)

	// 这里返回一个错误，建议使用GetURL获取下载链接
	return nil, fmt.Errorf("please use GetURL to get download link: %s", url)
}

// Delete 从七牛云删除文件
func (p *QiniuProvider) Delete(ctx context.Context, key string) error {
	cfg := storage.Config{
		Zone:          &storage.ZoneHuadong,
		UseHTTPS:      true,
		UseCdnDomains: false,
	}

	bucketManager := storage.NewBucketManager(p.mac, &cfg)
	err := bucketManager.Delete(p.bucket, key)
	if err != nil {
		return fmt.Errorf("failed to delete from qiniu: %w", err)
	}

	return nil
}

// Exists 检查七牛云文件是否存在
func (p *QiniuProvider) Exists(ctx context.Context, key string) (bool, error) {
	cfg := storage.Config{
		Zone:          &storage.ZoneHuadong,
		UseHTTPS:      true,
		UseCdnDomains: false,
	}

	bucketManager := storage.NewBucketManager(p.mac, &cfg)
	_, err := bucketManager.Stat(p.bucket, key)
	if err != nil {
		if err.Error() == "no such file or directory" {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat file: %w", err)
	}

	return true, nil
}

// GetURL 获取七牛云文件私有URL
func (p *QiniuProvider) GetURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	deadline := time.Now().Add(expire).Unix()
	privateURL := storage.MakePrivateURL(p.mac, p.domain, key, deadline)
	return privateURL, nil
}

// GetPublicURL 获取七牛云文件公开URL
func (p *QiniuProvider) GetPublicURL(key string) string {
	return fmt.Sprintf("https://%s/%s", p.domain, key)
}

// List 列出七牛云文件
func (p *QiniuProvider) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	cfg := storage.Config{
		Zone:          &storage.ZoneHuadong,
		UseHTTPS:      true,
		UseCdnDomains: false,
	}

	bucketManager := storage.NewBucketManager(p.mac, &cfg)
	entries, _, _, _, err := bucketManager.ListFiles(p.bucket, prefix, "", "", 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := make([]FileInfo, 0)
	for _, entry := range entries {
		files = append(files, FileInfo{
			Name: entry.Key,
			Type: "file",
			Size: entry.Fsize,
			URL:  p.GetPublicURL(entry.Key),
			Time: time.Unix(entry.PutTime/10000000, 0),
		})
	}

	return files, nil
}
