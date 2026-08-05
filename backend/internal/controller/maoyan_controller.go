package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"maoyan-service/backend/internal/model"
	"maoyan-service/backend/internal/pkg"
	"maoyan-service/backend/internal/service"
	"maoyan-service/backend/tools"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Msg: "success", Data: data})
}
func Error(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Response{Code: code, Msg: msg})
}

// parseTimeRange 解析 start_date/end_date 查询参数，默认过去7天
func parseTimeRange(c *gin.Context) (startTime, endTime time.Time) {
	endDateStr := c.DefaultQuery("end_date", "")
	if endDateStr != "" {
		endTime, _ = time.ParseInLocation("2006-01-02", endDateStr, time.Local)
		if !endTime.IsZero() {
			endTime = endTime.Add(24*time.Hour - time.Second) // 包含当天
		}
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}
	startDateStr := c.DefaultQuery("start_date", "")
	if startDateStr != "" {
		startTime, _ = time.ParseInLocation("2006-01-02", startDateStr, time.Local)
	} else {
		startTime = endTime.AddDate(0, 0, -7) // 默认过去7天
	}
	return
}

// MaoyanController HTTP 控制器
type MaoyanController struct {
	svc  *service.DataService
	auth *service.AuthService
}

func NewMaoyanController(svc *service.DataService, auth *service.AuthService) *MaoyanController {
	return &MaoyanController{svc: svc, auth: auth}
}

// ==================== 认证 ====================

func (ctl *MaoyanController) Register(c *gin.Context) {
	var req model.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, 400, "参数错误: "+err.Error())
		return
	}
	resp, err := ctl.auth.Register(c.Request.Context(), req)
	if err != nil {
		Error(c, 400, err.Error())
		return
	}
	Success(c, resp)
}

func (ctl *MaoyanController) Login(c *gin.Context) {
	var req model.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, 400, "参数错误: "+err.Error())
		return
	}
	resp, err := ctl.auth.Login(c.Request.Context(), req)
	if err != nil {
		Error(c, 401, err.Error())
		return
	}
	Success(c, resp)
}

// ==================== 城市 ====================

func (ctl *MaoyanController) GetCities(c *gin.Context) {
	cities, err := ctl.svc.GetCities(c.Request.Context())
	if err != nil {
		Error(c, 500, "获取城市列表失败")
		return
	}
	Success(c, cities)
}

func (ctl *MaoyanController) GetDistricts(c *gin.Context) {
	cityID, err := strconv.Atoi(c.Query("city_id"))
	if err != nil || cityID <= 0 {
		Error(c, 400, "参数 city_id 无效")
		return
	}
	districts, err := ctl.svc.GetDistricts(c.Request.Context(), cityID)
	if err != nil {
		Error(c, 500, "获取区县失败: "+err.Error())
		return
	}
	Success(c, districts)
}

// ==================== 电影 ====================

func (ctl *MaoyanController) GetHotMovies(c *gin.Context) {
	cityID := 1
	if v := c.Query("city_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil && id > 0 {
			cityID = id
		}
	}
	movies, err := ctl.svc.GetHotMovies(c.Request.Context(), cityID)
	if err != nil {
		Error(c, 500, "获取热映电影失败")
		return
	}
	Success(c, movies)
}

func (ctl *MaoyanController) SearchMovies(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword == "" {
		Error(c, 400, "参数 keyword 不能为空")
		return
	}
	movies, err := ctl.svc.SearchMovies(c.Request.Context(), keyword)
	if err != nil {
		Error(c, 500, "搜索电影失败")
		return
	}
	Success(c, movies)
}

// ==================== 影院 ====================

// ==================== 排片票价查询 ====================

func (ctl *MaoyanController) QueryShows(c *gin.Context) {
	cityID, err := strconv.Atoi(c.DefaultQuery("city_id", "0"))
	if err != nil || cityID <= 0 {
		Error(c, 400, "参数 city_id 无效")
		return
	}
	districtID, _ := strconv.Atoi(c.DefaultQuery("district_id", "-1"))
	areaID, _ := strconv.Atoi(c.DefaultQuery("area_id", "-1"))
	movieID, _ := strconv.Atoi(c.DefaultQuery("movie_id", "0"))
	if movieID <= 0 {
		Error(c, 400, "参数 movie_id 无效")
		return
	}
	lat, _ := strconv.ParseFloat(c.DefaultQuery("lat", "0"), 64)
	lng, _ := strconv.ParseFloat(c.DefaultQuery("lng", "0"), 64)
	maxCinemas, _ := strconv.Atoi(c.DefaultQuery("max", "20"))

	shows, err := ctl.svc.GetCinemaShows(c.Request.Context(), cityID, districtID, areaID, movieID, lat, lng, maxCinemas)
	if err != nil {
		Error(c, 500, "查询排片失败: "+err.Error())
		return
	}
	Success(c, gin.H{"cinema_count": len(groupByCinema(shows)), "show_count": len(shows), "shows": shows})
}

func (ctl *MaoyanController) ExportShowsCSV(c *gin.Context) {
	cityID, _ := strconv.Atoi(c.DefaultQuery("city_id", "0"))
	districtID, _ := strconv.Atoi(c.DefaultQuery("district_id", "-1"))
	areaID, _ := strconv.Atoi(c.DefaultQuery("area_id", "-1"))
	movieID, _ := strconv.Atoi(c.DefaultQuery("movie_id", "0"))
	lat, _ := strconv.ParseFloat(c.DefaultQuery("lat", "0"), 64)
	lng, _ := strconv.ParseFloat(c.DefaultQuery("lng", "0"), 64)
	maxCinemas, _ := strconv.Atoi(c.DefaultQuery("max", "50"))
	if movieID <= 0 || cityID <= 0 {
		Error(c, 400, "参数 movie_id 和 city_id 不能为空")
		return
	}
	shows, err := ctl.svc.GetCinemaShows(c.Request.Context(), cityID, districtID, areaID, movieID, lat, lng, maxCinemas)
	if err != nil {
		Error(c, 500, "查询排片失败")
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=maoyan_export.csv")
	if err := pkg.ExportShowsInfoToCSV(shows, c.Writer); err != nil {
		Error(c, 500, "导出CSV失败")
	}
}

// ==================== 订阅管理 ====================

func (ctl *MaoyanController) CreateSubscription(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}

	var req model.SubscriptionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, 400, "参数错误: "+err.Error())
		return
	}

	resp, err := ctl.svc.CreateSubscription(c.Request.Context(), uid, req)
	if err != nil {
		Error(c, 500, "创建订阅失败: "+err.Error())
		return
	}
	Success(c, resp)
}

func (ctl *MaoyanController) ToggleSubscription(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}

	var req model.ToggleSubscriptionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, 400, "参数错误")
		return
	}

	if err := ctl.svc.ToggleSubscription(c.Request.Context(), uid, c.Param("id"), req.Status); err != nil {
		Error(c, 500, "操作失败: "+err.Error())
		return
	}
	Success(c, gin.H{"status": req.Status})
}

func (ctl *MaoyanController) ListSubscriptions(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}
	subs, err := ctl.svc.ListSubscriptions(c.Request.Context(), uid)
	tools.PrettyPrint(err)

	if err != nil {
		Error(c, 500, "查询订阅失败")
		return
	}
	Success(c, subs)
}

func (ctl *MaoyanController) GetSubscriptionDetail(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	detail, err := ctl.svc.GetSubscriptionDetail(c.Request.Context(), uid, c.Param("id"), page, pageSize)
	if err != nil {
		Error(c, 400, err.Error())
		return
	}
	Success(c, detail)
}

// GetCrawlRecords 获取订阅影院的采集记录（仪表盘）
func (ctl *MaoyanController) GetCrawlRecords(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}

	startTime, endTime := parseTimeRange(c)

	dash, err := ctl.svc.GetCrawlRecords(c.Request.Context(), uid, c.Param("id"), startTime, endTime)
	if err != nil {
		Error(c, 400, err.Error())
		return
	}
	Success(c, dash)
}

// GetSnapshotMovieShows 获取某次采集快照中某部电影的所有场次明细
func (ctl *MaoyanController) GetSnapshotMovieShows(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}
	subID := c.Param("id")
	snapshotIDStr := c.Param("snapshot_id")
	snapshotID, err := strconv.ParseUint(snapshotIDStr, 10, 64)
	if err != nil {
		Error(c, 400, "snapshot_id 无效")
		return
	}
	movieID := c.Query("movie_id")
	if movieID == "" {
		Error(c, 400, "参数 movie_id 无效")
		return
	}

	shows, err := ctl.svc.GetSnapshotMovieShows(c.Request.Context(), uid, subID, snapshotID, movieID)
	if err != nil {
		Error(c, 400, err.Error())
		return
	}
	Success(c, shows)
}

func (ctl *MaoyanController) QuerySubscriptionPrices(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}

	prices, err := ctl.svc.GetSubscriptionPrices(c.Request.Context(), uid, c.Param("id"))
	if err != nil {
		Error(c, 400, err.Error())
		return
	}
	Success(c, prices)
}

func (ctl *MaoyanController) ExportSubscriptionCSV(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}

	items, err := ctl.svc.ExportSubscriptionHistory(c.Request.Context(), uid, c.Param("id"))
	if err != nil {
		Error(c, 400, err.Error())
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=subscription_export.csv")
	if err := pkg.ExportSnapshotsToCSV(items, c.Writer); err != nil {
		Error(c, 500, "导出失败")
	}
}

func (ctl *MaoyanController) UpdateSubscription(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}

	var req model.SubscriptionUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, 400, "参数错误")
		return
	}
	if err := ctl.svc.UpdateSubscription(c.Request.Context(), uid, c.Param("id"), req); err != nil {
		Error(c, 400, err.Error())
		return
	}
	Success(c, gin.H{"message": "更新成功"})
}

func (ctl *MaoyanController) DeleteSubscription(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}
	if err := ctl.svc.DeleteSubscription(c.Request.Context(), uid, c.Param("id")); err != nil {
		Error(c, 400, err.Error())
		return
	}
	Success(c, gin.H{"message": "已删除"})
}

func (ctl *MaoyanController) GetSubscriptionLogs(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status") // 可选状态筛选：success / fail / pending / skip

	var startDate, endDate *time.Time
	if s := c.Query("start_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			startDate = &t
		}
	}
	if s := c.Query("end_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			endDate = &t
		}
	}

	logs, total, err := ctl.svc.GetSubscriptionLogs(c.Request.Context(), uid, startDate, endDate, status, page, pageSize)
	if err != nil {
		Error(c, 500, "查询日志失败")
		return
	}
	Success(c, gin.H{"items": logs, "total": total, "page": page, "page_size": pageSize})
}

func (ctl *MaoyanController) GetUserSubscribedCinemas(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}
	cinemas, err := ctl.svc.GetUserSubscribedCinemas(c.Request.Context(), uid)
	if err != nil {
		Error(c, 500, "查询影院失败")
		return
	}
	Success(c, cinemas)
}

// GetSubscribedCinemaMovies 获取用户订阅的影院+电影组合列表（票价变化页筛选用）
func (ctl *MaoyanController) GetSubscribedCinemaMovies(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}
	items, err := ctl.svc.GetSubscribedCinemaMovies(c.Request.Context(), uid)
	if err != nil {
		Error(c, 500, "查询订阅电影失败")
		return
	}
	Success(c, items)
}

func (ctl *MaoyanController) GetPriceChanges(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		Error(c, 401, "未登录")
		return
	}
	cinemaID, _ := strconv.Atoi(c.DefaultQuery("cinema_id", "0"))
	if cinemaID <= 0 {
		Error(c, 400, "参数 cinema_id 无效")
		return
	}
	movieID := c.DefaultQuery("movie_id", "")
	if movieID == "" {
		Error(c, 400, "参数 movie_id 无效")
		return
	}

	// 时间范围筛选，默认过去7天
	var startTime, endTime time.Time
	endDateStr := c.DefaultQuery("end_date", "")
	if endDateStr != "" {
		endTime, _ = time.ParseInLocation("2006-01-02", endDateStr, time.Local)
		if !endTime.IsZero() {
			endTime = endTime.Add(24*time.Hour - time.Second) // 包含当天
		}
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}
	startDateStr := c.DefaultQuery("start_date", "")
	if startDateStr != "" {
		startTime, _ = time.ParseInLocation("2006-01-02", startDateStr, time.Local)
	} else {
		startTime = endTime.AddDate(0, 0, -7) // 默认过去7天
	}

	changes, err := ctl.svc.GetPriceChanges(c.Request.Context(), cinemaID, movieID, startTime, endTime, uid)
	if err != nil {
		Error(c, 500, "查询失败")
		return
	}
	Success(c, changes)
}


// ==================== 运维 ====================

func (ctl *MaoyanController) TriggerFetch(c *gin.Context) {
	if err := ctl.svc.FetchAllSubscriptionData(c.Request.Context()); err != nil {
		Error(c, 500, "数据更新失败: "+err.Error())
		return
	}
	Success(c, gin.H{"message": "数据更新完成"})
}

// ManualCrawl 手动触发指定影院的采集任务（调试用）
func (ctl *MaoyanController) ManualCrawl(c *gin.Context) {
	cinemaIDStr := c.Param("cinema_id")
	cinemaID, err := strconv.ParseUint(cinemaIDStr, 10, 64)
	if err != nil || cinemaID == 0 {
		Error(c, 400, "参数 cinema_id 无效")
		return
	}
	if err := ctl.svc.ManualCrawlByCinemaID(c.Request.Context(), cinemaID); err != nil {
		Error(c, 500, "采集失败: "+err.Error())
		return
	}
	Success(c, gin.H{"message": "采集完成", "cinema_id": cinemaID})
}

// ==================== 辅助 ====================

func groupByCinema(shows []model.ShowInfo) map[string]int {
	result := make(map[string]int)
	for _, s := range shows {
		result[s.CinemaName]++
	}
	return result
}

// 保留 fmt 引用避免 unused
var _ = fmt.Sprintf
