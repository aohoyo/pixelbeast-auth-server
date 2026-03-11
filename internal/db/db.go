// Package db 数据库连接管理
package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"license-server/internal/model"
)

var DB *gorm.DB

// Config 数据库配置
type Config struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	Charset  string `yaml:"charset"`
	MaxIdle  int    `yaml:"max_idle"`
	MaxOpen  int    `yaml:"max_open"`
}

// createDatabaseIfNotExists 自动创建数据库（如果不存在）
// 使用root用户先连接MySQL，执行CREATE DATABASE IF NOT EXISTS
func createDatabaseIfNotExists(cfg *Config) error {
	// 构建不指定数据库的DSN，用于创建数据库
	// 格式：user:password@tcp(host:port)/?charset=utf8mb4&parseTime=True&loc=Local
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=%s&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Charset)

	// 打开数据库连接（不指定具体数据库）
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL server: %w", err)
	}
	defer db.Close()

	// 测试连接
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping MySQL server: %w", err)
	}

	// 创建数据库（如果不存在）
	createDBSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET %s COLLATE %s_unicode_ci",
		cfg.DBName, cfg.Charset, cfg.Charset)
	if _, err := db.Exec(createDBSQL); err != nil {
		return fmt.Errorf("failed to create database %s: %w", cfg.DBName, err)
	}

	return nil
}

// Init 初始化数据库连接
func Init(cfg *Config) (*gorm.DB, error) {
	// 第一步：自动创建数据库（如果不存在）
	if err := createDatabaseIfNotExists(cfg); err != nil {
		return nil, fmt.Errorf("failed to create database if not exists: %w", err)
	}

	// 第二步：连接到具体的数据库
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.Charset)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB = db

	// 自动迁移
	if err := model.AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// 初始化默认管理员
	if err := initDefaultAdmin(db); err != nil {
		log.Printf("[WARN] failed to init default admin: %v", err)
	}

	return db, nil
}

// initDefaultAdmin 初始化默认管理员账号
// 只在 user 表为空时创建默认管理员
func initDefaultAdmin(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	// 如果已有用户，跳过初始化
	if count > 0 {
		log.Printf("[INFO] user table already has %d records, skip default admin init", count)
		return nil
	}

	// 使用 bcrypt 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// 创建默认管理员
	admin := &model.User{
		Username: "admin",
		Password: string(hashedPassword),
		Email:    "admin@pixelbeast.com",
		Role:     "admin",
		Status:   1,
	}

	if err := db.Create(admin).Error; err != nil {
		return fmt.Errorf("failed to create default admin: %w", err)
	}

	log.Printf("[INFO] default admin created successfully: username=admin, email=admin@pixelbeast.com")
	return nil
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return DB
}

// Close 关闭数据库连接
func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
