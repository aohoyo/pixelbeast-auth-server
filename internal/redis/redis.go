// Package redis Redis连接管理
// 提供Redis连接初始化和全局访问方法
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient 全局Redis客户端实例
var RedisClient *redis.Client

// Config Redis配置结构体
type Config struct {
	Host     string `yaml:"host"`     // Redis服务器地址
	Port     int    `yaml:"port"`     // Redis服务器端口
	Password string `yaml:"password"` // Redis密码（无密码为空字符串）
	DB       int    `yaml:"db"`       // Redis数据库编号
}

// Init 初始化Redis连接
// 根据配置创建Redis客户端并测试连接
// 参数：
//   - cfg: Redis配置信息
// 返回：
//   - *redis.Client: Redis客户端实例
//   - error: 初始化过程中的错误
func Init(cfg *Config) (*redis.Client, error) {
	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		// 设置连接池参数
		PoolSize:     10,             // 连接池大小
		MinIdleConns: 2,              // 最小空闲连接数
		MaxRetries:   3,              // 最大重试次数
		DialTimeout:  5 * time.Second, // 连接超时时间
		ReadTimeout:  3 * time.Second, // 读取超时时间
		WriteTimeout: 3 * time.Second, // 写入超时时间
	})

	// 使用context进行连接测试
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 测试Redis连接
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	RedisClient = client
	return client, nil
}

// GetRedis 获取全局Redis客户端实例
// 如果Redis未初始化，返回nil
// 返回：
//   - *redis.Client: Redis客户端实例
func GetRedis() *redis.Client {
	return RedisClient
}

// Close 关闭Redis连接
// 在应用关闭时调用，释放连接池资源
// 返回：
//   - error: 关闭过程中的错误
func Close() error {
	if RedisClient != nil {
		if err := RedisClient.Close(); err != nil {
			return fmt.Errorf("failed to close Redis connection: %w", err)
		}
		RedisClient = nil
	}
	return nil
}

// HealthCheck Redis健康检查
// 用于检查Redis连接是否正常
// 返回：
//   - error: 健康检查过程中的错误，nil表示健康
func HealthCheck() error {
	if RedisClient == nil {
		return fmt.Errorf("Redis client is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis health check failed: %w", err)
	}

	return nil
}
