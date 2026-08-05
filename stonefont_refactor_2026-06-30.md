# stonefont.go 重构 - 2026-06-30

## 目标
按 Python 方案文档的 4 步管线重新组织 Go 版 stonefont 解码引擎，使代码结构更清晰、注释风格一致。

## 改动

### 结构重组（按 4 步管线分段）
1. **步骤①** 下载 WOFF 字体（提取 URL）
2. **步骤②** 解析 WOFF → cmap/glyf/loca → 提取 PUA 字形轮廓点
3. **步骤③** 归一化轮廓 + 双向 Chamfer 距离匹配（与 Arial 0-9 基准比对）
4. **步骤④** 建立映射表 → `&#xNNNN;` entity 解码为浮点数

### 函数重命名
- `outlineDistance` → `bidirectionalChamferDistance`（更准确描述算法）
- `parseSimpleGlyph` → `parseGlyphContour`（更符合 TrueType 术语）
- `buildLocaOffsets` → `buildLoca`

### 注释改进
- 文件头对齐 Python 方案：问题背景 + 4 步管线 + 缓存 + 验证结果
- 每个段落用 `━━━` 分隔，标注步骤编号
- WOFF/TTF 数据结构注释对齐 Apple TrueType Reference Manual 字段名
- Arial 基准轮廓块标注自验证结果

### 日志增强
- 构建成功时输出完整 mapping 值（方便排查）
- 去掉冗余的中间步骤日志

### 测试更新
- `outlineDistance` → `bidirectionalChamferDistance`
- 全自验证测试通过：10 个数字自匹配 dist=0，最小跨数字 dist=0.041 (3↔8)

## 功能不变
核心逻辑无改动：WOFF 1.0 解析、TrueType delta 解码、Chamfer 距离匹配、缓存策略全部保持原有实现。
