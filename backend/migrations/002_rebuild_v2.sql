-- =====================================================
-- v2 数据库重建迁移脚本
-- 按 PDF 设计文档完整重建所有表
-- 数据库：PostgreSQL
-- 不加物理外键约束，但遵循外键原理设计
-- 2026-07-11
-- =====================================================
-- ⚠️ 执行前请备份数据库！
-- 执行方式: psql -d <database> -f 002_rebuild_v2.sql
-- =====================================================

BEGIN;

-- ========== 0. 删除旧表 ==========
DROP TABLE IF EXISTS notify_logs CASCADE;
DROP TABLE IF EXISTS price_snapshot_items CASCADE;
DROP TABLE IF EXISTS price_snapshot_batches CASCADE;
DROP TABLE IF EXISTS execute_logs CASCADE;
DROP TABLE IF EXISTS crawl_tasks CASCADE;
DROP TABLE IF EXISTS subscription_logs CASCADE;
DROP TABLE IF EXISTS subscriptions CASCADE;
DROP TABLE IF EXISTS price_history CASCADE;
DROP TABLE IF EXISTS show_snapshots CASCADE;
DROP TABLE IF EXISTS movies CASCADE;
DROP TABLE IF EXISTS districts CASCADE;
DROP TABLE IF EXISTS cinemas CASCADE;
DROP TABLE IF EXISTS cities CASCADE;
DROP TABLE IF EXISTS city CASCADE;
-- 保留 users 表（认证需要），不删除

-- ========== 1. 影院基础表 ==========
CREATE TABLE IF NOT EXISTS cinema (
    id              BIGSERIAL PRIMARY KEY,
    maoyan_city_id  INT NOT NULL,
    maoyan_cinema_id VARCHAR(100) NOT NULL,
    name            VARCHAR(200) NOT NULL,
    address         VARCHAR(500),
    latitude        DECIMAL(10, 7),
    longitude       DECIMAL(11, 7),
    phone           VARCHAR(50),
    status          SMALLINT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_cinema_maoyan_cinema_id ON cinema(maoyan_cinema_id);
CREATE INDEX IF NOT EXISTS idx_cinema_maoyan_city_id ON cinema(maoyan_city_id);
CREATE INDEX IF NOT EXISTS idx_cinema_status ON cinema(status);
CREATE INDEX IF NOT EXISTS idx_cinema_city_status ON cinema(maoyan_city_id, status);

-- ========== 2. 订阅规则表 ==========
CREATE TABLE IF NOT EXISTS subscription (
    id                  BIGSERIAL PRIMARY KEY,
    cinema_id           BIGINT NOT NULL,
    email               VARCHAR(255) NOT NULL,
    target_price        DECIMAL(10, 2) NOT NULL,
    notify_enabled      SMALLINT NOT NULL DEFAULT 1,
    status              SMALLINT NOT NULL DEFAULT 1,
    baseline_min_price  DECIMAL(10, 2),
    baseline_max_price  DECIMAL(10, 2),
    last_notify_at      TIMESTAMPTZ,
    notify_count        INT NOT NULL DEFAULT 0,
    user_id             UUID,
    remark              VARCHAR(500),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_subscription_cinema_email ON subscription(cinema_id, email);
CREATE INDEX IF NOT EXISTS idx_subscription_status ON subscription(status);
CREATE INDEX IF NOT EXISTS idx_subscription_email ON subscription(email);
CREATE INDEX IF NOT EXISTS idx_subscription_cinema_status ON subscription(cinema_id, status);
CREATE INDEX IF NOT EXISTS idx_subscription_target_price ON subscription(target_price);

-- ========== 3. 影院轮询任务表 ==========
CREATE TABLE IF NOT EXISTS crawl_task (
    id                  BIGSERIAL PRIMARY KEY,
    cinema_id           BIGINT NOT NULL,
    interval_minutes    INT NOT NULL DEFAULT 30,
    next_run_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_run_at         TIMESTAMPTZ,
    last_success_at     TIMESTAMPTZ,
    run_count           INT NOT NULL DEFAULT 0,
    fail_count          INT NOT NULL DEFAULT 0,
    success_count       INT NOT NULL DEFAULT 0,
    status              SMALLINT NOT NULL DEFAULT 1,
    priority            INT NOT NULL DEFAULT 100,
    timeout_seconds     INT NOT NULL DEFAULT 60,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_crawl_task_cinema_id ON crawl_task(cinema_id);
CREATE INDEX IF NOT EXISTS idx_crawl_task_status ON crawl_task(status);
CREATE INDEX IF NOT EXISTS idx_crawl_task_next_run_at ON crawl_task(next_run_at);
CREATE INDEX IF NOT EXISTS idx_crawl_task_status_next_run ON crawl_task(status, next_run_at);
CREATE INDEX IF NOT EXISTS idx_crawl_task_priority_status ON crawl_task(priority, status);

-- ========== 4. 任务执行日志表 ==========
CREATE TABLE IF NOT EXISTS execute_log (
    id                  BIGSERIAL PRIMARY KEY,
    crawl_task_id       BIGINT NOT NULL,
    cinema_id           BIGINT NOT NULL,
    batch_id            BIGINT,
    started_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at            TIMESTAMPTZ,
    duration_ms         INT,
    status              VARCHAR(20) NOT NULL,
    error_code          VARCHAR(50),
    error_msg           TEXT,
    fetched_count       INT NOT NULL DEFAULT 0,
    matched_count       INT NOT NULL DEFAULT 0,
    notified_count      INT NOT NULL DEFAULT 0,
    skipped_count       INT NOT NULL DEFAULT 0,
    cooldown_count      INT NOT NULL DEFAULT 0,
    summary_json        TEXT,
    request_params      TEXT,
    response_size       INT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_execute_log_crawl_task_id ON execute_log(crawl_task_id);
CREATE INDEX IF NOT EXISTS idx_execute_log_cinema_id ON execute_log(cinema_id);
CREATE INDEX IF NOT EXISTS idx_execute_log_status ON execute_log(status);
CREATE INDEX IF NOT EXISTS idx_execute_log_started_at ON execute_log(started_at);
CREATE INDEX IF NOT EXISTS idx_execute_log_cinema_started ON execute_log(cinema_id, started_at);
CREATE INDEX IF NOT EXISTS idx_execute_log_task_started ON execute_log(crawl_task_id, started_at);
CREATE INDEX IF NOT EXISTS idx_execute_log_status_started ON execute_log(status, started_at);

-- ========== 5. 票价快照批次表 ==========
CREATE TABLE IF NOT EXISTS price_snapshot_batch (
    id                      BIGSERIAL PRIMARY KEY,
    crawl_task_id           BIGINT NOT NULL,
    cinema_id               BIGINT NOT NULL,
    execute_log_id          BIGINT,
    fetched_at              TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    source                  VARCHAR(50) NOT NULL DEFAULT 'maoyan',
    request_url             VARCHAR(500),
    request_params_json     TEXT,
    response_status_code    INT,
    raw_response_json       TEXT,
    raw_response_size       BIGINT,
    parse_status            VARCHAR(20) NOT NULL DEFAULT 'success',
    parse_error_msg         TEXT,
    total_movies            INT NOT NULL DEFAULT 0,
    total_showtimes         INT NOT NULL DEFAULT 0,
    price_range_min         DECIMAL(10, 2),
    price_range_max         DECIMAL(10, 2),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_batch_crawl_task_id ON price_snapshot_batch(crawl_task_id);
CREATE INDEX IF NOT EXISTS idx_batch_cinema_id ON price_snapshot_batch(cinema_id);
CREATE INDEX IF NOT EXISTS idx_batch_execute_log_id ON price_snapshot_batch(execute_log_id);
CREATE INDEX IF NOT EXISTS idx_batch_fetched_at ON price_snapshot_batch(fetched_at);
CREATE INDEX IF NOT EXISTS idx_batch_cinema_fetched ON price_snapshot_batch(cinema_id, fetched_at);
CREATE INDEX IF NOT EXISTS idx_batch_source ON price_snapshot_batch(source);

-- ========== 6. 票价快照明细表 ==========
CREATE TABLE IF NOT EXISTS price_snapshot_item (
    id                  BIGSERIAL PRIMARY KEY,
    batch_id            BIGINT NOT NULL,
    cinema_id           BIGINT NOT NULL,
    movie_id            VARCHAR(100) NOT NULL,
    movie_name          VARCHAR(200) NOT NULL,
    show_date           DATE NOT NULL,
    show_start_at       TIMESTAMPTZ NOT NULL,
    show_end_at         TIMESTAMPTZ,
    hall_name           VARCHAR(100),
    hall_type           VARCHAR(50),
    language            VARCHAR(50),
    version             VARCHAR(50),
    seat_type           VARCHAR(50),
    price               DECIMAL(10, 2) NOT NULL,
    original_price      DECIMAL(10, 2),
    discount_info       VARCHAR(200),
    available_seats     INT,
    total_seats         INT,
    booking_url         VARCHAR(500),
    raw_data_json       TEXT,
    observed_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_item_batch_id ON price_snapshot_item(batch_id);
CREATE INDEX IF NOT EXISTS idx_item_cinema_id ON price_snapshot_item(cinema_id);
CREATE INDEX IF NOT EXISTS idx_item_movie_id ON price_snapshot_item(movie_id);
CREATE INDEX IF NOT EXISTS idx_item_show_date ON price_snapshot_item(show_date);
CREATE INDEX IF NOT EXISTS idx_item_price ON price_snapshot_item(price);
CREATE INDEX IF NOT EXISTS idx_item_cinema_movie_date ON price_snapshot_item(cinema_id, movie_id, show_date);
CREATE INDEX IF NOT EXISTS idx_item_cinema_price ON price_snapshot_item(cinema_id, price);
CREATE INDEX IF NOT EXISTS idx_item_movie_price ON price_snapshot_item(movie_id, price);
CREATE INDEX IF NOT EXISTS idx_item_observed_at ON price_snapshot_item(observed_at);
CREATE INDEX IF NOT EXISTS idx_item_cinema_showtime ON price_snapshot_item(cinema_id, show_start_at);

-- ========== 7. 通知日志表 ==========
CREATE TABLE IF NOT EXISTS notify_log (
    id                  BIGSERIAL PRIMARY KEY,
    subscription_id     BIGINT NOT NULL,
    execute_log_id      BIGINT,
    cinema_id           BIGINT NOT NULL,
    email               VARCHAR(255) NOT NULL,
    notify_type         VARCHAR(50) NOT NULL DEFAULT 'price_alert',
    notify_status       VARCHAR(20) NOT NULL,
    target_price        DECIMAL(10, 2) NOT NULL,
    matched_price       DECIMAL(10, 2) NOT NULL,
    matched_items_json  TEXT,
    email_message_id    VARCHAR(255),
    email_response      TEXT,
    error_code          VARCHAR(50),
    error_msg           TEXT,
    retry_count         INT NOT NULL DEFAULT 0,
    max_retry           INT NOT NULL DEFAULT 3,
    next_retry_at       TIMESTAMPTZ,
    sent_at             TIMESTAMPTZ,
    opened_at           TIMESTAMPTZ,
    opened              SMALLINT NOT NULL DEFAULT 0,
    ip_address          VARCHAR(50),
    user_agent          VARCHAR(500),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_notify_log_subscription_id ON notify_log(subscription_id);
CREATE INDEX IF NOT EXISTS idx_notify_log_execute_log_id ON notify_log(execute_log_id);
CREATE INDEX IF NOT EXISTS idx_notify_log_cinema_id ON notify_log(cinema_id);
CREATE INDEX IF NOT EXISTS idx_notify_log_email ON notify_log(email);
CREATE INDEX IF NOT EXISTS idx_notify_log_notify_status ON notify_log(notify_status);
CREATE INDEX IF NOT EXISTS idx_notify_log_sent_at ON notify_log(sent_at);
CREATE INDEX IF NOT EXISTS idx_notify_log_subscription_sent ON notify_log(subscription_id, sent_at);
CREATE INDEX IF NOT EXISTS idx_notify_log_status_sent ON notify_log(notify_status, sent_at);
CREATE INDEX IF NOT EXISTS idx_notify_log_email_status ON notify_log(email, notify_status);

COMMIT;

-- ========== 验证 ==========
SELECT 'cinema', count(*) FROM information_schema.columns WHERE table_name = 'cinema'
UNION ALL SELECT 'subscription', count(*) FROM information_schema.columns WHERE table_name = 'subscription'
UNION ALL SELECT 'crawl_task', count(*) FROM information_schema.columns WHERE table_name = 'crawl_task'
UNION ALL SELECT 'execute_log', count(*) FROM information_schema.columns WHERE table_name = 'execute_log'
UNION ALL SELECT 'price_snapshot_batch', count(*) FROM information_schema.columns WHERE table_name = 'price_snapshot_batch'
UNION ALL SELECT 'price_snapshot_item', count(*) FROM information_schema.columns WHERE table_name = 'price_snapshot_item'
UNION ALL SELECT 'notify_log', count(*) FROM information_schema.columns WHERE table_name = 'notify_log'
UNION ALL SELECT 'users', count(*) FROM information_schema.columns WHERE table_name = 'users'
ORDER BY table_name;
