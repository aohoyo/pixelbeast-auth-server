# 软件升级管理系统

Go + Gin + MySQL + GORM 实现的软件升级管理服务。

## 功能特性

- **软件管理**: 创建、列表、详情、更新、删除
- **版本管理**: 发布版本、版本列表、上传安装包
- **升级服务**: 检查更新、下载版本包
- **存储插件化**: 支持 local / 阿里云OSS / 腾讯云COS / 七牛云
- **多租户预留**: 模型字段、中间件钩子
- **用量统计预留**: 下载次数统计

## 项目结构

```
license-server/
├── cmd/server/main.go          # 主入口
├── internal/
│   ├── api/                    # API处理器
│   │   ├── admin.go           # 管理后台API
│   │   ├── update.go          # 升级服务API
│   │   └── tenant.go          # 租户管理API
│   ├── service/                # 业务逻辑
│   │   ├── software.go        # 软件服务
│   │   ├── update.go          # 升级服务
│   │   └── usage.go           # 用量统计服务
│   ├── model/                  # 数据模型
│   │   └── models.go          # 所有模型定义
│   ├── middleware/             # 中间件
│   │   ├── auth.go            # 认证中间件
│   │   └── tenant.go          # 多租户中间件
│   ├── storage/                # 存储实现
│   │   ├── storage.go         # 存储接口
│   │   ├── local.go           # 本地存储
│   │   ├── aliyun.go          # 阿里云OSS
│   │   ├── tencent.go         # 腾讯云COS
│   │   └── qiniu.go           # 七牛云
│   └── db/                     # 数据库
│       └── db.go              # 数据库连接
├── config/
│   └── config.yaml            # 配置文件
├── go.mod
├── go.sum
├── Dockerfile
└── docker-compose.yml
```

## 快速开始

### 1. Docker 一键启动

```bash
docker-compose up -d
```

服务启动后：
- API服务: http://localhost:8080
- MySQL: localhost:3306

### 2. 本地开发

```bash
# 1. 启动MySQL
docker run -d \
  --name mysql \
  -e MYSQL_ROOT_PASSWORD=root123456 \
  -e MYSQL_DATABASE=license_server \
  -p 3306:3306 \
  mysql:8.0

# 2. 安装依赖
go mod download

# 3. 运行
go run cmd/server/main.go
```

## API 文档

### 认证接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/admin/login | 管理员登录 |

### 软件管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/admin/software | 创建软件 |
| GET | /api/v1/admin/software | 软件列表 |
| GET | /api/v1/admin/software/:id | 软件详情 |
| PUT | /api/v1/admin/software/:id | 更新软件 |
| DELETE | /api/v1/admin/software/:id | 删除软件 |

### 版本管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/admin/versions | 创建版本 |
| GET | /api/v1/admin/versions | 版本列表 |
| GET | /api/v1/admin/versions/:id | 版本详情 |
| PUT | /api/v1/admin/versions/:id | 更新版本 |
| DELETE | /api/v1/admin/versions/:id | 删除版本 |
| POST | /api/v1/admin/versions/:id/publish | 发布版本 |
| POST | /api/v1/admin/versions/:id/upload | 上传安装包 |

### 升级服务

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/update/check | 检查更新 |
| GET | /api/v1/update/download/:id | 下载版本包 |

## 配置说明

```yaml
server:
  port: 8080
  mode: debug  # debug/release

database:
  host: mysql
  port: 3306
  user: root
  password: root123456
  dbname: license_server

# 存储配置，支持 local/aliyun/tencent/qiniu
storage:
  type: local
  local:
    base_path: ./uploads
    base_url: http://localhost:8080/uploads
  aliyun:
    endpoint: oss-cn-hangzhou.aliyuncs.com
    access_key_id: your-key
    access_key_secret: your-secret
    bucket: your-bucket
  tencent:
    region: ap-guangzhou
    secret_id: your-id
    secret_key: your-key
    bucket: your-bucket
  qiniu:
    access_key: your-key
    secret_key: your-secret
    bucket: your-bucket
    domain: your-domain.qiniudn.com
```

## 使用示例

### 1. 登录

```bash
curl -X POST http://localhost:8080/api/v1/admin/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

### 2. 创建软件

```bash
curl -X POST http://localhost:8080/api/v1/admin/software \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "name": "MyApp",
    "slug": "myapp",
    "description": "My Application",
    "platform": "windows"
  }'
```

### 3. 创建版本

```bash
curl -X POST http://localhost:8080/api/v1/admin/versions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "software_id": 1,
    "version": "1.0.0",
    "changelog": "Initial release",
    "is_forced": false
  }'
```

### 4. 检查更新

```bash
curl "http://localhost:8080/api/v1/update/check?software_slug=myapp&current_version=0.9.0"
```

## 开发计划

- [x] 基础架构
- [x] 软件管理API
- [x] 版本管理API
- [x] 升级服务API
- [x] 存储插件化
- [x] 多租户预留
- [x] 用量统计预留
- [x] Web管理界面
- [ ] 增量更新支持
- [ ] 灰度发布
- [ ] 签名验证

## License

MIT
