# 猫眼服务中台 Go 后端设计回顾

**时间**: 2026-06-26 16:36-16:48
**目标**: 基于 Python 猫眼爬虫实现 + todo.md 大纲，设计完整的 Go 后端服务

## 设计决策

### 架构遵循 DDD 分层
Controller → Service → Repository → Model，严格单向依赖，Repository 全部接口化。

### Python → Go 关键迁移

| Python 实现 | Go 实现 |
|-------------|---------|
| `requests.Session` | `*http.Client` |
| `fontTools.ttLib.TTFont` 系统字体比对 | 纯 Go SFNT 解析 `glyf/loca/cmap/hmtx` 表 + 边界框距离比对 |
| tkinter GUI | 纯 REST API（前端另行开发） |
| `pd.DataFrame.to_csv` | 自定义 CSV writer + BOM |
| IP定位 `api.uomg.com` | 前端传 lat/lng |
| 无持久化 | Supabase PostgreSQL + GORM |
| 城市硬编码 class 变量 | 独立 city_map.go + DB seeds |

### 数据库设计核心

- `show_snapshots`: 唯一索引 `(movie_id, cinema_id, show_date, show_time, hall_name, lang)` 保证同一排片只一条记录
- `lowest_price` / `first_price`: 由 upsert 逻辑自动维护
- `price_history`: 价格变动时自动写入
- `subscriptions.last_triggered_price`: 防骚扰状态机关键字段

### 防骚扰订阅通知逻辑

```
if LastTriggeredPrice == nil → 首次，只记录不通知
if currentPrice <= targetPrice && currentPrice != LastTriggeredPrice → 通知
if targetPrice == 0 && currentPrice < LastTriggeredPrice → 降价通知
```

### Stonefont 解码 (纯Go)

不走 fontTools，直接在 Go 中解析 woff SFNT 结构：
1. 解析 cmap 表（format 4/12）提取 PUA 码点→glyphIndex
2. 解析 glyf 表提取每个字形归一化边界框
3. 与 Arial 数字 0-9 参考边界框做欧几里得距离比对
4. 最近邻匹配确定码点→数字映射

### 文件清单 (17个文件, ~120KB)

- `backend/cmd/server/main.go` - 入口，DI 组装
- `backend/cmd/server/city_map.go` - 1094城市ID映射
- `backend/internal/model/models.go` - GORM实体 + DTO
- `backend/internal/repository/repos.go` - 8个接口 + 实现
- `backend/internal/service/data_service.go` - 核心业务逻辑
- `backend/internal/controller/maoyan_controller.go` - 13个API端点
- `backend/internal/pkg/maoyan.go` - 猫眼爬虫（6个接口）
- `backend/internal/pkg/stonefont.go` - 字体解码引擎
- `backend/internal/pkg/email.go` - 邮件通知
- `backend/internal/pkg/csv_exporter.go` - CSV导出
- `backend/internal/scheduler/scheduler.go` - cron调度器
- `backend/migrations/001_init.sql` - 数据库DDL
- `.vscode/launch.json` + `settings.json` - 调试配置
- `README.md` - 完整文档
