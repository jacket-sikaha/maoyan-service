# ── Build stage ──
FROM golang:1.25-alpine AS builder

# 安装 git（go mod 私有依赖可能需要）
RUN apk add --no-cache git

WORKDIR /app

# 先拷依赖文件，利用 Docker 层缓存
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

# 拷源码
COPY backend/ ./backend/

# 编译二进制（静态链接，适合 scratch/alpine 运行）
RUN cd backend && CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=$(git rev-parse --short HEAD 2>/dev/null || echo dev)" \
    -o /app/bin/server \
    ./cmd/server

# ── Runtime stage ──
FROM alpine:3.20

# 时区 + ca-cert（HTTPS 请求需要）
RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# 拷编译产物 + 迁移文件
COPY --from=builder /app/bin/server .
COPY backend/migrations ./migrations
COPY backend/.env.template .env.template

EXPOSE 8080

ENTRYPOINT ["./server"]
