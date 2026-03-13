// Package main 软件升级管理系统主入口
//
// @title           软件升级管理系统 API
// @version         1.0
// @description     支持多租户、多存储后端的软件升级管理服务
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@example.com

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gopkg.in/yaml.v3"

	_ "license-server/docs" // swagger docs

	"license-server/internal/api"
	"license-server/internal/db"
	"license-server/internal/redis"
	"license-server/internal/service"
	"license-server/internal/storage"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig     `yaml:"server"`
	Database db.Config        `yaml:"database"`
	Redis    redis.Config     `yaml:"redis"`
	Storage  storage.Config   `yaml:"storage"`
	JWT      JWTConfig        `yaml:"jwt"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
}

func main() {
	// 加载配置
	cfg, err := loadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 设置Gin模式
	gin.SetMode(cfg.Server.Mode)

	// 初始化数据库
	database, err := db.Init(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}
	defer db.Close()

	// 初始化Redis
	_, err = redis.Init(&cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to init Redis: %v", err)
	}
	defer redis.Close()

	// 初始化存储服务（纯平台模式：租户自配存储）
	// 注意：不再使用全局存储配置，改为每个租户独立配置
	storageService := service.NewStorageService(database, cfg.JWT.Secret)

	// 创建服务
	softwareService := service.NewSoftwareService(database)
	updateService := service.NewUpdateService(database, storageService)
	usageService := service.NewUsageService(database)
	uploadService := service.NewUploadService(database, storageService)
	fileService := service.NewFileService(database, storageService)
	signingService := service.NewSigningService(database)

	// 创建处理器
	adminHandler := api.NewAdminHandler(softwareService, updateService, usageService, signingService, cfg.JWT.Secret)
	updateHandler := api.NewUpdateHandler(updateService, usageService, softwareService)
	tenantHandler := api.NewTenantHandler(database, cfg.JWT.Secret)
	storageHandler := api.NewStorageHandler(storageService)
	uploadHandler := api.NewUploadHandler(uploadService)
	fileHandler := api.NewFileHandler(fileService, database)

	// 创建路由
	r := gin.Default()

	// 配置 CORS 中间件
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API v1 路由组
	// 新路由结构：
	// - /software/* - 软件管理（需认证）
	// - /version/* - 版本管理（需认证）
	// - /stats/* - 统计（需认证）
	// - /update/* - 公开接口（API Key）
	// - /tenant/* - 租户管理（需管理员权限）
	// - /storage/* - 存储配置（需认证）
	apiV1 := r.Group("/api/v1")
	{
		// 注册各模块路由
		adminHandler.RegisterRoutes(apiV1)
		updateHandler.RegisterRoutes(apiV1)
		tenantHandler.RegisterRoutes(apiV1)
		storageHandler.RegisterRoutes(apiV1, cfg.JWT.Secret)
		uploadHandler.RegisterRoutes(apiV1, cfg.JWT.Secret)
		fileHandler.RegisterRoutes(apiV1, cfg.JWT.Secret)
	}


	// 创建HTTP服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: r,
	}

	// 优雅关闭
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Printf("Server started on port %d", cfg.Server.Port)
	log.Printf("Swagger UI: http://localhost:%d/swagger/index.html", cfg.Server.Port)
	log.Printf("API documentation: http://localhost:%d/api/v1/docs", cfg.Server.Port)

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// 优雅关闭，设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// loadConfig 加载配置文件
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}