# stonefont 解码验证 & 清理 - 2026-06-30

## 排查结论

用户看到 Go 端查出的价格是 33 元，Python 端是 38 元，怀疑 stonefont 解码有 bug。

**根因：不同影院查询不同数据，价格本就不同。stonefont 解码没有 bug。**

### 验证过程

1. 用 `probe_prices` CLI 工具调用真实猫眼 API (`cinemaDetail`) 
2. Go stonefont 新版（WOFF 1.0 正确解析 + TrueType 轮廓匹配）成功下载字体 → 提取 10 个 PUA 码点 → 正确映射 0-9
3. 解码 `&#xe583;&#xe583;` → 33（sellPr stonefont 字段），与猫眼 H5 页面显示一致
4. 四个价格字段全部正常：`price=33, vip_price=31, base_price=33, discount_price=33`
5. 通过 HTTP API (`/api/shows?city_id=92&movie_id=1490532`) 拉取全城数据，返回 200+ 条排片，价格区间 29.9~39，与 Python 端结果一致

### 为什么不同

- Python 测试用的是某个具体影院对玩具总动员5 的查询，拿到了 38 元的场次
- Go 测试用的是 cinema_id=14588（华夏星汇影城），该影院只有 2 场 33 元的排片
- **不同影院/场次的 stonefont 字体文件也不同**（含不同的 PUA 码点映射），但轮廓匹配算法按字体动态计算，能正确解码每个字体

## 清理操作

- 删除临时调试工具 `cmd/probe_prices/`
- 删除临时二进制 `tmp/probe.exe`
- 删除 `GetCinemaDetailRawForDebug` 函数
- 后端编译通过 ✅

## 当前状态

- stonefont 解码：WOFF 1.0 轮廓匹配，实测准确
- 四个价格字段：`price`(sellPr) + `vip_price`(vipPrice) + `base_price`(baseSellPrice) + `discount_price`(discountSellPrice) 全部正常
- CSV 导出和前端表格均已包含这些字段
