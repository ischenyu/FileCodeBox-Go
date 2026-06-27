<div align="center">

# FileCodeBox

### 文件快递柜 - 匿名口令分享文本和文件

<img src="https://fastly.jsdelivr.net/gh/vastsa/FileCodeBox@V1.6/static/banners/img_1.png" alt="FileCodeBox Logo" width="400">

像拿快递一样取文件，无需注册，输入口令即可获取
---
</div>


## 📝 项目简介

FileCodeBox-Go 是一个轻量级的文件分享工具，基于 **Gin + Vue3** 开发。用户可以通过简单的方式匿名分享文本和文件，接收者只需输入提取码即可获取内容——就像从快递柜取出快递一样简单。本项目是 [FileCodeBox](https://github.com/vastsa/FileCodeBox) 的 Go 语言重写版本。

### 应用场景

| 场景 | 描述 |
|------|------|
| 📁 **临时文件分享** | 快速分享文件，无需注册登录 |
| 📝 **代码片段分享** | 分享代码、配置文件等文本内容 |
| 🕶️ **匿名文件传输** | 保护隐私的点对点传输 |
| 🔄 **跨设备传输** | 在不同设备间快速同步文件 |
| 💾 **临时存储** | 支持自定义过期时间的云存储 |
| 🌐 **私有服务** | 搭建企业或个人专属分享服务 |


## 🖼️ 界面预览

> 前端源码仓库：[2024主题](https://github.com/vastsa/FileCodeBoxFronted) | [2023主题](https://github.com/vastsa/FileCodeBoxFronted2023)

<details open>
<summary><b>🎨 新版界面 (2024)</b></summary>
<br>
<div align="center">
<table>
<tr>
<td><img src="./.github/images/img_7.png" alt="文件上传"></td>
<td><img src="./.github/images/img_8.png" alt="文本分享"></td>
</tr>
<tr>
<td><img src="./.github/images/img_10.png" alt="文件管理"></td>
<td><img src="./.github/images/img_9.png" alt="系统设置"></td>
</tr>
<tr>
<td><img src="./.github/images/img_11.png" alt="移动端"></td>
<td><img src="./.github/images/img_12.png" alt="深色模式"></td>
</tr>
</table>
</div>
</details>

<details>
<summary><b>📦 经典界面 (2023)</b></summary>
<br>
<div align="center">
<table>
<tr>
<td><img src="./.github/images/img.png" alt="首页"></td>
<td><img src="./.github/images/img_1.png" alt="上传"></td>
</tr>
<tr>
<td><img src="./.github/images/img_2.png" alt="管理"></td>
<td><img src="./.github/images/img_3.png" alt="设置"></td>
</tr>
</table>
</div>
</details>

---

## 🚀 快速开始

### Docker 部署（推荐）

```bash
docker run -d --restart always -p 12345:12345 -v /opt/FileCodeBox:/app/data --name filecodebox filecodebox-go:latest
```

### 环境变量说明

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `HOST` | `0.0.0.0` | 监听地址 |
| `PORT` | `12345` | 服务端口 |
| `LOG_LEVEL` | `info` | 日志级别：`debug` / `info` / `warn` / `error` |

---

## 📖 使用指南

### 基础操作

| 操作 | 步骤 |
|------|------|
| **分享文件** | 打开网页 → 选择/拖拽文件 → 设置有效期 → 获取提取码 |
| **获取文件** | 打开网页 → 输入提取码 → 下载文件或查看文本 |
| **管理后台** | 首次访问站点完成初始化 → 访问 `/#/admin` → 输入初始化时设置的密码 |

### 命令行使用（curl）

<details>
<summary><b>点击展开 curl 使用示例</b></summary>

**上传文件**

```bash
# 基础上传（默认 1 天有效期）
curl -X POST "http://localhost:12345/share/file/" \
  -F "file=@/path/to/file.txt"

# 指定 1 小时有效期
curl -X POST "http://localhost:12345/share/file/" \
  -F "file=@/path/to/file.txt" \
  -F "expire_value=1" \
  -F "expire_style=hour"

# 指定下载 10 次后过期
curl -X POST "http://localhost:12345/share/file/" \
  -F "file=@/path/to/file.txt" \
  -F "expire_value=10" \
  -F "expire_style=count"
```

**分享文本**

```bash
curl -X POST "http://localhost:12345/share/text/" \
  -F "text=要分享的文本内容"
```

**下载文件**

```bash
curl -L "http://localhost:12345/share/select/?code=提取码" -o filename
```

**有效期参数**

| `expire_style` | 说明 |
|----------------|------|
| `day` | 天数 |
| `hour` | 小时 |
| `minute` | 分钟 |
| `count` | 下载次数 |
| `forever` | 永久有效 |

**返回示例**

```json
{
  "code": 200,
  "msg": "success",
  "detail": {
    "code": "abcd1234",
    "name": "file.txt"
  }
}
```

**需要认证时**（管理员关闭游客上传后）

```bash
# 1. 获取 token
curl -X POST "http://localhost:12345/admin/login" \
  -H "Content-Type: application/json" \
  -d '{"password": "<初始化时设置的管理员密码>"}'

# 2. 携带 token 上传
curl -X POST "http://localhost:12345/share/file/" \
  -H "Authorization: Bearer <token>" \
  -F "file=@/path/to/file.txt"
```

</details>

---

## 🛠 开发指南

### 项目结构

```
FileCodeBox-Go/
├── main.go                              # 入口：配置→DB→路由→后台任务→HTTP服务
├── go.mod / go.sum                      # 依赖管理
├── Dockerfile                           # 多阶段构建（golang:1.23-alpine → alpine:3.21）
├── themes/                              # 前端主题目录（构建时填充）
├── data/                                # 运行时数据目录（SQLite DB + 文件存储）
└── internal/
    ├── utils/                           # 工具包
    │   ├── response.go                  # APIResponse 统一响应
    │   ├── version.go                   # 版本号 (3.0.0)
    │   ├── password.go                  # SHA256+salt 密码哈希/验证
    │   ├── code.go                      # 随机提取码生成
    │   ├── file.go                      # 文件名安全处理 + GetFileURL
    │   └── token.go                     # 手写 JWT 创建/验证
    ├── config/settings.go               # 运行时配置（Settings 类 + DefaultConfig）
    ├── database/database.go             # SQLite 初始化/GORM 迁移/启动锁
    ├── models/models.go                 # 4 个 GORM 模型（FileCodes/UploadChunk/KeyValue/PresignUploadSession）
    ├── storage/
    │   ├── interface.go                 # FileStorageInterface 接口
    │   ├── registry.go                  # 工厂注册表（5种后端）
    │   ├── local.go                     # 本地文件系统
    │   ├── s3.go                        # AWS S3 兼容（aws-sdk-go-v2）
    │   ├── onedrive.go                  # OneDrive（基础实现）
    │   ├── opendal.go                   # OpenDAL（基础实现）
    │   └── webdav.go                    # WebDAV（HTTP PUT/GET/DELETE/MKCOL）
    ├── middleware/
    │   ├── auth.go                      # JWT 认证（AdminRequired + ShareRequiredLogin）
    │   ├── ratelimit.go                 # IP 频率限制（上传/取件错误）
    │   └── setup.go                     # 系统初始化拦截（428 → /setup）
    ├── handlers/
    │   ├── setup.go                     # /setup 初始化向导（完整 HTML 页面）
    │   ├── public.go                    # / /health /robots.txt /assets /api/v1/config
    │   ├── share.go                     # /share/* 文本/文件分享/获取/下载
    │   ├── chunk.go                     # /chunk/* 切片上传（init/upload/complete/cancel）
    │   ├── presign.go                   # /presign/* 预签名上传（S3直传/代理）
    │   └── admin.go                     # /admin/* 登录/仪表盘/文件管理/配置
    ├── server/server.go                 # Gin 路由注册 + CORS 中间件
    └── tasks/cleanup.go                 # 后台任务（过期文件清理/未完成上传清理）
```

### 本地开发

```bash
# 1. 安装依赖
go mod tidy

# 2. 启动服务
go run .
```

### 技术栈

| 类别 | 技术 |
|------|------|
| **后端框架** | Gin 1.x
| **数据库** | SQLite + GORM |
| **语言** | Go 1.23+ |
| **对象存储** | S3 协议 / OneDrive / OpenDAL / WebDAV |
| **认证** | 手写 JWT (HMAC-SHA256) |
| **前端框架** | Vue 3 + Element Plus + Vite |
| **运行环境** | Go 可执行文件 / Node.js 18+ |
| **容器化** | Docker 多阶段构建 |

---

## ❓ 常见问题

<details>
<summary><b>如何修改上传大小限制？</b></summary>

在管理面板中修改 `uploadSize` 配置项。如果使用 Nginx 反向代理，还需修改 `client_max_body_size`。
</details>

<details>
<summary><b>如何配置存储引擎？</b></summary>

在管理面板中选择存储引擎类型并配置相应参数。支持本地存储、S3、OneDrive、OpenDAL 等。
</details>

<details>
<summary><b>如何备份数据？</b></summary>

备份 `data` 目录即可，包含数据库和上传的文件。
</details>

<details>
<summary><b>如何修改管理员密码？</b></summary>

登录管理面板后，在系统设置中修改 `adminPassword` 配置项。
</details>

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

```bash
# 1. Fork 并克隆
git clone https://github.com/your-username/FileCodeBox-Go.git

# 2. 创建分支
git checkout -b feature/your-feature

# 3. 提交更改
git commit -m "feat: add your feature"

# 4. 推送并创建 PR
git push origin feature/your-feature
```

---


## 📜 免责声明

本项目开源仅供学习交流使用，不得用于任何违法用途，否则后果自负，与作者无关。使用本项目时请保留项目地址和版权信息。

---

<div align="center">

**如果觉得项目不错，欢迎 ⭐ Star 支持！**

</div>
