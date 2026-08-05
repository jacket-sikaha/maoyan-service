# 任务：后端 Go 项目 Dockerfile + GitHub Action 镜像构建

## 目标

参照模板 `.github/workflows/docker.yml`，为 `maoyan-service` 后端 Go 项目编写多阶段 Dockerfile 和 GitHub Action CI 文件，实现 push 到 main 分支后自动构建并推送镜像到 Docker Hub。

## 项目结构

- 后端入口：`backend/cmd/server/main.go`
- Go 模块路径：`maoyan-service/backend`，Go 1.25
- 框架：Gin + GORM + PostgreSQL
- 数据库迁移文件：`backend/migrations/`

## 产出文件

### 1. `Dockerfile`（项目根目录）

- **多阶段构建**：`golang:1.25-alpine` 编译 → `alpine:3.20` 运行
- 先拷 `go.mod/go.sum` 利用层缓存，再拷源码编译
- 静态链接 `CGO_ENABLED=0`，二进制更小
- 运行镜像含 `ca-certificates`（HTTPS）、`tzdata`（Asia/Shanghai 时区）
- 拷贝 `migrations/` 和 `.env.template` 到运行镜像

### 2. `.github/workflows/docker.yml`

- **触发**：仅 `backend/**`、`Dockerfile`、workflow 自身变更时触发
- **镜像名**：`docker.io/sikaha/maoyan-service`
- **标签策略**：`latest` + git short sha + 分支名
- **缓存**：`cache-from/cache-to` 用 GitHub Actions cache 加速构建
- **Action 版本**：`docker/login-action@v3`、`docker/metadata-action@v5`、`docker/build-push-action@v6`、`actions/attest-build-provenance@v2`（相对原模板已升级）

## 前置条件

- 仓库 Secrets 需配置 `DOCKER_USERNAME` 和 `DOCKER_PASSWORD`
- Docker Hub 仓库名：`sikaha/maoyan-service`
