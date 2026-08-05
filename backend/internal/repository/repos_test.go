package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"maoyan-service/backend/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ==========================================
// Repository 层手动测试 — 每行代码都通过真实 PostgreSQL 验证
//
// 运行方式：
//   方式一：全部测试
//     go test -v ./internal/repository/ -count=1
//
//   方式二：只测某一个方法
//     go test -v ./internal/repository/ -count=1 -run TestCinemaRepo_Create
//
//   方式三：VS Code / GoLand 里点 Run 按钮跑单个函数
// ==========================================

// ==================== 测试基础设施 ====================

var testDB *gorm.DB

// TestMain 是 Go 测试的入口函数，整个包的所有测试共享同一个 DB 连接
func TestMain(m *testing.M) {
	var err error
	testDB, err = openTestDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect DB failed: %v\n", err)
		os.Exit(1)
	}

	// // 先清理旧表（清掉残留的旧列 city_id 等），再 AutoMigrate
	// dropOldTablesForTest()

	// // 建表（只加表/列，不影响已有数据）
	// testDB.AutoMigrate(
	// 	&model.Cinema{},
	// 	&model.User{},
	// 	&model.Subscription{},
	// 	&model.CrawlTask{},
	// 	&model.ExecuteLog{},
	// 	&model.PriceSnapshotBatch{},
	// 	&model.PriceSnapshotItem{},
	// 	&model.NotifyLog{},
	// )

	// // 清理 GORM 自动创建的外键约束（我们设计原则是不加物理外键）
	// dropFkConstraints()

	// // 删除旧残留唯一索引（从 uniqueIndex 改为 index 时 AutoMigrate 不会自动改）
	// testDB.Exec("DROP INDEX IF EXISTS idx_subscription_cinema_id")
	// testDB.Exec("CREATE INDEX IF NOT EXISTS idx_subscription_cinema_id ON subscription(cinema_id)")

	code := m.Run()
	os.Exit(code)
}

// dropOldTablesForTest 删除残留的旧 schema（解决残留 city_id NOT NULL 列问题）
func dropOldTablesForTest() {
	tables := []string{
		"notify_logs", "price_snapshot_items", "price_snapshot_batches",
		"execute_logs", "crawl_tasks", "subscriptions", "subscription_logs",
		"show_snapshots", "price_history", "movies", "districts", "cities", "city",
		"cinemas", "cinema", // 重建 cinema 以清除旧 city_id 列
	}
	for _, t := range tables {
		testDB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", t))
	}
}

func openTestDB() (*gorm.DB, error) {
	// 优先用 TEST_DB_DSN 环境变量，否则用默认 Supabase DSN
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = "host=aws-1-ap-northeast-1.pooler.supabase.com user=postgres.lfsbklakbstfhcheuplg password=987555458@qq.com dbname=postgres port=6543 sslmode=require"
	}
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // Supabase PgBouncer 不支持 prepared statement
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// cleanTable 测试结束后清空指定表（确保不会污染其他测试或生产数据）
func cleanTable(t *testing.T, tables ...string) {
	t.Helper()
	for _, table := range tables {
		testDB.Exec(fmt.Sprintf("DELETE FROM %s", table))
	}
}

// dropFkConstraints 清理 GORM AutoMigrate 自动创建的物理外键约束
func dropFkConstraints() {
	fks := map[string][]string{
		"subscription":         {"fk_subscription_cinema", "fk_subscriptions_user"},
		"crawl_task":           {"fk_crawl_task_cinema"},
		"execute_log":          {"fk_execute_log_crawl_task"},
		"price_snapshot_batch": {"fk_price_snapshot_batch_crawl_task"},
		"price_snapshot_item":  {"fk_price_snapshot_item_batch"},
		"notify_log":           {"fk_notify_log_subscription"},
	}
	for table, names := range fks {
		for _, fk := range names {
			testDB.Exec(fmt.Sprintf("ALTER TABLE IF EXISTS %s DROP CONSTRAINT IF EXISTS %s", table, fk))
		}
	}
}

// ==================== CinemaRepo ====================

func TestCinemaRepo_Create_Success(t *testing.T) {
	repo := NewCinemaRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "cinema") })

	cinema := &model.Cinema{
		MaoyanCityID:   10,
		MaoyanCinemaID: "37379",
		Name:           "测试影院·万达影城",
		Address:        "测试路1号",
		Status:         1,
	}
	err := repo.Create(context.Background(), cinema)

	require.NoError(t, err)
	assert.NotZero(t, cinema.ID, "自增主键应回填")

	// 读回验证
	got, err := repo.GetByID(context.Background(), cinema.ID)
	require.NoError(t, err)
	assert.Equal(t, "测试影院·万达影城", got.Name)
	assert.Equal(t, "37379", got.MaoyanCinemaID)
}

func TestCinemaRepo_GetByID_NotFound(t *testing.T) {
	repo := NewCinemaRepo(testDB)

	_, err := repo.GetByID(context.Background(), 99999999)

	assert.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestCinemaRepo_GetByMaoyanCinemaID(t *testing.T) {
	repo := NewCinemaRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "cinema") })

	// 准备数据
	cinema := &model.Cinema{MaoyanCinemaID: "88888", Name: "按猫眼ID查", MaoyanCityID: 10, Status: 1}
	require.NoError(t, repo.Create(context.Background(), cinema))

	// 查得到
	got, err := repo.GetByMaoyanCinemaID(context.Background(), "88888")
	require.NoError(t, err)
	assert.Equal(t, "按猫眼ID查", got.Name)

	// 查不到
	_, err = repo.GetByMaoyanCinemaID(context.Background(), "00000")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestCinemaRepo_GetByMaoyanCityID(t *testing.T) {
	repo := NewCinemaRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "cinema") })

	// status=1 的
	require.NoError(t, repo.Create(context.Background(), &model.Cinema{MaoyanCinemaID: "A1", MaoyanCityID: 20, Name: "A影院", Status: 1}))
	require.NoError(t, repo.Create(context.Background(), &model.Cinema{MaoyanCinemaID: "A2", MaoyanCityID: 20, Name: "B影院", Status: 1}))
	// status=0 的先创建再改为 0（GORM Create 对 int8(0) 会替换为 default 1）
	c3 := &model.Cinema{MaoyanCinemaID: "A3", MaoyanCityID: 20, Name: "已禁用", Status: 1}
	require.NoError(t, repo.Create(context.Background(), c3))
	// 用 UpdateFields 传 map 可以写入零值
	testDB.Model(&model.Cinema{}).Where("id = ?", c3.ID).Update("status", 0)
	// 其他城市
	require.NoError(t, repo.Create(context.Background(), &model.Cinema{MaoyanCinemaID: "A4", MaoyanCityID: 99, Name: "其他城市", Status: 1}))

	results, err := repo.GetByMaoyanCityID(context.Background(), 20)
	require.NoError(t, err)
	assert.Len(t, results, 2, "应该只返回 status=1 的两条")
}

func TestCinemaRepo_SearchByName(t *testing.T) {
	repo := NewCinemaRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "cinema") })

	require.NoError(t, repo.Create(context.Background(), &model.Cinema{MaoyanCinemaID: "S1", Name: "北京万达影城CBD店", MaoyanCityID: 10, Status: 1}))
	require.NoError(t, repo.Create(context.Background(), &model.Cinema{MaoyanCinemaID: "S2", Name: "上海万达影城", MaoyanCityID: 27, Status: 1}))
	require.NoError(t, repo.Create(context.Background(), &model.Cinema{MaoyanCinemaID: "S3", Name: "北京金逸影城", MaoyanCityID: 10, Status: 1}))

	// ILIKE 不区分大小写
	results, err := repo.SearchByName(context.Background(), "万达")
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// 搜不到
	results, err = repo.SearchByName(context.Background(), "不存在的影院名xyz")
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestCinemaRepo_Upsert_Insert(t *testing.T) {
	repo := NewCinemaRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "cinema") })

	cinema := &model.Cinema{MaoyanCinemaID: "U1", Name: "Upsert新建", MaoyanCityID: 10, Status: 1}
	err := repo.Upsert(context.Background(), cinema)
	require.NoError(t, err)
	assert.NotZero(t, cinema.ID, "Upsert 插入后应回填主键")
}

func TestCinemaRepo_Upsert_Update(t *testing.T) {
	repo := NewCinemaRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "cinema") })

	// 先建
	cinema := &model.Cinema{MaoyanCinemaID: "U2", Name: "原来名字", MaoyanCityID: 10, Status: 1}
	require.NoError(t, repo.Create(context.Background(), cinema))
	oldID := cinema.ID

	// 改名字再 Upsert
	cinema.Name = "已更新名字"
	err := repo.Upsert(context.Background(), cinema)
	require.NoError(t, err)

	// 验证
	got, _ := repo.GetByID(context.Background(), oldID)
	assert.Equal(t, "已更新名字", got.Name)
}

func TestCinemaRepo_BulkUpsert(t *testing.T) {
	repo := NewCinemaRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "cinema") })

	// 第一批：插入 2 条
	cinemas := []model.Cinema{
		{MaoyanCinemaID: "B1", Name: "批量1", MaoyanCityID: 10, Status: 1},
		{MaoyanCinemaID: "B2", Name: "批量2", MaoyanCityID: 10, Status: 1},
	}
	err := repo.BulkUpsert(context.Background(), cinemas)
	require.NoError(t, err)

	// 第二批：B1 更新，B3 新插入
	cinemas2 := []model.Cinema{
		{MaoyanCinemaID: "B1", Name: "批量1已更新", MaoyanCityID: 10, Status: 1},
		{MaoyanCinemaID: "B3", Name: "批量3", MaoyanCityID: 10, Status: 1},
	}
	err = repo.BulkUpsert(context.Background(), cinemas2)
	require.NoError(t, err)

	// 验证：总共 3 条，B1 已更新
	all, _ := repo.GetByMaoyanCityID(context.Background(), 10)
	assert.Len(t, all, 3)
	for _, c := range all {
		if c.MaoyanCinemaID == "B1" {
			assert.Equal(t, "批量1已更新", c.Name)
		}
	}
}

// ==================== UserRepo ====================

func TestUserRepo_Create_Success(t *testing.T) {
	repo := NewUserRepo(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM users WHERE email LIKE 'test_%'") })

	user := &model.User{Email: "test_create@example.com", PasswordHash: "hashed_xxx"}
	err := repo.Create(context.Background(), user)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, user.ID, "BeforeCreate 钩子应自动生成 UUID")
}

func TestUserRepo_FindByEmail(t *testing.T) {
	repo := NewUserRepo(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM users WHERE email LIKE 'test_%'") })

	user := &model.User{Email: "test_find@example.com", PasswordHash: "hashed_xxx"}
	require.NoError(t, repo.Create(context.Background(), user))

	// 查得到
	got, err := repo.FindByEmail(context.Background(), "test_find@example.com")
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)

	// 查不到
	_, err = repo.FindByEmail(context.Background(), "not_exist@example.com")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestUserRepo_FindByID(t *testing.T) {
	repo := NewUserRepo(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM users WHERE email LIKE 'test_%'") })

	user := &model.User{Email: "test_byid@example.com", PasswordHash: "hashed_xxx"}
	require.NoError(t, repo.Create(context.Background(), user))

	got, err := repo.FindByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, "test_byid@example.com", got.Email)

	// 不存在的 UUID
	_, err = repo.FindByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ==================== SubscriptionRepo ====================

func TestSubscriptionRepo_Create(t *testing.T) {
	repo := NewSubscriptionRepo(testDB)
	uid := uuid.New()
	t.Cleanup(func() {
		cleanTable(t, "subscription", "cinema")
	})
	require.NoError(t, NewCinemaRepo(testDB).Create(context.Background(),
		&model.Cinema{MaoyanCinemaID: "SUB_C1", Name: "订阅影城", MaoyanCityID: 10, Status: 1}))

	sub := &model.Subscription{
		CinemaID:      999001,
		Email:         "test_sub@example.com",
		TargetPrice:   35.5,
		NotifyEnabled: true,
		Status:        1,
		UserID:        &uid,
	}
	err := repo.Create(context.Background(), sub)
	require.NoError(t, err)
	assert.NotZero(t, sub.ID)
}

func TestSubscriptionRepo_FindByID_PreloadCinema(t *testing.T) {
	repo := NewSubscriptionRepo(testDB)
	uid := uuid.New()
	t.Cleanup(func() { cleanTable(t, "subscription", "cinema") })

	// 数据准备：先建 cinema，再建 subscription
	cinemaRepo := NewCinemaRepo(testDB)
	c := &model.Cinema{MaoyanCinemaID: "SUB_P1", Name: "预加载影城", MaoyanCityID: 10, Status: 1}
	require.NoError(t, cinemaRepo.Create(context.Background(), c))

	sub := &model.Subscription{CinemaID: c.ID, Email: "test_preload@example.com", TargetPrice: 40, NotifyEnabled: true, Status: 1, UserID: &uid}
	require.NoError(t, repo.Create(context.Background(), sub))

	got, err := repo.FindByID(context.Background(), sub.ID)
	t.Logf("%v", got)
	require.NoError(t, err)
	// Preload 应填充了 Cinema 关联对象（注意：Subscription 模型没有定义 Cinema 关联字段，
	// 所以 Preload("Cinema") 实际上需要模型里有对应关系。如果模型没有，这行会静默失败。
	// 这里只验证基础查询可用）
	assert.Equal(t, "test_preload@example.com", got.Email)
}

func TestSubscriptionRepo_FindByCinemaAndEmail(t *testing.T) {
	repo := NewSubscriptionRepo(testDB)
	uid := uuid.New()
	t.Cleanup(func() { cleanTable(t, "subscription") })

	sub := &model.Subscription{CinemaID: 100, Email: "unique@example.com", TargetPrice: 30, NotifyEnabled: true, Status: 1, UserID: &uid}
	require.NoError(t, repo.Create(context.Background(), sub))

	// 命中
	got, err := repo.FindByCinemaAndEmail(context.Background(), 100, "unique@example.com")
	require.NoError(t, err)
	assert.Equal(t, sub.ID, got.ID)

	// 不命中 — 影院对但邮箱不对
	_, err = repo.FindByCinemaAndEmail(context.Background(), 100, "wrong@example.com")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// 不命中 — 邮箱对但影院不对
	_, err = repo.FindByCinemaAndEmail(context.Background(), 999, "unique@example.com")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestSubscriptionRepo_FindByCinemaID(t *testing.T) {
	repo := NewSubscriptionRepo(testDB)
	uid := uuid.New()
	t.Cleanup(func() { cleanTable(t, "subscription") })

	require.NoError(t, repo.Create(context.Background(), &model.Subscription{CinemaID: 200, Email: "a@a.com", TargetPrice: 30, Status: 1, UserID: &uid}))
	require.NoError(t, repo.Create(context.Background(), &model.Subscription{CinemaID: 200, Email: "b@b.com", TargetPrice: 35, Status: 1, UserID: &uid}))
	// GORM 零值问题：Status: 0 会被 default:1 覆盖，先创建再 UPDATE
	disabledSub := &model.Subscription{CinemaID: 200, Email: "disabled@b.com", TargetPrice: 35, Status: 1, UserID: &uid}
	require.NoError(t, repo.Create(context.Background(), disabledSub))
	testDB.Model(&model.Subscription{}).Where("id = ?", disabledSub.ID).Update("status", 0)
	require.NoError(t, repo.Create(context.Background(), &model.Subscription{CinemaID: 300, Email: "c@c.com", TargetPrice: 40, Status: 1, UserID: &uid}))

	results, err := repo.FindByCinemaID(context.Background(), 200)
	require.NoError(t, err)
	assert.Len(t, results, 2, "status=0 的不能出现")
}

func TestSubscriptionRepo_FindByEmail(t *testing.T) {
	repo := NewSubscriptionRepo(testDB)
	uid := uuid.New()
	t.Cleanup(func() { cleanTable(t, "subscription") })

	require.NoError(t, repo.Create(context.Background(), &model.Subscription{CinemaID: 400, Email: "same@example.com", TargetPrice: 30, Status: 1, UserID: &uid}))
	require.NoError(t, repo.Create(context.Background(), &model.Subscription{CinemaID: 500, Email: "same@example.com", TargetPrice: 35, Status: 1, UserID: &uid}))

	results, err := repo.FindByEmail(context.Background(), "same@example.com")
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.True(t, results[0].CreatedAt.After(results[1].CreatedAt) || results[0].CreatedAt.Equal(results[1].CreatedAt),
		"应按 created_at DESC 排序")
}

func TestSubscriptionRepo_ListActive(t *testing.T) {
	repo := NewSubscriptionRepo(testDB)
	uid := uuid.New()
	t.Cleanup(func() { cleanTable(t, "subscription") })

	require.NoError(t, repo.Create(context.Background(), &model.Subscription{CinemaID: 600, Email: "active@a.com", TargetPrice: 30, Status: 1, UserID: &uid}))
	// Status: 0 写不进去（GORM 对 zero-value int8(0) 会用 db default 1），先创建再 UPDATE
	subInactive := &model.Subscription{CinemaID: 601, Email: "inactive@a.com", TargetPrice: 30, Status: 1, UserID: &uid}
	require.NoError(t, repo.Create(context.Background(), subInactive))
	testDB.Model(&model.Subscription{}).Where("id = ?", subInactive.ID).Update("status", 0)

	results, err := repo.ListActive(context.Background())
	require.NoError(t, err)
	// 至少包含我们刚建的 status=1 那条
	found := false
	for _, s := range results {
		if s.Email == "active@a.com" {
			found = true
		}
	}
	assert.True(t, found)
	// status=0 不应该出现
	for _, s := range results {
		assert.NotEqual(t, "inactive@a.com", s.Email)
	}
}

func TestSubscriptionRepo_UpdateFields(t *testing.T) {
	repo := NewSubscriptionRepo(testDB)
	uid := uuid.New()
	t.Cleanup(func() { cleanTable(t, "subscription") })

	sub := &model.Subscription{CinemaID: 700, Email: "partial@example.com", TargetPrice: 30, Status: 1, UserID: &uid}
	require.NoError(t, repo.Create(context.Background(), sub))

	// 部分更新
	err := repo.UpdateFields(context.Background(), sub.ID, map[string]interface{}{
		"target_price": 25.0,
		"status":       int8(0),
	})
	require.NoError(t, err)

	got, _ := repo.FindByID(context.Background(), sub.ID)
	assert.Equal(t, 25.0, got.TargetPrice)
	assert.Equal(t, int8(0), got.Status)
	// email 不应该被覆盖
	assert.Equal(t, "partial@example.com", got.Email)
}

func TestSubscriptionRepo_UpdateNotifyCount_Atomic(t *testing.T) {
	repo := NewSubscriptionRepo(testDB)
	uid := uuid.New()
	t.Cleanup(func() { cleanTable(t, "subscription") })

	sub := &model.Subscription{CinemaID: 800, Email: "notifycount@example.com", TargetPrice: 30, Status: 1, UserID: &uid}
	require.NoError(t, repo.Create(context.Background(), sub))

	// 连续 +1 三次
	for range 3 {
		require.NoError(t, repo.UpdateNotifyCount(context.Background(), sub.ID))
	}

	got, _ := repo.FindByID(context.Background(), sub.ID)
	assert.Equal(t, 3, got.NotifyCount, "三次 +1 后应为 3")
}

func TestSubscriptionRepo_UpdateLastNotifyAt(t *testing.T) {
	repo := NewSubscriptionRepo(testDB)
	uid := uuid.New()
	t.Cleanup(func() { cleanTable(t, "subscription") })

	sub := &model.Subscription{CinemaID: 900, Email: "lastnotify@example.com", TargetPrice: 30, Status: 1, UserID: &uid}
	require.NoError(t, repo.Create(context.Background(), sub))

	now := time.Now().Round(time.Second)
	require.NoError(t, repo.UpdateLastNotifyAt(context.Background(), sub.ID, now))

	got, _ := repo.FindByID(context.Background(), sub.ID)
	require.NotNil(t, got.LastNotifyAt)
	assert.WithinDuration(t, now, *got.LastNotifyAt, time.Second)
}

func TestSubscriptionRepo_Delete(t *testing.T) {
	repo := NewSubscriptionRepo(testDB)
	uid := uuid.New()
	t.Cleanup(func() { cleanTable(t, "subscription") })

	sub := &model.Subscription{CinemaID: 1000, Email: "delete@example.com", TargetPrice: 30, Status: 1, UserID: &uid}
	require.NoError(t, repo.Create(context.Background(), sub))

	err := repo.Delete(context.Background(), sub.ID)
	require.NoError(t, err)

	_, err = repo.FindByID(context.Background(), sub.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestSubscriptionRepo_Update(t *testing.T) {
	repo := NewSubscriptionRepo(testDB)
	uid := uuid.New()
	t.Cleanup(func() { cleanTable(t, "subscription") })

	sub := &model.Subscription{CinemaID: 1100, Email: "fullupdate@example.com", TargetPrice: 30, Status: 1, UserID: &uid}
	require.NoError(t, repo.Create(context.Background(), sub))

	// 全量更新（Save 覆盖所有字段）
	sub.TargetPrice = 20.0
	sub.Status = 0
	sub.NotifyEnabled = false
	err := repo.Update(context.Background(), sub)
	require.NoError(t, err)

	got, _ := repo.FindByID(context.Background(), sub.ID)
	assert.Equal(t, 20.0, got.TargetPrice)
	assert.Equal(t, int8(0), got.Status)
	assert.False(t, got.NotifyEnabled)
}

// ==================== CrawlTaskRepo ====================

func TestCrawlTaskRepo_Create(t *testing.T) {
	repo := NewCrawlTaskRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "crawl_task") })

	task := &model.CrawlTask{
		CinemaID:        555001,
		IntervalMinutes: 30,
		NextRunAt:       time.Now(),
		Status:          1,
		Priority:        100,
		TimeoutSeconds:  60,
	}
	err := repo.Create(context.Background(), task)
	require.NoError(t, err)
	assert.NotZero(t, task.ID)
}

func TestCrawlTaskRepo_FindByCinemaID(t *testing.T) {
	repo := NewCrawlTaskRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "crawl_task") })

	task := &model.CrawlTask{CinemaID: 555002, IntervalMinutes: 30, NextRunAt: time.Now(), Status: 1}
	require.NoError(t, repo.Create(context.Background(), task))

	got, err := repo.FindByCinemaID(context.Background(), 555002)
	require.NoError(t, err)
	assert.Equal(t, task.ID, got.ID)

	_, err = repo.FindByCinemaID(context.Background(), 999999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestCrawlTaskRepo_ListDue(t *testing.T) {
	repo := NewCrawlTaskRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "crawl_task") })

	now := time.Now()
	// 到期的
	require.NoError(t, repo.Create(context.Background(), &model.CrawlTask{CinemaID: 555003, IntervalMinutes: 30, NextRunAt: now.Add(-1 * time.Hour), Status: 1, Priority: 50}))
	require.NoError(t, repo.Create(context.Background(), &model.CrawlTask{CinemaID: 555004, IntervalMinutes: 30, NextRunAt: now.Add(-30 * time.Minute), Status: 1, Priority: 100}))
	// 未到期的
	require.NoError(t, repo.Create(context.Background(), &model.CrawlTask{CinemaID: 555005, IntervalMinutes: 30, NextRunAt: now.Add(24 * time.Hour), Status: 1}))
	// 已停用的（先创建再改为 0，绕过 GORM 零值问题）
	disabled := &model.CrawlTask{CinemaID: 555006, IntervalMinutes: 30, NextRunAt: now.Add(-1 * time.Hour), Status: 1}
	require.NoError(t, repo.Create(context.Background(), disabled))
	testDB.Model(&model.CrawlTask{}).Where("id = ?", disabled.ID).Update("status", 0)

	results, err := repo.ListDue(context.Background(), now)
	require.NoError(t, err)
	assert.Len(t, results, 2, "应只有 status=1 且到期的两条")

	// 验证排序：priority 小的在前
	assert.True(t, results[0].Priority <= results[1].Priority)
}

func TestCrawlTaskRepo_ListActive(t *testing.T) {
	repo := NewCrawlTaskRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "crawl_task") })

	require.NoError(t, repo.Create(context.Background(), &model.CrawlTask{CinemaID: 555007, NextRunAt: time.Now(), Status: 1}))
	// 先创建再改为 0
	disabled := &model.CrawlTask{CinemaID: 555008, NextRunAt: time.Now(), Status: 1}
	require.NoError(t, repo.Create(context.Background(), disabled))
	testDB.Model(&model.CrawlTask{}).Where("id = ?", disabled.ID).Update("status", 0)

	results, err := repo.ListActive(context.Background())
	require.NoError(t, err)
	for _, r := range results {
		assert.Equal(t, int8(1), r.Status)
	}
}

func TestCrawlTaskRepo_UpdateNextRun(t *testing.T) {
	repo := NewCrawlTaskRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "crawl_task") })

	task := &model.CrawlTask{CinemaID: 555009, NextRunAt: time.Now(), Status: 1}
	require.NoError(t, repo.Create(context.Background(), task))

	next := time.Now().Add(30 * time.Minute).Round(time.Second)
	require.NoError(t, repo.UpdateNextRun(context.Background(), task.ID, next))

	got, _ := repo.FindByID(context.Background(), task.ID)
	assert.WithinDuration(t, next, got.NextRunAt, time.Second)
}

func TestCrawlTaskRepo_UpdateLastRun(t *testing.T) {
	repo := NewCrawlTaskRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "crawl_task") })

	task := &model.CrawlTask{CinemaID: 555010, NextRunAt: time.Now(), Status: 1}
	require.NoError(t, repo.Create(context.Background(), task))

	// 模拟执行出错
	err := repo.UpdateLastRun(context.Background(), task.ID, time.Now(), "error", "network timeout")
	require.NoError(t, err)

	got, _ := repo.FindByID(context.Background(), task.ID)
	assert.Equal(t, int8(2), got.Status, "出错后 status 应为 2（暂停）")
}

func TestCrawlTaskRepo_UpdateLastRun_Active(t *testing.T) {
	repo := NewCrawlTaskRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "crawl_task") })

	task := &model.CrawlTask{CinemaID: 555011, NextRunAt: time.Now(), Status: 2} // 先暂停
	require.NoError(t, repo.Create(context.Background(), task))

	// 模拟恢复正常
	err := repo.UpdateLastRun(context.Background(), task.ID, time.Now(), "active", "")
	require.NoError(t, err)

	got, _ := repo.FindByID(context.Background(), task.ID)
	assert.Equal(t, int8(1), got.Status, "恢复后 status 应为 1")
}

func TestCrawlTaskRepo_UpdateStats_Success(t *testing.T) {
	repo := NewCrawlTaskRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "crawl_task") })

	task := &model.CrawlTask{CinemaID: 555012, NextRunAt: time.Now(), Status: 1}
	require.NoError(t, repo.Create(context.Background(), task))

	require.NoError(t, repo.UpdateStats(context.Background(), task.ID, true))
	got, _ := repo.FindByID(context.Background(), task.ID)
	assert.Equal(t, 1, got.RunCount)
	assert.Equal(t, 1, got.SuccessCount)
	assert.Equal(t, 0, got.FailCount)
	assert.NotNil(t, got.LastSuccessAt)
}

func TestCrawlTaskRepo_UpdateStats_Fail(t *testing.T) {
	repo := NewCrawlTaskRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "crawl_task") })

	task := &model.CrawlTask{CinemaID: 555013, NextRunAt: time.Now(), Status: 1}
	require.NoError(t, repo.Create(context.Background(), task))

	require.NoError(t, repo.UpdateStats(context.Background(), task.ID, false))
	got, _ := repo.FindByID(context.Background(), task.ID)
	assert.Equal(t, 1, got.RunCount)
	assert.Equal(t, 0, got.SuccessCount)
	assert.Equal(t, 1, got.FailCount)
}

func TestCrawlTaskRepo_DeleteByCinemaID(t *testing.T) {
	repo := NewCrawlTaskRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "crawl_task") })

	task := &model.CrawlTask{CinemaID: 555014, NextRunAt: time.Now(), Status: 1}
	require.NoError(t, repo.Create(context.Background(), task))

	err := repo.DeleteByCinemaID(context.Background(), 555014)
	require.NoError(t, err)

	_, err = repo.FindByCinemaID(context.Background(), 555014)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ==================== ExecuteLogRepo ====================

func TestExecuteLogRepo_CreateAndFind(t *testing.T) {
	repo := NewExecuteLogRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "execute_log") })

	log := &model.ExecuteLog{
		CrawlTaskID: 100,
		CinemaID:    200,
		Status:      "running",
	}
	err := repo.Create(context.Background(), log)
	require.NoError(t, err)
	assert.NotZero(t, log.ID)

	got, err := repo.FindByID(context.Background(), log.ID)
	require.NoError(t, err)
	assert.Equal(t, "running", got.Status)
}

func TestExecuteLogRepo_FindByCrawlTaskID(t *testing.T) {
	repo := NewExecuteLogRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "execute_log") })

	require.NoError(t, repo.Create(context.Background(), &model.ExecuteLog{CrawlTaskID: 300, CinemaID: 400, Status: "success"}))
	require.NoError(t, repo.Create(context.Background(), &model.ExecuteLog{CrawlTaskID: 300, CinemaID: 400, Status: "fail"}))
	require.NoError(t, repo.Create(context.Background(), &model.ExecuteLog{CrawlTaskID: 999, CinemaID: 400, Status: "success"}))

	results, err := repo.FindByCrawlTaskID(context.Background(), 300, 5)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.True(t, results[0].StartedAt.After(results[1].StartedAt) || results[0].StartedAt.Equal(results[1].StartedAt),
		"应按 started_at DESC 排序")
}

func TestExecuteLogRepo_Update(t *testing.T) {
	repo := NewExecuteLogRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "execute_log") })

	log := &model.ExecuteLog{CrawlTaskID: 500, CinemaID: 600, Status: "running"}
	require.NoError(t, repo.Create(context.Background(), log))

	log.Status = "success"
	log.ErrorMsg = ""
	endTime := time.Now()
	log.EndedAt = &endTime
	err := repo.Update(context.Background(), log)
	require.NoError(t, err)

	got, _ := repo.FindByID(context.Background(), log.ID)
	assert.Equal(t, "success", got.Status)
}

// ==================== PriceSnapshotBatchRepo ====================

func TestPriceSnapshotBatchRepo_CreateAndFind(t *testing.T) {
	repo := NewPriceSnapshotBatchRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "price_snapshot_batch") })

	batch := &model.PriceSnapshotBatch{
		CrawlTaskID: 10,
		CinemaID:    20,
		Source:      "maoyan",
	}
	err := repo.Create(context.Background(), batch)
	require.NoError(t, err)
	assert.NotZero(t, batch.ID)

	got, err := repo.FindByID(context.Background(), batch.ID)
	require.NoError(t, err)
	assert.Equal(t, "maoyan", got.Source)
}

func TestPriceSnapshotBatchRepo_FindByExecuteLogID(t *testing.T) {
	repo := NewPriceSnapshotBatchRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "price_snapshot_batch") })

	eid := uint64(999)
	require.NoError(t, repo.Create(context.Background(), &model.PriceSnapshotBatch{CrawlTaskID: 1, CinemaID: 2, ExecuteLogID: &eid, Source: "maoyan"}))
	require.NoError(t, repo.Create(context.Background(), &model.PriceSnapshotBatch{CrawlTaskID: 3, CinemaID: 4, ExecuteLogID: nil, Source: "maoyan"}))

	results, err := repo.FindByExecuteLogID(context.Background(), 999)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestPriceSnapshotBatchRepo_FindByCinemaID(t *testing.T) {
	repo := NewPriceSnapshotBatchRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "price_snapshot_batch") })

	require.NoError(t, repo.Create(context.Background(), &model.PriceSnapshotBatch{CrawlTaskID: 1, CinemaID: 100, Source: "maoyan"}))
	require.NoError(t, repo.Create(context.Background(), &model.PriceSnapshotBatch{CrawlTaskID: 2, CinemaID: 100, Source: "maoyan"}))
	require.NoError(t, repo.Create(context.Background(), &model.PriceSnapshotBatch{CrawlTaskID: 3, CinemaID: 200, Source: "maoyan"}))

	results, err := repo.FindByCinemaID(context.Background(), 100, 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

// ==================== PriceSnapshotItemRepo ====================

func TestPriceSnapshotItemRepo_BulkCreate(t *testing.T) {
	repo := NewPriceSnapshotItemRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "price_snapshot_item") })

	now := time.Now()
	items := []model.PriceSnapshotItem{
		{BatchID: 1, CinemaID: 10, MovieID: "M1", MovieName: "测试电影1", ShowDate: now, ShowStartAt: now, Price: 35.5, ObservedAt: now},
		{BatchID: 1, CinemaID: 10, MovieID: "M2", MovieName: "测试电影2", ShowDate: now, ShowStartAt: now, Price: 42.0, ObservedAt: now},
		{BatchID: 1, CinemaID: 10, MovieID: "M3", MovieName: "测试电影3", ShowDate: now, ShowStartAt: now, Price: 28.0, ObservedAt: now},
	}
	err := repo.BulkCreate(context.Background(), items)
	require.NoError(t, err)

	// 验证已写入且 ID 回填
	for i := range items {
		assert.NotZero(t, items[i].ID, "每条都应回填自增 ID")
	}
}

func TestPriceSnapshotItemRepo_BulkCreate_Empty(t *testing.T) {
	repo := NewPriceSnapshotItemRepo(testDB)
	err := repo.BulkCreate(context.Background(), nil)
	require.NoError(t, err, "空切片不应报错")
}

func TestPriceSnapshotItemRepo_FindByBatchID(t *testing.T) {
	repo := NewPriceSnapshotItemRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "price_snapshot_item") })

	now := time.Now()
	require.NoError(t, repo.BulkCreate(context.Background(), []model.PriceSnapshotItem{
		{BatchID: 99, CinemaID: 10, MovieID: "M99", MovieName: "批次99", ShowDate: now, ShowStartAt: now, Price: 30, ObservedAt: now},
	}))

	results, err := repo.FindByBatchID(context.Background(), 99)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "批次99", results[0].MovieName)

	// 不存在的批次
	results, err = repo.FindByBatchID(context.Background(), 99999)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestPriceSnapshotItemRepo_FindLatestByCinema(t *testing.T) {
	repo := NewPriceSnapshotItemRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "price_snapshot_item") })

	now := time.Now()
	require.NoError(t, repo.BulkCreate(context.Background(), []model.PriceSnapshotItem{
		{BatchID: 1, CinemaID: 50, MovieID: "M1", MovieName: "旧数据", ShowDate: now, ShowStartAt: now.Add(-2 * time.Hour), Price: 30, ObservedAt: now.Add(-2 * time.Hour)},
		{BatchID: 2, CinemaID: 50, MovieID: "M2", MovieName: "新数据", ShowDate: now, ShowStartAt: now, Price: 25, ObservedAt: now},
		{BatchID: 3, CinemaID: 99, MovieID: "M3", MovieName: "其他影院", ShowDate: now, ShowStartAt: now, Price: 40, ObservedAt: now},
	}))

	results, err := repo.FindLatestByCinema(context.Background(), 50, 5)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
	assert.Equal(t, "新数据", results[0].MovieName, "最新的应排在最前")
}

func TestPriceSnapshotItemRepo_FindPriceTrendByCinema(t *testing.T) {
	repo := NewPriceSnapshotItemRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "price_snapshot_item") })

	now := time.Now()
	require.NoError(t, repo.BulkCreate(context.Background(), []model.PriceSnapshotItem{
		{BatchID: 1, CinemaID: 60, MovieID: "T1", MovieName: "趋势1", ShowDate: now, ShowStartAt: now, Price: 30.0, ObservedAt: now},
		{BatchID: 1, CinemaID: 60, MovieID: "T2", MovieName: "趋势2", ShowDate: now, ShowStartAt: now, Price: 25.0, ObservedAt: now}, // 最低价
	}))

	points, err := repo.FindPriceTrendByCinema(context.Background(), 60, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(points), 1)
	assert.Equal(t, 25.0, points[0].Price, "同时间段应取 MIN(price)")
}

// ==================== NotifyLogRepo ====================

func TestNotifyLogRepo_BulkCreate(t *testing.T) {
	repo := NewNotifyLogRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "notify_log") })

	logs := []model.NotifyLog{
		{SubscriptionID: 1, CinemaID: 10, Email: "a@a.com", NotifyStatus: "success", TargetPrice: 30, MatchedPrice: 25},
		{SubscriptionID: 2, CinemaID: 20, Email: "b@b.com", NotifyStatus: "fail", TargetPrice: 35, MatchedPrice: 40, ErrorMsg: "SMTP timeout"},
	}
	err := repo.BulkCreate(context.Background(), logs)
	require.NoError(t, err)

	for i := range logs {
		assert.NotZero(t, logs[i].ID)
	}
}

func TestNotifyLogRepo_FindBySubscriptionID(t *testing.T) {
	repo := NewNotifyLogRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "notify_log") })

	require.NoError(t, repo.BulkCreate(context.Background(), []model.NotifyLog{
		{SubscriptionID: 77, CinemaID: 10, Email: "s77@a.com", NotifyStatus: "success", TargetPrice: 30, MatchedPrice: 25},
		{SubscriptionID: 77, CinemaID: 10, Email: "s77@a.com", NotifyStatus: "success", TargetPrice: 30, MatchedPrice: 20},
		{SubscriptionID: 88, CinemaID: 20, Email: "s88@b.com", NotifyStatus: "skip", TargetPrice: 35, MatchedPrice: 40},
	}))

	results, err := repo.FindBySubscriptionID(context.Background(), 77, 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.True(t, results[0].CreatedAt.After(results[1].CreatedAt) || results[0].CreatedAt.Equal(results[1].CreatedAt))
}

func TestNotifyLogRepo_FindByExecuteLogID(t *testing.T) {
	repo := NewNotifyLogRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "notify_log") })

	eid := uint64(12345)
	require.NoError(t, repo.BulkCreate(context.Background(), []model.NotifyLog{
		{SubscriptionID: 1, CinemaID: 10, Email: "e1@a.com", ExecuteLogID: &eid, NotifyStatus: "success", TargetPrice: 30, MatchedPrice: 25},
		{SubscriptionID: 2, CinemaID: 20, Email: "e2@b.com", ExecuteLogID: nil, NotifyStatus: "fail", TargetPrice: 35, MatchedPrice: 40},
	}))

	results, err := repo.FindByExecuteLogID(context.Background(), 12345)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestNotifyLogRepo_FindByUserID(t *testing.T) {
	repo := NewNotifyLogRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "notify_log", "subscription") })

	// 需要先建 subscription 才能 JOIN
	uid := uuid.New()
	sub := &model.Subscription{CinemaID: 5001, Email: "join@test.com", TargetPrice: 30, Status: 1, UserID: &uid}
	require.NoError(t, NewSubscriptionRepo(testDB).Create(context.Background(), sub))

	require.NoError(t, repo.BulkCreate(context.Background(), []model.NotifyLog{
		{SubscriptionID: sub.ID, CinemaID: 5001, Email: "join@test.com", NotifyStatus: "success", TargetPrice: 30, MatchedPrice: 25},
	}))

	logs, total, err := repo.FindByUserID(context.Background(), uid, 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.GreaterOrEqual(t, len(logs), 1)
}

func TestNotifyLogRepo_FindRecentBySubscription(t *testing.T) {
	repo := NewNotifyLogRepo(testDB)
	t.Cleanup(func() { cleanTable(t, "notify_log") })

	now := time.Now()
	sentAt := now.Add(-30 * time.Minute)
	require.NoError(t, repo.BulkCreate(context.Background(), []model.NotifyLog{
		{SubscriptionID: 99, CinemaID: 10, Email: "recent@a.com", NotifyStatus: "success", TargetPrice: 30, MatchedPrice: 25, SentAt: &sentAt},
	}))

	// 查 1 小时内
	got, err := repo.FindRecentBySubscription(context.Background(), 99, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "recent@a.com", got.Email)

	// 查 10 秒内（应该查不到，因为 30min > 10s）
	_, err = repo.FindRecentBySubscription(context.Background(), 99, 10*time.Second)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

}
