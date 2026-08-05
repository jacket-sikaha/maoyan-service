// api/endpoints.ts — 所有后端 API 端点的 TypeScript 封装
// 已适配后端 PDF 8 表重构：影院级订阅、单邮箱、status/notify_enabled 双字段
import api from './client'

interface AuthResponse { token: string; user: { id: string; email: string } }
type ApiResponse<T = any> = { code: number; msg: string; data: T }

// ==================== 认证 ====================
export const register = (data: { email: string; password: string }) =>
  api.post<any, ApiResponse<AuthResponse>>('/auth/register', data)
export const login = (data: { email: string; password: string }) =>
  api.post<any, ApiResponse<AuthResponse>>('/auth/login', data)

// ==================== 城市 & 区县 ====================
export const getCities = () => api.get<any, ApiResponse<{id:number;name:string;py:string}[]>>('/cities')
export const getDistricts = (cityId: number) =>
  api.get<any, ApiResponse<{id:number;city_id:number;parent_id:number;name:string;cinema_count:number;sub_areas?:any[]}[]>>('/districts', { params: { city_id: cityId } })

// ==================== 电影 ====================
export const getHotMovies = (cityId: number) => api.get<any, ApiResponse<any[]>>('/movies/hot', { params: { city_id: cityId } })
export const searchMovies = (keyword: string) => api.get<any, ApiResponse<any[]>>('/movies/search', { params: { keyword } })

// ==================== 影院搜索 ====================
export const searchCinemas = (keyword: string) => api.get<any, ApiResponse<any[]>>('/cinemas/search', { params: { keyword } })
export const getCinemasByCity = (cityId: number) =>
  api.get<any, ApiResponse<{cinema_id:number;name:string;address:string;distance:number}[]>>('/cinemas/by-city', { params: { city_id: cityId } })

// ==================== 排片票价查询 ====================
export const queryShows = (params: { city_id: number; district_id?: number; area_id?: number; movie_id: number; lat?: number; lng?: number; max?: number }) =>
  api.get<any, ApiResponse<{cinema_count:number;show_count:number;shows:any[]}>>('/shows', { params })

// 票价 CSV 导出（需登录，Blob 响应）
export const exportShowsCSV = (params: { city_id: number; movie_id: number; district_id?: number; area_id?: number; max?: number }) =>
  api.get('/shows/export', { params, responseType: 'blob' })

// ==================== 订阅管理（影院级订阅） ====================

// 创建订阅：cinema_id + email + target_price
export const createSubscription = (data: { cinema_id: number; cinema_name?: string; maoyan_city_id?: number; movie_id?: string; movie_name?: string; email: string; target_price: number; remark?: string }) =>
  api.post<any, ApiResponse<any>>('/subscriptions', data)

// 订阅列表：返回 SubscriptionFullInfo[]
export const listSubscriptions = () => api.get<any, ApiResponse<any[]>>('/subscriptions')

// 订阅详情：返回 SubscriptionDetail
export const getSubscriptionDetail = (id: string, page = 1, pageSize = 20) =>
  api.get<any, ApiResponse<any>>(`/subscriptions/${id}`, { params: { page, page_size: pageSize } })

// 切换订阅状态：status 0=停用 1=启用
export const toggleSubscription = (id: string, status: number) =>
  api.patch<any, ApiResponse<any>>(`/subscriptions/${id}/toggle`, { status })

// 刷新订阅影院当前行情
export const refreshSubscription = (id: string) =>
  api.post<any, ApiResponse<any[]>>(`/subscriptions/${id}/refresh`)

// 导出订阅历史 CSV
export const exportSubscriptionCSV = (id: string) =>
  api.get(`/subscriptions/${id}/export`, { responseType: 'blob' })

// 更新订阅：部分字段更新（仅 target_price/remark/status）
export const updateSubscription = (id: string, data: { target_price?: number; status?: number; remark?: string }) =>
  api.put<any, ApiResponse<any>>(`/subscriptions/${id}`, data)

// 删除订阅
export const deleteSubscription = (id: string) =>
  api.delete<any, ApiResponse<any>>(`/subscriptions/${id}`)

// ==================== 订阅通知日志 ====================
export const getSubscriptionLogs = (params: { page?: number; page_size?: number; start_date?: string; end_date?: string; status?: string }) =>
  api.get<any, ApiResponse<{ items: any[]; total: number; page: number; page_size: number }>>('/subscriptions/logs', { params })

// ==================== 用户已订阅的影院 ====================
export const getUserSubscribedCinemas = () =>
  api.get<any, ApiResponse<any[]>>('/subscriptions/cinemas')

// ==================== 用户订阅的影院+电影组合（票价变化页筛选用） ====================
export interface CinemaMovieItem {
  cinema_id: number
  cinema_name: string
  cinema_address?: string
  maoyan_city_id?: number
  movie_id: string
  movie_name: string
}
export const getSubscribedCinemaMovies = () =>
  api.get<any, ApiResponse<CinemaMovieItem[]>>('/subscriptions/cinema-movies')

// ==================== 票价变化（折线图数据） ====================
// 返回 { trend: PricePoint[] }
// startDate/endDate 格式 YYYY-MM-DD，默认后端返回过去7天
export const getPriceChanges = (cinemaId: number, movieId: string, startDate?: string, endDate?: string) =>
  api.get<any, ApiResponse<{ trend: { time: string; price_min: number; price_avg: number; price_max: number }[] }>>('/price-changes', {
    params: { cinema_id: cinemaId, movie_id: movieId, start_date: startDate, end_date: endDate },
  })

// ==================== 手动采集（调试用） ====================
export const manualCrawl = (cinemaId: number) =>
  api.post<any, ApiResponse<{ message: string; cinema_id: number }>>(`/admin/crawl/${cinemaId}`)

// ==================== 采集记录仪表盘 ====================
export interface CrawlRecordItem {
  snapshot_id: number
  fetched_at: string
  total_movies: number
  total_showtimes: number
  parse_status: string
}

export interface MovieCrawlDetail {
  movie_id: string
  movie_name: string
  min_price: number
  avg_price: number
  max_price: number
  showtimes: number
}

export interface CrawlRecordsDashboard {
  cinema_name: string
  cinema_id: number
  total_snapshots: number
  total_showtimes: number
  total_movies: number
  global_min_price: number
  global_avg_price: number
  global_max_price: number
  records: CrawlRecordItem[]
  movies: MovieCrawlDetail[]
}

export const getCrawlRecords = (subId: string, startDate?: string, endDate?: string) =>
  api.get<any, ApiResponse<CrawlRecordsDashboard>>(`/subscriptions/${subId}/crawl-records`, {
    params: { start_date: startDate, end_date: endDate },
  })

export const getSnapshotMovieShows = (subId: string, snapshotId: number, movieId: string) =>
  api.get<any, ApiResponse<any[]>>(`/subscriptions/${subId}/snapshots/${snapshotId}/shows`, {
    params: { movie_id: movieId },
  })
