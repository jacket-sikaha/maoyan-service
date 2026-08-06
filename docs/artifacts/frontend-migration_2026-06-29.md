# 猫眼票价监控 — Ant Design v6 前端重构 + base_price/discount_price 全链路

**时间**: 2026-06-29 10:20
**目标**: 前端全面迁移到 Ant Design 6，同时在表格和 CSV 导出中加入 base_price / discount_price 两个新字段

## 完成的改动

### 后端 (Go)
1. **maoyan.go** — 添加 `strings` import（之前缺失导致编译失败）
2. **data_service.go** — `GetHotMovies()` 和 `SearchMovies()` 的缓存/实时数据返回都补上 `Img` 字段；`compareAndSave()` 保存 `BasePrice` / `DiscountPrice` 到 `ShowSnapshot`
3. 模型层 `ShowSnapshot` / `ShowPriceForSubscription` 已有 `BasePrice` / `DiscountPrice` 字段（之前就加了），无需修改

### 前端 (React + TypeScript)
全面重写为 Ant Design v6 组件：

| 文件 | 改动 |
|------|------|
| **main.tsx** | 引入 `ConfigProvider` + `zhCN` 国际化 + 主题色 `#1677ff` |
| **App.tsx** | 使用 `Layout` 包裹，深色 Header |
| **Navbar.tsx** | 用 `Layout.Header` + antd `Button`/`Space` 重建导航栏 |
| **HomePage.tsx** | `Select`(城市)、`Input.Search`(搜索)、`Card`(电影)、`Table`(排片) — 新增 base_price/discount_price 列 |
| **SubscriptionsPage.tsx** | `Table`+`Switch` 订阅列表、`Table` 详情行情表（含 base_price/discount_price）、recharts 折线图 |
| **LoginPage.tsx** | antd `Form`+`Input.Password`+`Alert` 重构登录/注册表单 |
| **index.css** | 精简为全局重置 + antd 微调 |

### 安装的依赖
- `antd@6.5.0` — 使用 pnpm 安装（npm 无法解析 workspace:* 协议）
- `@ant-design/icons@6.3.2`

## 新增的 UI 列
- **排片查询页**: 基础价(blue) / 折扣(orange Tag)
- **订阅详情页**: 基础价 / 折扣 — 对应数据库的 base_price / discount_price

## 编译状态
- 后端 `go build ./cmd/server/` ✅
- 前端 `pnpm run build` ✅ (tsc + vite)
