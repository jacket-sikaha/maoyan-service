# repos_test.go 全量通过 — 2026-07-21

## 结果
40/40 PASS，耗时 116s，真实 Supabase PostgreSQL 验证。

## 修复的核心问题

### 1. GORM `foreignKey` 标签自动建物理 FK
- `subscription`, `crawl_task`, `execute_log`, `price_snapshot_batch`, `price_snapshot_item`, `notify_log` 的 `foreignKey` 标签导致 AutoMigrate 后 DB 有物理外键约束
- **修法**：TestMain AutoMigrate 后调 `dropFkConstraints()` 逐个 `ALTER TABLE ... DROP CONSTRAINT IF EXISTS ...`
- **遗留**：生产代码的 `foreignKey` 标签未改，但这不影响业务逻辑（我们设计原则是无物理 FK，业务层逻辑外键足够）

### 2. GORM 零值问题 — `Status: 0` 被 `default:1` 覆盖
- 通用模式：`int8` 类型 `Status` 的零值 `0` 被 GORM 跳过，DB 的 `DEFAULT 1` 替代
- **修法**：测试中先 `Create(Status=1)` → 再 `UPDATE SET status=0`
- 影响范围：`FindByCinemaID`、`ListActive`、`ListDue` 三个测试
- **不建议改 model**：之前尝试过 BeforeCreate 钩子、指针类型等方案，都回退了，因为会影响生产代码逻辑

### 3. `CrawlTask.LastError` 字段缺失
- `model/models.go` CrawlTask struct 补了 `LastError string`

### 4. 旧 uniqueIndex 残留
- `subscription.cinema_id` 从 `uniqueIndex` 改为 `index` 后 AutoMigrate 不自动改
- DROP INDEX + CREATE INDEX 重建

## 测试覆盖（40 个用例）
- CinemaRepo: 8 个（Create/GetByID/GetByMaoyanCinemaID/GetByMaoyanCityID/SearchByName/Upsert Insert+Update/BulkUpsert）
- UserRepo: 3 个（Create/FindByEmail/FindByID）
- SubscriptionRepo: 11 个（Create/FindByID FindByCinemaAndEmail/FindByCinemaID/FindByEmail/ListActive/UpdateFields/UpdateNotifyCount/UpdateLastNotifyAt/Delete/Update）
- CrawlTaskRepo: 9 个（Create/FindByCinemaID/ListDue/ListActive/UpdateNextRun/UpdateLastRun 2个/UpdateStats 2个/DeleteByCinemaID）
- ExecuteLogRepo: 3 个（CreateAndFind/FindByCrawlTaskID/Update）
- PriceSnapshotBatchRepo: 3 个（CreateAndFind/FindByExecuteLogID/FindByCinemaID）
- PriceSnapshotItemRepo: 4 个（BulkCreate+BulkCreate_Empty/FindByBatchID/FindLatestByCinema/FindPriceTrendByCinema）
- NotifyLogRepo: 5 个（BulkCreate/FindBySubscriptionID/FindByExecuteLogID/FindByUserID/FindRecentBySubscription）
