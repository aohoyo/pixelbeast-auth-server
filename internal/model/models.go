// Package model 定义数据模型
// 包含软件、版本、租户、用量统计等核心实体
package model

import (
	"time"

	"gorm.io/gorm"
)

// Software 软件信息
// 多租户预留：包含 tenant_id 字段
type Software struct {
	ID          uint64         `json:"id" gorm:"primaryKey"`
	TenantID    uint64         `json:"tenant_id" gorm:"index;comment:租户ID"` // 多租户预留字段
	Name        string         `json:"name" gorm:"size:100;not null;comment:软件名称"`
	Slug        string         `json:"slug" gorm:"size:100;uniqueIndex;not null;comment:软件标识"`
	APIKey      string         `json:"api_key" gorm:"size:64;uniqueIndex;comment:API密钥"`
	Description string         `json:"description" gorm:"size:500;comment:软件描述"`
	Platform    string         `json:"platform" gorm:"size:50;comment:支持平台 windows/macos/linux"`
	Icon        string         `json:"icon" gorm:"size:500;comment:软件图标URL"`
	Status      int            `json:"status" gorm:"default:1;comment:状态 1启用 0禁用"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// 关联
	Versions []Version `json:"versions,omitempty" gorm:"foreignKey:SoftwareID"`
}

// TableName 指定表名
func (Software) TableName() string {
	return "software"
}

// Version 软件版本
// 支持增量更新、强制更新等策略
type Version struct {
	ID            uint64         `json:"id" gorm:"primaryKey"`
	TenantID      uint64         `json:"tenant_id" gorm:"index;comment:租户ID"` // 多租户预留字段
	SoftwareID    uint64         `json:"software_id" gorm:"index;not null"`
	Version       string         `json:"version" gorm:"size:50;not null;comment:版本号"`
	VersionCode   int            `json:"version_code" gorm:"comment:版本代码，用于比较"`
	ChangeLog     string         `json:"changelog" gorm:"type:text;comment:更新日志"`
	Screenshots   string         `json:"screenshots" gorm:"type:text;comment:版本截图URLs(JSON数组)"`
	PackageURL    string         `json:"package_url" gorm:"size:500;comment:安装包URL"`
	PackageSize   int64          `json:"package_size" gorm:"comment:包大小(字节)"`
	PackageHash   string         `json:"package_hash" gorm:"size:64;comment:包SHA256哈希"`
	IsForced      bool           `json:"is_forced" gorm:"default:false;comment:是否强制更新"`
	IsIncremental bool           `json:"is_incremental" gorm:"default:false;comment:是否增量包"`
	MinVersion    string         `json:"min_version" gorm:"size:50;comment:最低支持升级版本"`
	Status        int            `json:"status" gorm:"default:1;comment:状态 1发布 0草稿"`
	PublishedAt   *time.Time     `json:"published_at" gorm:"comment:发布时间"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`

	// 关联
	Software Software `json:"software,omitempty" gorm:"foreignKey:SoftwareID"`
}

// TableName 指定表名
func (Version) TableName() string {
	return "version"
}

// Tenant 租户信息
// 多租户预留：支持多租户隔离
type Tenant struct {
	ID        uint64         `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name" gorm:"size:100;not null;comment:租户名称"`
	ApiKey    string         `json:"api_key" gorm:"size:64;uniqueIndex;comment:API密钥"`
	Status    int            `json:"status" gorm:"default:1;comment:状态 1启用 0禁用"`
	ExpiredAt *time.Time     `json:"expired_at" gorm:"comment:过期时间"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定表名
func (Tenant) TableName() string {
	return "tenant"
}

// DownloadLog 下载日志
// 用量统计预留：记录每次下载行为
type DownloadLog struct {
	ID         uint64    `json:"id" gorm:"primaryKey"`
	TenantID   uint64    `json:"tenant_id" gorm:"index;comment:租户ID"` // 多租户预留字段
	VersionID  uint64    `json:"version_id" gorm:"index"`
	ClientIP   string    `json:"client_ip" gorm:"size:50;comment:客户端IP"`
	UserAgent  string    `json:"user_agent" gorm:"size:500;comment:UserAgent"`
	DeviceID   string    `json:"device_id" gorm:"size:100;comment:设备标识"`
	DownloadedAt time.Time `json:"downloaded_at"`
}

// TableName 指定表名
func (DownloadLog) TableName() string {
	return "download_log"
}

// UsageStats 用量统计
// 用量统计预留：按天统计下载次数
type UsageStats struct {
	ID           uint64    `json:"id" gorm:"primaryKey"`
	TenantID     uint64    `json:"tenant_id" gorm:"index;comment:租户ID"` // 多租户预留字段
	SoftwareID   uint64    `json:"software_id" gorm:"index"`
	VersionID    uint64    `json:"version_id" gorm:"index"`
	StatsDate    time.Time `json:"stats_date" gorm:"index;comment:统计日期"`
	DownloadCount int64    `json:"download_count" gorm:"comment:下载次数"`
	UniqueDevices int64    `json:"unique_devices" gorm:"comment:独立设备数"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 指定表名
func (UsageStats) TableName() string {
	return "usage_stats"
}

// User 管理员用户
type User struct {
	ID        uint64         `json:"id" gorm:"primaryKey"`
	Username  string         `json:"username" gorm:"size:50;uniqueIndex;not null"`
	Password  string         `json:"-" gorm:"size:128;not null"`
	Email     string         `json:"email" gorm:"size:100"`
	Role      string         `json:"role" gorm:"size:20;default:admin"`
	Status    int            `json:"status" gorm:"default:1"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定表名
func (User) TableName() string {
	return "user"
}

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Software{},
		&Version{},
		&Tenant{},
		&DownloadLog{},
		&UsageStats{},
		&User{},
	)
}
