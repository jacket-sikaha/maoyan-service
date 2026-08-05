-- 005: 修复 subscription 唯一约束
-- 旧约束 uk_subscription_cinema_email 仅 (cinema_id, email)，需改为 (cinema_id, movie_id, email)
-- AutoMigrate 不会自动删除/重建旧约束，手动处理

-- 1. 删除旧约束（如果存在）
ALTER TABLE subscription DROP CONSTRAINT IF EXISTS uk_subscription_cinema_email;

-- 2. 删除可能存在的其他旧唯一约束
ALTER TABLE subscription DROP CONSTRAINT IF EXISTS uk_subscription_cinema_movie_email;

-- 3. 删除旧索引（如果存在）
DROP INDEX IF EXISTS uk_subscription_cinema_email;
DROP INDEX IF EXISTS idx_subscription_cinema_id;  -- 旧的单字段唯一索引

-- 4. 创建新的三字段联合唯一约束
CREATE UNIQUE INDEX IF NOT EXISTS uk_subscription_cinema_movie_email 
    ON subscription (cinema_id, movie_id, email);
