-- =====================================================
-- v2 订阅重构：按影院采集，一次采集 + 多订阅共享判断
-- 2026-07-02
-- =====================================================

-- 订阅表改造：movie_id 改为 nullable（历史兼容），新增 notify_email / user_email
ALTER TABLE subscriptions
    ALTER COLUMN movie_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS notify_email VARCHAR(512) DEFAULT '',
    ADD COLUMN IF NOT EXISTS user_email VARCHAR(256) NOT NULL DEFAULT '';

-- 订阅日志表改造：新增 status / result_msg / executed_at（替换旧表语义）
ALTER TABLE subscription_logs
    ADD COLUMN IF NOT EXISTS status VARCHAR(16) DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS result_msg VARCHAR(512) DEFAULT '',
    ADD COLUMN IF NOT EXISTS executed_at TIMESTAMPTZ;

-- 影院级采集任务表
CREATE TABLE IF NOT EXISTS crawl_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cinema_id       INTEGER      NOT NULL UNIQUE,   -- 一影院最多一个任务
    city_id         INTEGER      NOT NULL DEFAULT 0,
    interval_minutes INTEGER     NOT NULL DEFAULT 30,
    next_run_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    last_run_at     TIMESTAMPTZ,
    status          VARCHAR(16)  NOT NULL DEFAULT 'active',  -- active / paused / error
    last_error      VARCHAR(512)  DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_crawl_tasks_status ON crawl_tasks(status);
CREATE INDEX IF NOT EXISTS idx_crawl_tasks_next_run ON crawl_tasks(next_run_at) WHERE status = 'active';

-- 采集执行日志表（每次爬取一条记录）
CREATE TABLE IF NOT EXISTS execute_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    crawl_task_id   UUID         NOT NULL,
    cinema_id       INTEGER      NOT NULL,
    started_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    ended_at        TIMESTAMPTZ,
    status          VARCHAR(16)  NOT NULL DEFAULT 'running',  -- running / success / fail / partial
    error_code      VARCHAR(32)  DEFAULT '',
    error_msg       VARCHAR(512) DEFAULT '',
    fetched_count   INTEGER      DEFAULT 0,   -- 抓取到多少条场次
    matched_count   INTEGER      DEFAULT 0,   -- 命中多少个订阅
    notified_count  INTEGER      DEFAULT 0,   -- 成功通知多少次
    summary_json    TEXT         DEFAULT '',   -- 额外摘要 JSON
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_exec_logs_task ON execute_logs(crawl_task_id);
CREATE INDEX IF NOT EXISTS idx_exec_logs_cinema ON execute_logs(cinema_id);
CREATE INDEX IF NOT EXISTS idx_exec_logs_started ON execute_logs(started_at DESC);

-- 票价快照批次表（一次采集 = 一个批次）
CREATE TABLE IF NOT EXISTS price_snapshot_batches (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execute_log_id  UUID         NOT NULL,
    cinema_id       INTEGER      NOT NULL,
    fetched_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    source          VARCHAR(16)  NOT NULL DEFAULT 'maoyan',
    request_params  TEXT         DEFAULT '',   -- 请求参数 JSON
    raw_response    TEXT         DEFAULT '',   -- 原始响应（可空，数据量大时不存）
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_batch_log ON price_snapshot_batches(execute_log_id);
CREATE INDEX IF NOT EXISTS idx_batch_cinema ON price_snapshot_batches(cinema_id);
CREATE INDEX IF NOT EXISTS idx_batch_fetched ON price_snapshot_batches(fetched_at DESC);

-- 票价快照明细表（每条场次一条记录）
CREATE TABLE IF NOT EXISTS price_snapshot_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id        UUID         NOT NULL,
    cinema_id       INTEGER      NOT NULL,
    movie_id        INTEGER      NOT NULL,
    movie_name      VARCHAR(128) NOT NULL DEFAULT '',
    show_start_at   TIMESTAMPTZ  NOT NULL,
    show_end_at     TIMESTAMPTZ,
    hall_name       VARCHAR(64)  DEFAULT '',
    language        VARCHAR(32)  DEFAULT '',
    version         VARCHAR(16)  DEFAULT '',
    price           DECIMAL(8,1) NOT NULL DEFAULT 0,
    original_price  DECIMAL(8,1) DEFAULT 0,
    observed_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_items_batch ON price_snapshot_items(batch_id);
CREATE INDEX IF NOT EXISTS idx_items_cinema ON price_snapshot_items(cinema_id);
CREATE INDEX IF NOT EXISTS idx_items_movie ON price_snapshot_items(movie_id);
CREATE INDEX IF NOT EXISTS idx_items_observed ON price_snapshot_items(observed_at DESC);

-- 通知日志表
CREATE TABLE IF NOT EXISTS notify_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID         NOT NULL,
    execute_log_id  UUID         NOT NULL,
    email           VARCHAR(256) NOT NULL DEFAULT '',
    notify_status   VARCHAR(16)  NOT NULL DEFAULT 'pending',  -- pending / success / fail / skipped
    triggered_price DECIMAL(8,1) DEFAULT 0,
    notify_content  TEXT         DEFAULT '',
    error_msg       VARCHAR(512) DEFAULT '',
    sent_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_notify_sub ON notify_logs(subscription_id);
CREATE INDEX IF NOT EXISTS idx_notify_log ON notify_logs(execute_log_id);
CREATE INDEX IF NOT EXISTS idx_notify_email ON notify_logs(email);
CREATE INDEX IF NOT EXISTS idx_notify_sent ON notify_logs(sent_at DESC);


-- 城市表（缓存猫眼城市数据）
CREATE TABLE IF NOT EXISTS cities (
    id              INTEGER PRIMARY KEY,        -- 猫眼城市ID
    name            VARCHAR(64)  NOT NULL,
    pinyin          VARCHAR(64)  DEFAULT '',
    initial         CHAR(1)      DEFAULT '',
    lat             DOUBLE PRECISION DEFAULT 0,
    lng             DOUBLE PRECISION DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 区县表（缓存猫眼区县+商圈层级）
CREATE TABLE IF NOT EXISTS districts (
    id              INTEGER PRIMARY KEY,        -- 猫眼区县/商圈ID
    city_id         INTEGER      NOT NULL,
    parent_id       INTEGER      DEFAULT -1,    -- 父节点ID（-1表示区县，>0表示商圈）
    name            VARCHAR(64)  NOT NULL,
    cinema_count    INTEGER      DEFAULT 0,     -- 该区域影院数
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_districts_city ON districts(city_id);

-- 电影表
CREATE TABLE IF NOT EXISTS movies (
    id              INTEGER PRIMARY KEY,        -- 猫眼电影ID
    name            VARCHAR(128) NOT NULL,
    score           DECIMAL(3,1) DEFAULT 0,
    release_date    VARCHAR(32)  DEFAULT '',
    category        VARCHAR(64)  DEFAULT '',    -- 类型标签
    city_id         INTEGER      DEFAULT 1,     -- 所属城市（热映按城市不同）
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_movies_name ON movies(name);

-- 影院表
CREATE TABLE IF NOT EXISTS cinemas (
    id              INTEGER PRIMARY KEY,        -- 猫眼影院ID
    city_id         INTEGER      NOT NULL,
    district_id     INTEGER      DEFAULT -1,
    area_id         INTEGER      DEFAULT -1,    -- 商圈ID
    name            VARCHAR(128) NOT NULL,
    address         VARCHAR(256) DEFAULT '',
    distance        INTEGER      DEFAULT 0,     -- 距离(米)
    lat             DOUBLE PRECISION DEFAULT 0,
    lng             DOUBLE PRECISION DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_cinemas_city_district ON cinemas(city_id, district_id);

-- 排片快照表（存储每次查询到的排片价格）
CREATE TABLE IF NOT EXISTS show_snapshots (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id        INTEGER      NOT NULL,
    cinema_id       INTEGER      NOT NULL,
    show_date       VARCHAR(16)  NOT NULL,      -- YYYY-MM-DD
    show_time       VARCHAR(16)  NOT NULL,
    end_time        VARCHAR(16)  DEFAULT '',
    hall_name       VARCHAR(64)  DEFAULT '',
    lang            VARCHAR(32)  DEFAULT '',
    sell_price      DECIMAL(8,1) NOT NULL DEFAULT 0,  -- 当前原价（sellPr）
    vip_price       DECIMAL(8,1) DEFAULT 0,           -- 影城卡价
    base_price      DECIMAL(8,1) DEFAULT 0,           -- baseSellPrice
    discount_price  DECIMAL(8,1) DEFAULT 0,           -- discountSellPrice
    lowest_price    DECIMAL(8,1) NOT NULL DEFAULT 0,  -- 史低价格
    first_price     DECIMAL(8,1) NOT NULL DEFAULT 0,  -- 首次记录价格
    fetched_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_shows_movie_cinema ON show_snapshots(movie_id, cinema_id);
CREATE INDEX IF NOT EXISTS idx_shows_fetched ON show_snapshots(fetched_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_shows_unique
    ON show_snapshots(movie_id, cinema_id, show_date, show_time, hall_name, lang);

-- 价格变动历史表
CREATE TABLE IF NOT EXISTS price_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    show_snapshot_id UUID        NOT NULL,
    old_price       DECIMAL(8,1) NOT NULL,
    new_price       DECIMAL(8,1) NOT NULL,
    changed_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_price_history_snapshot ON price_history(show_snapshot_id);

-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(128) NOT NULL UNIQUE,
    password_hash   VARCHAR(256) NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- 订阅表
CREATE TABLE IF NOT EXISTS subscriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id        INTEGER      NOT NULL,
    cinema_id       INTEGER      NOT NULL,
    city_id         INTEGER      NOT NULL,
    district_id     INTEGER      DEFAULT -1,
    user_id         UUID         NOT NULL,
    target_price    DECIMAL(8,1) DEFAULT 0,     -- 目标价（0=任意低价都通知）
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    rate_limit_until          TIMESTAMPTZ,           -- 冷却期截至时间
    last_triggered_price DECIMAL(8,1) DEFAULT NULL, -- 上次触发通知时的价格
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_active ON subscriptions(is_active);
CREATE INDEX IF NOT EXISTS idx_subscriptions_movie_cinema ON subscriptions(movie_id, cinema_id);

-- 订阅通知日志
CREATE TABLE IF NOT EXISTS subscription_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID         NOT NULL,
    movie_id        INTEGER      NOT NULL,
    cinema_id       INTEGER      NOT NULL,
    price           DECIMAL(8,1) NOT NULL,
    notified_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sub_logs_subscription ON subscription_logs(subscription_id);
