-- 004_add_initial_target_price.sql
-- 为 subscription 表添加 initial_target_price 字段（初始目标价，编辑时只能调低）
-- 已有数据用 target_price 填充

ALTER TABLE subscription ADD COLUMN IF NOT EXISTS initial_target_price DECIMAL(10,2) NOT NULL DEFAULT 0;

-- 用当前 target_price 填充已有记录的 initial_target_price
UPDATE subscription SET initial_target_price = target_price WHERE initial_target_price = 0;
