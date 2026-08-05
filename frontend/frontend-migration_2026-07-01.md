# 猫眼前端改造完成报告

## 执行时间
2026-07-01 14:56 GMT+8

## 目标
按方案完整改造猫眼前端，新增订阅管理、添加订阅、订阅详情（含编辑/删除）、执行日志、票价变化 5 个页面，并更新路由和导航栏。

## 执行步骤

### 步骤 1: 更新 api/endpoints.ts ✅
在 `C:\Users\Administrator\Desktop\cron\gogogo\maoyan-service\frontend\src\api\endpoints.ts` 末尾追加了 4 个新 API：
- `updateSubscription` — PUT 更新订阅
- `deleteSubscription` — DELETE 删除订阅
- `getSubscriptionLogs` — GET 获取执行日志（分页+日期范围）
- `getUserSubscribedCinemas` — GET 获取用户已订阅影院
- `getPriceChanges` — GET 获取影院票价变化数据

### 步骤 2: 创建 pages/SubscribePage.tsx ✅
- 需登录（useAuth guard）
- 城市选择（PinyinCityPicker）
- 搜索电影（Select + searchMovies，防抖 400ms）
- 搜索影院（Select + searchCinemas，防抖 400ms）
- 通知邮箱（默认填 user.email）
- 目标价格输入（数字，可选）
- 提交后 redirect 到 /subscriptions
- 面包屑导航

### 步骤 3: 创建 pages/SubscriptionDetailPage.tsx ✅
- 路由：/subscription/:id
- 订阅信息卡片（电影名、影院、城市、评分、目标价、邮箱、状态 Tag）
- 编辑 Modal（可改 movie_id / cinema_id / target_price / notify_email）
- 删除按钮 → 确认弹窗 → 跳回 /subscriptions
- 价格走势折线图（recharts LineChart，来自 detail.price_trend）
- 今日行情表格（含 current_price、vip_price、base_price、discount_price、lowest_price、first_price 列）
- CSV 导出按钮

### 步骤 4: 创建 pages/SubscriptionHistoryPage.tsx ✅
- 路由：/logs
- 日期范围筛选（DatePicker.RangePicker）
- 分页表格，列：电影/影院合并列、触发价格(Tag)、执行状态(Tag 着色)、结果信息(ellipsis)、执行时间

### 步骤 5: 创建 pages/PriceChangesPage.tsx ✅
- 路由：/price-changes
- 需登录 guard
- 筛选：PinyinCityPicker + TreeSelect 区县 + Select 已订阅影院
- 主体：按电影分组，每部电影一个 Card + recharts LineChart
- 无数据时 Empty

### 步骤 6: 更新 App.tsx 路由 ✅
新增 5 条路由：/subscribe, /subscriptions, /subscription/:id, /logs, /price-changes

### 步骤 7: 更新 Navbar.tsx 导航栏 ✅
删除"我的订阅"链接，新增：订阅管理、添加订阅、票价变化、执行日志 4 个导航按钮

### 步骤 8: 最终验证 ✅
```
pnpm build → tsc -b && vite build → SUCCESS (exit code 0)
```
TypeScript 编译通过，所有 4812 个模块转换成功。

## 文件变更清单
| 文件 | 操作 |
|------|------|
| src/api/endpoints.ts | 追加 5 个 API 导出 |
| src/pages/SubscribePage.tsx | 新建 |
| src/pages/SubscriptionDetailPage.tsx | 新建 |
| src/pages/SubscriptionHistoryPage.tsx | 新建 |
| src/pages/PriceChangesPage.tsx | 新建 |
| src/App.tsx | 新增 4 个 import + 5 条路由 |
| src/components/Navbar.tsx | 新增 4 个导航链接 |

## 技术栈复用
- 所有新页面使用 `useAuth()` 判断登录态
- PinyinCityPicker 组件正确复用（路径 `../components/PinyinCityPicker`）
- API 统一使用 endpoints.ts 导出的函数
- Ant Design 6.x 组件：Card, Table, Form, Input, Button, Tag, Select, DatePicker, Modal, message, Spin, Empty, Typography, Space, Breadcrumb
- recharts: LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer
- react-router-dom: useParams, useNavigate, Link
