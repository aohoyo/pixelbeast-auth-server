// Package storage 本地存储实现
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalProvider 本地存储提供者
type LocalProvider struct {
	basePath string
	baseURL  string
}

// NewLocalProvider 创建本地存储提供者
func NewLocalProvider(cfg *LocalConfig) (*LocalProvider, error) {
	// 确保基础目录存在
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}

	return &LocalProvider{
		basePath: cfg.BasePath,
		baseURL:  cfg.BaseURL,
	}, nil
}

// Upload 上传文件到本地存储
func (p *LocalProvider) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	fullPath := filepath.Join(p.basePath, key)

	// 创建目录
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 创建文件
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// 写入内容
	written, err := io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if written != size && size > 0 {
		return fmt.Errorf("written size mismatch: expected %d, got %d", size, written)
	}

	return nil
}

// Download 从本地存储下载文件
func (p *LocalProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	fullPath := filepath.Join(p.basePath, key)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", key)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// Delete 删除本地文件
func (p *LocalProvider) Delete(ctx context.Context, key string) error {
	fullPath := filepath.Join(p.basePath, key)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在视为删除成功
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// Move 移动/重命名本地文件
func (p *LocalProvider) Move(ctx context.Context, srcKey, dstKey string) error {
	srcPath := filepath.Join(p.basePath, srcKey)
	dstPath := filepath.Join(p.basePath, dstKey)

	// 创建目标目录
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 移动文件
	if err := os.Rename(srcPath, dstPath); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	return nil
}

// Copy 复制本地文件
func (p *LocalProvider) Copy(ctx context.Context, srcKey, dstKey string) error {
	srcPath := filepath.Join(p.basePath, srcKey)
	dstPath := filepath.Join(p.basePath, dstKey)

	// 创建目标目录
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 打开源文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// 创建目标文件
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// 复制内容
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// Exists 检查本地文件是否存在
func (p *LocalProvider) Exists(ctx context.Context, key string) (bool, error) {
	fullPath := filepath.Join(p.basePath, key)

	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// GetURL 获取本地文件URL（本地存储不支持临时URL，返回公开URL）
func (p *LocalProvider) GetURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	return p.GetPublicURL(key), nil
}

// GetPublicURL 获取本地文件公开URL
func (p *LocalProvider) GetPublicURL(key string) string {
	return fmt.Sprintf("%s/%s", p.baseURL, key)
}

// List 列出本地文件
func (p *LocalProvider) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	fullPath := filepath.Join(p.basePath, prefix)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []FileInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		filePath := filepath.Join(fullPath, name)
		
		info := FileInfo{
			Name: filepath.Join(prefix, name),
			Type: "file",
		}

		if fileInfo, err := os.Stat(filePath); err == nil {
			info.Size = fileInfo.Size()
			info.Time = fileInfo.ModTime()
		}

		info.URL = p.GetPublicURL(info.Name)
		files = append(files, info)
	}

	return files, nil
}
