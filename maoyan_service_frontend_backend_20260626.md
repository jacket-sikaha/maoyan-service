# 猫眼票价监控 - 全栈设计回顾

## 2026-06-26 改动摘要

### 后端新增（基于 todo.md 更新）
- **用户认证系统**：model 新增 `User` 表、DTO 结构；repository 新增 `UserRepo`；service 新增 `AuthService`（JWT 签发/验证 + bcrypt）
- **订阅按用户管理**：`Subscription` 表移除 `email` 字段，改为 `user_id` FK `REFERENCES users(id)`
- **中间件**：`middleware/auth.go` - Bearer Token 解析 + 注入 `userID` 到 `gin.Context`
- **controller**：新增 `AuthController`（Register/Login），订阅相关端点全部走认证路由组
- **main.go**：Auth 路由组注册、User 模型迁移、JWT_SECRET 环境变量
- **migrations**：新建 `users` 表，subscriptions 表添加 `user_id FK` 替代原 `email` 字段

### 前端全新创建（React + TypeScript + Vite）
| 文件 | 功能 |
|---|---|
| `api/client.ts` | Axios 实例，拦截器自动注入 JWT，401 自动清 token |
| `api/endpoints.ts` | 全部后端 API 对接函数 |
| `store/auth.tsx` | AuthContext：登录/注册/登出/持久化 |
| `components/Navbar.tsx` | 导航栏：首页/订阅/登录/用户信息 |
| `pages/HomePage.tsx` | 城市选择 → 电影搜索/热映 → 选电影 → 排片票价表格 → 订阅 |
| `pages/LoginPage.tsx` | 登录/注册切换表单 |
| `pages/SubscriptionsPage.tsx` | 订阅列表 + 开关 + 详情展开（价格走势图 recharts + 行情表）+ CSV 导出 |
| `index.css` | 完整设计系统（变量/按钮/卡片/表格/Toggle/响应式） |

### 技术栈
- **后端**：Go 1.22 + Gin + GORM + Supabase PostgreSQL + robfig/cron + JWT + bcrypt
- **前端**：React 18 + TypeScript + Vite + React Router + Axios + Recharts
- **图表**：recharts LineChart（票价走势）
- **通信**：Vite 代理 `/api` → `localhost:8080`

### 启动步骤
1. 后端：`cd backend && cp .env.template .env`（填入 DB_DSN、SMTP、JWT_SECRET）→ `go run ./cmd/server/`
2. 前端：`cd frontend && npm install && npm run dev`（需在新终端中执行，当前 PowerShell 环境中 npm 存在兼容问题）
3. 访问 `http://localhost:3000`

### 已知事项
- 当前 PowerShell 环境中 `npm` 因环境变量注入钩子中包含括号（中文编码残留）导致命令解析异常；需切换到 CMD 或干净的 PowerShell 窗口执行 `npm install`
- HomePage 中订阅按钮传 `cinema_id: 0` 占位，需确认后端 `/api/shows` 接口应答是否包含 `cinema_id` 字段后修正
