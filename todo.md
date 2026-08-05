
# 项目终极开发大纲：猫眼服务中台 

> **版本定位**：V4.0 Final。
> **开发阶段**：纯本地开发（非部署阶段），数据库使用 Supabase 云 PostgreSQL。
> **核心哲学**：后端高内聚（DDD 分层）、前端低耦合（原子化组件）、契约先行（TypeScript 全栈类型感知）。

---

## 1. 项目核心目标
- 将py文件里 查询热映电影 查询城市的区县id 查询影院各种电影的票价等这些服务 封装成接口供前端使用
- 核心目标就一个我是要使用这些接口 查询我需要城市的区县id下附近的影院电影票价, 还可以发布邮件订阅, 当我定义的区域有更低电影票价时, 可以通过邮件通知我
- 补充功能细节
 
  - 如果有订阅服务, 我可能需要定时从猫眼数据每个半小时更新一次到数据库，数据库我觉得是一定要记录两个重要数据，一个是一开始请求接口获取到的正常价格和还有可能变化的史低价格就行，其他数据你根据需求来添加
  - 开启订阅服务 我觉得需要参数有电影ID 影院ID 城市/区域ID 邮箱地址,剩下有需要你再根据需求来添加
  - 如果有订阅服务, 我需要支持数据导出csv功能，文件输出内容参考py文件里的实现
  - 如果有订阅服务, 我需要支持取消订阅功能, 取消订阅时需要提供邮箱地址;还需要一个主动查询最新我订阅影院票价的接口, 每1min最多查询一次,查询完也得数据同步数据库
- 前端页面需求
  - 简单的邮箱登录注册
  - 一个页面展示热映电影
  - 一个页面提供按区域查询影院各种电影的票价
  - 订阅服务页面
    - 创建订阅服务，只需要提供要订阅的影院和电影，影院要支持按区域的多选择（默认三十分钟系统会去查一次，因为担心猫眼服务的频率限制）
    - 订阅服务列表页面展示我订阅的区域和电影，支持开启/取消订阅服务（服务的启动与暂停 不带删除因为要记录票价历史记录）
    - 订阅详情展示我订阅的区域和电影的票价变化记录折线图，按时间排序展示；还附带csv导出功能 和 在线表格展示（20条数据的分页）

  
  

---

## 2. 技术栈精确选型（版本锁定）

| 层级 | 技术选型 | 版本/备注 |
| :--- | :--- | :--- |
| **前端框架** | React 19 + Vite 8 | 包管理器 `pnpm`，严格 TypeScript |
| **前端样式** | Tailwind CSS 3.4 | 禁用 CSS Modules，全面使用原子类 |
| **前端 UI 基础** | 可以基于antd或者shadcn/ui自建原子组件库，增加组件复用性 | 推荐使用antd或者shadcn/ui第三方组件库，保持轻量可控 |
| **后端语言** | Go 1.22+ | 启用 Go Modules |
| **后端框架** | Gin | 大厂主流，轻量高性能 |
| **数据库** | Supabase (PostgreSQL 15) | 远程云库，连接强制 `sslmode=require` |
| **ORM** | GORM v2 | 适配 PGX 驱动 |
| **调度器** | `github.com/robfig/cron/v3` | 支持秒级调度，从数据库动态加载任务 |
| **配置管理** | `github.com/spf13/viper` | 读取 `.env` 文件 |
| **日志** | `golang.org/x/exp/slog` | 结构化日志（JSON 格式输出到控制台） |

---

## 3. 前后端协作契约（解耦核心）

为了彻底解耦，前后端必须遵守以下 **铁律**：

- **统一响应格式**：后端所有接口返回 JSON 必须为：
  ```json
  { "code": 0, "msg": "success", "data": { ... } }
  ```
  （HTTP 状态码永远返回 200，业务错误通过 `code` 区分，`code=0` 代表成功）
- **前端契约定义**：在 `frontend/src/types/api.ts` 中定义泛型 `interface ApiResponse<T>`，所有 API 请求函数均返回此类型。
- **本地代理**：Vite 配置代理 `/api` 至 `http://localhost:8080`，彻底规避跨域问题。

---

## 4. 项目目录结构（Monorepo 企业级规范）

Codex 必须严格按照以下树形结构生成文件：

```
project-root/
├── .vscode/                         # IDE 配置（一键调试）
│   ├── launch.json                  # 配置 Go 和 React 的 Debug 启动
│   └── settings.json                # 保存自动格式化 (gofmt, prettier)
│
├── frontend/                        # 前端源码（React + Vite）
│   ├── src/
│   │   ├── api/                     # 接口请求层（按模块划分：datasource.ts, subscription.ts）
│   │   ├── assets/                  # 静态资源（图片/字体）
│   │   ├── components/
│   │   │   ├── ui/                  # 基础原子组件（Button, Input, Card, Badge, Skeleton）
│   │   │   ├── layout/              # 布局组件（AppLayout, Sidebar, Header）
│   │   │   └── features/            # 业务复合组件（DataSourceCard, SubscriptionForm）
│   │   ├── hooks/                   # 自定义 Hooks（useAsync, useLocalStorage）
│   │   ├── pages/                   # 页面容器（Dashboard, SubscriptionManage）
│   │   ├── stores/                  # Zustand 状态管理（UI 交互状态）
│   │   ├── types/                   # TypeScript 全局类型（含 api.ts）
│   │   ├── utils/                   # 工具函数（日期格式化、防抖）
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── .env.development             # 环境变量（VITE_API_BASE_URL=/api）
│   ├── tailwind.config.js           # Tailwind 配置（含 content 扫描路径）
│   ├── vite.config.ts               # Vite 配置（含代理设置）
│   └── package.json
│
├── backend/                         # 后端源码（Golang + Gin）
│   ├── cmd/
│   │   └── server/
│   │       └── main.go              # 入口：读取配置、初始化 DB、启动 Gin 路由
│   ├── internal/                    # 私有业务包（不可外部导入）
│   │   ├── controller/              # 传输层（参数校验 binding，调用 Service）
│   │   ├── service/                 # 业务逻辑层（核心编排，事务控制）
│   │   ├── repository/              # 数据访问层（纯 CRUD，接口定义）
│   │   ├── model/                   # 实体模型层（GORM 结构体 + JSON Tag）
│   │   └── pkg/                     # 内部工具包（HTTP 抓取、GJSON 解析、SMTP 发送）
│   ├── migrations/                  # 数据库迁移 DDL（手动 SQL 或 GORM AutoMigrate）
│   ├── go.mod
│   └── .env                         # 本地环境变量（含 Supabase DSN + SMTP 配置）
└── README.md
```

---

## 5. 数据库设计（Supabase 专用 DDL）

生成 Model 时，主键必须使用 **UUID**（适配 Supabase 默认），字段蛇形命名。

---

## 6. 后端架构规范（高内聚 DDD 轻量版）

Codex 在编写 Go 代码时，必须遵守以下分层依赖方向：
**Controller -> Service -> Repository -> Model**（严禁反向依赖）。

- **Controller 层**：只做参数校验（`ShouldBindJSON`）和响应渲染，**不包含任何业务逻辑**。
- **Service 层**：核心调度逻辑。包含事务包裹，组合多个 Repository 操作。
- **Repository 层**：必须定义为接口（`type DataSourceRepo interface{ ... }`），便于未来单元测试 Mock。
- **核心状态机逻辑（防骚扰）**：Service 层必须实现“仅状态翻转时触发”逻辑（伪代码如下）：

```go
// 伪代码：严格用于 Codex 实现参考
if sub.LastTriggeredValue == "" {
    // 首次拉取：只记录，不通知
    repo.UpdateLastValue(sub.ID, currentValue)
    return
}
if isConditionMet && currentValue != sub.LastTriggeredValue {
    // 条件满足且值已变化 -> 触发通知
    notifier.SendEmail(...)
    repo.UpdateLastValue(sub.ID, currentValue) // 更新防止重复
    repo.CreateHistory(...)
}
```

- **调度器**：使用 `robfig/cron`，服务启动时从数据库加载所有 `is_active=true` 的数据源，注入到调度器中。

---

## 7. 前端架构规范（高复用原子化设计）

Codex 生成前端代码时，必须严格按 **四层组件树** 组织，禁止在 Pages 中写大块 JSX：

1. **UI 原子层（`components/ui/`）**：
   - 如 `<Button>`, `<Input>`, `<Card>`，仅通过 `props` 控制样式（`variant`, `size`）。
   - 交互反馈：按钮必须带 `active:scale-95`，卡片必须带 `hover:shadow-lg transition-all`。
2. **布局层（`components/layout/`）**：定义侧边栏和顶部导航。
3. **功能复合层（`components/features/`）**：如 `<DataSourceCard>`，通过 `props` 接收数据和回调，**内部不直接调用 API**。
4. **页面容器层（`pages/`）**：**仅在此层调用 API**（使用 React Query），并将数据分发至 Features 组件。

- **状态管理**：
  - **组件简单状态**：使用  state 或者context                。
  - **服务端数据**：使用 `@tanstack/react-query`（负责缓存、轮询、重试）。
  - **客户端 UI 状态或者复杂的全局状态**：使用 `zustand`（负责侧边栏折叠、弹窗开关）。
- **Tailwind 美学指令**：
  - 使用 `slate` 和 `indigo` 配色方案。
  - 骨架屏使用 `<Skeleton>` 组件（脉冲动画）。
  - 容器使用 `max-w-7xl mx-auto px-4 sm:px-6 lg:px-8`。
  - 利用 Frontend Design 思维：空状态显示 `😅` 引导语，错误状态使用 `bg-rose-50 border-rose-200` 警示条。

---

## 8. VS Code 开发环境配置（开箱即用）

Codex 必须生成 `.vscode/launch.json`，支持一键 F5 调试：

- **后端配置**：`"program": "${workspaceFolder}/backend/cmd/server/main.go"`，环境变量从 `backend/.env` 加载。
- **前端配置**：`"type": "node"`，执行 `npm run dev`。
- **复合启动**：配置 `"compounds"` 实现前后端同时启动。

---

## 9. 环境变量与 Mock 数据策略

- **后端 `.env` 模板**：

  ```env
  PORT=8080
  DB_DSN=host=aws-0-xx.pooler.supabase.com user=postgres password=xxx dbname=postgres port=6543 sslmode=require
  SMTP_HOST=smtp.qq.com
  SMTP_PORT=465
  SMTP_USER=xxx@qq.com
  SMTP_PASS=xxxxxx
  ```

- **Mock 数据**：在 `main.go` 启动时，若表为空，自动插入一条示例数据（如模拟支付宝 QDII 限购接口，返回 JSON 硬编码 `{"status":"open"}`）。方便开发阶段直接测试。

---


**最终指示**：请依据此终极大纲生成完整可运行代码。所有功能模块必须保持低耦合,前端页面需达到现代 Web 应用的美学标准（柔和阴影、圆角、微交互动画）。现在请开始生成。