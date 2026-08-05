# 猫眼服务后端 — ShowSnapshot 模型增加 base_price / discount_price 字段

## 背景
Python 项目 `index.py` 的 `ShowInfo` 模型定义了 `base_price` (baseSellPrice) 和 `discount_price` (discountSellPrice) 字段，但 Go 后端 `ShowSnapshot` 模型缺少这两个字段。需要对齐并为表格展示和 CSV 导入导出增加这两列。

## 修改范围（6个文件）

### 1. `internal/model/models.go`
- `ShowSnapshot`：新增 `BasePrice` / `DiscountPrice` 字段（decimal(8,1) / float64）
- `ShowInfo`：新增 `BasePrice` / `DiscountPrice`（omitempty，向后兼容）
- `ShowPriceForSubscription`：新增 `BasePrice` / `DiscountPrice`（omitempty）

### 2. `internal/pkg/maoyan.go`
- `ShowRaw`：新增 `BasePrice` / `DiscountPrice` 字段
- `extractPrice`：**拆分逻辑** — 仅解码 `sellPr`（原价），不再混入 `baseSellPrice` / `discountSellPrice`
- 新增 `extractBaseAndDiscount()`：单独解码 `baseSellPrice` 和 `discountSellPrice`（stonefont）
- `GetCinemaShows` / `GetCinemaAllShows`：所有 ShowRaw 构造处传入 base/discount

### 3. `internal/service/data_service.go`
- `GetCinemaShows`：ShowSnapshot upsert 传 BasePrice / DiscountPrice；ShowInfo 返回这两个字段
- `fetchCurrentShows`：ShowSnapshot upsert 传新字段
- `toPriceInfo` / `snapToPriceInfo`：映射新字段
- `FetchAllSubscriptionData`：snapshot 构造传新字段

### 4. `internal/repository/repos.go`
- `Upsert`：updates map 增加 `base_price` / `discount_price`

### 5. `internal/pkg/csv_exporter.go`
- `ExportShowsToCSV`：表头 + 数据增加「影城卡价(元)」「原价(元)」「优惠价(元)」
- `ExportSnapshotsToCSV`：表头 + 数据增加 `base_price` / `discount_price`

### 6. `migrations/001_init.sql`
- `show_snapshots` 表新增 `base_price DECIMAL(8,1)` / `discount_price DECIMAL(8,1)`
- `subscriptions` 表新增 `rate_limit_until TIMESTAMPTZ`（之前 model 有字段但 DDL 缺失）

## 关键设计决策
- **低价判断逻辑不变**：仍使用 `sellPr` 解码后的 `SellPrice`（原价），不因增加新字段改变通知触发逻辑
- **extractPrice 回归单一职责**：仅取 `sellPr`，不再 fallback 到 base/discount
- 新增字段都带 `omitempty`，API 向后兼容

## 验证
- `go build ./cmd/server/` 编译通过（exit 0）
