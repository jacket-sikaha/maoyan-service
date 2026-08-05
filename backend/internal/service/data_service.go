package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	"maoyan-service/backend/internal/model"
	"maoyan-service/backend/internal/pkg"
	"maoyan-service/backend/internal/repository"
	"maoyan-service/backend/tools"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ==================== AuthService 认证服务 ====================
// 负责用户注册、登录、JWT Token 生成与验证。
// 密码使用 bcrypt 哈希存储，Token 有效期 7 天。

type AuthService struct {
	userRepo  repository.UserRepo
	jwtSecret []byte
}

func NewAuthService(userRepo repository.UserRepo) *AuthService {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "maoyan-service-dev-secret-change-in-production"
	}
	return &AuthService{userRepo: userRepo, jwtSecret: []byte(secret)}
}

// Register 用户注册：校验邮箱唯一性 → bcrypt 加密密码 → 写入 DB → 生成 JWT
func (s *AuthService) Register(ctx context.Context, req model.RegisterReq) (*model.AuthResponse, error) {
	existing, _ := s.userRepo.FindByEmail(ctx, req.Email)
	if existing != nil {
		return nil, fmt.Errorf("邮箱已注册")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败")
	}

	user := &model.User{
		Email:        req.Email,
		PasswordHash: string(hash),
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("创建用户失败")
	}

	token, err := s.generateToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		Token: token,
		User: struct {
			ID    uuid.UUID `json:"id"`
			Email string    `json:"email"`
		}{ID: user.ID, Email: user.Email},
	}, nil
}

// Login 用户登录：按邮箱查用户 → bcrypt 校验密码 → 生成 JWT
func (s *AuthService) Login(ctx context.Context, req model.LoginReq) (*model.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("邮箱或密码错误")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("邮箱或密码错误")
	}

	token, err := s.generateToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		Token: token,
		User: struct {
			ID    uuid.UUID `json:"id"`
			Email string    `json:"email"`
		}{ID: user.ID, Email: user.Email},
	}, nil
}

// ValidateToken 验证 JWT Token，返回用户 UUID（中间件调用）
func (s *AuthService) ValidateToken(tokenStr string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid token")
	}
	sub, _ := claims["sub"].(string)
	return uuid.Parse(sub)
}

// generateToken 生成 JWT Token（sub=user_id, exp=7天）
func (s *AuthService) generateToken(userID uuid.UUID, email string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID.String(),
		"email": email,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ==================== DataService 数据服务 ====================
// 系统核心服务层，包含：
//   - 城市/影院/电影数据查询
//   - 订阅 CRUD 管理
//   - 采集调度与执行（核心业务逻辑）
//   - 通知判断与发送
//
// 设计理念（按 PDF 设计文档）：
//   - 影院为中心：一次采集，多订阅复用
//   - 两层快照：batch（批次）+ item（明细）
//   - 通知冷却：2 小时内不重复通知同一订阅

type DataService struct {
	cinemaRepo    repository.CinemaRepo
	userRepo      repository.UserRepo
	subRepo       repository.SubscriptionRepo
	crawlTaskRepo repository.CrawlTaskRepo
	execLogRepo   repository.ExecuteLogRepo
	snapshotRepo  repository.PriceSnapshotRepo
	notifyLogRepo repository.NotifyLogRepo

	crawler  *pkg.MaoyanCrawler // 猫眼 API 爬虫
	notifier *pkg.EmailNotifier // 邮件通知器
}

func NewDataService(
	cinemaRepo repository.CinemaRepo,
	userRepo repository.UserRepo,
	subRepo repository.SubscriptionRepo,
	crawlTaskRepo repository.CrawlTaskRepo,
	execLogRepo repository.ExecuteLogRepo,
	snapshotRepo repository.PriceSnapshotRepo,
	notifyLogRepo repository.NotifyLogRepo,
	crawler *pkg.MaoyanCrawler,
	notifier *pkg.EmailNotifier,
) *DataService {
	return &DataService{
		cinemaRepo:    cinemaRepo,
		userRepo:      userRepo,
		subRepo:       subRepo,
		crawlTaskRepo: crawlTaskRepo,
		execLogRepo:   execLogRepo,
		snapshotRepo:  snapshotRepo,
		notifyLogRepo: notifyLogRepo,
		crawler:       crawler,
		notifier:      notifier,
	}
}

// ==================== 城市数据 ====================

// GetCities 获取城市列表（直接调猫眼 API，无需本地缓存）
func (s *DataService) GetCities(ctx context.Context) ([]pkg.CityItem, error) {
	return s.crawler.GetCities()
}

// GetDistricts 获取区县（直接透传猫眼 API，不做缓存）
func (s *DataService) GetDistricts(ctx context.Context, cityID int) ([]pkg.DistrictItem, error) {
	return s.crawler.GetDistricts(cityID)
}

// ==================== 电影数据 ====================

// GetHotMovies 获取热映电影（透传猫眼 API）
func (s *DataService) GetHotMovies(ctx context.Context, cityID int) ([]model.HotMovieItem, error) {
	movies, err := s.crawler.GetHotMovies(cityID)
	if err != nil {
		return nil, err
	}
	items := make([]model.HotMovieItem, 0, len(movies))
	for _, m := range movies {
		items = append(items, model.HotMovieItem{
			MovieID:        m.MovieID,
			Name:           m.Name,
			Img:            m.Img,
			Score:          m.Score,
			Version:        m.Version,
			Star:           m.Star,
			ReleaseDate:    m.ReleaseDate,
			ShowInfo:       m.ShowInfo,
			ShowState:      m.ShowState,
			Wish:           m.Wish,
			GlobalReleased: m.GlobalReleased,
			ComingTitle:    m.ComingTitle,
		})
	}
	return items, nil
}

// SearchMovies 搜索电影（透传猫眼 API）
func (s *DataService) SearchMovies(ctx context.Context, keyword string) ([]model.HotMovieItem, error) {
	movies, err := s.crawler.SearchMovies(keyword)
	if err != nil {
		return nil, err
	}
	items := make([]model.HotMovieItem, 0, len(movies))
	for _, m := range movies {
		items = append(items, model.HotMovieItem{
			MovieID:        m.MovieID,
			Name:           m.Name,
			Img:            m.Img,
			Score:          m.Score,
			Version:        m.Version,
			Star:           m.Star,
			ReleaseDate:    m.ReleaseDate,
			ShowInfo:       m.ShowInfo,
			ShowState:      m.ShowState,
			Wish:           m.Wish,
			GlobalReleased: m.GlobalReleased,
			ComingTitle:    m.ComingTitle,
		})
	}
	return items, nil
}

// ==================== 影院查询 ====================

// ==================== 排片票价查询 ====================

// GetCinemaShows 查询影院排片票价
// 流程：从猫眼获取影院列表 → 4 并发拉取每个影院的排片 → 聚合返回
func (s *DataService) GetCinemaShows(ctx context.Context,
	cityID, districtID, areaID, movieID int,
	lat, lng float64, maxCinemas int) ([]model.ShowInfo, error) {

	if maxCinemas <= 0 {
		maxCinemas = 20
	}

	cinemas, total, err := s.crawler.GetCinemaList(cityID, districtID, areaID, lat, lng)
	if err != nil {
		return nil, fmt.Errorf("get cinema list: %w", err)
	}
	slog.Info("cinema list", "count", len(cinemas), "total", total)

	if maxCinemas > 0 && len(cinemas) > maxCinemas {
		cinemas = cinemas[:maxCinemas]
	}

	// 4 worker 并发拉取各影院排片
	const concurrency = 4
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		tasks    = make(chan pkg.CinemaRaw, len(cinemas))
		allShows []model.ShowInfo
	)

	for _, cinema := range cinemas {
		tasks <- cinema
	}
	close(tasks)

	for i := 0; i < min(concurrency, len(cinemas)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cinema := range tasks {
				s.crawler.RandomDelay() // 随机延迟，避免被反爬
				shows, _, err := s.crawler.GetCinemaShows(cinema.CinemaID, movieID)
				if err != nil {
					slog.Warn("get cinema shows failed", "cinema_id", cinema.CinemaID, "error", err)
					continue
				}
				mu.Lock()
				for _, show := range shows {
					allShows = append(allShows, model.ShowInfo{
						CinemaID:      uint64(cinema.CinemaID),
						CinemaName:    cinema.Name,
						CinemaAddress: cinema.Address,
						DistanceKm:    float64(cinema.Distance) / 1000.0,
						ShowDate:      show.ShowDate,
						ShowTime:      show.ShowTime,
						EndTime:       show.EndTime,
						HallName:      show.HallName,
						Lang:          show.Lang,
						Price:         show.SellPrice,
						VIPPrice:      show.VIPPrice,
						BasePrice:     show.BasePrice,
						DiscountPrice: show.DiscountPrice,
					})
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	return allShows, nil
}

// ==================== 订阅管理 ====================

// subscriptionToFullInfo 将 Subscription 模型转换为前端展示用的 FullInfo DTO
func subscriptionToFullInfo(sub model.Subscription, cinema *model.Cinema) model.SubscriptionFullInfo {
	info := model.SubscriptionFullInfo{
		ID:                 sub.ID,
		CinemaID:           sub.CinemaID,
		MovieID:            sub.MovieID,
		MovieName:          sub.MovieName,
		Email:              sub.Email,
		TargetPrice:        sub.TargetPrice,
		InitialTargetPrice: sub.InitialTargetPrice,
		NotifyEnabled:      sub.NotifyEnabled,
		Status:             sub.Status,
		BaselineMinPrice:   sub.BaselineMinPrice,
		BaselineMaxPrice:   sub.BaselineMaxPrice,
		LastNotifyAt:       sub.LastNotifyAt,
		NotifyCount:        sub.NotifyCount,
		Remark:             sub.Remark,
		CreatedAt:          sub.CreatedAt,
		UpdatedAt:          sub.UpdatedAt,
	}
	if cinema != nil {
		info.CinemaName = cinema.Name
		info.CinemaAddress = cinema.Address
	}
	return info
}

// ensureCrawlTask 确保影院有对应的采集任务
// 设计：一个影院最多一条 crawl_task（cinema_id 唯一约束）
// 如果任务被暂停（status!=1），重新激活
func (s *DataService) ensureCrawlTask(ctx context.Context, cinemaID uint64) {
	existing, err := s.crawlTaskRepo.FindByCinemaID(ctx, cinemaID)
	if err == nil && existing != nil {
		// 已有任务，若被暂停则重新激活
		if existing.Status != 1 {
			existing.Status = 1
			s.crawlTaskRepo.Update(ctx, existing)
		}
		return
	}
	// 创建新任务：默认 30 分钟间隔，优先级 100
	task := &model.CrawlTask{
		CinemaID:        cinemaID,
		IntervalMinutes: 30,
		NextRunAt:       time.Now(),
		Status:          1,
		Priority:        100,
		TimeoutSeconds:  60,
	}
	if err := s.crawlTaskRepo.Create(ctx, task); err != nil {
		slog.Warn("create crawl task failed", "cinema_id", cinemaID, "error", err)
	}
}

// CreateSubscription 创建订阅
// 流程：用猫眼影院 ID 查/建 cinema 记录 → 去重检查（cinema_id + email 唯一）→ 写入 subscription → 确保有采集任务
func (s *DataService) CreateSubscription(ctx context.Context, userID uuid.UUID, req model.SubscriptionReq) (*model.SubscribeResponse, error) {
	// Step 1: 用猫眼影院 ID 查 cinema 记录，查不到则创建

	// Step 2: 去重 — 同一影院+同一电影+同一邮箱只能订阅一次
	if req.MovieID != "" {
		existing, _ := s.subRepo.FindByCinemaAndMovieAndEmail(ctx, req.CinemaID, req.MovieID, req.Email)
		if existing != nil {
			return nil, fmt.Errorf("该影院+该电影已用此邮箱订阅")
		}
	} else {
		existing, _ := s.subRepo.FindByCinemaAndEmail(ctx, req.CinemaID, req.Email)
		if existing != nil {
			return nil, fmt.Errorf("该影院已用此邮箱订阅")
		}
	}

	// Step 3: 写入订阅（cinema_id 存猫眼影院 ID，与采集任务一致）
	sub := &model.Subscription{
		CinemaID:           req.CinemaID,
		MovieID:            req.MovieID,
		MovieName:          req.MovieName,
		Email:              req.Email,
		TargetPrice:        req.TargetPrice,
		InitialTargetPrice: req.TargetPrice,
		NotifyEnabled:      true,
		Status:             1,
		UserID:             &userID,
		Remark:             req.Remark,
		CinemaName:         req.CinemaName,
	}
	if err := s.subRepo.Create(ctx, sub); err != nil {
		return nil, fmt.Errorf("创建订阅失败: %w", err)
	}

	// Step 4: 确保该影院有采集任务（没有则自动创建）
	s.ensureCrawlTask(ctx, req.CinemaID)

	return &model.SubscribeResponse{
		ID:            sub.ID,
		CinemaID:      sub.CinemaID,
		Email:         sub.Email,
		TargetPrice:   sub.TargetPrice,
		Status:        sub.Status,
		NotifyEnabled: sub.NotifyEnabled,
		Message:       fmt.Sprintf("订阅成功：影院「%s」目标价 ¥%.2f", sub.CinemaName, req.TargetPrice),
	}, nil
}

// ToggleSubscription 切换订阅状态（启用/停用）
func (s *DataService) ToggleSubscription(ctx context.Context, userID uuid.UUID, subIDStr string, status int8) error {
	tools.PrettyPrint(userID)
	tools.PrettyPrint(subIDStr)
	subID, err := parseUint64(subIDStr)
	if err != nil {
		return fmt.Errorf("subscription_id 格式错误")
	}
	sub, err := s.subRepo.FindByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("订阅不存在")
	}
	// 权限校验：仅订阅所有者可操作
	if sub.UserID == nil || *sub.UserID != userID {
		return fmt.Errorf("无权操作该订阅")
	}
	return s.subRepo.UpdateFields(ctx, subID, map[string]interface{}{"status": status})
}

// ListSubscriptions 查询用户所有订阅
// 返回 SubscriptionFullInfo（含影院名称等关联字段）
func (s *DataService) ListSubscriptions(ctx context.Context, userID uuid.UUID) ([]model.Subscription, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	subs, err := s.subRepo.FindByEmail(ctx, user.Email)
	if err != nil {
		return nil, err
	}
	return subs, nil
}

// GetSubscriptionDetail 获取订阅详情
// 包含：订阅基本信息 + 当前行情（最新快照）+ 价格趋势折线图 + 历史记录数
func (s *DataService) GetSubscriptionDetail(ctx context.Context, userID uuid.UUID, subIDStr string, page, pageSize int) (*model.SubscriptionDetail, error) {
	subID, err := parseUint64(subIDStr)
	tools.PrettyPrint(subID)
	if err != nil {
		return nil, fmt.Errorf("subscription_id 格式错误")
	}
	sub, err := s.subRepo.FindByID(ctx, subID)
	if err != nil {
		return nil, fmt.Errorf("订阅不存在")
	}
	if sub.UserID == nil || *sub.UserID != userID {
		return nil, fmt.Errorf("无权访问该订阅")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	currentShows := s.fetchCurrentShowsForCinema(ctx, sub.CinemaID, sub.CinemaName, sub.TargetPrice)
	// 订阅详情价格趋势：电影级订阅用指定电影，影院级订阅从 RawJSON 取第一部电影
	trendMovieID := sub.MovieID
	if trendMovieID == "" {
		// 从最新快照的 RawJSON 中解析出第一部电影 ID
		snapshots, _ := s.snapshotRepo.FindByCinemaID(ctx, sub.CinemaID, 1)
		if len(snapshots) > 0 && snapshots[0].RawJSON != "" {
			var items []showItem
			if json.Unmarshal([]byte(snapshots[0].RawJSON), &items) == nil && len(items) > 0 {
				trendMovieID = items[0].MovieID
			}
		}
	}
	var trend []model.PriceTrendPoint
	if trendMovieID != "" {
		trend, _ = s.snapshotRepo.FindPriceTrendByCinemaAndMovie(ctx, sub.CinemaID, trendMovieID, time.Time{}, time.Time{}, 50)
	}

	var historyTotal int64
	snapshots, _ := s.snapshotRepo.FindByCinemaID(ctx, sub.CinemaID, 1)
	historyTotal = int64(len(snapshots))

	detail := &model.SubscriptionDetail{
		Subscription: *sub,
		CurrentShows: currentShows,
		PriceTrend:   trend,
		HistoryTotal: historyTotal,
	}

	return detail, nil
}

// GetCrawlRecords 获取影院采集记录（仪表盘风格）
func (s *DataService) GetCrawlRecords(ctx context.Context, userID uuid.UUID, subIDStr string, startTime, endTime time.Time) (*model.CrawlRecordsDashboard, error) {
	subID, err := parseUint64(subIDStr)
	if err != nil {
		return nil, fmt.Errorf("subscription_id 格式错误")
	}
	sub, err := s.subRepo.FindByID(ctx, subID)
	if err != nil {
		return nil, fmt.Errorf("订阅不存在")
	}
	if sub.UserID == nil || *sub.UserID != userID {
		return nil, fmt.Errorf("无权访问该订阅")
	}

	// 查询时间范围内的快照
	snapshots, err := s.snapshotRepo.FindByCinemaIDWithTimeRange(ctx, sub.CinemaID, startTime, endTime, 500)
	if err != nil {
		return nil, fmt.Errorf("查询采集记录失败: %v", err)
	}

	dash := &model.CrawlRecordsDashboard{
		CinemaName: sub.CinemaName,
		CinemaID:   sub.CinemaID,
	}
	if len(snapshots) == 0 {
		return dash, nil
	}

	// 解析所有快照，构建仪表盘数据
	type dayStat struct {
		MinPrice float64 `json:"min_price"`
		AvgPrice float64 `json:"avg_price"`
		MaxPrice float64 `json:"max_price"`
		Count    int     `json:"-"`
		Sum      float64 `json:"-"`
	}
	type movieStat struct {
		MovieID     string             `json:"movie_id"`
		MovieName   string             `json:"movie_name"`
		DailyStats  map[string]dayStat `json:"daily_stats"`
		DateOrder   []string           `json:"-"`
	}

	movieAgg := make(map[string]*model.MovieCrawlDetail) // key: movieID
	totalShowtimes := 0
	var allPrices []float64

	for _, snap := range snapshots {
		record := model.CrawlRecordItem{
			SnapshotID:     snap.ID,
			FetchedAt:      snap.FetchedAt,
			TotalMovies:    snap.TotalMovies,
			TotalShowtimes: snap.TotalShowtimes,
			ParseStatus:    snap.ParseStatus,
		}
		dash.Records = append(dash.Records, record)
		totalShowtimes += snap.TotalShowtimes

		// 解析 MovieStatsJSON 汇总各电影价格
		if snap.MovieStatsJSON == "" {
			continue
		}
		var stats []movieStat
		if json.Unmarshal([]byte(snap.MovieStatsJSON), &stats) != nil {
			continue
		}
		for _, ms := range stats {
			for _, ds := range ms.DailyStats {
				if ds.MinPrice > 0 {
					allPrices = append(allPrices, ds.MinPrice)
				}
			}
			// 汇总到电影维度
			agg, ok := movieAgg[ms.MovieID]
			if !ok {
				agg = &model.MovieCrawlDetail{
					MovieID:   ms.MovieID,
					MovieName: ms.MovieName,
				}
				movieAgg[ms.MovieID] = agg
			}
			for _, ds := range ms.DailyStats {
				if ds.MinPrice > 0 {
					if agg.MinPrice == 0 || ds.MinPrice < agg.MinPrice {
						agg.MinPrice = ds.MinPrice
					}
				}
				if ds.MaxPrice > agg.MaxPrice {
					agg.MaxPrice = ds.MaxPrice
				}
				agg.Sum += ds.Sum
				agg.Count += ds.Count
				agg.Showtimes += ds.Count
			}
		}
	}

	dash.TotalSnapshots = len(snapshots)
	dash.TotalShowtimes = totalShowtimes
	dash.TotalMovies = len(movieAgg)

	// 计算全局价格统计
	if len(allPrices) > 0 {
		min := allPrices[0]
		max := allPrices[0]
		sum := 0.0
		for _, p := range allPrices {
			if p < min {
				min = p
			}
			if p > max {
				max = p
			}
			sum += p
		}
		dash.GlobalMinPrice = mathRound1(min)
		dash.GlobalAvgPrice = mathRound1(sum / float64(len(allPrices)))
		dash.GlobalMaxPrice = mathRound1(max)
	}

	// 电影列表（按最低价排序）
	for _, agg := range movieAgg {
		if agg.Count > 0 {
			agg.AvgPrice = mathRound1(agg.Sum / float64(agg.Count))
		}
		agg.MinPrice = mathRound1(agg.MinPrice)
		agg.MaxPrice = mathRound1(agg.MaxPrice)
		dash.Movies = append(dash.Movies, *agg)
	}
	sort.Slice(dash.Movies, func(i, j int) bool {
		return dash.Movies[i].MinPrice < dash.Movies[j].MinPrice
	})

	return dash, nil
}

// GetSnapshotMovieShows 获取某次采集快照中某部电影的所有场次明细
func (s *DataService) GetSnapshotMovieShows(ctx context.Context, userID uuid.UUID, subIDStr string, snapshotID uint64, movieID string) ([]model.ShowPriceForSubscription, error) {
	subID, err := parseUint64(subIDStr)
	if err != nil {
		return nil, fmt.Errorf("subscription_id 格式错误")
	}
	sub, err := s.subRepo.FindByID(ctx, subID)
	if err != nil {
		return nil, fmt.Errorf("订阅不存在")
	}
	if sub.UserID == nil || *sub.UserID != userID {
		return nil, fmt.Errorf("无权访问该订阅")
	}

	snap, err := s.snapshotRepo.FindByID(ctx, snapshotID)
	if err != nil || snap.CinemaID != sub.CinemaID {
		return nil, fmt.Errorf("快照不存在")
	}

	var items []showItem
	if err := json.Unmarshal([]byte(snap.RawJSON), &items); err != nil {
		return nil, fmt.Errorf("解析快照数据失败")
	}

	result := make([]model.ShowPriceForSubscription, 0)
	for _, item := range items {
		if item.MovieID != movieID {
			continue
		}
		result = append(result, model.ShowPriceForSubscription{
			MovieName:    item.MovieName,
			CinemaName:   sub.CinemaName,
			HallName:     item.HallName,
			ShowDate:     item.ShowDate,
			ShowTime:     item.ShowTime,
			CurrentPrice: item.Price,
		})
	}
	return result, nil
}

// mathRound1 四舍五入到1位小数
func mathRound1(v float64) float64 {
	return math.Round(v*10) / 10
}
// fetchCurrentShowsForCinema 获取影院当前行情（从最新快照中读取）
func (s *DataService) fetchCurrentShowsForCinema(ctx context.Context, cinemaID uint64, cinemaName string, targetPrice float64) []model.ShowPriceForSubscription {
	snapshots, err := s.snapshotRepo.FindByCinemaID(ctx, cinemaID, 1)
	if err != nil || len(snapshots) == 0 {
		return nil
	}

	// 从最新快照的 RawJSON 中解析出场次明细
	latest := snapshots[0]
	var showItems []struct {
		MovieName string  `json:"movie_name"`
		HallName  string  `json:"hall_name"`
		ShowDate  string  `json:"show_date"`
		ShowTime  string  `json:"show_time"`
		Price     float64 `json:"price"`
	}
	if err := json.Unmarshal([]byte(latest.RawJSON), &showItems); err != nil {
		slog.Warn("unmarshal raw_json failed", "cinema_id", cinemaID, "error", err)
		return nil
	}

	result := make([]model.ShowPriceForSubscription, 0, len(showItems))
	for _, item := range showItems {
		result = append(result, model.ShowPriceForSubscription{
			MovieName:    item.MovieName,
			CinemaName:   cinemaName,
			HallName:     item.HallName,
			ShowDate:     item.ShowDate,
			ShowTime:     item.ShowTime,
			CurrentPrice: item.Price,
		})
	}
	return result
}

// GetSubscriptionPrices 主动刷新订阅影院的当前行情
func (s *DataService) GetSubscriptionPrices(ctx context.Context, userID uuid.UUID, subIDStr string) ([]model.ShowPriceForSubscription, error) {
	subID, err := parseUint64(subIDStr)
	if err != nil {
		return nil, fmt.Errorf("subscription_id 格式错误")
	}
	sub, err := s.subRepo.FindByID(ctx, subID)
	if err != nil {
		return nil, fmt.Errorf("订阅不存在")
	}
	if sub.UserID == nil || *sub.UserID != userID {
		return nil, fmt.Errorf("无权访问")
	}
	if sub.Status != 1 {
		return nil, fmt.Errorf("订阅已停用")
	}

	return s.fetchCurrentShowsForCinema(ctx, sub.CinemaID, sub.CinemaName, sub.TargetPrice), nil
}

// UpdateSubscription 更新订阅字段（仅所有者可修改，支持部分更新）
func (s *DataService) UpdateSubscription(ctx context.Context, userID uuid.UUID, subIDStr string, req model.SubscriptionUpdateReq) error {
	subID, err := parseUint64(subIDStr)
	if err != nil {
		return fmt.Errorf("subscription_id 格式错误")
	}
	sub, err := s.subRepo.FindByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("订阅不存在")
	}
	if sub.UserID == nil || *sub.UserID != userID {
		return fmt.Errorf("无权修改该订阅")
	}

	updates := map[string]interface{}{}
	if req.TargetPrice != nil {
		if *req.TargetPrice > sub.InitialTargetPrice {
			return fmt.Errorf("目标票价不能高于初始值 ¥%.2f", sub.InitialTargetPrice)
		}
		if *req.TargetPrice <= 0 {
			return fmt.Errorf("目标票价必须大于 0")
		}
		updates["target_price"] = *req.TargetPrice
	}
	if req.Remark != nil {
		updates["remark"] = *req.Remark
	}
	if req.Status != nil {
		if *req.Status != 0 && *req.Status != 1 {
			return fmt.Errorf("状态值无效")
		}
		updates["status"] = *req.Status
	}

	if len(updates) == 0 {
		return nil
	}
	return s.subRepo.UpdateFields(ctx, subID, updates)
}

// DeleteSubscription 删除订阅（仅所有者可删除）
func (s *DataService) DeleteSubscription(ctx context.Context, userID uuid.UUID, subIDStr string) error {
	subID, err := parseUint64(subIDStr)
	if err != nil {
		return fmt.Errorf("subscription_id 格式错误")
	}
	sub, err := s.subRepo.FindByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("订阅不存在")
	}
	if sub.UserID == nil || *sub.UserID != userID {
		return fmt.Errorf("无权删除该订阅")
	}
	return s.subRepo.Delete(ctx, subID)
}

// GetSubscriptionLogs 分页查询用户通知日志
// GetSubscriptionLogs 分页查询用户通知日志
func (s *DataService) GetSubscriptionLogs(ctx context.Context, userID uuid.UUID, startDate, endDate *time.Time, status string, page, pageSize int) ([]model.SubscriptionLogFullInfo, int64, error) {
	logs, total, err := s.notifyLogRepo.FindByUserID(ctx, userID, startDate, endDate, status, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]model.SubscriptionLogFullInfo, 0, len(logs))
	for _, log := range logs {
		// 从订阅表获取影院名
		sub, _ := s.subRepo.FindByID(ctx, log.SubscriptionID)
		cinemaName := ""
		if sub != nil {
			cinemaName = sub.CinemaName
		}
		result = append(result, model.SubscriptionLogFullInfo{
			ID:             log.ID,
			SubscriptionID: log.SubscriptionID,
			CinemaName:     cinemaName,
			Email:          log.Email,
			NotifyType:     log.NotifyType,
			NotifyStatus:   log.NotifyStatus,
			TargetPrice:    log.TargetPrice,
			MatchedPrice:   log.MatchedPrice,
			ErrorMsg:       log.ErrorMsg,
			SentAt:         log.SentAt,
			CreatedAt:      log.CreatedAt,
		})
	}
	return result, total, nil
}

// GetUserSubscribedCinemas 获取用户已订阅的影院列表（去重）
func (s *DataService) GetUserSubscribedCinemas(ctx context.Context, userID uuid.UUID) ([]model.CinemaItem, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	subs, err := s.subRepo.FindByEmail(ctx, user.Email)
	if err != nil {
		return nil, err
	}
	seen := make(map[uint64]bool)
	var cinemas []model.CinemaItem
	for _, sub := range subs {
		if seen[sub.CinemaID] {
			continue
		}
		seen[sub.CinemaID] = true
		item := model.CinemaItem{
			CinemaID: sub.CinemaID,
			Name:     sub.CinemaName,
		}
		cinemas = append(cinemas, item)
	}
	return cinemas, nil
}

// GetPriceChanges 获取影院+电影的价格变化趋势（折线图数据）
// startTime/endTime 为时间范围筛选，零值表示不限制
func (s *DataService) GetPriceChanges(ctx context.Context, cinemaID int, movieID string, startTime, endTime time.Time, userID uuid.UUID) (map[string][]model.PriceTrendPoint, error) {
	trend, err := s.snapshotRepo.FindPriceTrendByCinemaAndMovie(ctx, uint64(cinemaID), movieID, startTime, endTime, 200)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]model.PriceTrendPoint)
	result["trend"] = trend
	return result, nil
}

// GetSubscribedCinemaMovies 获取用户订阅的影院+电影组合列表（去重，票价变化页筛选用）
func (s *DataService) GetSubscribedCinemaMovies(ctx context.Context, userID uuid.UUID) ([]model.CinemaMovieItem, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	subs, err := s.subRepo.FindByEmail(ctx, user.Email)
	if err != nil {
		return nil, err
	}
	type key struct {
		cinemaID uint64
		movieID  string
	}
	seen := make(map[key]bool)
	var result []model.CinemaMovieItem
	for _, sub := range subs {
		k := key{cinemaID: sub.CinemaID, movieID: sub.MovieID}
		if seen[k] {
			continue
		}
		seen[k] = true

		item := model.CinemaMovieItem{
			CinemaID:   sub.CinemaID,
			CinemaName: sub.CinemaName,
			MovieID:    sub.MovieID,
			MovieName:  sub.MovieName,
		}
		// 影院级订阅（MovieID 为空）显示"全部电影"
		if item.MovieID == "" {
			item.MovieName = "全部电影"
		}
		result = append(result, item)
	}
	return result, nil
}

// showItem 快照中的单场次信息（用于 RawJSON 序列化和通知判断）
type showItem struct {
	MovieID   string  `json:"movie_id"`
	MovieName string  `json:"movie_name"`
	HallName  string  `json:"hall_name"`
	ShowDate  string  `json:"show_date"`
	ShowTime  string  `json:"show_time"`
	Language  string  `json:"language"`
	Price     float64 `json:"price"`
}

// ==================== 采集调度（核心业务逻辑） ====================

// FetchAllSubscriptionData 采集调度入口（由 scheduler 定时调用）
// 流程：查询所有到期任务 → 逐个执行 processCrawlTask
func (s *DataService) FetchAllSubscriptionData(ctx context.Context) error {
	tasks, err := s.crawlTaskRepo.ListDue(ctx, time.Now())
	if err != nil {
		return fmt.Errorf("query crawl_tasks: %w", err)
	}
	slog.Info("v2 crawl schedule", "due_tasks", len(tasks))

	for _, task := range tasks {
		s.processCrawlTask(ctx, &task)
	}
	return nil
}

// processCrawlTask 处理单个影院采集任务
// 完整流程（6 步）：
//  1. 创建执行日志（execute_log）
//  2. 调用猫眼 API 拉取该影院全部排片
//  3. 批量写入票价快照（batch + items 两层）
//  4. 查找该影院下所有启用订阅，逐个判断是否触发通知
//  5. 批量写入通知日志（notify_log）
//  6. 更新执行日志和采集任务状态（next_run_at、统计计数）
//
// ManualCrawlByCinemaID 手动触发指定影院的采集任务（调试用）
// 如果该影院没有采集任务记录，返回错误
func (s *DataService) ManualCrawlByCinemaID(ctx context.Context, cinemaID uint64) error {
	task, err := s.crawlTaskRepo.FindByCinemaID(ctx, cinemaID)
	if err != nil {
		return fmt.Errorf("影院 %d 无采集任务记录: %w", cinemaID, err)
	}
	slog.Info("manual crawl triggered", "cinema_id", cinemaID, "task_id", task.ID)
	s.processCrawlTask(ctx, task)
	return nil
}

func (s *DataService) processCrawlTask(ctx context.Context, task *model.CrawlTask) {
	startedAt := time.Now()

	// Step 1: 创建执行日志
	execLog := &model.ExecuteLog{
		CrawlTaskID: task.ID,
		CinemaID:    task.CinemaID,
		StartedAt:   startedAt,
		Status:      "running",
	}
	if err := s.execLogRepo.Create(ctx, execLog); err != nil {
		slog.Error("create execLog failed", "task_id", task.ID, "error", err)
		return
	}

	s.crawler.RandomDelay() // 随机延迟，避免被反爬

	// Step 2: 拉取该影院全部排片
	allShows, err := s.crawler.GetCinemaAllShows(int(task.CinemaID))
	endedAt := time.Now()

	if err != nil {
		// 采集失败：更新日志 + 统计失败次数
		execLog.Status = "fail"
		execLog.ErrorMsg = err.Error()
		execLog.EndedAt = &endedAt
		s.execLogRepo.Update(ctx, execLog)
		s.crawlTaskRepo.UpdateStats(ctx, task.ID, false)
		slog.Error("crawl cinema failed", "cinema_id", task.CinemaID, "error", err)
		return
	}

	// Step 3: 构建票价快照（合并为单表 PriceSnapshot）
	// 遍历所有电影的所有场次，构建 RawJSON + 按电影×日期统计价格
	totalFetched := 0
	movieSet := make(map[string]bool)
	var allItems []showItem

	// 按电影×日期分组统计价格
	type dayStat struct {
		MinPrice float64 `json:"min_price"`
		AvgPrice float64 `json:"avg_price"`
		MaxPrice float64 `json:"max_price"`
		Count    int     `json:"-"`
		Sum      float64 `json:"-"`
	}
	type movieStat struct {
		MovieID    string             `json:"movie_id"`
		MovieName  string             `json:"movie_name"`
		DailyStats map[string]dayStat `json:"daily_stats"`
		DateOrder  []string           `json:"-"` // 保持日期顺序
	}
	movieStatsMap := make(map[string]*movieStat) // key: movieID string

	for movieID, shows := range allShows {
		movieIDStr := fmt.Sprintf("%d", movieID)
		movieSet[movieIDStr] = true
		ms := &movieStat{
			MovieID:    movieIDStr,
			MovieName:  "",
			DailyStats: make(map[string]dayStat),
		}
		movieStatsMap[movieIDStr] = ms // ← 关键：写入 map

		for _, show := range shows {
			allItems = append(allItems, showItem{
				MovieID:   movieIDStr,
				MovieName: show.MovieName,
				HallName:  show.HallName,
				ShowDate:  show.ShowDate,
				ShowTime:  show.ShowTime,
				Language:  show.Lang,
				Price:     show.SellPrice,
			})

			if ms.MovieName == "" {
				ms.MovieName = show.MovieName
			}

			date := show.ShowDate
			if date == "" {
				continue
			}
			ds := ms.DailyStats[date]
			if ds.Count == 0 {
				ds.MinPrice = show.SellPrice
				ds.MaxPrice = show.SellPrice
				ms.DateOrder = append(ms.DateOrder, date)
			} else {
				if show.SellPrice < ds.MinPrice {
					ds.MinPrice = show.SellPrice
				}
				if show.SellPrice > ds.MaxPrice {
					ds.MaxPrice = show.SellPrice
				}
			}
			ds.Sum += show.SellPrice
			ds.Count++
			ms.DailyStats[date] = ds

			totalFetched++
		}
	}
	tools.PrettyPrint(movieStatsMap)
	// 计算均价并构建最终 movieStats 列表
	var movieStatsList []movieStat
	for _, ms := range movieStatsMap {
		for date, ds := range ms.DailyStats {
			if ds.Count > 0 {
				ds.AvgPrice = ds.Sum / float64(ds.Count)
				ms.DailyStats[date] = ds
			}
		}
		movieStatsList = append(movieStatsList, *ms)
		_ = ms // avoid unused warning
	}

	rawJSONBytes, _ := json.Marshal(allItems)
	movieStatsBytes, _ := json.Marshal(movieStatsList)

	snapshot := &model.PriceSnapshot{
		CrawlTaskID:    task.ID,
		CinemaID:       task.CinemaID,
		ExecuteLogID:   &execLog.ID,
		FetchedAt:      endedAt,
		Source:         "maoyan",
		TotalMovies:    len(movieSet),
		TotalShowtimes: totalFetched,
		RawJSON:        string(rawJSONBytes),
		MovieStatsJSON: string(movieStatsBytes),
		ParseStatus:    "success",
	}
	if err := s.snapshotRepo.Create(ctx, snapshot); err != nil {
		slog.Error("create snapshot failed", "error", err)
		execLog.Status = "fail"
		execLog.ErrorMsg = err.Error()
		execLog.EndedAt = &endedAt
		s.execLogRepo.Update(ctx, execLog)
		return
	}

	// Step 4: 查找该影院下所有启用订阅，逐个判断通知
	subs, err := s.subRepo.FindByCinemaID(ctx, task.CinemaID)
	if err != nil {
		slog.Warn("find subs by cinema failed", "cinema_id", task.CinemaID, "error", err)
	}

	matchedCount := 0
	notifiedCount := 0
	var notifyLogs []model.NotifyLog

	for _, sub := range subs {
		if sub.Status != 1 || !sub.NotifyEnabled {
			continue
		}
		matchedCount++

		// 通知判断：检查影院最低价是否 ≤ 订阅目标价
		nl := s.checkAndNotifyV2(ctx, &sub, allItems, execLog.ID, snapshot.ID)
		if len(nl) > 0 {
			notifyLogs = append(notifyLogs, nl...)
			for _, n := range nl {
				if n.NotifyStatus == "success" {
					notifiedCount++
				}
			}
		}
	}

	// Step 5: 批量写入通知日志
	if len(notifyLogs) > 0 {
		s.notifyLogRepo.BulkCreate(ctx, notifyLogs)
	}

	// Step 6: 更新执行日志和采集任务状态
	durationMs := int(endedAt.Sub(startedAt).Milliseconds())
	execLog.Status = "success"
	execLog.FetchedCount = totalFetched
	execLog.MatchedCount = matchedCount
	execLog.NotifiedCount = notifiedCount
	execLog.EndedAt = &endedAt
	execLog.DurationMs = &durationMs
	s.execLogRepo.Update(ctx, execLog)

	// 更新采集任务：下次执行时间 + 统计计数
	nextRun := time.Now().Add(time.Duration(task.IntervalMinutes) * time.Minute)
	s.crawlTaskRepo.UpdateNextRun(ctx, task.ID, nextRun)
	s.crawlTaskRepo.UpdateStats(ctx, task.ID, true)

	slog.Info("crawl task done",
		"cinema_id", task.CinemaID,
		"fetched", totalFetched,
		"matched_subs", matchedCount,
		"notified", notifiedCount,
	)
}

// checkAndNotifyV2 通知判断核心逻辑
// 判断流程：
//  1. 从快照明细中找最低价场次
//  2. 首次运行：记录基准价（baseline_min_price），不发通知
//  3. 最低价 ≤ target_price → 触发通知
//  4. 冷却检查：上次通知在 2 小时内则跳过
//  5. 发送邮件 → 记录通知日志 → 更新订阅通知状态
func (s *DataService) checkAndNotifyV2(ctx context.Context, sub *model.Subscription, items []showItem, execLogID uint64, snapshotID uint64) []model.NotifyLog {
	var logs []model.NotifyLog

	if len(items) == 0 {
		return nil
	}

	// 找最低价场次
	var minItem *showItem
	for i := range items {
		if minItem == nil || items[i].Price < minItem.Price {
			minItem = &items[i]
		}
	}
	if minItem == nil {
		return nil
	}

	// 如果是电影级订阅，只匹配指定电影
	if sub.MovieID != "" && minItem.MovieID != sub.MovieID {
		// 找该电影的最低价
		var movieMinItem *showItem
		for i := range items {
			if items[i].MovieID == sub.MovieID {
				if movieMinItem == nil || items[i].Price < movieMinItem.Price {
					movieMinItem = &items[i]
				}
			}
		}
		if movieMinItem == nil {
			return nil
		}
		minItem = movieMinItem
	}

	currentLowest := minItem.Price

	// 判断是否触发：最低价 ≤ 目标价
	shouldNotify := false
	if sub.TargetPrice > 0 && currentLowest <= sub.TargetPrice {
		shouldNotify = true
	}

	// 首次运行：记录基准价，不发通知（建立价格基线）
	if sub.BaselineMinPrice == nil {
		s.subRepo.UpdateFields(ctx, sub.ID, map[string]interface{}{
			"baseline_min_price": currentLowest,
		})
		return nil
	}

	if !shouldNotify {
		return nil
	}

	// 冷却检查：2 小时内已通知过则跳过（避免频繁打扰）
	if sub.LastNotifyAt != nil && time.Since(*sub.LastNotifyAt) < 2*time.Hour {
		return nil
	}

	// 构建匹配项 JSON（记录命中场次的详细信息）
	matchedItems := []map[string]interface{}{
		{
			"movie_name":  minItem.MovieName,
			"hall_name":   minItem.HallName,
			"show_date":   minItem.ShowDate,
			"show_time":   minItem.ShowTime,
			"price":       minItem.Price,
			"cinema_name": sub.CinemaName,
		},
	}
	matchedJSON, _ := json.Marshal(matchedItems)

	now := time.Now()

	// 发送邮件通知
	err := s.notifier.SendPriceAlert(
		sub.Email,
		minItem.MovieName,
		sub.CinemaName,
		minItem.HallName,
		minItem.ShowDate+" "+minItem.ShowTime,
		currentLowest,
		0,
		sub.TargetPrice,
	)

	// 构建通知日志
	nl := model.NotifyLog{
		SubscriptionID:   sub.ID,
		ExecuteLogID:     &execLogID,
		CinemaID:         sub.CinemaID,
		Email:            sub.Email,
		NotifyType:       "price_alert",
		TargetPrice:      sub.TargetPrice,
		MatchedPrice:     currentLowest,
		MatchedItemsJSON: string(matchedJSON),
		SentAt:           &now,
	}

	if err != nil {
		nl.NotifyStatus = "fail"
		nl.ErrorMsg = err.Error()
		slog.Error("notify failed", "sub_id", sub.ID, "email", sub.Email, "error", err)
	} else {
		nl.NotifyStatus = "success"
	}

	logs = append(logs, nl)

	// 更新订阅通知状态：最后通知时间 + 通知次数
	s.subRepo.UpdateFields(ctx, sub.ID, map[string]interface{}{
		"last_notify_at": now,
	})
	s.subRepo.UpdateNotifyCount(ctx, sub.ID)

	return logs
}

// ==================== CSV 导出 ====================

// ExportSubscriptionHistory 导出订阅历史数据为 CSV
// 返回该订阅影院的所有快照记录，由 controller 层写入 CSV
func (s *DataService) ExportSubscriptionHistory(ctx context.Context, userID uuid.UUID, subIDStr string) ([]model.PriceSnapshot, error) {
	subID, err := parseUint64(subIDStr)
	if err != nil {
		return nil, fmt.Errorf("subscription_id 格式错误")
	}
	sub, err := s.subRepo.FindByID(ctx, subID)
	if err != nil {
		return nil, fmt.Errorf("订阅不存在")
	}
	if sub.UserID == nil || *sub.UserID != userID {
		return nil, fmt.Errorf("无权访问")
	}

	snapshots, err := s.snapshotRepo.FindByCinemaID(ctx, sub.CinemaID, 1000)
	if err != nil {
		return nil, err
	}
	return snapshots, nil
}

// ==================== 工具函数 ====================

// parseUint64 将字符串 ID 解析为 uint64（路径参数转换用）
func parseUint64(s string) (uint64, error) {
	var n uint64
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
