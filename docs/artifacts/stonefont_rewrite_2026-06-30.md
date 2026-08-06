# Stonefont 解码引擎重写 — 轮廓匹配方案

**时间**: 2026-06-30

## 问题诊断

Go 版 stonefont 解析有两个根本性问题：

### 1. WOFF 解析方式错误
- 旧代码把 WOFF 当 ZIP 容器解析（`zip.NewReader`），WOFF 不是 ZIP
- zlib 解压是空壳 — `decompressWoffTable` 检测到 0x78 magic 就原样返回
- 导致 `tables` map 基本为空，`stoneMapping` 始终返回空

### 2. 匹配算法太粗糙
- 只用 4 值包围盒 `(xMin, yMin, xMax, yMax)` 做欧氏距离匹配
- 0/6/8/9 的包围盒几乎一模一样（都是约 0.5×0.7），必然混淆

## 新方案（对齐 Python 原版）

### 链路
```
cinemaDetail stone.cssSource → woff URL → 下载 woff
  → 解析 WOFF 1.0 header (44 bytes)
  → TableDirectory → zlib 解压各字体表
  → cmap 表 → PUA 码点 → glyph index
  → glyf 表 (delta解码) → 轮廓点坐标
  → 归一化 (重心归零 + 缩放到[-0.5,0.5])
  → 降采样到 ~25 点
  → 双向最近邻轮廓距离 → 匹配 Arial 0-9 基准
```

### 关键实现
- `parseWOFF()` — 正确解析 WOFF 1.0 header + TableDirectory + zlib 解压
- `parseSimpleGlyph()` — TrueType delta 编码坐标解析（flags/x/y）
- `normalizeOutline()` — 重心归零 + 缩放到 [-0.5, 0.5] + 降采样
- `outlineDistance()` — 双向最近邻平均距离（与 Python `_outline_distance` 一致）
- `referenceDigitOutlines()` — Arial 数字 0-9 内置常量（Python fontTools 预提取）
- `BuildStonefontMap()` — LRU 缓存逻辑不变

### 测试结果
- 10 个数字自匹配全部通过 (dist=0.0)
- 45 对不同数字最小距离 0.041 (3↔8，足够区分)
- 归一化边界检查、HTML 解码均通过
