# 电影票价查询页 + 首页修改 — 2026-06-29 11:30

## 需求回顾

### 新增页面 `/movie/:id` — 热映电影票价查询
1. 顶部城市（必选，拼音分组+搜索）→ 区/县（树形，带影院数）筛选表单
2. 中间按影院分组的票价表格，时间/价格字段可排序
3. CSV 导出（唯一需登录的功能），其余公开

### 首页修改
1. 去掉顶部 Hero 区的城市选择框和搜索框
2. 点击电影卡片 → 直接跳转到 `/movie/:id?name=电影名`

## 前端改动的文件

### 新增: `pages/MoviePricePage.tsx`
- 暗色主题全屏页，与首页风格统一
- 顶部面包屑导航 (`首页 > 电影名`)
- **筛选表单卡片** — 毛玻璃效果:
  - `Select` 城市（拼音首字母 A-Z 分组 + 搜索）
  - `TreeSelect` 区/县（树形: 区名（N家） → 片区（M家），带影院数展示）
  - `Button` 查询票价
  - `Button` 导出 CSV（用 `Tooltip` 提示需登录，`exportShowsCSV` 走 auth 路由）
- **价格统计摘要** Row(总场次/最低/最高/均价)
- **影院分组表格**:
  - 每行含影厅/日期/开场/散场/版本/票价/优惠价/原价/会员
  - click 表头切换排序 (`ArrowUpOutlined`/`ArrowDownOutlined` 图标)
  - 最低价高亮红字
  - 优惠价橙色 Tag，原价为0时 `-`
- 排序字段: `price`, `base_price`, `discount_price`, `show_time`
- 区/县 ID 解析: 校验顶层 district vs 子片区，传给 `district_id` / `area_id`

### 修改: `pages/HomePage.tsx`
- 去掉 `Select` 城市 + `Input.Search` 搜索框
- 去掉 Drawer 排片详情
- 去掉 `getCities`/`searchMovies`/`queryShows`/`createSubscription` 依赖
- 简化: 只 `getHotMovies(1)`，卡片点击 `navigate(/movie/:id?name=)`
- 保留原 UI 风格

### 修改: `App.tsx`
- 新增 Route: `/movie/:id` → `MoviePricePage`

### 修改: `api/endpoints.ts`
- `exportShowsCSV` 移到 auth 接口组（带 JWT）
- 新增 `getDistricts` 带完整类型

## 后端改动

### 路由调整 `cmd/server/main.go`
- `GET /api/shows/export` 从公开路由移到 auth 路由组（需 JWT）

### 新增导出 `internal/pkg/csv_exporter.go`
- 新增 `ExportShowsInfoToCSV()` — 直接从 `[]model.ShowInfo` 写 BOM+CSV 到 io.Writer
- 完全保留 `ExportShowsToCSV`（文件路径版）

### Controller 改动 `maoyan_controller.go`
- `ExportShowsCSV` 改用 `ExportShowsInfoToCSV` + HTTP writer 直写（不再磁盘落地）
- 移除了 `ExportSnapshotsToCSV(showsToSnaps(...))` 旧方式

## 编译验证
- 后端 `go build ./cmd/server/` ✅
- 前端 `pnpm run build` (tsc + vite) ✅

## 功能覆盖表
| 需求项 | 状态 |
|--------|------|
| 城市必选（拼音分组+搜索） | ✅ Select 组件 |
| 区/县树形选择（带影院数） | ✅ TreeSelect + cinema_count |
| 票价表格多列排序 | ✅ Table sorter + 状态图标 |
| CSV 导出需登录 | ✅ auth 路由组 + 前端 Tooltip |
| 首页去搜索框+城市选择 | ✅ |
| 卡片跳转票价页 | ✅ navigate(/movie/:id?name=) |
| 无登录限制使用查询 | ✅ 公开路由 |

## 访问方式
- 首页: `http://localhost:5173/`
- 票价页: `http://localhost:5173/movie/1545588?name=四渡`
