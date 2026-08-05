-- =====================================================
-- v3 合并 price_snapshot_batch + price_snapshot_item → price_snapshot
-- Subscription 新增电影维度（movie_id, movie_name）
-- 2026-07-25
-- =====================================================
-- ⚠️ 执行前请备份数据库！
-- 执行方式: psql -d <database> -f 003_merge_snapshot.sql
-- =====================================================

BEGIN;

-- ========== 0. 删除旧表（batch + item 合并为 snapshot） ==========
DROP TABLE IF EXISTS price_snapshot_item CASCADE;
DROP TABLE IF EXISTS price_snapshot_batch CASCADE;
DROP TABLE IF EXISTS price_snapshot CASCADE;

-- ========== 1. 创建合并后的 price_snapshot 表 ==========
CREATE TABLE IF NOT EXISTS price_snapshot (
    id                  BIGSERIAL PRIMARY KEY,
    crawl_task_id       BIGINT NOT NULL,
    cinema_id           BIGINT NOT NULL,
    execute_log_id      BIGINT,
    fetched_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    source              VARCHAR(50) NOT NULL DEFAULT 'maoyan',
    total_movies        INT NOT NULL DEFAULT 0,
    total_showtimes     INT NOT NULL DEFAULT 0,
    price_min           DECIMAL(10, 2),
    price_avg           DECIMAL(10, 2),
    price_max           DECIMAL(10, 2),
    raw_json            TEXT,
    parse_status        VARCHAR(20) NOT NULL DEFAULT 'success',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_snapshot_crawl_task_id ON price_snapshot(crawl_task_id);
CREATE INDEX IF NOT EXISTS idx_snapshot_cinema_id ON price_snapshot(cinema_id);
CREATE INDEX IF NOT EXISTS idx_snapshot_execute_log_id ON price_snapshot(execute_log_id);
CREATE INDEX IF NOT EXISTS idx_snapshot_fetched_at ON price_snapshot(fetched_at);
CREATE INDEX IF NOT EXISTS idx_snapshot_cinema_fetched ON price_snapshot(cinema_id, fetched_at);
CREATE INDEX IF NOT EXISTS idx_snapshot_source ON price_snapshot(source);

-- ========== 2. Subscription 新增电影维度 ==========
ALTER TABLE subscription ADD COLUMN IF NOT EXISTS movie_id VARCHAR(100) DEFAULT '';
ALTER TABLE subscription ADD COLUMN IF NOT EXISTS movie_name VARCHAR(200) DEFAULT '';

-- 删除旧唯一索引（cinema_id, email）
DROP INDEX IF EXISTS uk_subscription_cinema_email;
DROP INDEX IF EXISTS idx_subscription_cinema_id;

-- 创建新唯一索引：cinema_id + movie_id + email
CREATE UNIQUE INDEX IF NOT EXISTS uk_subscription_cinema_movie_email ON subscription(cinema_id, movie_id, email);
CREATE INDEX IF NOT EXISTS idx_subscription_movie_id ON subscription(movie_id);
CREATE INDEX IF NOT EXISTS idx_subscription_cinema_movie ON subscription(cinema_id, movie_id);

COMMIT;

-- ========== 验证 ==========
SELECT 'price_snapshot', count(*) FROM information_schema.columns WHERE table_name = 'price_snapshot'
UNION ALL SELECT 'subscription', count(*) FROM information_schema.columns WHERE table_name = 'subscription'
ORDER BY table_name;
