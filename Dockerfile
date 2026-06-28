# syntax=docker/dockerfile:1
# --- 前端构建阶段 ---
FROM node:22-alpine AS frontend-builder

WORKDIR /frontend

# 安装 pnpm
RUN npm install -g pnpm@10

# 先复制依赖清单以利用 Docker 层缓存
COPY frontend/package.json frontend/pnpm-lock.yaml ./

# 安装依赖（缓存 pnpm store 加速重复构建）
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --prefer-offline

# 复制其余源码
COPY frontend/ .

# 构建
RUN pnpm build-only --outDir /assets-build

# --- Go 构建阶段 ---
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# 先复制依赖清单以利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制 Go 源码（排除 frontend/，通过 .dockerignore）
COPY . .

# 复制前端构建产物
COPY --from=frontend-builder /assets-build ./assets

# 编译（匹配目标平台架构，避免 QEMU 模拟）
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o filecodebox .

# --- 最小运行镜像 ---
FROM alpine:3.21

LABEL author="ischenyu"
LABEL email="sparkchenyu@outlook.com"
LABEL description="FileCodeBox-Go"

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata && \
    ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo 'Asia/Shanghai' > /etc/timezone

COPY --from=builder /build/filecodebox .

RUN mkdir -p /app/data

ENV HOST="0.0.0.0" \
    PORT=12345 \
    LOG_LEVEL="info"

EXPOSE 12345

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:${PORT}/health || exit 1

CMD ["./filecodebox"]
