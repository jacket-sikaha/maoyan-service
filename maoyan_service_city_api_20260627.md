# 猫眼城市数据源优化 → 官方 API 代理缓存 2026-06-27

## 改动目标
将 1094 条硬编码的城市映射（`city_map.go`）替换为**代理缓存模式**：
- 数据源：猫眼官方 `https://m.maoyan.com/dianying/cities.json`（含拼音）
- 首次请求 → 调 API 获取 → 写入 PostgreSQL
- 后续请求 → 直接从 DB 返回（按 py 拼音排序）
- 前端：城市下拉框支持拼音搜索 + 首字母分组

## 后端改动

### 删除
- `cmd/server/city_map.go` — 1094 行硬编码映射（已被官方 API 替代）
- `tools/gen_citymap.go` — 城市映射生成工具（已被官方 API 替代）
- `internal/service/city_service.go` — 残留空壳（逻辑已合并到 data_service.go）

### 新增
- `pkg/maoyan.go` — `GetCities()` 方法：调官方 `cities.json`，返回 `CityRaw{ID, Name, Py}`
- `pkg/maoyan.go` — `CityRaw` 结构体

### 修改
| 文件 | 改动 |
|------|------|
| `model/models.go` | City 字段 `Pinyin` → `Py`（对齐官方字段），移除 Lat/Lng（官方接口不含经纬度） |
| `service/data_service.go` | `GetCities()` 重写：DB 有缓存直接返回 → 无缓存调 API → 写入 DB → 返回 |
| `repository/repos.go` | CityRepo 注释更新，标注数据源 |
| `cmd/server/main.go` | 移除 `context` 导入、CityMap 引用、seedCityData goroutine；注释改为"懒加载"说明 |

### 编译验证
- `go build ./cmd/server/` ✅
- `go vet ./...` ✅

## 前端改动

### 修改
| 文件 | 改动 |
|------|------|
| `pages/HomePage.tsx` | 城市下拉改为可搜索 + 首字母分组面板 |
|   | 新增 state: `citySearch` / `showCityDropdown` |
|   | 新增 `cityGroups` useMemo：按 py 拼音过滤 + 首字母分组 |
|   | City 类型加 `py` 字段 |
| `api/endpoints.ts` | `getCities()` 返回类型细化 `{id:number;name:string;py:string}[]` |

### 交互流程
1. 点击城市输入框 → 弹出下拉面板（遮罩层点击关闭）
2. 面板顶部搜索框 → 支持中文/拼音实时过滤
3. 城市按拼音首字母分组（A-Z），字母标题 sticky 定位
4. 点击城市 → 关闭面板，触发热映电影刷新

## 数据流
```
前端 getCities()
  → 后端 GetCities()
    → DB cities 表有数据？ → 直接返回（按 py 排序）
    → DB 无数据 → 调 https://m.maoyan.com/dianying/cities.json
                 → 解析 cts[] → BulkUpsert 写入 DB（py + initial）
                 → 返回
```


## 后续优化 2026-06-27T17:05
- **GetDistricts**: 删除 DB 缓存层，直接调 /ajax/filterCinemas 返回 tree 结构
- **GetHotMovies**: 删除 DB 缓存层，直接调 /ajax/movieOnInfoList 返回
- 理由：区县/热映数据实时性要求高，请求量不大，缓存收益低
- go build + go vet 通过

