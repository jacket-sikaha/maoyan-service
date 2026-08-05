package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==========================================
// GORM 实体 — 按 PDF 设计文档重建
// 设计原则：
//   1. 不加物理外键约束（PostgreSQL 层面无 FK）
//   2. 但遵循外键原理：字段名用 xxx_id，建索引保证查询效率
//   3. 主键统一 BIGSERIAL（int64），users 表例外保留 UUID
//   4. 所有表含 status 字段（0=禁用, 1=启用）和 created_at/updated_at
// ==========================================

// Cinema 影院基础表 — 整个系统的中心对象
// 设计核心：一次采集多次复用 → 一个影院对应一条 crawl_task，
// 采集结果以批次（batch）+ 明细（item）两层存储，多个 subscription 共享。
// maoyan_city_id 直接存猫眼城市 ID（int），无需中间映射表。
type Cinema struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	MaoyanCityID   int       `gorm:"not null;index" json:"maoyan_city_id"`                  // 猫眼城市 ID，直接存（无需 city 表映射）
	MaoyanCinemaID string    `gorm:"size:100;not null;uniqueIndex" json:"maoyan_cinema_id"` // 猫眼影院 ID，唯一标识
	Name           string    `gorm:"size:200;not null" json:"name"`
	Address        string    `gorm:"size:500" json:"address"`
	Latitude       float64   `gorm:"type:decimal(10,7)" json:"latitude"`
	Longitude      float64   `gorm:"type:decimal(11,7)" json:"longitude"`
	Phone          string    `gorm:"size:50" json:"phone"`
	Status         int8      `gorm:"not null;default:1" json:"status"` // 0-禁用, 1-启用
	CreatedAt      time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt      time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (Cinema) TableName() string { return "cinema" }

// User 用户表
// PDF 设计中未包含此表，但系统认证依赖它，予以保留。
// 与 subscription 的关系：subscription.email 对应 user.email（非强绑定）。
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email        string    `gorm:"size:128;uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"size:256;not null" json:"-"` // bcrypt 哈希，JSON 输出时隐藏
	CreatedAt    time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (User) TableName() string { return "users" }

// BeforeCreate GORM 钩子：插入前自动生成 UUID
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// Subscription 订阅规则表 — 影院×电影级订阅
// 核心设计：一个影院 + 一部电影 + 一个邮箱 = 一条订阅（唯一约束）
// 当采集到的该电影最低价 ≤ target_price 时触发邮件通知。
// MovieID 为 0 表示监控该影院所有电影（影院级订阅）。
type Subscription struct {
	ID                 uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CinemaID           uint64     `gorm:"not null;index" json:"cinema_id"`                          // 逻辑外键 → cinema.id
	MovieID            string     `gorm:"size:100;index;default:''" json:"movie_id"`                // 猫眼电影 ID，空串表示影院级订阅
	MovieName          string     `gorm:"size:200;default:''" json:"movie_name"`                    // 电影名称（冗余，展示用）
	Email              string     `gorm:"size:255;not null" json:"email"`                           // 通知邮箱
	CinemaName         string     `json:"cinema_name"`                                              // 影院名称（冗余，展示用）
	TargetPrice        float64    `gorm:"type:decimal(10,2);not null" json:"target_price"`          // 当前目标价，低于此价触发通知
	InitialTargetPrice float64    `gorm:"type:decimal(10,2);default:0" json:"initial_target_price"` // 创建时的初始目标价（编辑时只能调低）
	NotifyEnabled      bool       `gorm:"not null;default:true" json:"notify_enabled"`              // 是否启用通知
	Status             int8       `gorm:"not null;default:1" json:"status"`                         // 0-停用, 1-启用
	BaselineMinPrice   *float64   `gorm:"type:decimal(10,2)" json:"baseline_min_price,omitempty"`   // 首次采集记录的基准最低价
	BaselineMaxPrice   *float64   `gorm:"type:decimal(10,2)" json:"baseline_max_price,omitempty"`   // 基准最高价
	LastNotifyAt       *time.Time `gorm:"type:timestamptz" json:"last_notify_at,omitempty"`         // 最近一次通知时间，用于冷却判断
	NotifyCount        int        `gorm:"not null;default:0" json:"notify_count"`                   // 累计通知次数
	UserID             *uuid.UUID `gorm:"type:uuid" json:"user_id,omitempty"`                       // 可选，逻辑外键 → users.id
	Remark             string     `gorm:"size:500" json:"remark,omitempty"`                         // 用户备注
	CreatedAt          time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (Subscription) TableName() string { return "subscription" }

// CrawlTask 影院轮询任务表
// 设计：一个影院最多一条采集任务（cinema_id 唯一约束）
// 调度器定时查询 status=1 AND next_run_at <= now() 的任务执行
type CrawlTask struct {
	ID              uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CinemaID        uint64     `gorm:"not null;uniqueIndex" json:"cinema_id"`                                  // 逻辑外键 → cinema.id，唯一
	IntervalMinutes int        `gorm:"not null;default:30" json:"interval_minutes"`                            // 采集间隔（分钟），默认 30
	NextRunAt       time.Time  `gorm:"type:timestamptz;not null;default:CURRENT_TIMESTAMP" json:"next_run_at"` // 下次执行时间，调度器据此判断到期
	LastRunAt       *time.Time `gorm:"type:timestamptz" json:"last_run_at,omitempty"`                          // 最近一次执行时间
	LastSuccessAt   *time.Time `gorm:"type:timestamptz" json:"last_success_at,omitempty"`                      // 最近一次成功时间
	RunCount        int        `gorm:"not null;default:0" json:"run_count"`                                    // 总执行次数
	FailCount       int        `gorm:"not null;default:0" json:"fail_count"`                                   // 失败次数
	SuccessCount    int        `gorm:"not null;default:0" json:"success_count"`                                // 成功次数
	Status          int8       `gorm:"not null;default:1" json:"status"`                                       // 0-停用, 1-启用, 2-暂停（连续失败自动暂停）
	Priority        int        `gorm:"not null;default:100" json:"priority"`                                   // 优先级，数值越小越先执行
	TimeoutSeconds  int        `gorm:"not null;default:60" json:"timeout_seconds"`                             // 单次执行超时秒数
	LastError       string     `gorm:"size:1000" json:"last_error,omitempty"`                                  // 最近一次执行错误信息
	CreatedAt       time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (CrawlTask) TableName() string { return "crawl_task" }

// ExecuteLog 任务执行日志表
// 每次执行 crawl_task 产生一条记录，记录采集结果统计和耗时。
// 一个 crawl_task → 多条 execute_log（一对多）。
type ExecuteLog struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CrawlTaskID   uint64     `gorm:"not null;index" json:"crawl_task_id"` // 逻辑外键 → crawl_task.id
	CinemaID      uint64     `gorm:"not null;index" json:"cinema_id"`     // 逻辑外键 → cinema.id（冗余，方便查询）
	SnapshotID    *uint64    `gorm:"index" json:"snapshot_id,omitempty"`  // 逻辑外键 → price_snapshot.id
	StartedAt     time.Time  `gorm:"type:timestamptz;not null;default:CURRENT_TIMESTAMP" json:"started_at"`
	EndedAt       *time.Time `gorm:"type:timestamptz" json:"ended_at,omitempty"`
	DurationMs    *int       `gorm:"" json:"duration_ms,omitempty"`  // 执行耗时（毫秒）
	Status        string     `gorm:"size:20;not null" json:"status"` // running/success/fail/partial
	ErrorCode     string     `gorm:"size:50" json:"error_code,omitempty"`
	ErrorMsg      string     `gorm:"type:text" json:"error_msg,omitempty"`
	FetchedCount  int        `gorm:"not null;default:0" json:"fetched_count"`   // 本次拉取的场次总数
	MatchedCount  int        `gorm:"not null;default:0" json:"matched_count"`   // 匹配到订阅的数量
	NotifiedCount int        `gorm:"not null;default:0" json:"notified_count"`  // 实际发送通知的数量
	SkippedCount  int        `gorm:"not null;default:0" json:"skipped_count"`   // 跳过的数量（如冷却期内）
	CooldownCount int        `gorm:"not null;default:0" json:"cooldown_count"`  // 因冷却跳过的数量
	SummaryJSON   string     `gorm:"type:text" json:"summary_json,omitempty"`   // 执行摘要 JSON
	RequestParams string     `gorm:"type:text" json:"request_params,omitempty"` // 请求参数 JSON
	ResponseSize  *int       `gorm:"" json:"response_size,omitempty"`
	CreatedAt     time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (ExecuteLog) TableName() string { return "execute_log" }

// PriceSnapshot 票价快照表（合并原 batch + item 为单表）
// 每次采集每个影院产生一条记录，存储原始 JSON + 按天统计的各电影价格数据。
// 一个影院 → 多条 price_snapshot（按采集时间倒序）。
type PriceSnapshot struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CrawlTaskID    uint64    `gorm:"not null;index" json:"crawl_task_id"`                                   // 逻辑外键 → crawl_task.id
	CinemaID       uint64    `gorm:"not null;index" json:"cinema_id"`                                       // 逻辑外键 → cinema.id
	ExecuteLogID   *uint64   `gorm:"index" json:"execute_log_id,omitempty"`                                 // 逻辑外键 → execute_log.id
	FetchedAt      time.Time `gorm:"type:timestamptz;not null;default:CURRENT_TIMESTAMP" json:"fetched_at"` // 采集时间
	Source         string    `gorm:"size:50;not null;default:'maoyan'" json:"source"`                       // 数据来源
	TotalMovies    int       `gorm:"not null;default:0" json:"total_movies"`                                // 本次采集涉及电影数
	TotalShowtimes int       `gorm:"not null;default:0" json:"total_showtimes"`                             // 本次采集总场次数
	RawJSON        string    `gorm:"type:text" json:"raw_json,omitempty"`                                   // 原始采集数据 JSON（含所有场次明细）
	MovieStatsJSON string    `gorm:"type:text" json:"movie_stats_json,omitempty"`                            // 按电影×日期统计的价格 JSON（最低/平均/最高）
	ParseStatus    string    `gorm:"size:20;not null;default:'success'" json:"parse_status"`                // success/fail/partial
	ParseErrorMsg  string    `gorm:"type:text" json:"parse_error_msg,omitempty"`
	CreatedAt      time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (PriceSnapshot) TableName() string { return "price_snapshot" }

// NotifyLog 通知日志表
// 每次触发邮件通知（无论成功失败）都记录一条。
// 用于通知历史查询、重试、打开追踪。
type NotifyLog struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	SubscriptionID   uint64     `gorm:"not null;index" json:"subscription_id"`                     // 逻辑外键 → subscription.id
	ExecuteLogID     *uint64    `gorm:"index" json:"execute_log_id,omitempty"`                     // 逻辑外键 → execute_log.id
	CinemaID         uint64     `gorm:"not null;index" json:"cinema_id"`                           // 逻辑外键 → cinema.id（冗余）
	Email            string     `gorm:"size:255;not null" json:"email"`                            // 收件人邮箱
	NotifyType       string     `gorm:"size:50;not null;default:'price_alert'" json:"notify_type"` // price_alert/initial/daily_summary
	NotifyStatus     string     `gorm:"size:20;not null" json:"notify_status"`                     // pending/success/fail/skip
	TargetPrice      float64    `gorm:"type:decimal(10,2);not null" json:"target_price"`           // 订阅目标价
	MatchedPrice     float64    `gorm:"type:decimal(10,2);not null" json:"matched_price"`          // 命中的实际票价
	MatchedItemsJSON string     `gorm:"type:text" json:"matched_items_json,omitempty"`             // 匹配到的场次详情 JSON
	EmailMessageID   string     `gorm:"size:255" json:"email_message_id,omitempty"`                // SMTP Message-ID，用于追踪
	EmailResponse    string     `gorm:"type:text" json:"email_response,omitempty"`
	ErrorCode        string     `gorm:"size:50" json:"error_code,omitempty"`
	ErrorMsg         string     `gorm:"type:text" json:"error_msg,omitempty"`
	RetryCount       int        `gorm:"not null;default:0" json:"retry_count"`           // 已重试次数
	MaxRetry         int        `gorm:"not null;default:3" json:"max_retry"`             // 最大重试次数
	NextRetryAt      *time.Time `gorm:"type:timestamptz" json:"next_retry_at,omitempty"` // 下次重试时间
	SentAt           *time.Time `gorm:"type:timestamptz" json:"sent_at,omitempty"`       // 发送时间
	OpenedAt         *time.Time `gorm:"type:timestamptz" json:"opened_at,omitempty"`     // 邮件打开时间（追踪）
	Opened           bool       `gorm:"not null;default:false" json:"opened"`            // 是否已打开
	IPAddress        string     `gorm:"size:50" json:"ip_address,omitempty"`
	UserAgent        string     `gorm:"size:500" json:"user_agent,omitempty"`
	CreatedAt        time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (NotifyLog) TableName() string { return "notify_log" }

// ==========================================
// 请求/响应 DTO（Data Transfer Objects）
// 用于 API 层的数据传输，与数据库模型分离
// ==========================================

// --- 认证 ---

type RegisterReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  struct {
		ID    uuid.UUID `json:"id"`
		Email string    `json:"email"`
	} `json:"user"`
}

// --- 公共 DTO ---

// HotMovieItem 热映电影信息（透传猫眼 API）
type HotMovieItem struct {
	MovieID        int     `json:"movie_id"`
	Name           string  `json:"name"`
	Img            string  `json:"img"`
	Score          float64 `json:"score"`
	Version        string  `json:"version"`
	Star           string  `json:"star"`
	ReleaseDate    string  `json:"release_date"`
	ShowInfo       string  `json:"show_info"`
	ShowState      int     `json:"showst"`
	Wish           int     `json:"wish"`
	GlobalReleased bool    `json:"global_released"`
	ComingTitle    string  `json:"coming_title"`
}

// CinemaItem 影院列表项（前端展示用）
type CinemaItem struct {
	CinemaID     uint64 `json:"cinema_id"`
	Name         string `json:"name"`
	Address      string `json:"address"`
	Distance     int    `json:"distance"`
	MaoyanCityID int    `json:"maoyan_city_id,omitempty"`
}

// CinemaMovieItem 影院+电影组合（票价变化页筛选用）
type CinemaMovieItem struct {
	CinemaID     uint64 `json:"cinema_id"`
	CinemaName   string `json:"cinema_name"`
	CinemaAddress string `json:"cinema_address,omitempty"`
	MaoyanCityID int    `json:"maoyan_city_id,omitempty"`
	MovieID      string `json:"movie_id"`
	MovieName    string `json:"movie_name"`
}

// ShowInfo 场次票价信息（查询结果）
type ShowInfo struct {
	CinemaID      uint64  `json:"cinema_id"`
	CinemaName    string  `json:"cinema_name"`
	CinemaAddress string  `json:"cinema_address"`
	DistanceKm    float64 `json:"distance_km"`
	ShowDate      string  `json:"show_date"`
	ShowTime      string  `json:"show_time"`
	EndTime       string  `json:"end_time"`
	HallName      string  `json:"hall_name"`
	Lang          string  `json:"language"`
	Price         float64 `json:"price"`
	VIPPrice      float64 `json:"vip_price"`
	BasePrice     float64 `json:"base_price,omitempty"`
	DiscountPrice float64 `json:"discount_price,omitempty"`
	LowestPrice   float64 `json:"lowest_price,omitempty"`
}

// --- 订阅 DTO ---

// SubscriptionReq 创建订阅请求
type SubscriptionReq struct {
	CinemaID     uint64  `json:"cinema_id" binding:"required"` // 猫眼影院 ID
	CinemaName   string  `json:"cinema_name"`                  // 影院名称
	MaoyanCityID int     `json:"maoyan_city_id"`               // 猫眼城市 ID
	MovieID      string  `json:"movie_id"`                     // 猫眼电影 ID（空串表示影院级订阅）
	MovieName    string  `json:"movie_name"`                   // 电影名称
	Email        string  `json:"email" binding:"required,email"`
	TargetPrice  float64 `json:"target_price" binding:"required"`
	Remark       string  `json:"remark"`
}

// SubscribeResponse 创建订阅响应
type SubscribeResponse struct {
	ID            uint64  `json:"id"`
	CinemaID      uint64  `json:"cinema_id"`
	MovieID       string  `json:"movie_id"`
	MovieName     string  `json:"movie_name"`
	Email         string  `json:"email"`
	TargetPrice   float64 `json:"target_price"`
	Status        int8    `json:"status"`
	NotifyEnabled bool    `json:"notify_enabled"`
	Message       string  `json:"message"`
}

// ToggleSubscriptionReq 切换订阅状态
type ToggleSubscriptionReq struct {
	Status int8 `json:"status"` // 0-停用, 1-启用
}

// SubscriptionFullInfo 订阅完整信息（列表页用）
type SubscriptionFullInfo struct {
	ID                 uint64     `json:"id"`
	CinemaID           uint64     `json:"cinema_id"`
	MovieID            string     `json:"movie_id"`
	MovieName          string     `json:"movie_name"`
	Email              string     `json:"email"`
	TargetPrice        float64    `json:"target_price"`
	InitialTargetPrice float64    `json:"initial_target_price"`
	NotifyEnabled      bool       `json:"notify_enabled"`
	Status             int8       `json:"status"`
	BaselineMinPrice   *float64   `json:"baseline_min_price,omitempty"`
	BaselineMaxPrice   *float64   `json:"baseline_max_price,omitempty"`
	LastNotifyAt       *time.Time `json:"last_notify_at,omitempty"`
	NotifyCount        int        `json:"notify_count"`
	Remark             string     `json:"remark,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	// 关联信息（从 cinema 表拼接）
	CinemaName    string `json:"cinema_name,omitempty"`
	CinemaAddress string `json:"cinema_address,omitempty"`
}

// SubscriptionUpdateReq 更新订阅请求（允许修改目标票价、备注、状态）
type SubscriptionUpdateReq struct {
	TargetPrice *float64 `json:"target_price,omitempty"` // 新目标票价，必须 ≤ initial_target_price
	Remark      *string  `json:"remark,omitempty"`
	Status      *int8    `json:"status,omitempty"` // 0=停用, 1=启用
}

// --- 订阅详情 DTO ---

// ShowPriceForSubscription 订阅详情页的场次票价信息
type ShowPriceForSubscription struct {
	MovieName     string  `json:"movie_name"`
	CinemaName    string  `json:"cinema_name"`
	HallName      string  `json:"hall_name"`
	ShowDate      string  `json:"show_date"`
	ShowTime      string  `json:"show_time"`
	CurrentPrice  float64 `json:"current_price"`
	VIPPrice      float64 `json:"vip_price"`
	BasePrice     float64 `json:"base_price,omitempty"`
	DiscountPrice float64 `json:"discount_price,omitempty"`
}

// PriceTrendPoint 价格趋势数据点（前端折线图用）
type PriceTrendPoint struct {
	Time     string  `json:"time"`
	PriceMin float64 `json:"price_min"`
	PriceAvg float64 `json:"price_avg"`
	PriceMax float64 `json:"price_max"`
}

// CrawlRecordItem 单次采集记录
type CrawlRecordItem struct {
	SnapshotID    uint64    `json:"snapshot_id"`
	FetchedAt     time.Time `json:"fetched_at"`
	TotalMovies   int       `json:"total_movies"`
	TotalShowtimes int      `json:"total_showtimes"`
	ParseStatus   string    `json:"parse_status"`
}

// MovieCrawlDetail 某部电影的采集详情
type MovieCrawlDetail struct {
	MovieID   string  `json:"movie_id"`
	MovieName string  `json:"movie_name"`
	MinPrice  float64 `json:"min_price"`
	AvgPrice  float64 `json:"avg_price"`
	MaxPrice  float64 `json:"max_price"`
	Showtimes int     `json:"showtimes"`
	Sum       float64 `json:"-"`
	Count     int     `json:"-"`
}

// CrawlRecordsDashboard 采集记录仪表盘
type CrawlRecordsDashboard struct {
	CinemaName      string            `json:"cinema_name"`
	CinemaID        uint64            `json:"cinema_id"`
	TotalSnapshots  int               `json:"total_snapshots"`
	TotalShowtimes  int               `json:"total_showtimes"`
	TotalMovies     int               `json:"total_movies"`
	GlobalMinPrice  float64           `json:"global_min_price"`
	GlobalAvgPrice  float64           `json:"global_avg_price"`
	GlobalMaxPrice  float64           `json:"global_max_price"`
	Records         []CrawlRecordItem `json:"records"`
	Movies          []MovieCrawlDetail `json:"movies"`
}

// SubscriptionDetail 订阅详情页数据
type SubscriptionDetail struct {
	Subscription Subscription               `json:"subscription"`
	CurrentShows []ShowPriceForSubscription `json:"current_shows"` // 当前行情
	PriceTrend   []PriceTrendPoint          `json:"price_trend"`   // 价格趋势折线图
	HistoryTotal int64                      `json:"history_total"` // 历史记录总数
}

// SubscriptionLogFullInfo 通知日志完整信息（日志页用）
type SubscriptionLogFullInfo struct {
	ID             uint64     `json:"id"`
	SubscriptionID uint64     `json:"subscription_id"`
	CinemaName     string     `json:"cinema_name"`
	Email          string     `json:"email"`
	NotifyType     string     `json:"notify_type"`
	NotifyStatus   string     `json:"notify_status"`
	TargetPrice    float64    `json:"target_price"`
	MatchedPrice   float64    `json:"matched_price"`
	ErrorMsg       string     `json:"error_msg,omitempty"`
	SentAt         *time.Time `json:"sent_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
