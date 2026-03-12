// Package model 文件管理模型
package model

import (
	"time"
)

// File 文件/文件夹模型
type File struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID  uint64    `gorm:"index:idx_file_tenant_id" json:"tenant_id"`
	ParentID  uint64    `gorm:"index:idx_file_parent_id" json:"parent_id"` // 父文件夹ID，0表示根目录
	Name      string    `gorm:"size:255;not null" json:"name"`
	Type      string    `gorm:"size:20;not null" json:"type"` // folder/file
	FileType  string    `gorm:"size:50" json:"file_type"`     // image/pdf/word/excel等
	Size      int64     `json:"size"`
	Path      string    `gorm:"size:500" json:"path"` // 存储路径
	URL       string    `gorm:"size:500" json:"url"`  // 访问URL
	// 文件来源信息
	Source   string `gorm:"size:50;index:idx_file_source" json:"source"`   // 来源：software/version/file
	SourceID uint64 `gorm:"index:idx_file_source_id" json:"source_id"`    // 关联的业务ID
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (File) TableName() string {
	return "file"
}

// IsFolder 判断是否为文件夹
func (f *File) IsFolder() bool {
	return f.Type == "folder"
}

// IsImage 判断是否为图片
func (f *File) IsImage() bool {
	return f.FileType == "image"
}

// GetFileTypeByExt 根据扩展名获取文件类型
func GetFileTypeByExt(ext string) string {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg":
		return "image"
	case ".pdf":
		return "pdf"
	case ".doc", ".docx":
		return "word"
	case ".xls", ".xlsx":
		return "excel"
	case ".ppt", ".pptx":
		return "ppt"
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv":
		return "video"
	case ".mp3", ".wav", ".flac", ".aac", ".ogg":
		return "audio"
	case ".zip", ".rar", ".7z", ".tar", ".gz":
		return "archive"
	case ".txt", ".md", ".log":
		return "text"
	case ".js", ".ts", ".jsx", ".tsx", ".vue", ".html", ".css", ".scss", ".less",
		".py", ".java", ".go", ".php", ".rb", ".swift", ".kt", ".rs", ".c", ".cpp", ".h":
		return "code"
	default:
		return "file"
	}
}
