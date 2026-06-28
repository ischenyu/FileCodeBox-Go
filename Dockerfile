# 与 Python 版 Dockerfile 保持一致的环境变量和端口

# --- 前端构建阶段 ---
FROM node:22-alpine AS frontend-builder

WORKDIR /frontend

# 安装 pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# 复制前端源码（由 CI checkout 到 frontend/ 目录）
COPY frontend/ .

# 安装依赖并构建
RUN pnpm install && \
    pnpm build-only --outDir /assets-build

# --- Go 构建阶段 ---
FROM golang:1.26-alpine AS builder

# 安装构建所需工具
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# 复制 go.mod / go.sum 并下载依赖（利用 Docker 缓存）
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 复制前端构建产物到 assets 目录
COPY --from=frontend-builder /assets-build ./assets

# 编译（静态链接，去除调试信息，减小体积）
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o filecodebox .

# --- 最小运行镜像 ---
FROM alpine:3.21

LABEL author="ischenyu"
LABEL email="sparkchenyu@outlook.com"
LABEL description="FileCodeBox-Go"

WORKDIR /app

# 安装 ca 证书（HTTPS 请求需要）和时区数据
RUN apk add --no-cache ca-certificates tzdata

# 设置时区
RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo 'Asia/Shanghai' > /etc/timezone

# 从构建阶段复制二进制文件
COPY --from=builder /build/filecodebox .

# 创建数据目录
RUN mkdir -p /app/data

# 环境变量
ENV HOST="0.0.0.0" \
    PORT=12345 \
    LOG_LEVEL="info"

EXPOSE 12345

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:${PORT}/health || exit 1

# 启动命令
CMD ["./filecodebox"]
