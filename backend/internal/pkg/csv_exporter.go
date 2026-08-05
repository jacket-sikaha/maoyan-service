package pkg

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"

	"maoyan-service/backend/internal/model"
)

// ExportShowsInfoToCSV 将 ShowInfo 列表写入 CSV（带 BOM）
func ExportShowsInfoToCSV(shows []model.ShowInfo, writer io.Writer) error {
	f, ok := writer.(*os.File)
	if ok {
		f.Write([]byte{0xEF, 0xBB, 0xBF})
	}
	w := csv.NewWriter(writer)
	_ = w.Write([]string{"距离(km)", "影院名称", "影院地址", "日期", "开场时间", "散场时间", "影厅", "版本", "票价(元)", "影城卡价(元)", "原价(元)", "优惠价(元)"})
	for _, s := range shows {
		_ = w.Write([]string{
			fmt.Sprintf("%.2f", s.DistanceKm),
			s.CinemaName,
			s.CinemaAddress,
			s.ShowDate,
			s.ShowTime,
			s.EndTime,
			s.HallName,
			s.Lang,
			fmt.Sprintf("%.1f", s.Price),
			fmt.Sprintf("%.1f", s.VIPPrice),
			fmt.Sprintf("%.1f", s.BasePrice),
			fmt.Sprintf("%.1f", s.DiscountPrice),
		})
	}
	w.Flush()
	return w.Error()
}

// ExportSnapshotsToCSV 将票价快照导出为 CSV
func ExportSnapshotsToCSV(snapshots []model.PriceSnapshot, writer io.Writer) error {
	f, ok := writer.(*os.File)
	if ok {
		f.Write([]byte{0xEF, 0xBB, 0xBF})
	}
	w := csv.NewWriter(writer)
	_ = w.Write([]string{
		"cinema_id", "fetched_at", "total_movies", "total_showtimes",
		"movie_stats_json", "parse_status",
	})

	for _, snap := range snapshots {
		row := []string{
			fmt.Sprintf("%d", snap.CinemaID),
			snap.FetchedAt.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("%d", snap.TotalMovies),
			fmt.Sprintf("%d", snap.TotalShowtimes),
			snap.MovieStatsJSON,
			snap.ParseStatus,
		}
		_ = w.Write(row)
	}
	w.Flush()
	return w.Error()
}
