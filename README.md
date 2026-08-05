# 🎬 Maoyan Service

> 猫眼电影票价监控与低价订阅通知系统 —— Go 后端 + React 前端，按影院级采集任务调度，自动追踪票价变化并在命中目标价时邮件通知。

## ✨ 核心特性

- **🎟️ 票价查询** — 按城市/区县/电影实时查询全区影院排片票价，支持距离排序、CSV 导出
- **🔔 低价订阅** — 设定目标价，影院票价低于阈值时自动邮件通知（支持多邮箱）
- **🏪 影院级采集** — 每个影院一个采集任务（CrawlTask），一次采集复用到该影院下所有订阅规则，避免重复请求
- **📊 价格快照** — 每次采集写入快照批次 + 明细，完整保留票价历史轨迹
- **📉 价格趋势** — 订阅详情页展示历史价格折线图，采集记录仪表盘按电影维度统计最低/均价/最高价
- **🛡️ 防骚扰机制** — 首次采集仅记录基准价不通知、12h 冷却期、乐观锁防并发重复通知
- **📝 通知日志** — 完整记录每次通知（成功/失败/跳过），支持按时间范围 + 状态分页查询
- **⏱️ 自动调度** — 内置 cron 定时器，按任务间隔自动轮询到期采集任务
- **🔐 JWT 认证** — 用户注册/登录/Token 校验，订阅管理需登录

## 🏗️ 系统架构

```
                    ┌─────────────┐
                    │   React UI  │  Ant Design + Vite
                    └──────┬──────┘
                           │ HTTP / JWT
                    ┌──────▼──────┐
                    │  Gin Router │  Controller 层
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  Service    │  业务逻辑（订阅创建、采集调度、通知判断）
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ Repository  │  GORM 数据访问层
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
        ┌─────▼─────┐ ┌───▼────┐ ┌────▼─────┐
        │ PostgreSQL │ │ Maoyan │ │ SMTP     │
        │ (Supabase) │ │ Crawler│ │ Mailer   │
        └────────────┘ └────────┘ └──────────┘
```

### 采集调度流程

```
cron 触发 → 查询到期 CrawlTask → 逐影院调用猫眼 API
    → 写入 PriceSnapshot（快照） → 匹配该影院下所有 Subscription
    → 命中目标价 → 发邮件 + 写 NotifyLog → 更新任务 next_run_at
```

## 🛠️ 技术栈

### 后端

| 组件 | 选型 |
|------|------|
| 语言 | Go 1.25 |
| Web 框架 | Gin |
| 数据库 | PostgreSQL 15（Supabase） |
| ORM | GORM v2 + pgx |
| 调度 | robfig/cron v3 |
| 配置 | spf13/viper |
| 日志 | log/slog (JSON) |
| 邮件 | gopkg.in/gomail.v2 |
| 认证 | golang-jwt/jwt v5 |
| ID | google/uuid |

### 前端

| 组件 | 选型 |
|------|------|
| 框架 | React 18 + TypeScript |
| UI 库 | Ant Design v6 |
| 构建 | Vite 5 |
| 路由 | React Router v6 |
| HTTP | Axios |
| 图表 | Recharts |

## 📦 项目结构

```
maoyan-service/
├── backend/
│   ├── cmd/server/
│   │   ├── main.go              # 入口：DB 初始化、DI 注入、路由注册、调度启动
│   │   └── city_map.go          # 1094 城市数据种子
│   ├── internal/
│   │   ├── controller/          # 传输层：参数校验 + 响应封装
│   │   │   └── maoyan_controller.go
│   │   ├── service/             # 业务逻辑层
│   │   │   └── data_service.go  # 订阅管理、采集调度、通知判断
│   │   ├── repository/          # 数据访问层（接口 + GORM 实现）
│   │   │   └── repos.go
│   │   ├── model/               # GORM 实体 + DTO 定义
│   │   │   └── models.go
│   │   ├── middleware/          # JWT 认证中间件
│   │   │   └── auth.go
│   │   ├── pkg/                 # 内部工具包
│   │   │   ├── maoyan.go        #   猫眼爬虫（城市/区县/影院/排片）
│   │   │   ├── stonefont.go     #   动态字体解码引擎（纯 Go SFNT 解析）
│   │   │   ├── email.go         #   邮件通知
│   │   │   └── csv_exporter.go  #   CSV 导出
│   │   └── scheduler/           # cron 定时调度器
│   │       └── scheduler.go
│   ├── migrations/
│   │   └── 001_init.sql         # 完整 DDL（7 张表 + 索引）
│   ├── go.mod
│   └── .env.template
├── frontend/
│   ├── src/
│   │   ├── api/                 # Axios 客户端 + 接口定义
│   │   │   ├── client.ts
│   │   │   └── endpoints.ts
│   │   ├── components/          # 复用组件
│   │   │   ├── Navbar.tsx       #   导航栏
│   │   │   ├── PinyinCityPicker.tsx  # 拼音城市选择器
│   │   │   └── SubscriptionDrawer.tsx # 订阅抽屉（创建/编辑）
│   │   ├── pages/               # 页面
│   │   │   ├── HomePage.tsx     #   首页（热映电影）
│   │   │   ├── MoviePricePage.tsx    # 票价查询（含订阅入口）
│   │   │   ├── SubscriptionsPage.tsx # 订阅管理列表
│   │   │   ├── SubscriptionDetailPage.tsx # 订阅详情（价格趋势图）
│   │   │   ├── CinemaCrawlRecordsPage.tsx  # 采集记录仪表盘
│   │   │   ├── SubscriptionHistoryPage.tsx # 通知日志（分页+筛选）
│   │   │   ├── PriceChangesPage.tsx  # 票价变化对比
│   │   │   └── LoginPage.tsx    # 登录/注册
│   │   ├── store/
│   │   │   └── auth.tsx         # Auth Context
│   │   ├── App.tsx              # 路由根组件
│   │   └── main.tsx
│   ├── package.json
│   └── vite.config.ts
└── README.md
```

## 🚀 快速开始

### 环境要求

- Go 1.25+
- Node.js 18+
- PostgreSQL 15+（推荐 [Supabase](https://supabase.com) 免费托管）

### 后端

```bash
cd backend

# 1. 配置环境变量
cp .env.template .env
# 编辑 .env，填入数据库连接串和 SMTP 配置

# 2. 安装依赖
go mod tidy

# 3. 启动（自动建表 + 城市数据种子）
go run cmd/server/main.go
```

### 前端

```bash
cd frontend

# 1. 安装依赖
pnpm install   # 或 npm install

# 2. 启动开发服务器
pnpm dev       # 或 npm run dev

# 3. 生产构建
pnpm build
```

### 验证

```bash
# 健康检查
curl http://localhost:8080/health

# 获取城市列表
curl http://localhost:8080/api/cities

# 查询排片票价
curl "http://localhost:8080/api/shows?city_id=1&movie_id=344&district_id=-1&lat=39.9&lng=116.4"
```

## 📡 API 文档

所有接口返回统一格式：`{ "code": 0, "msg": "success", "data": {...} }`

### 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| POST | `/api/auth/register` | 用户注册 |
| POST | `/api/auth/login` | 用户登录 |
| GET | `/api/cities` | 城市列表（1094 城市） |
| GET | `/api/districts?city_id=1` | 区县 + 商圈（含影院数） |
| GET | `/api/movies/hot?city_id=1` | 热映电影 |
| GET | `/api/movies/search?keyword=xxx` | 搜索电影 |
| GET | `/api/shows?city_id=&movie_id=&district_id=&area_id=&lat=&lng=&max=` | 查询排片票价 |

### 需登录接口（JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/subscriptions` | 创建订阅（自动创建影院采集任务） |
| GET | `/api/subscriptions` | 我的订阅列表 |
| GET | `/api/subscriptions/:id` | 订阅详情（当前行情 + 价格趋势图） |
| PATCH | `/api/subscriptions/:id/toggle` | 启用/停用订阅 |
| PUT | `/api/subscriptions/:id` | 更新订阅（目标价、备注） |
| DELETE | `/api/subscriptions/:id` | 删除订阅 |
| GET | `/api/subscriptions/logs?page=&page_size=&start_date=&end_date=&status=` | 通知日志（分页 + 时间/状态筛选） |
| GET | `/api/subscriptions/cinemas` | 已订阅影院列表 |
| GET | `/api/subscriptions/cinema-movies` | 已订阅影院+电影组合 |
| POST | `/api/subscriptions/:id/refresh` | 手动刷新订阅票价行情 |
| GET | `/api/subscriptions/:id/export` | 导出订阅历史 CSV |
| GET | `/api/subscriptions/:id/crawl-records` | 采集记录仪表盘 |
| GET | `/api/subscriptions/:id/snapshots/:snapshot_id/shows` | 快照明细 |
| GET | `/api/shows/export` | 导出查询结果 CSV |
| GET | `/api/price-changes?cinema_id=` | 票价变化趋势 |
| POST | `/api/admin/fetch` | 手动触发全量采集（管理员） |
| POST | `/api/admin/crawl/:cinema_id` | 手动触发单影院采集 |

## 🗄️ 数据模型

系统采用 7 张核心表，**不使用物理外键**（仅逻辑外键 + 索引），应用层维护关联：

```
cinema                影院基础表（系统中心对象）
  │
  ├── crawl_task      影院采集任务（一影院一任务，唯一约束）
  │     │
  │     ├── execute_log     每次执行日志（耗时、采集数、匹配数、通知数）
  │     │
  │     └── price_snapshot  票价快照（原始 JSON + 按电影统计）
  │
  ├── subscription    订阅规则（影院×电影×邮箱，目标价触发通知）
  │
  └── notify_log      通知日志（每次邮件发送记录，含成功/失败/跳过状态）
```

**设计要点：**
- 一个影院只有一个 `crawl_task`（`cinema_id` 唯一约束），采集结果被该影院下所有订阅共享
- 订阅创建时自动确保对应 `crawl_task` 存在
- 删除订阅时不立即删任务（仅当影院下无订阅时禁用/删除任务）
- `notify_log` 同时支持按时间范围和通知状态筛选

## ⚙️ 配置项

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `PORT` | 服务端口 | `8080` |
| `DB_DSN` | PostgreSQL 连接串 | — |
| `SMTP_HOST` | SMTP 服务器地址 | — |
| `SMTP_PORT` | SMTP 端口 | — |
| `SMTP_USER` | SMTP 用户名 | — |
| `SMTP_PASS` | SMTP 密码 | — |
| `JWT_SECRET` | JWT 签名密钥 | — |
| `MAOYAN_FETCH_INTERVAL_MIN` | 采集间隔（分钟） | `30` |
| `MAOYAN_REQUEST_DELAY_MIN` | 请求延迟下限（秒） | `1.0` |
| `MAOYAN_REQUEST_DELAY_MAX` | 请求延迟上限（秒） | `2.0` |

## 🔒 安全设计

- **无物理外键** — 所有表通过应用层维护逻辑外键，避免级联删除风险
- **JWT 认证** — 订阅管理接口需登录，Token 7 天有效期
- **乐观锁** — 通知触发时通过 `CompareAndUpdateTriggeredPrice` 原子更新，防止并发重复通知
- **密码哈希** — bcrypt 存储，JSON 响应中隐藏 `password_hash`
- **请求节流** — 猫眼 API 调用间随机延迟，避免被封禁

## 📜 License

MIT
