package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"maoyan-service/backend/internal/controller"
	"maoyan-service/backend/internal/middleware"
	"maoyan-service/backend/internal/pkg"
	"maoyan-service/backend/internal/repository"
	"maoyan-service/backend/internal/scheduler"
	"maoyan-service/backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// main.go — 猫眼票价监控服务入口
func main() {
	// ========== 1. 基础设施初始化 ==========
	initLogger()

	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		slog.Warn("no .env file found, using env vars", "error", err)
	}

	// ========== 2. 数据库连接 ==========
	db, err := initDB()
	if err != nil {
		slog.Error("database init failed", "error", err)
		os.Exit(1)
	}
	slog.Info("database connected")

	// // 先清理旧表（必须在 AutoMigrate 之前，否则旧列残留）
	// dropOldTables(db)

	// // AutoMigrate：按设计文档的表体系
	// if err := db.AutoMigrate(
	// 	&model.Cinema{},
	// 	&model.User{},
	// 	&model.Subscription{},
	// 	&model.CrawlTask{},
	// 	&model.ExecuteLog{},
	// 	&model.PriceSnapshot{},
	// 	&model.NotifyLog{},
	// ); err != nil {
	// 	slog.Error("auto migrate failed", "error", err)
	// 	os.Exit(1)
	// }
	// slog.Info("auto migrate completed")

	// // AutoMigrate 会根据 model 中的 gorm:"foreignKey" 标签自动创建物理外键约束
	// // 但我们的设计原则是「不加物理外键」，所以迁移后立即删除所有自动创建的 FK 约束
	// dropAutoForeignKeys(db)

	// // 运行手动 migration（建索引）
	// runManualMigrations(db)

	// ========== 3. 依赖注入：仓库层 ==========
	cinemaRepo := repository.NewCinemaRepo(db)
	userRepo := repository.NewUserRepo(db)
	subRepo := repository.NewSubscriptionRepo(db)
	crawlTaskRepo := repository.NewCrawlTaskRepo(db)
	execLogRepo := repository.NewExecuteLogRepo(db)
	snapshotRepo := repository.NewPriceSnapshotRepo(db)
	notifyLogRepo := repository.NewNotifyLogRepo(db)

	// ========== 4. 依赖注入：基础组件 ==========
	delayMin := viper.GetFloat64("MAOYAN_REQUEST_DELAY_MIN")
	delayMax := viper.GetFloat64("MAOYAN_REQUEST_DELAY_MAX")
	if delayMin <= 0 {
		delayMin = 1.0
	}
	if delayMax <= 0 {
		delayMax = 2.0
	}
	crawler := pkg.NewMaoyanCrawler(delayMin, delayMax)

	notifier := pkg.NewEmailNotifier(pkg.EmailConfig{
		Host: viper.GetString("SMTP_HOST"),
		Port: viper.GetInt("SMTP_PORT"),
		User: viper.GetString("SMTP_USER"),
		Pass: viper.GetString("SMTP_PASS"),
	})

	// ========== 5. 依赖注入：服务层 ==========
	authSvc := service.NewAuthService(userRepo)
	dataSvc := service.NewDataService(
		cinemaRepo, userRepo,
		subRepo, crawlTaskRepo, execLogRepo,
		snapshotRepo, notifyLogRepo,
		crawler, notifier,
	)

	// ========== 6. 定时调度器 ==========
	intervalMin := viper.GetInt("MAOYAN_FETCH_INTERVAL_MIN")
	if intervalMin <= 0 {
		intervalMin = 30
	}
	sched := scheduler.New(dataSvc)
	if err := sched.Start(intervalMin); err != nil {
		slog.Error("scheduler start failed", "error", err)
	}
	defer sched.Stop()

	// ========== 7. HTTP 路由 ==========
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// r.Use(func(c *gin.Context) {
	// 	c.Header("Access-Control-Allow-Origin", "*")
	// 	c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	// 	c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
	// 	if c.Request.Method == "OPTIONS" {
	// 		c.AbortWithStatus(204)
	// 		return
	// 	}
	// 	c.Next()
	// })

	ctl := controller.NewMaoyanController(dataSvc, authSvc)

	// 公开路由
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "time": time.Now().Format(time.RFC3339)}) })
	r.POST("/api/auth/register", ctl.Register)
	r.POST("/api/auth/login", ctl.Login)
	r.GET("/api/cities", ctl.GetCities)
	r.GET("/api/districts", ctl.GetDistricts)
	r.GET("/api/movies/hot", ctl.GetHotMovies)
	r.GET("/api/movies/search", ctl.SearchMovies)
	r.GET("/api/shows", ctl.QueryShows)

	// 需登录路由
	auth := r.Group("/api", middleware.AuthRequired(authSvc))
	{
		auth.POST("/subscriptions", ctl.CreateSubscription)
		auth.GET("/subscriptions", ctl.ListSubscriptions)
		auth.GET("/subscriptions/:id", ctl.GetSubscriptionDetail)
		auth.PATCH("/subscriptions/:id/toggle", ctl.ToggleSubscription)
		auth.PUT("/subscriptions/:id", ctl.UpdateSubscription)
		auth.DELETE("/subscriptions/:id", ctl.DeleteSubscription)
		auth.GET("/subscriptions/logs", ctl.GetSubscriptionLogs)
		auth.GET("/subscriptions/cinemas", ctl.GetUserSubscribedCinemas)
		auth.GET("/subscriptions/cinema-movies", ctl.GetSubscribedCinemaMovies)
		auth.POST("/subscriptions/:id/refresh", ctl.QuerySubscriptionPrices)
		auth.GET("/subscriptions/:id/export", ctl.ExportSubscriptionCSV)
		auth.GET("/subscriptions/:id/crawl-records", ctl.GetCrawlRecords)
		auth.GET("/subscriptions/:id/snapshots/:snapshot_id/shows", ctl.GetSnapshotMovieShows)
		auth.GET("/shows/export", ctl.ExportShowsCSV)
		auth.GET("/price-changes", ctl.GetPriceChanges)
		auth.POST("/admin/fetch", ctl.TriggerFetch)
		auth.POST("/admin/crawl/:cinema_id", ctl.ManualCrawl)
	}

	// ========== 8. 启动 HTTP 服务 ==========
	port := viper.GetString("PORT")
	if port == "" {
		port = "8080"
	}

	go func() {
		slog.Info("server starting", "port", port)
		if err := r.Run(":" + port); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down...")
}

// initLogger 初始化日志：stdout（Info 级别）+ 文件（Warn 级别，./logs/app.log）
func initLogger() {
	_ = os.MkdirAll("logs", 0o755)
	f, err := os.OpenFile("logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stdout, nil)).Error("log file open failed", "error", err)
		return
	}

	// stdout=Info（容器捕获）+ file=Warn（持久化）
	stdoutW := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	fileW := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelWarn})
	slog.SetDefault(slog.New(ioMultiHandler{handlers: []slog.Handler{stdoutW, fileW}}))
}

// ioMultiHandler 扇出到多个 slog handler
// Go 1.25 slog 无内置 multi handler，轻量实现
type ioMultiHandler struct {
	handlers []slog.Handler
}

func (h ioMultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h ioMultiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			_ = handler.Handle(ctx, r.Clone())
		}
	}
	return nil
}

func (h ioMultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return ioMultiHandler{handlers}
}

func (h ioMultiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return ioMultiHandler{handlers}
}

// initDB 初始化 PostgreSQL 连接池
func initDB() (*gorm.DB, error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = viper.GetString("DB_DSN")
	}
	if dsn == "" {
		slog.Error("DB_DSN not set")
		os.Exit(1)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // Supabase PgBouncer 不支持 prepared statement
	}), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Warn),
		DisableForeignKeyConstraintWhenMigrating: true, // 禁止 AutoMigrate 创建物理外键（设计原则：只用逻辑外键）
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

// dropOldTables 清理旧表（必须在 AutoMigrate 之前执行）
// 这些表要么是 PDF 设计中已删除的，要么需要重建以清除旧列约束
func dropOldTables(db *gorm.DB) {
	drops := []string{
		`DROP TABLE IF EXISTS show_snapshots CASCADE`,
		`DROP TABLE IF EXISTS price_history CASCADE`,
		`DROP TABLE IF EXISTS subscription_logs CASCADE`,
		`DROP TABLE IF EXISTS movies CASCADE`,
		`DROP TABLE IF EXISTS districts CASCADE`,
		`DROP TABLE IF EXISTS cities CASCADE`,
		`DROP TABLE IF EXISTS city CASCADE`,
		`DROP TABLE IF EXISTS cinemas CASCADE`,
		`DROP TABLE IF EXISTS cinema CASCADE`, // 重建以清除旧 city_id 列
		`DROP TABLE IF EXISTS subscriptions CASCADE`,
		`DROP TABLE IF EXISTS crawl_tasks CASCADE`,
		`DROP TABLE IF EXISTS execute_logs CASCADE`,
		`DROP TABLE IF EXISTS price_snapshot_batches CASCADE`,
		`DROP TABLE IF EXISTS price_snapshot_items CASCADE`,
		`DROP TABLE IF EXISTS price_snapshot_batch CASCADE`,
		`DROP TABLE IF EXISTS price_snapshot_item CASCADE`,
		`DROP TABLE IF EXISTS price_snapshot CASCADE`,
		`DROP TABLE IF EXISTS notify_logs CASCADE`,
	}
	for _, sql := range drops {
		if err := db.Exec(sql).Error; err != nil {
			slog.Warn("drop old table skipped", "sql", sql, "error", err)
		}
	}
	slog.Info("old tables dropped")
}

// runManualMigrations 执行额外 SQL：补索引
func runManualMigrations(db *gorm.DB) {
	migrations := []struct {
		name string
		sql  string
	}{
		// 补充索引
		{name: "idx_subscription_cinema_email", sql: `CREATE UNIQUE INDEX IF NOT EXISTS uk_subscription_cinema_email ON subscription(cinema_id, email)`},
		{name: "backfill_initial_target_price", sql: `UPDATE subscription SET initial_target_price = target_price WHERE initial_target_price = 0`},
		{name: "idx_crawl_task_cinema", sql: `CREATE UNIQUE INDEX IF NOT EXISTS uk_crawl_task_cinema_id ON crawl_task(cinema_id)`},
		{name: "idx_crawl_task_status_next_run", sql: `CREATE INDEX IF NOT EXISTS idx_crawl_task_status_next_run ON crawl_task(status, next_run_at)`},
		{name: "idx_execute_log_cinema_started", sql: `CREATE INDEX IF NOT EXISTS idx_execute_log_cinema_started ON execute_log(cinema_id, started_at)`},
		{name: "idx_item_cinema_movie_date", sql: `CREATE INDEX IF NOT EXISTS idx_item_cinema_movie_date ON price_snapshot_item(cinema_id, movie_id, show_date)`},
		{name: "idx_item_cinema_price", sql: `CREATE INDEX IF NOT EXISTS idx_item_cinema_price ON price_snapshot_item(cinema_id, price)`},
		{name: "idx_notify_log_subscription_sent", sql: `CREATE INDEX IF NOT EXISTS idx_notify_log_subscription_sent ON notify_log(subscription_id, sent_at)`},
	}
	for _, m := range migrations {
		if err := db.Exec(m.sql).Error; err != nil {
			slog.Warn("migration skipped", "name", m.name, "error", err)
		}
	}
}

// dropAutoForeignKeys 删除 AutoMigrate 自动创建的物理外键约束
// 设计原则：不加物理外键，仅通过应用层维护逻辑外键
// GORM 看到 struct 中的 `gorm:"foreignKey:xxx"` 标签会自动建 FK，迁移后统一清除
func dropAutoForeignKeys(db *gorm.DB) {
	var constraints []struct {
		ConstraintName string `gorm:"column:constraint_name"`
		TableName      string `gorm:"column:table_name"`
	}
	db.Raw(`
		SELECT con.conname AS constraint_name, rel.relname AS table_name
		FROM pg_constraint con
		JOIN pg_class rel ON rel.oid = con.conrelid
		WHERE con.contype = 'f'
		  AND rel.relname IN ('subscription', 'crawl_task', 'execute_log', 'price_snapshot', 'notify_log')
	`).Scan(&constraints)

	for _, c := range constraints {
		sql := fmt.Sprintf(`ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s`, c.TableName, c.ConstraintName)
		if err := db.Exec(sql).Error; err != nil {
			slog.Warn("drop FK constraint failed", "table", c.TableName, "constraint", c.ConstraintName, "error", err)
		} else {
			slog.Info("dropped auto FK constraint", "table", c.TableName, "constraint", c.ConstraintName)
		}
	}
}
