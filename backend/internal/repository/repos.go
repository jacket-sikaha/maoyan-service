package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"maoyan-service/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==========================================
// 仓储接口定义 — 按 PDF 设计文档重建
// 设计原则：
//   1. 接口与实现分离，方便 Mock 测试
//   2. 所有方法接收 ctx，支持上下文超时/取消
//   3. ID 类型统一 uint64（对应数据库 BIGSERIAL）
//   4. 不使用物理外键，通过逻辑外键 + 索引保证一致性
// ==========================================

// CinemaRepo 影院仓储接口
type CinemaRepo interface {
	Create(ctx context.Context, cinema *model.Cinema) error                                // 创建单条
	Upsert(ctx context.Context, cinema *model.Cinema) error                                // 插入或更新单条
	BulkUpsert(ctx context.Context, cinemas []model.Cinema) error                          // 批量插入或更新
	GetByID(ctx context.Context, id uint64) (*model.Cinema, error)                         // 按主键查询
	GetByMaoyanCinemaID(ctx context.Context, maoyanCinemaID string) (*model.Cinema, error) // 按猫眼影院 ID 查询
	GetByMaoyanCityID(ctx context.Context, maoyanCityID int) ([]model.Cinema, error)       // 按猫眼城市 ID 查询影院列表
	SearchByName(ctx context.Context, keyword string) ([]model.Cinema, error)              // 按名称模糊搜索
}

// UserRepo 用户仓储接口
type UserRepo interface {
	Create(ctx context.Context, user *model.User) error                 // 创建用户
	FindByEmail(ctx context.Context, email string) (*model.User, error) // 按邮箱查询（登录用）
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)    // 按 UUID 查询
}

// SubscriptionRepo 订阅仓储接口
type SubscriptionRepo interface {
	Create(ctx context.Context, sub *model.Subscription) error                                                             // 创建订阅
	Update(ctx context.Context, sub *model.Subscription) error                                                             // 更新整条订阅
	UpdateFields(ctx context.Context, id uint64, updates map[string]interface{}) error                                     // 按字段更新
	Delete(ctx context.Context, id uint64) error                                                                           // 删除订阅
	FindByID(ctx context.Context, id uint64) (*model.Subscription, error)                                                  // 按主键查询（预加载 Cinema）
	FindByCinemaID(ctx context.Context, cinemaID uint64) ([]model.Subscription, error)                                     // 按影院查询所有订阅（采集任务用）
	FindByEmail(ctx context.Context, email string) ([]model.Subscription, error)                                           // 按邮箱查询所有订阅
	FindByCinemaAndEmail(ctx context.Context, cinemaID uint64, email string) (*model.Subscription, error)                  // 精确查询（影院级去重）
	FindByCinemaAndMovieAndEmail(ctx context.Context, cinemaID uint64, movieID, email string) (*model.Subscription, error) // 精确查询（电影级去重）
	ListActive(ctx context.Context) ([]model.Subscription, error)                                                          // 查询所有启用订阅
	UpdateNotifyCount(ctx context.Context, id uint64) error                                                                // 通知次数 +1
	UpdateLastNotifyAt(ctx context.Context, id uint64, at time.Time) error                                                 // 更新最后通知时间
}

// CrawlTaskRepo 采集任务仓储接口
type CrawlTaskRepo interface {
	Create(ctx context.Context, task *model.CrawlTask) error                                             // 创建采集任务
	Update(ctx context.Context, task *model.CrawlTask) error                                             // 更新整条任务
	FindByID(ctx context.Context, id uint64) (*model.CrawlTask, error)                                   // 按主键查询
	FindByCinemaID(ctx context.Context, cinemaID uint64) (*model.CrawlTask, error)                       // 按影院查询（一个影院最多一个任务）
	ListDue(ctx context.Context, before time.Time) ([]model.CrawlTask, error)                            // 查询到期任务（调度器核心方法）
	ListActive(ctx context.Context) ([]model.CrawlTask, error)                                           // 查询所有启用任务
	DeleteByCinemaID(ctx context.Context, cinemaID uint64) error                                         // 按影院删除任务
	UpdateNextRun(ctx context.Context, id uint64, nextRun time.Time) error                               // 更新下次执行时间
	UpdateLastRun(ctx context.Context, id uint64, lastRun time.Time, status string, errMsg string) error // 更新执行状态
	UpdateStats(ctx context.Context, id uint64, success bool) error                                      // 更新统计计数（run_count/success_count/fail_count）
}

// ExecuteLogRepo 执行日志仓储接口
type ExecuteLogRepo interface {
	Create(ctx context.Context, log *model.ExecuteLog) error                                          // 创建日志
	Update(ctx context.Context, log *model.ExecuteLog) error                                          // 更新日志
	FindByID(ctx context.Context, id uint64) (*model.ExecuteLog, error)                               // 按主键查询
	FindByCrawlTaskID(ctx context.Context, crawlTaskID uint64, limit int) ([]model.ExecuteLog, error) // 按任务查询历史日志
}

// PriceSnapshotRepo 票价快照仓储接口（合并原 BatchRepo + ItemRepo）
type PriceSnapshotRepo interface {
	Create(ctx context.Context, snapshot *model.PriceSnapshot) error                                         // 创建快照
	FindByID(ctx context.Context, id uint64) (*model.PriceSnapshot, error)                                   // 按主键查询
	FindByExecuteLogID(ctx context.Context, execLogID uint64) ([]model.PriceSnapshot, error)                 // 按执行日志查询
	FindByCinemaID(ctx context.Context, cinemaID uint64, limit int) ([]model.PriceSnapshot, error)           // 按影院查询历史快照
	FindByCinemaIDWithTimeRange(ctx context.Context, cinemaID uint64, startTime, endTime time.Time, limit int) ([]model.PriceSnapshot, error) // 按影院+时间范围查询快照
	FindPriceTrendByCinemaAndMovie(ctx context.Context, cinemaID uint64, movieID string, startTime, endTime time.Time, limit int) ([]model.PriceTrendPoint, error) // 查询影院+电影价格趋势（按时间范围）
}

// NotifyLogRepo 通知日志仓储接口
type NotifyLogRepo interface {
	BulkCreate(ctx context.Context, logs []model.NotifyLog) error                                              // 批量写入通知日志
	FindBySubscriptionID(ctx context.Context, subID uint64, limit int) ([]model.NotifyLog, error)              // 按订阅查询通知历史
	FindByExecuteLogID(ctx context.Context, execLogID uint64) ([]model.NotifyLog, error)                       // 按执行日志查询
	FindByUserID(ctx context.Context, userID uuid.UUID, startDate, endDate *time.Time, status string, page, pageSize int) ([]model.NotifyLog, int64, error) // 按用户分页查询（JOIN subscription），支持时间+状态筛选
	FindRecentBySubscription(ctx context.Context, subID uint64, since time.Duration) (*model.NotifyLog, error) // 查询近期通知（冷却判断用）
}

// ==========================================
// GORM 实现
// ==========================================

type cinemaRepo struct{ db *gorm.DB }
type userRepo struct{ db *gorm.DB }
type subscriptionRepo struct{ db *gorm.DB }
type crawlTaskRepo struct{ db *gorm.DB }
type executeLogRepo struct{ db *gorm.DB }
type priceSnapshotRepo struct{ db *gorm.DB }
type notifyLogRepo struct{ db *gorm.DB }

func NewCinemaRepo(db *gorm.DB) CinemaRepo               { return &cinemaRepo{db} }
func NewUserRepo(db *gorm.DB) UserRepo                   { return &userRepo{db} }
func NewSubscriptionRepo(db *gorm.DB) SubscriptionRepo   { return &subscriptionRepo{db} }
func NewCrawlTaskRepo(db *gorm.DB) CrawlTaskRepo         { return &crawlTaskRepo{db} }
func NewExecuteLogRepo(db *gorm.DB) ExecuteLogRepo       { return &executeLogRepo{db} }
func NewPriceSnapshotRepo(db *gorm.DB) PriceSnapshotRepo { return &priceSnapshotRepo{db} }
func NewNotifyLogRepo(db *gorm.DB) NotifyLogRepo         { return &notifyLogRepo{db} }

// ========== Cinema 实现 ==========

func (r *cinemaRepo) Create(ctx context.Context, cinema *model.Cinema) error {
	return r.db.WithContext(ctx).Create(cinema).Error
}

func (r *cinemaRepo) Upsert(ctx context.Context, cinema *model.Cinema) error {
	r.db.WithContext(ctx)
	return r.db.WithContext(ctx).Save(cinema).Error
}

// BulkUpsert 批量插入或更新：按 maoyan_cinema_id 判断是否存在
func (r *cinemaRepo) BulkUpsert(ctx context.Context, cinemas []model.Cinema) error {
	if len(cinemas) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, c := range cinemas {
			var existing model.Cinema
			err := tx.Where("maoyan_cinema_id = ?", c.MaoyanCinemaID).First(&existing).Error
			if err == gorm.ErrRecordNotFound {
				if err := tx.Create(&c).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				c.ID = existing.ID
				if err := tx.Save(&c).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *cinemaRepo) GetByID(ctx context.Context, id uint64) (*model.Cinema, error) {
	var cinema model.Cinema
	err := r.db.WithContext(ctx).First(&cinema, id).Error
	if err != nil {
		return nil, err
	}
	fmt.Printf("cinema: %v\n", cinema)
	return &cinema, nil
}

func (r *cinemaRepo) GetByMaoyanCinemaID(ctx context.Context, maoyanCinemaID string) (*model.Cinema, error) {
	var cinema model.Cinema
	err := r.db.WithContext(ctx).Where("maoyan_cinema_id = ?", maoyanCinemaID).First(&cinema).Error
	if err != nil {
		return nil, err
	}
	return &cinema, nil
}

func (r *cinemaRepo) GetByMaoyanCityID(ctx context.Context, maoyanCityID int) ([]model.Cinema, error) {
	var cinemas []model.Cinema
	err := r.db.WithContext(ctx).Where("maoyan_city_id = ? AND status = 1", maoyanCityID).Order("name").Find(&cinemas).Error
	return cinemas, err
}

// SearchByName 按名称模糊搜索影院（PostgreSQL ILIKE 不区分大小写）
func (r *cinemaRepo) SearchByName(ctx context.Context, keyword string) ([]model.Cinema, error) {
	var cinemas []model.Cinema
	err := r.db.WithContext(ctx).Where("name ILIKE ? AND status = 1", "%"+keyword+"%").Limit(50).Find(&cinemas).Error
	return cinemas, err
}

// ========== User 实现 ==========

func (r *userRepo) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ========== Subscription 实现 ==========

func (r *subscriptionRepo) Create(ctx context.Context, sub *model.Subscription) error {
	return r.db.WithContext(ctx).Create(sub).Error
}

func (r *subscriptionRepo) Update(ctx context.Context, sub *model.Subscription) error {
	return r.db.WithContext(ctx).Save(sub).Error
}

// UpdateFields 按字段更新（部分更新），避免覆盖整条记录
func (r *subscriptionRepo) UpdateFields(ctx context.Context, id uint64, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.Subscription{}).Where("id = ?", id).Updates(updates).Error
}

func (r *subscriptionRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Subscription{}).Error
}

// FindByID 按主键查询
func (r *subscriptionRepo) FindByID(ctx context.Context, id uint64) (*model.Subscription, error) {
	var sub model.Subscription
	err := r.db.WithContext(ctx).
		First(&sub, id).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// FindByCinemaID 查询某影院下所有启用的订阅（采集任务执行时调用）
func (r *subscriptionRepo) FindByCinemaID(ctx context.Context, cinemaID uint64) ([]model.Subscription, error) {
	var subs []model.Subscription
	err := r.db.WithContext(ctx).
		Where("cinema_id = ? AND status = 1", cinemaID).
		Find(&subs).Error
	return subs, err
}

// FindByEmail 按邮箱查询订阅（用户订阅列表页用）
func (r *subscriptionRepo) FindByEmail(ctx context.Context, email string) ([]model.Subscription, error) {
	var subs []model.Subscription
	err := r.db.WithContext(ctx).
		Where("email = ?", email).
		Order("created_at DESC").
		Find(&subs).Error
	return subs, err
}

// FindByCinemaAndEmail 精确查询（影院级去重判断用）
func (r *subscriptionRepo) FindByCinemaAndEmail(ctx context.Context, cinemaID uint64, email string) (*model.Subscription, error) {
	var sub model.Subscription
	err := r.db.WithContext(ctx).
		Where("cinema_id = ? AND email = ? AND movie_id = ''", cinemaID, email).
		First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// FindByCinemaAndMovieAndEmail 精确查询（电影级去重判断用）
func (r *subscriptionRepo) FindByCinemaAndMovieAndEmail(ctx context.Context, cinemaID uint64, movieID, email string) (*model.Subscription, error) {
	var sub model.Subscription
	err := r.db.WithContext(ctx).
		Where("cinema_id = ? AND movie_id = ? AND email = ?", cinemaID, movieID, email).
		First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *subscriptionRepo) ListActive(ctx context.Context) ([]model.Subscription, error) {
	var subs []model.Subscription
	err := r.db.WithContext(ctx).
		Where("status = 1").
		Find(&subs).Error
	return subs, err
}

// UpdateNotifyCount 通知次数原子 +1（并发安全）
func (r *subscriptionRepo) UpdateNotifyCount(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.Subscription{}).
		Where("id = ?", id).
		UpdateColumn("notify_count", gorm.Expr("notify_count + 1")).Error
}

func (r *subscriptionRepo) UpdateLastNotifyAt(ctx context.Context, id uint64, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.Subscription{}).
		Where("id = ?", id).
		Update("last_notify_at", at).Error
}

// ========== CrawlTask 实现 ==========

func (r *crawlTaskRepo) Create(ctx context.Context, task *model.CrawlTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *crawlTaskRepo) Update(ctx context.Context, task *model.CrawlTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}

func (r *crawlTaskRepo) FindByID(ctx context.Context, id uint64) (*model.CrawlTask, error) {
	var task model.CrawlTask
	err := r.db.WithContext(ctx).First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// FindByCinemaID 按影院查询采集任务（一个影院最多一个，因为 cinema_id 有唯一约束）
func (r *crawlTaskRepo) FindByCinemaID(ctx context.Context, cinemaID uint64) (*model.CrawlTask, error) {
	var task model.CrawlTask
	err := r.db.WithContext(ctx).Where("cinema_id = ?", cinemaID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// ListDue 查询所有到期且启用的任务（调度器核心查询）
// 条件：status=1（启用）AND next_run_at <= before
// 排序：priority ASC（数值小优先），next_run_at ASC（先到期的先执行）
func (r *crawlTaskRepo) ListDue(ctx context.Context, before time.Time) ([]model.CrawlTask, error) {
	var tasks []model.CrawlTask
	err := r.db.WithContext(ctx).
		Where("status = 1 AND next_run_at <= ?", before).
		Order("priority ASC, next_run_at ASC").
		Find(&tasks).Error
	return tasks, err
}

func (r *crawlTaskRepo) ListActive(ctx context.Context) ([]model.CrawlTask, error) {
	var tasks []model.CrawlTask
	err := r.db.WithContext(ctx).Where("status = 1").Find(&tasks).Error
	return tasks, err
}

func (r *crawlTaskRepo) DeleteByCinemaID(ctx context.Context, cinemaID uint64) error {
	return r.db.WithContext(ctx).Where("cinema_id = ?", cinemaID).Delete(&model.CrawlTask{}).Error
}

// UpdateNextRun 更新下次执行时间（执行完成后调用）
func (r *crawlTaskRepo) UpdateNextRun(ctx context.Context, id uint64, nextRun time.Time) error {
	return r.db.WithContext(ctx).Model(&model.CrawlTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"next_run_at": nextRun, "updated_at": time.Now()}).Error
}

// UpdateLastRun 更新最近执行状态
// status 传入 "error" → status=2（暂停）, "active" → status=1（启用）
func (r *crawlTaskRepo) UpdateLastRun(ctx context.Context, id uint64, lastRun time.Time, status string, errMsg string) error {
	updates := map[string]interface{}{
		"last_run_at": lastRun,
		"updated_at":  time.Now(),
	}
	if status == "error" {
		updates["status"] = int8(2)
		updates["last_error"] = errMsg
	} else if status == "active" {
		updates["status"] = int8(1)
		updates["last_error"] = ""
	}
	return r.db.WithContext(ctx).Model(&model.CrawlTask{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateStats 更新执行统计（原子操作）
// success=true → run_count+1, success_count+1, last_success_at=now
// success=false → run_count+1, fail_count+1
func (r *crawlTaskRepo) UpdateStats(ctx context.Context, id uint64, success bool) error {
	updates := map[string]interface{}{
		"run_count":  gorm.Expr("run_count + 1"),
		"updated_at": time.Now(),
	}
	if success {
		updates["success_count"] = gorm.Expr("success_count + 1")
		updates["last_success_at"] = time.Now()
	} else {
		updates["fail_count"] = gorm.Expr("fail_count + 1")
	}
	return r.db.WithContext(ctx).Model(&model.CrawlTask{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// ========== ExecuteLog 实现 ==========

func (r *executeLogRepo) Create(ctx context.Context, log *model.ExecuteLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *executeLogRepo) Update(ctx context.Context, log *model.ExecuteLog) error {
	return r.db.WithContext(ctx).Save(log).Error
}

func (r *executeLogRepo) FindByID(ctx context.Context, id uint64) (*model.ExecuteLog, error) {
	var log model.ExecuteLog
	err := r.db.WithContext(ctx).First(&log, id).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// FindByCrawlTaskID 按采集任务查询执行历史（按时间倒序，限制条数）
func (r *executeLogRepo) FindByCrawlTaskID(ctx context.Context, crawlTaskID uint64, limit int) ([]model.ExecuteLog, error) {
	var logs []model.ExecuteLog
	err := r.db.WithContext(ctx).
		Where("crawl_task_id = ?", crawlTaskID).
		Order("started_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// ========== PriceSnapshot 实现 ==========

func (r *priceSnapshotRepo) Create(ctx context.Context, snapshot *model.PriceSnapshot) error {
	return r.db.WithContext(ctx).Create(snapshot).Error
}

func (r *priceSnapshotRepo) FindByID(ctx context.Context, id uint64) (*model.PriceSnapshot, error) {
	var snapshot model.PriceSnapshot
	err := r.db.WithContext(ctx).First(&snapshot, id).Error
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *priceSnapshotRepo) FindByExecuteLogID(ctx context.Context, execLogID uint64) ([]model.PriceSnapshot, error) {
	var snapshots []model.PriceSnapshot
	err := r.db.WithContext(ctx).Where("execute_log_id = ?", execLogID).
		Order("fetched_at DESC").Find(&snapshots).Error
	return snapshots, err
}

func (r *priceSnapshotRepo) FindByCinemaID(ctx context.Context, cinemaID uint64, limit int) ([]model.PriceSnapshot, error) {
	var snapshots []model.PriceSnapshot
	err := r.db.WithContext(ctx).
		Where("cinema_id = ?", cinemaID).
		Order("fetched_at DESC").
		Limit(limit).
		Find(&snapshots).Error
	return snapshots, err
}

// FindByCinemaIDWithTimeRange 按影院+时间范围查询快照
func (r *priceSnapshotRepo) FindByCinemaIDWithTimeRange(ctx context.Context, cinemaID uint64, startTime, endTime time.Time, limit int) ([]model.PriceSnapshot, error) {
	var snapshots []model.PriceSnapshot
	q := r.db.WithContext(ctx).Where("cinema_id = ?", cinemaID)
	if !startTime.IsZero() {
		q = q.Where("fetched_at >= ?", startTime)
	}
	if !endTime.IsZero() {
		q = q.Where("fetched_at <= ?", endTime)
	}
	err := q.Order("fetched_at DESC").Limit(limit).Find(&snapshots).Error
	return snapshots, err
}

// FindPriceTrendByCinemaAndMovie 查询影院+电影的价格趋势（折线图用）
// 从 MovieStatsJSON 中解析指定电影的按天价格数据，返回三种价格的时间序列
func (r *priceSnapshotRepo) FindPriceTrendByCinemaAndMovie(ctx context.Context, cinemaID uint64, movieID string, startTime, endTime time.Time, limit int) ([]model.PriceTrendPoint, error) {
	// 取指定时间范围内的快照
	q := r.db.WithContext(ctx).
		Where("cinema_id = ?", cinemaID).
		Order("fetched_at DESC").
		Limit(limit).
		Select("fetched_at", "movie_stats_json")
	if !startTime.IsZero() {
		q = q.Where("fetched_at >= ?", startTime)
	}
	if !endTime.IsZero() {
		q = q.Where("fetched_at <= ?", endTime)
	}
	var snapshots []model.PriceSnapshot
	err := q.Find(&snapshots).Error
	if err != nil {
		return nil, err
	}

	// 解析每条快照的 MovieStatsJSON，提取指定电影的按天价格
	type dayStat struct {
		MinPrice float64 `json:"min_price"`
		AvgPrice float64 `json:"avg_price"`
		MaxPrice float64 `json:"max_price"`
	}
	type movieStat struct {
		MovieID   string              `json:"movie_id"`
		MovieName string              `json:"movie_name"`
		DailyStats map[string]dayStat `json:"daily_stats"` // key: show_date (YYYY-MM-DD)
	}

	var points []model.PriceTrendPoint
	for _, snap := range snapshots {
		if snap.MovieStatsJSON == "" {
			continue
		}
		var stats []movieStat
		if err := json.Unmarshal([]byte(snap.MovieStatsJSON), &stats); err != nil {
			continue
		}
		for _, ms := range stats {
			if ms.MovieID != movieID {
				continue
			}
			for date, ds := range ms.DailyStats {
				points = append(points, model.PriceTrendPoint{
					Time:     date,
					PriceMin: math.Round(ds.MinPrice*10) / 10,
					PriceAvg: math.Round(ds.AvgPrice*10) / 10,
					PriceMax: math.Round(ds.MaxPrice*10) / 10,
				})
			}
			break // 找到匹配的电影即可
		}
	}

	// 按时间正序排列（折线图从左到右）
	sort.Slice(points, func(i, j int) bool {
		return points[i].Time < points[j].Time
	})

	// 如果同一日期有多条（多次采集），只保留最后一条
	if len(points) > 1 {
		deduped := []model.PriceTrendPoint{points[0]}
		for i := 1; i < len(points); i++ {
			if points[i].Time == deduped[len(deduped)-1].Time {
				deduped[len(deduped)-1] = points[i] // 用最新的覆盖
			} else {
				deduped = append(deduped, points[i])
			}
		}
		points = deduped
	}

	return points, nil
}


// ========== NotifyLog 实现 ==========

// BulkCreate 批量写入通知日志（每次 200 条分批写入）
func (r *notifyLogRepo) BulkCreate(ctx context.Context, logs []model.NotifyLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(logs, 200).Error
}

func (r *notifyLogRepo) FindBySubscriptionID(ctx context.Context, subID uint64, limit int) ([]model.NotifyLog, error) {
	var logs []model.NotifyLog
	err := r.db.WithContext(ctx).
		Where("subscription_id = ?", subID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func (r *notifyLogRepo) FindByExecuteLogID(ctx context.Context, execLogID uint64) ([]model.NotifyLog, error) {
	var logs []model.NotifyLog
	err := r.db.WithContext(ctx).Where("execute_log_id = ?", execLogID).Find(&logs).Error
	return logs, err
}

// FindByUserID 按用户分页查询通知日志
// JOIN subscription 表，通过 subscription.user_id 过滤
func (r *notifyLogRepo) FindByUserID(ctx context.Context, userID uuid.UUID, startDate, endDate *time.Time, status string, page, pageSize int) ([]model.NotifyLog, int64, error) {
	var logs []model.NotifyLog
	var total int64

	query := r.db.WithContext(ctx).
		Joins("JOIN subscription ON subscription.id = notify_log.subscription_id").
		Where("subscription.user_id = ?", userID)

	if startDate != nil {
		query = query.Where("notify_log.created_at >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("notify_log.created_at < ?", endDate.Add(24*time.Hour))
	}
	if status != "" {
		query = query.Where("notify_log.notify_status = ?", status)
	}

	if err := query.Model(&model.NotifyLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("notify_log.created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// FindRecentBySubscription 查询订阅近期的通知记录（冷却判断用）
func (r *notifyLogRepo) FindRecentBySubscription(ctx context.Context, subID uint64, since time.Duration) (*model.NotifyLog, error) {
	var log model.NotifyLog
	err := r.db.WithContext(ctx).
		Where("subscription_id = ? AND sent_at > ?", subID, time.Now().Add(-since)).
		Order("sent_at DESC").
		First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}
