# 猫眼票价监控项目 — 全量代码注释 2026-06-27

## 任务
为 maoyan-service 后端（Go）和前端（React+TS）的核心文件添加中文注释，覆盖类型定义、方法职责、业务逻辑、API 路由等。

## 改动范围

### 后端 (backend/)
| 文件 | 改动 |
|------|------|
| `internal/model/models.go` | 14 struct/DTO 注释：City/District/ShowSnapshot/Subscription/User 及 Auth DTO |
| `internal/repository/repos.go` | 接口+方法注释：ShowSnapshotRepo Upsert 四步逻辑、SubscriptionRepo 去重/鉴权 |
| `internal/service/data_service.go` | 12处方法注释：AuthService JWT、DataService 缓存策略/检查-and-Notify/查价 |
| `internal/middleware/auth.go` | AuthRequired 中间件流程注释 |
| `internal/controller/maoyan_controller.go` | 6处 handler 注释：公开/认证路由分组、参数说明 |
| `internal/scheduler/scheduler.go` | Scheduler 类型注释（robfig/cron 秒级） |
| `internal/pkg/maoyan.go` | 5处注释：双客户端/GetCinemaList分页/stonefont解码/extractPrice 多策略 |
| `internal/pkg/email.go` | EmailNotifier 类型注释（gomail SMTP） |
| `internal/pkg/csv_exporter.go` | 完整重写注释：ExportShowsToCSV/ExportSnapshotsToCSV + safeFileName |
| `internal/pkg/stonefont.go` | **完整重写**：包级设计文档（woff解码背景+5步策略） + 所有函数注释 |
| `cmd/server/main.go` | **分段重构注释**：9步启动流程 + initDB/seedCityData 注释 |
| `cmd/server/city_map.go` | **重新生成**：来自 city_data_clean.json 的 1094 条映射，添加包级注释 |

### 前端 (frontend/src/)
| 文件 | 改动 |
|------|------|
| `api/client.ts` | Axios 拦截器注释（JWT注入 + 401自动清理） |
| `api/endpoints.ts` | 分段注释 + 每个端点说明 |
| `store/auth.tsx` | AuthContext 状态管理模式 + useAuth hook 注释 |
| `components/Navbar.tsx` | 已登录/未登录两种状态渲染说明 |
| `pages/HomePage.tsx` | 四步交互流程注释（城市→搜索→排片→订阅） |
| `pages/LoginPage.tsx` | 登录/注册切换表单注释 |
| `pages/SubscriptionsPage.tsx` | 订阅管理页功能说明（列表+Toggle+图表+CSV） |
| `App.tsx` | 路由根组件注释 |

### 修复
- `city_map.go` 重新从 `py/city_data_clean.json` 生成（1094条），之前文件丢失
- 修正 main.go 中 `pkg.CityMap` → `CityMap`（同 package main 无需前缀）
- 工具文件 `backend/tools/gen_citymap.go`（build-tag ignored）保留用于以后重新生成

## 编译验证
- `go build ./cmd/server/` ✅ 通过
- `go vet ./...` ✅ 无警告
