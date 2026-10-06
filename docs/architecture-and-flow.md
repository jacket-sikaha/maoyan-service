# 猫眼服务中台 — 架构思维导图 & 业务流程图

> 生成日期: 2026-08-12 | 基于最新代码结构

---

## 一、架构思维导图

```mermaid
mindmap
  root((🎬 猫眼服务中台))
    🖥️ 前端 React 18 + TS
      UI 框架 Ant Design v6
      构建 Vite 5
      HTTP Axios
      图表 Recharts
      路由 React Router v6
      状态 Auth Context
      📄 页面 8 个
        首页 HomePage.tsx 热映电影
        票价查询 MoviePricePage.tsx 含订阅入口
        订阅列表 SubscriptionsPage.tsx 增删改启停
        订阅详情 SubscriptionDetailPage.tsx 价格折线图
        采集记录 CinemaCrawlRecordsPage.tsx 仪表盘
        通知日志 SubscriptionHistoryPage.tsx 分页筛选
        票价变化 PriceChangesPage.tsx 趋势对比
        登录 LoginPage.tsx 注册+登录
      🧩 组件 3 个
        Navbar.tsx 导航栏
        PinyinCityPicker.tsx 拼音城市选择器
        SubscriptionDrawer.tsx 订阅抽屉
    ⚙️ 后端 Go 1.25 + Gin
      🔗 入口 cmd/server/main.go
        日志 slog JSON 双写
        配置 viper .env
        DB GORM PostgreSQL
        DI 依赖注入
        路由注册 + CORS
        调度启动 + 优雅关闭
      🧱 分层架构
        Controller 层
          参数校验 + 响应封装
          JWT 鉴权中间件
          公开路由 8 个
          需登录路由 16 个
        Service 层
          AuthService JWT签发校验 bcrypt
          DataService 订阅管理 采集调度 通知判断
        Repository 层
          GORM 实现 接口抽象
          CinemaRepo UserRepo SubscriptionRepo
          CrawlTaskRepo ExecuteLogRepo
          PriceSnapshotRepo NotifyLogRepo
          CompareAndUpdate 乐观锁
        Model 层
          GORM 实体 8 张核心表
          DTO Request/Response 结构体
          TableName 手动映射
      🔧 内部工具 internal/pkg
        maoyan.go 猫眼爬虫 城市/区县/影院/排片API
        stonefont.go 动态字体解码 SFNT解析
        email.go SMTP邮件通知 gomail.v2
        csv_exporter.go CSV导出
      ⏱️ 调度 internal/scheduler
        robfig/cron v3
        按间隔轮询到期 CrawlTask
    🗄️ 数据层 PostgreSQL 15 Supabase
      📊 核心表 8 张
        cinema 影院基础表 中心对象
        users 用户表 UUID主键
        subscription 订阅规则 影院×电影×邮箱
        crawl_task 采集任务 一影院一任务
        execute_log 采集执行日志
        price_snapshot 票价快照 聚合层
        notify_log 通知日志 status/fail/skipped
        crawl_summary 采集摘要统计
      🔗 设计原则
        无物理外键 逻辑外键+索引
        GORM AutoMigrate 禁用FK
        乐观锁 防并发重复通知
        唯一约束 影院×邮箱 场次去重
      📜 迁移文件 5 个
        001_init.sql 初始建表
        002_rebuild_v2.sql 订阅重构
        003_merge_snapshot.sql 快照合并
        004_add_initial_target_price.sql 初始目标价
        005_fix_subscription_unique_constraint.sql 唯一约束修复
    🚀 部署与运维
      🐳 Docker
        Dockerfile 多阶段构建 golang:1.25-alpine → alpine:3.20
        docker-compose.yml 端口映射+日志挂载+migrations映射
        Arial字体内置 猫眼字体解码依赖
      🔁 CI/CD GitHub Actions
        docker.yml v* tag触发 构建推送到Docker Hub
        fe-deploy.yml main分支frontend变更 部署到GitHub Pages
      📝 日志
        stdout JSON Info级别 容器捕获
        logs/app.log Warn级别 文件持久化
        ioMultiHandler 自定义扇出
    🔒 安全
      JWT 认证 7天有效期
      bcrypt 密码哈希 JSON隐藏password_hash
      乐观锁 CompareAndUpdateTriggeredPrice 原子更新
      请求节流 猫眼API调用间随机延迟
      防骚扰 首次采集仅记基准价 12h冷却期
```

---

## 二、业务流程图

### 2.1 用户核心流程

```mermaid
flowchart TD
    A[👤 用户访问] --> B{已登录?}
    B -->|No| C[📝 注册/登录]
    C --> D[JWT Token 签发]
    D --> E[进入系统]
    B -->|Yes| E
    
    E --> F{选择操作}
    F -->|浏览| G[🏠 首页热映电影]
    F -->|查价| H[🔍 票价查询]
    F -->|订阅| I[📋 订阅管理]
    F -->|数据| J[📊 数据查看]
    
    G --> H
    H --> K[选择城市 → 选择区县/商圈]
    K --> L[按距离排序查看影院排片票价]
    L --> M{价格合适?}
    M -->|订阅| N[📬 创建订阅]
    M -->|导出| O[📥 CSV 导出]
    
    N --> P[填写: 影院 + 电影 + 邮箱 + 目标价]
    P --> Q[系统自动创建 CrawlTask]
    Q --> R[✅ 订阅生效]
    
    I --> S[查看订阅列表]
    S --> T[启用/停用/编辑/删除]
    S --> U[手动刷新票价]
    S --> V[导出订阅历史 CSV]
    
    J --> W[📈 订阅详情 价格折线图]
    J --> X[📊 采集记录仪表盘]
    J --> Y[📝 通知日志 分页筛选]
    J --> Z[📉 票价变化趋势]
```

### 2.2 采集调度与通知流程

```mermaid
flowchart TD
    subgraph Scheduler["⏱️ Cron 定时器 每N分钟"]
        A[轮询到期 CrawlTask] --> B[按 cinema_id 逐个采集]
    end
    
    B --> C[🌐 调用猫眼 API]
    C --> D{响应成功?}
    D -->|Yes| E[解码动态字体价格]
    D -->|No| F[记录错误 更新 next_run_at]
    
    E --> G[遍历返回的场次数据]
    G --> H[识别电影信息]
    H --> I[写入 PriceSnapshot 快照批次+明细]
    
    I --> J[查询该影院下所有 Subscription]
    J --> K{遍历每个订阅}
    
    K --> L{订阅匹配当前电影?}
    L -->|No| K
    L -->|Yes| M{当前最低价 ≤ 目标价?}
    M -->|No| K
    M -->|Yes| N{通知冷却期内?}
    N -->|Yes| O[写入 NotifyLog status=skipped]
    N -->|No| P{是首次采集?}
    P -->|Yes| Q[仅记录基准价 不通知]
    P -->|No| R[📧 发送邮件通知]
    
    R --> S{邮件发送成功?}
    S -->|Yes| T[NotifyLog status=success 更新 last_notify_at]
    S -->|No| U[NotifyLog status=fail 记录失败原因]
    
    Q --> V[更新订阅 baseline_min_price]
    O --> W[继续下一个订阅]
    T --> W
    U --> W
    W --> K
    
    K -->|遍历完毕| X[更新 CrawlTask last_run_at + next_run_at]
    X --> Y[写入 ExecuteLog 执行摘要]
    Y --> Z[✅ 本轮采集完成]
```

### 2.3 数据模型关系图

```mermaid
erDiagram
    cinema ||--o{ subscription : "逻辑外键 cinema_id"
    cinema ||--|| crawl_task : "一影院一任务 cinema_id UNIQUE"
    cinema ||--o{ price_snapshot : "cinema_id"
    cinema ||--o{ execute_log : "cinema_id"
    
    crawl_task ||--o{ execute_log : "crawl_task_id"
    execute_log ||--o{ price_snapshot : "execute_log_id"
    
    subscription ||--o{ notify_log : "subscription_id"
    execute_log ||--o{ notify_log : "execute_log_id"
    subscription }o--|| users : "user_id 可选"
    
    price_snapshot {
        uuid id PK
        uuid execute_log_id
        int cinema_id
        int movie_id
        decimal price
        timestamp observed_at
    }
    
    subscription {
        uuid id PK
        int cinema_id
        int movie_id
        string email
        decimal target_price
        bool notify_enabled
        timestamp last_notify_at
    }
    
    crawl_task {
        uuid id PK
        int cinema_id UK
        int interval_minutes
        timestamp next_run_at
        string status
    }
    
    execute_log {
        uuid id PK
        uuid crawl_task_id
        int cinema_id
        int fetched_count
        int matched_count
        int notified_count
        string status
    }
    
    notify_log {
        uuid id PK
        uuid subscription_id
        uuid execute_log_id
        string email
        string status
        decimal triggered_price
    }
    
    cinema {
        int id PK
        string maoyan_cinema_id UK
        string name
        string address
        int maoyan_city_id
    }
    
    users {
        uuid id PK
        string email UK
        string password_hash
    }
```

### 2.4 CI/CD 流水线

```mermaid
flowchart LR
    subgraph Triggers["触发条件"]
        T1[推送 v* tag]
        T2[main 分支 frontend/ 变更]
        T3[手动 workflow_dispatch]
    end
    
    T1 --> W1[docker.yml]
    T3 --> W1
    W1 --> B1[Checkout代码]
    B1 --> B2[Docker Hub 登录]
    B2 --> B3[提取元数据 tags/labels]
    B3 --> B4[QEMU + Buildx 多架构构建]
    B4 --> B5[推送镜像到 docker.io/sikaha/maoyan-service]
    B5 --> B6[生成构建证明 attestation]
    
    T2 --> W2[fe-deploy.yml]
    T3 --> W2
    W2 --> F1[Checkout 代码]
    F1 --> F2[pnpm 安装依赖]
    F2 --> F3[pnpm build + VITE_ORIGIN_SERVER]
    F3 --> F4[上传 Pages artifact]
    F4 --> F5[部署到 GitHub Pages]
```

---

## 三、接口全景图

```
                    公开接口 (无需登录)
                    ─────────────────
GET  /health                     健康检查
POST /api/auth/register          注册
POST /api/auth/login             登录
GET  /api/cities                 城市列表(1094)
GET  /api/districts              区县+商圈
GET  /api/movies/hot             热映电影
GET  /api/movies/search          搜索电影
GET  /api/shows                  查询排片票价

                    需登录接口 (JWT Bearer)
                    ──────────────────────
POST   /api/subscriptions                创建订阅
GET    /api/subscriptions                订阅列表
GET    /api/subscriptions/:id            订阅详情+价格趋势
PATCH  /api/subscriptions/:id/toggle     启用/停用
PUT    /api/subscriptions/:id            更新订阅
DELETE /api/subscriptions/:id            删除订阅
GET    /api/subscriptions/logs           通知日志(分页+筛选)
GET    /api/subscriptions/cinemas        已订阅影院
GET    /api/subscriptions/cinema-movies  已订阅影院+电影
POST   /api/subscriptions/:id/refresh    手动刷新票价
GET    /api/subscriptions/:id/export     导出CSV
GET    /api/subscriptions/:id/crawl-records    采集记录仪表盘
GET    /api/subscriptions/:id/snapshots/:sid/shows  快照明细
GET    /api/shows/export                导出查询CSV
GET    /api/price-changes                票价变化趋势
POST   /api/admin/fetch                  全量采集(管理员)
POST   /api/admin/crawl/:cinema_id       单影院采集(管理员)
```
