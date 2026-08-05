package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

const (
	MobileBaseURL   = "https://m.maoyan.com/ajax"
	MobileCitiesAPI = "https://m.maoyan.com/dianying"

	PCBaseURL = "https://maoyan.com/ajax"

	DefaultDelayMin = 1.0
	DefaultDelayMax = 2.0
)

var mobileHeaders = map[string]string{
	"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
	"Referer":    "https://m.maoyan.com/",
	"Accept":     "application/json, text/plain, */*",
}

var pcHeaders = map[string]string{
	"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Referer":    "https://maoyan.com/films",
	"Accept":     "application/json, text/javascript, */*",
}

// MaoyanCrawler 猫眼数据爬虫
type MaoyanCrawler struct {
	mobileClient *http.Client
	pcClient     *http.Client
	delayMin     float64
	delayMax     float64
}

func NewMaoyanCrawler(delayMin, delayMax float64) *MaoyanCrawler {
	if delayMin <= 0 {
		delayMin = DefaultDelayMin
	}
	if delayMax <= 0 {
		delayMax = DefaultDelayMax
	}
	return &MaoyanCrawler{
		mobileClient: &http.Client{Timeout: 10 * time.Second},
		pcClient:     &http.Client{Timeout: 10 * time.Second},
		delayMin:     delayMin,
		delayMax:     delayMax,
	}
}

// RandomDelay 随机延迟（导出给 service 层使用）
func (c *MaoyanCrawler) RandomDelay() {
	ms := int((c.delayMin + rand.Float64()*(c.delayMax-c.delayMin)) * 1000)
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// randomDelay 内部随机延迟
func (c *MaoyanCrawler) randomDelay() {
	c.RandomDelay()
}

// request 通用请求（3次重试）
func (c *MaoyanCrawler) request(client *http.Client, apiName, url string, params map[string]string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 设置 headers
	var headers map[string]string = mobileHeaders
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// 设置 query params
	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			slog.Warn("maoyan http fail", "api", apiName, "attempt", attempt+1, "error", err)
			time.Sleep(time.Duration(1+attempt) * time.Second)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		elapsed := time.Since(start).Seconds()
		slog.Info("maoyan http ok", "api", apiName, "status", resp.StatusCode, "elapsed", fmt.Sprintf("%.2fs", elapsed), "size", len(body))

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = err
			continue
		}
		return result, nil
	}

	return nil, fmt.Errorf("request failed after 3 attempts: %w", lastErr)
}

// ------------------ 接口：电影 ------------------

// MovieItem 热映/搜索电影项 — 字段匹配 m.maoyan.com/ajax/movieOnInfoList 返回的 movieList item
type MovieItem struct {
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

// GetHotMovies 获取热映电影
func (c *MaoyanCrawler) GetHotMovies(cityID int) ([]MovieItem, error) {
	url := MobileBaseURL + "/movieOnInfoList"
	params := map[string]string{"ci": fmt.Sprintf("%d", cityID), "token": ""}

	data, err := c.request(c.mobileClient, "movieOnInfoList", url, params)
	if err != nil {
		return nil, err
	}

	movieList, ok := data["movieList"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("movieList not found in response")
	}

	var movies []MovieItem
	for _, m := range movieList {
		mi := m.(map[string]interface{})
		img := getString(mi, "img")
		if img != "" && !strings.HasPrefix(img, "http") {
			img = "https://" + strings.TrimPrefix(img, "//")
		}
		// 将小图替换为大图（缩放参数）
		if img != "" {
			img = strings.Replace(img, "/quality/80", "/thumbnail/500x700%3E", 1)
		}
		movies = append(movies, MovieItem{
			MovieID:        int(getFloat(mi, "id")),
			Name:           getString(mi, "nm"),
			Img:            img,
			Score:          getFloat(mi, "sc"),
			Version:        getString(mi, "version"),
			Star:           getString(mi, "star"),
			ReleaseDate:    getString(mi, "rt"),
			ShowInfo:       getString(mi, "showInfo"),
			ShowState:      int(getFloat(mi, "showst")),
			Wish:           int(getFloat(mi, "wish")),
			GlobalReleased: getBool(mi, "globalReleased"),
			ComingTitle:    getString(mi, "comingTitle"),
		})
	}
	return movies, nil
}

// SearchMovies 搜索电影
func (c *MaoyanCrawler) SearchMovies(keyword string) ([]MovieItem, error) {
	url := PCBaseURL + "/suggest"
	params := map[string]string{"kw": keyword}

	data, err := c.request(c.pcClient, "suggest", url, params)
	if err != nil {
		return nil, err
	}

	moviesData, ok := data["movies"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("movies not found in response")
	}
	list, ok := moviesData["list"].([]interface{})
	if !ok {
		// 兼容老的 JSON 格式
		list, ok = data["movies"].([]interface{})
		if !ok {
			return nil, nil
		}
	}

	var movies []MovieItem
	for _, m := range list {
		mi := m.(map[string]interface{})
		img := getString(mi, "img")
		if img != "" && !strings.HasPrefix(img, "http") {
			img = "https://" + strings.TrimPrefix(img, "//")
		}
		if img != "" {
			img = strings.Replace(img, "/quality/80", "/thumbnail/500x700%3E", 1)
		}
		movies = append(movies, MovieItem{
			MovieID:        int(getFloat(mi, "id")),
			Name:           getString(mi, "nm"),
			Img:            img,
			Score:          getFloat(mi, "sc"),
			Version:        getString(mi, "version"),
			Star:           getString(mi, "star"),
			ReleaseDate:    getString(mi, "rt"),
			ShowInfo:       getString(mi, "showInfo"),
			ShowState:      int(getFloat(mi, "showst")),
			Wish:           int(getFloat(mi, "wish")),
			GlobalReleased: getBool(mi, "globalReleased"),
			ComingTitle:    getString(mi, "comingTitle"),
		})
	}
	return movies, nil
}

// ------------------ 接口：区县 ------------------

// DistrictItem 区县/商圈项
type DistrictItem struct {
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	CinemaCount int            `json:"cinema_count"`
	SubItems    []DistrictItem `json:"sub_items"`
}

// CityItem 城市（来自猫眼 cities.json 接口）
type CityItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Py   string `json:"py"`
}

// GetCities 从猫眼接口获取全国城市列表
// GET https://m.maoyan.com/dianying/cities.json → { cts: [{id, nm, py}] }
func (c *MaoyanCrawler) GetCities() ([]CityItem, error) {
	resp, err := c.request(c.mobileClient, "cities", MobileCitiesAPI+"/cities.json", nil)
	if err != nil {
		return nil, err
	}

	// request() 返回已解析的 map[string]interface{}，重新 marshal 后 unmarshal 到强类型
	rawJSON, _ := json.Marshal(resp)
	var raw struct {
		Cts []struct {
			ID int    `json:"id"`
			Nm string `json:"nm"`
			Py string `json:"py"`
		} `json:"cts"`
	}
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return nil, fmt.Errorf("parse cities.json: %w", err)
	}
	cities := make([]CityItem, len(raw.Cts))
	for i, ct := range raw.Cts {
		cities[i] = CityItem{ID: ct.ID, Name: ct.Nm, Py: ct.Py}
	}
	slog.Info("maoyan cities loaded", "count", len(cities))
	return cities, nil
}

// GetDistricts 获取城市的区县和商圈
func (c *MaoyanCrawler) GetDistricts(cityID int) ([]DistrictItem, error) {
	url := MobileBaseURL + "/filterCinemas"
	params := map[string]string{
		"ci":      fmt.Sprintf("%d", cityID),
		"movieId": "0",
	}

	data, err := c.request(c.mobileClient, "filterCinemas", url, params)
	if err != nil {
		return nil, err
	}

	districtData, ok := data["district"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("district not found in response")
	}

	subItems, _ := districtData["subItems"].([]interface{})
	var districts []DistrictItem
	for _, di := range subItems {
		d := di.(map[string]interface{})
		did := int(getFloat(d, "id"))
		if did <= 0 {
			continue
		}
		item := DistrictItem{
			ID:          did,
			Name:        getString(d, "name"),
			CinemaCount: int(getFloat(d, "count")),
		}
		// 解析商圈子项
		if subList, ok := d["subItems"].([]interface{}); ok {
			for _, si := range subList {
				s := si.(map[string]interface{})
				sid := int(getFloat(s, "id"))
				if sid <= 0 {
					continue
				}
				item.SubItems = append(item.SubItems, DistrictItem{
					ID:          sid,
					Name:        getString(s, "name"),
					CinemaCount: int(getFloat(s, "count")),
				})
			}
		}
		districts = append(districts, item)
	}
	return districts, nil
}

// ------------------ 接口：影院 ------------------

// CinemaRaw 影院原始数据
type CinemaRaw struct {
	CinemaID  int     `json:"cinema_id"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	Distance  int     `json:"distance"`
	SellPrice float64 `json:"sell_price"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
}

// GetCinemaList 获取影院列表（返回全量，最多翻页5次）
func (c *MaoyanCrawler) GetCinemaList(cityID, districtID, areaID int, lat, lng float64) ([]CinemaRaw, int, error) {
	var allCinemas []CinemaRaw
	total := 0
	limit := 50

	for offset := 0; offset < 250; offset += limit {
		url := MobileBaseURL + "/cinemaList"
		params := map[string]string{
			"ci":     fmt.Sprintf("%d", cityID),
			"lat":    fmt.Sprintf("%.6f", lat),
			"lng":    fmt.Sprintf("%.6f", lng),
			"offset": fmt.Sprintf("%d", offset),
			"limit":  fmt.Sprintf("%d", limit),
		}
		if districtID > 0 {
			params["districtId"] = fmt.Sprintf("%d", districtID)
		}
		if areaID > 0 {
			params["areaId"] = fmt.Sprintf("%d", areaID)
		}

		data, err := c.request(c.mobileClient, "cinemaList", url, params)
		if err != nil {
			return allCinemas, total, err
		}

		if t, ok := data["total"].(float64); ok {
			total = int(t)
		}

		cinemaList, ok := data["cinemas"].([]interface{})
		if !ok {
			break
		}

		for _, ci := range cinemaList {
			cm := ci.(map[string]interface{})
			distStr := getString(cm, "distance")
			dist := parseDistance(distStr)

			sp := getFloat(cm, "sellPrice")
			// sellPrice 可能是分单位（如 3300 = 33元），需要规范化
			if sp > 100 {
				sp = sp / 100
			}

			allCinemas = append(allCinemas, CinemaRaw{
				CinemaID:  int(getFloat(cm, "id")),
				Name:      getString(cm, "nm"),
				Address:   getString(cm, "addr"),
				Distance:  dist,
				SellPrice: sp,
				Lat:       getFloat(cm, "lat"),
				Lng:       getFloat(cm, "lng"),
			})
		}

		// 如果返回数量不足 limit 则已取完
		if len(cinemaList) < limit {
			break
		}
		c.randomDelay()
	}

	// 按距离排序
	sortCinemasByDistance(allCinemas)
	slog.Info("get cinema list", "city_id", cityID, "district_id", districtID, "count", len(allCinemas), "total", total)
	return allCinemas, total, nil
}

// ------------------ 接口：排片详情 ------------------

// ShowRaw 排片原始数据
type ShowRaw struct {
	MovieID       int     `json:"movie_id"`
	MovieName     string  `json:"movie_name"`
	CinemaID      int     `json:"cinema_id"`
	ShowDate      string  `json:"show_date"`
	ShowTime      string  `json:"show_time"`
	EndTime       string  `json:"end_time"`
	HallName      string  `json:"hall_name"`
	Lang          string  `json:"lang"`
	SellPrice     float64 `json:"sell_price"`     // sellPr（stonefont解码）— 原价
	VIPPrice      float64 `json:"vip_price"`      // 影城卡/会员价
	BasePrice     float64 `json:"base_price"`     // baseSellPrice（stonefont解码）
	DiscountPrice float64 `json:"discount_price"` // discountSellPrice（stonefont解码）
	CinemaName    string  `json:"cinema_name"`
	CinemaAddress string  `json:"cinema_address"`
}

// GetCinemaShows 获取影院某电影的排片（含自动 stonefont 解码）
func (c *MaoyanCrawler) GetCinemaShows(cinemaID, movieID int) ([]ShowRaw, string, error) {
	url := MobileBaseURL + "/cinemaDetail"
	params := map[string]string{
		"cinemaId": fmt.Sprintf("%d", cinemaID),
		"movieId":  fmt.Sprintf("%d", movieID),
	}

	data, err := c.request(c.mobileClient, "cinemaDetail", url, params)
	if err != nil {
		return nil, "", err
	}

	// 解析 stonefont 映射
	stoneMapping := make(map[int]string)
	if stone, ok := data["stone"].(map[string]interface{}); ok {
		stoneMapping = BuildStonefontMap(stone)
		slog.Info("stonefont decoded", "mappings", stoneMapping)
	}

	// 解析排片
	showData, ok := data["showData"].(map[string]interface{})
	if !ok {
		return nil, "", fmt.Errorf("showData not found")
	}

	movies, ok := showData["movies"].([]interface{})
	if !ok {
		return nil, "", nil
	}

	var shows []ShowRaw
	for _, mi := range movies {
		m := mi.(map[string]interface{})
		if int(getFloat(m, "id")) != movieID {
			continue
		}

		showsList, _ := m["shows"].([]interface{})
		for _, si := range showsList {
			s := si.(map[string]interface{})
			showDate := getString(s, "showDate")

			plist, _ := s["plist"].([]interface{})
			for _, pi := range plist {
				p := pi.(map[string]interface{})
				price := extractPrice(p, stoneMapping)
				vipPrice := extractVIPPrice(p)

				basePrice, discountPrice := extractBaseAndDiscount(p, stoneMapping)
				shows = append(shows, ShowRaw{
					MovieID:       movieID,
					CinemaID:      cinemaID,
					ShowDate:      showDate,
					ShowTime:      getString(p, "tm"),
					EndTime:       getStringDefault(p, "end", getStringDefault(p, "endAt", "")),
					HallName:      getString(p, "th"),
					Lang:          getStringDefault(p, "lang", getStringDefault(p, "tp", "")),
					SellPrice:     price,
					VIPPrice:      vipPrice,
					BasePrice:     basePrice,
					DiscountPrice: discountPrice,
				})
			}
		}
		break
	}

	slog.Info("get cinema shows", "cinema_id", cinemaID, "movie_id", movieID, "count", len(shows))
	return shows, fmt.Sprintf("%v", stoneMapping), nil
}

// GetCinemaAllShows 获取影院所有电影的排片（按 cinemaID 聚合，供调度批量去重）
func (c *MaoyanCrawler) GetCinemaAllShows(cinemaID int) (map[int][]ShowRaw, error) {
	url := MobileBaseURL + "/cinemaDetail"
	params := map[string]string{"cinemaId": fmt.Sprintf("%d", cinemaID)}

	data, err := c.request(c.mobileClient, "cinemaDetail", url, params)
	if err != nil {
		return nil, err
	}

	stoneMapping := make(map[int]string)
	if stone, ok := data["stone"].(map[string]interface{}); ok {
		stoneMapping = BuildStonefontMap(stone)
	}

	showData, ok := data["showData"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("showData not found")
	}

	movies, ok := showData["movies"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("movies not found")
	}

	result := make(map[int][]ShowRaw)
	for _, mi := range movies {
		m := mi.(map[string]interface{})
		movieID := int(getFloat(m, "id"))
		movieName := getString(m, "nm")

		showsList, _ := m["shows"].([]interface{})
		for _, si := range showsList {
			s := si.(map[string]interface{})
			showDate := getString(s, "showDate")
			plist, _ := s["plist"].([]interface{})
			for _, pi := range plist {
				p := pi.(map[string]interface{})
				basePrice, discountPrice := extractBaseAndDiscount(p, stoneMapping)
				result[movieID] = append(result[movieID], ShowRaw{
					MovieID:       movieID,
					MovieName:     movieName,
					CinemaID:      cinemaID,
					ShowDate:      showDate,
					ShowTime:      getString(p, "tm"),
					EndTime:       getStringDefault(p, "end", getStringDefault(p, "endAt", "")),
					HallName:      getString(p, "th"),
					Lang:          getStringDefault(p, "lang", getStringDefault(p, "tp", "")),
					SellPrice:     extractPrice(p, stoneMapping),
					VIPPrice:      extractVIPPrice(p),
					BasePrice:     basePrice,
					DiscountPrice: discountPrice,
				})
			}
		}
	}

	slog.Info("get cinema all shows", "cinema_id", cinemaID, "movies", len(result))
	return result, nil
}

// ------------------ 辅助函数 ------------------

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}

func getStringDefault(m map[string]interface{}, key, def string) string {
	s := getString(m, key)
	if s == "" {
		return def
	}
	return s
}

func getBool(m map[string]interface{}, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func getFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}

// parseDistance 解析距离字符串（"1.5km" → 1500 米）
func parseDistance(s string) int {
	s = cleanString(s)
	var value float64
	var unit string
	fmt.Sscanf(s, "%f%s", &value, &unit)

	switch unit {
	case "km", "KM":
		return int(value * 1000)
	case "m", "M":
		return int(value)
	default:
		return 99999
	}
}

func cleanString(s string) string {
	// 去除不可见字符
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= 32 && s[i] <= 126 {
			result = append(result, s[i])
		}
	}
	return string(result)
}

func sortCinemasByDistance(cinemas []CinemaRaw) {
	// 简单冒泡排序（数据量小）
	for i := 0; i < len(cinemas); i++ {
		for j := i + 1; j < len(cinemas); j++ {
			if cinemas[i].Distance > cinemas[j].Distance {
				cinemas[i], cinemas[j] = cinemas[j], cinemas[i]
			}
		}
	}
}

// extractPrice stonefont解码提取票价（sellPr → 原价，优先）
// 数据源：cinemaDetail 接口 showData.movies[].shows[].plist[] 中每个场次对象。
// 价格字段说明（参考 Python 原版 _extract_price）：
//   sellPr         → 密文（&#xE...;），需用 stonefont 映射表解码 → 原价（元）
//   baseSellPrice  → 密文，stonefont 解码 → 基准原价（元）
//   discountSellPrice → 密文，stonefont 解码 → 折扣原价（元）
//   sellPrice      → 数字（分单位），sellPr 解码失败时的兜底
//   vipPrice       → 数字（元），影城卡/会员价（见 extractVIPPrice）
func extractPrice(show map[string]interface{}, mapping map[int]string) float64 {
	// 优先 stonefont 解码 sellPr（原价）
	if len(mapping) > 0 {
		raw := getString(show, "sellPr")
		if raw != "" {
			price := stonefontDecode(raw, mapping)
			if price > 0 {
				slog.Debug("price source: sellPr(stonefont)", "raw", raw, "decoded", price)
				return price
			}
			slog.Warn("price source: sellPr decoded to 0, fallback", "raw", raw, "mapping_size", len(mapping))
		}
	}

	// 兜底 sellPrice（分单位）
	sp := getFloat(show, "sellPrice")
	if sp > 0 {
		if sp > 100 {
			sp = sp / 100
		}
		slog.Debug("price source: sellPrice(fallback)", "decoded", sp)
		return sp
	}
	return 0
}

// extractBaseAndDiscount 提取 baseSellPrice 和 discountSellPrice（stonefont解码）
// 数据源同上：plist[] 中场次对象的 baseSellPrice / discountSellPrice 字段。
// 这两个字段同样是 stonefont 密文（&#xE...;），必须用同一个 mapping 解码。
// 注意：如果 stonefont 未匹配上对应 PUA 码点，会得到 0（与 Python 行为一致）。
func extractBaseAndDiscount(show map[string]interface{}, mapping map[int]string) (float64, float64) {
	var basePrice, discountPrice float64
	if len(mapping) > 0 {
		if raw := getString(show, "baseSellPrice"); raw != "" {
			basePrice = stonefontDecode(raw, mapping)
			if basePrice == 0 {
				slog.Warn("baseSellPrice decoded to 0", "raw", raw)
			}
		}
		if raw := getString(show, "discountSellPrice"); raw != "" {
			discountPrice = stonefontDecode(raw, mapping)
			if discountPrice == 0 {
				slog.Warn("discountSellPrice decoded to 0", "raw", raw)
			}
		}
	}
	return basePrice, discountPrice
}

// extractVIPPrice 提取 vipPrice 影城卡价（元）
// 数据源：plist[] 中场次对象的 vipPrice 字段。
// 与 sellPr 不同，vipPrice 是普通数字（元）不是密文，不需要 stonefont 解码。
// 与 sellPrice 不同，vipPrice 单位是元而不是分（参考 Python _extract_vip_price）。
// 注意：vipPrice 在响应中有时是数字（0.0）有时是字符串，要兼容处理。
func extractVIPPrice(show map[string]interface{}) float64 {
	vip := getFloat(show, "vipPrice")
	if vip > 0 {
		slog.Debug("vipPrice extracted", "value", vip)
		return vip
	}
	return 0
}
