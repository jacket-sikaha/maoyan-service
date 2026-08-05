package scheduler

import (
	"context"
	"log/slog"

	"maoyan-service/backend/internal/service"

	"github.com/robfig/cron/v3"
)

// Scheduler cron 调度器 — 每隔 intervalMin 分钟自动爬取所有活跃订阅的票价数据
// 使用 robfig/cron（秒级精度），表达式 "0 */N * * * *"
type Scheduler struct {
	cron *cron.Cron
	svc  *service.DataService
}

func New(svc *service.DataService) *Scheduler {
	return &Scheduler{
		cron: cron.New(cron.WithSeconds()),
		svc:  svc,
	}
}

// Start 启动调度器：每30分钟拉取一次所有订阅的票价数据
func (s *Scheduler) Start(intervalMin int) error {
	if intervalMin <= 0 {
		intervalMin = 30
	}

	// cron 表达式：每 N 分钟执行（带秒级随机抖动）
	expr := "0 */" + itoa(intervalMin) + " * * * *"
	slog.Info("scheduler starting", "interval", expr)

	_, err := s.cron.AddFunc(expr, func() {
		slog.Info("scheduler: start fetching subscription data")
		err := s.svc.FetchAllSubscriptionData(context.Background())
		if err != nil {
			slog.Error("scheduler: fetch failed", "error", err)
		} else {
			slog.Info("scheduler: fetch completed")
		}
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	slog.Info("scheduler started")
	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	slog.Info("scheduler stopped")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
