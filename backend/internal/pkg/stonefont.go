// Package pkg — stonefont.go 猫眼自定义字体（woff/ttf）解码引擎
//
// 核心设计：用 Go sfnt 工具链提取 Arial 基准数字轮廓，走和 PUA 字形完全相同的
// segmentsToPoints → normalizeOutline → bidirectionalChamferDistance 管道，
// 消除工具链差异。匈牙利算法做一对一最优匹配。
//
// 解码流程：
//   ① 下载 WOFF → ② WOFF→TTF 转换  → ③ sfnt 提取 PUA 字形 ←→ Arial 基准轮廓
//      (sfnt toolchain)     (zlib解压+重组)     (LoadGlyph→segmentsToPoints→normalize)
//   ④ 匈牙利一对一匹配  → ⑤ 建立映射表  → ⑥ &#xNNNN; entity 解码
package pkg

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"
	"log/slog"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"net/http"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//  缓存
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

var (
	cacheURL string
	cacheMap map[int]string
	cacheMu  sync.Mutex

	// 懒加载 Arial 基准轮廓（同工具链提取，避免 fontTools→sfnt 偏差）
	refContours     [10][]point2f
	refContoursOnce sync.Once
)

// BuildStonefontMap 从 cinemaDetail stone 字段构建码点→数字映射
func BuildStonefontMap(stoneData map[string]interface{}) map[int]string {
	css, ok := stoneData["cssSource"].(string)
	if !ok || css == "" {
		return nil
	}
	re := regexp.MustCompile(`url\("([^"]+\.woff2?)"\)`)
	match := re.FindStringSubmatch(css)
	if len(match) < 2 {
		return nil
	}
	fontURL := match[1]
	if strings.HasPrefix(fontURL, "//") {
		fontURL = "https:" + fontURL
	}

	cacheMu.Lock()
	if cacheURL == fontURL && cacheMap != nil {
		m := cacheMap
		cacheMu.Unlock()
		return m
	}
	cacheMu.Unlock()

	slog.Info("stonefont downloading", "url", fontURL)
	mapping := buildMapping(fontURL)
	if mapping == nil {
		return nil
	}

	slog.Info("stonefont mapping built",
		"url", fontURL, "matched", len(mapping),
		"uniq", countUnique(mapping),
	)

	cacheMu.Lock()
	cacheURL, cacheMap = fontURL, mapping
	cacheMu.Unlock()
	return mapping
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//  核心匹配管线
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func buildMapping(fontURL string) map[int]string {
	resp, err := http.Get(fontURL)
	if err != nil {
		slog.Warn("stonefont download fail", "url", fontURL, "err", err)
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	ttf, err := woffToTTF(body)
	if err != nil || ttf == nil {
		return nil
	}

	sf, err := sfnt.Parse(ttf)
	if err != nil {
		slog.Warn("sfnt parse fail", "err", err)
		return nil
	}

	// 确保 Arial 基准轮廓已加载（同工具链）
	refContoursOnce.Do(loadArialRef)

	// 提取 PUA 字形控制点（segmentsToPoints）
	puaOutlines := extractPUAOutlines(sf)
	if len(puaOutlines) == 0 {
		slog.Warn("no PUA glyphs")
		return nil
	}

	// 归一化所有轮廓
	upem := float64(sf.UnitsPerEm())
	normPUA := make(map[int][]point2f, len(puaOutlines))
	for cp, pts := range puaOutlines {
		normPUA[cp] = normalizeOutline(pts, upem)
	}
	normRefs := refContours // already normalized at load time

	// 匈牙利一对一最优匹配（Chamfer 距离）
	return hungarianChamfer(normPUA, normRefs)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//  PUA 字形提取（segmentsToPoints）
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

type point2f struct{ x, y float64 }

func extractPUAOutlines(sf *sfnt.Font) map[int][]point2f {
	buf := &sfnt.Buffer{}
	upem := float64(sf.UnitsPerEm())
	ppem := fixed.Int26_6(int(upem) << 6)

	result := make(map[int][]point2f)
	for cp := 0xE000; cp <= 0xF8FF; cp++ {
		gidx, err := sf.GlyphIndex(buf, rune(cp))
		if err != nil || gidx == 0 {
			continue
		}
		segs, err := sf.LoadGlyph(buf, gidx, ppem, nil)
		if err != nil || len(segs) == 0 {
			continue
		}
		pts := segmentsToPoints(segs)
		if len(pts) > 0 {
			result[cp] = pts
		}
	}
	slog.Info("stonefont pua outlines", "count", len(result))
	return result
}

// segmentsToPoints 提取所有控制点（不插值，对标 fontTools RecordingPen）
func segmentsToPoints(segs sfnt.Segments) []point2f {
	pts := make([]point2f, 0)
	for _, seg := range segs {
		for _, arg := range seg.Args {
			pts = append(pts, point2f{
				x: float64(arg.X) / 64.0,
				y: float64(arg.Y) / 64.0,
			})
		}
	}
	return pts
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//  轮廓归一化
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func normalizeOutline(pts []point2f, upem float64) []point2f {
	if len(pts) == 0 {
		return nil
	}
	// 居中
	var cx, cy float64
	for _, p := range pts {
		cx += p.x
		cy += p.y
	}
	cx /= float64(len(pts))
	cy /= float64(len(pts))

	// 缩放到 [-0.5, 0.5]
	maxExt := 0.0
	for _, p := range pts {
		dx := math.Abs(p.x/upem - cx/upem)
		dy := math.Abs(p.y/upem - cy/upem)
		if dx > maxExt {
			maxExt = dx
		}
		if dy > maxExt {
			maxExt = dy
		}
	}
	scale := 1.0
	if maxExt > 0 {
		scale = 0.5 / maxExt
	}

	result := make([]point2f, len(pts))
	for i, p := range pts {
		result[i] = point2f{
			x: (p.x/upem - cx/upem) * scale,
			y: (p.y/upem - cy/upem) * scale,
		}
	}
	return result
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//  Chamfer 双向最近邻距离
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func bidirectionalChamferDistance(a, b []point2f) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 1e308
	}
	dAB := chamferOneWay(a, b)
	dBA := chamferOneWay(b, a)
	return (dAB + dBA) / 2.0
}

func chamferOneWay(from, to []point2f) float64 {
	sum := 0.0
	for _, f := range from {
		minDist := 1e308
		for _, t := range to {
			dx := f.x - t.x
			dy := f.y - t.y
			dist := dx*dx + dy*dy
			if dist < minDist {
				minDist = dist
			}
		}
		sum += math.Sqrt(minDist)
	}
	return sum / float64(len(from))
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//  匈牙利算法（一对一最优匹配）
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func hungarianChamfer(pua map[int][]point2f, refs [10][]point2f) map[int]string {
	cpList := make([]int, 0, len(pua))
	for cp := range pua {
		cpList = append(cpList, cp)
	}
	n := len(cpList)
	m := 10

	cost := make([][]float64, n)
	for i := 0; i < n; i++ {
		cost[i] = make([]float64, m)
		for j := 0; j < m; j++ {
			cost[i][j] = bidirectionalChamferDistance(pua[cpList[i]], refs[j])
		}
	}

	// Kuhn-Munkres
	u := make([]float64, n+1)
	v := make([]float64, m+1)
	p := make([]int, m+1)
	way := make([]int, m+1)

	for i := 1; i <= n; i++ {
		p[0] = i
		j0 := 0
		minv := make([]float64, m+1)
		for j := 0; j <= m; j++ {
			minv[j] = math.MaxFloat64
		}
		used := make([]bool, m+1)
		for {
			used[j0] = true
			i0 := p[j0]
			delta := math.MaxFloat64
			j1 := 0
			for j := 1; j <= m; j++ {
				if used[j] {
					continue
				}
				cur := cost[i0-1][j-1] - u[i0] - v[j]
				if cur < minv[j] {
					minv[j] = cur
					way[j] = j0
				}
				if minv[j] < delta {
					delta = minv[j]
					j1 = j
				}
			}
			for j := 0; j <= m; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}
			j0 = j1
			if p[j0] == 0 {
				break
			}
		}
		for {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
			if j0 == 0 {
				break
			}
		}
	}

	result := make(map[int]string)
	for j := 1; j <= m; j++ {
		if p[j] > 0 {
			result[cpList[p[j]-1]] = strconv.Itoa(j - 1)
		}
	}
	return result
}

func countUnique(m map[int]string) int {
	s := make(map[string]bool)
	for _, v := range m {
		s[v] = true
	}
	return len(s)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//  Arial 基准轮廓加载（sfnt 同工具链）
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func loadArialRef() {
	paths := []string{
		`C:\Windows\Fonts\arial.ttf`,
		`C:\Windows\Fonts\Arial.ttf`,
		`/usr/share/fonts/truetype/msttcorefonts/Arial.ttf`,
		`/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf`,
		`/usr/share/fonts/TTF/arial.ttf`,
	}

	var data []byte
	for _, p := range paths {
		d, err := os.ReadFile(p)
		if err == nil {
			data = d
			slog.Info("stonefont arial ref loaded", "path", p)
			break
		}
	}
	if data == nil {
		slog.Warn("stonefont arial font not found, matching will fail")
		return
	}

	sf, err := sfnt.Parse(data)
	if err != nil {
		slog.Warn("stonefont arial sfnt parse fail", "err", err)
		return
	}

	buf := &sfnt.Buffer{}
	upem := float64(sf.UnitsPerEm())
	ppem := fixed.Int26_6(int(upem) << 6)
	digits := [10]rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}

	for i, d := range digits {
		gidx, _ := sf.GlyphIndex(buf, d)
		if gidx == 0 {
			slog.Warn("stonefont arial missing glyph", "digit", string(d))
			continue
		}
		segs, err := sf.LoadGlyph(buf, gidx, ppem, nil)
		if err != nil {
			continue
		}
		pts := segmentsToPoints(segs)
		if len(pts) > 0 {
			refContours[i] = normalizeOutline(pts, upem)
		}
	}
	slog.Info("stonefont arial ref contours ready")
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//  WOFF → TTF 转换
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func woffToTTF(woff []byte) ([]byte, error) {
	if len(woff) < 44 || string(woff[0:4]) != "wOFF" {
		return nil, nil
	}
	numTables := int(binary.BigEndian.Uint16(woff[12:14]))
	if numTables == 0 || numTables > 100 {
		return nil, nil
	}

	type entry struct {
		tag  string
		data []byte
	}
	var tables []entry
	pos := 44
	for i := 0; i < numTables; i++ {
		if pos+20 > len(woff) {
			break
		}
		tagBytes := woff[pos : pos+4]
		off := int(binary.BigEndian.Uint32(woff[pos+4 : pos+8]))
		compLen := int(binary.BigEndian.Uint32(woff[pos+8 : pos+12]))
		origLen := int(binary.BigEndian.Uint32(woff[pos+12 : pos+16]))
		pos += 20
		if off+compLen > len(woff) {
			continue
		}
		raw := woff[off : off+compLen]
		var data []byte
		if compLen < origLen {
			r, err := zlib.NewReader(bytes.NewReader(raw))
			if err == nil {
				d, _ := io.ReadAll(r)
				r.Close()
				if d != nil {
					data = d
				}
			}
		}
		if data == nil {
			data = raw
		}
		tag := strings.TrimRight(string(tagBytes), "\x00")
		tables = append(tables, entry{tag: tag, data: data})
	}

	hdrSize := 12
	dirSize := numTables * 16
	dataOffset := ((hdrSize + dirSize + 15) / 16) * 16

	tableOff := make(map[string]int)
	off := dataOffset
	for _, t := range tables {
		tableOff[t.tag] = off
		off += (len(t.data) + 15) / 16 * 16
	}
	out := make([]byte, off)

	binary.BigEndian.PutUint32(out[0:4], 0x00010000)
	binary.BigEndian.PutUint16(out[4:6], uint16(numTables))
	sr := uint16(1)
	es := uint16(0)
	for sr*2 <= uint16(numTables) {
		sr *= 2
		es++
	}
	binary.BigEndian.PutUint16(out[6:8], sr*16)
	binary.BigEndian.PutUint16(out[8:10], es)
	binary.BigEndian.PutUint16(out[10:12], uint16(numTables)*16-sr*16)

	dp := 12
	for _, t := range tables {
		tag4 := make([]byte, 4)
		copy(tag4, []byte(t.tag))
		copy(out[dp:dp+4], tag4)
		var sum uint32
		for j := 0; j+3 < len(t.data); j += 4 {
			sum += binary.BigEndian.Uint32(t.data[j : j+4])
		}
		binary.BigEndian.PutUint32(out[dp+4:dp+8], sum)
		binary.BigEndian.PutUint32(out[dp+8:dp+12], uint32(tableOff[t.tag]))
		binary.BigEndian.PutUint32(out[dp+12:dp+16], uint32(len(t.data)))
		dp += 16
	}
	for _, t := range tables {
		copy(out[tableOff[t.tag]:], t.data)
	}
	return out, nil
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//  HTML 密文解码
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func stonefontDecode(htmlStr string, mapping map[int]string) float64 {
	re := regexp.MustCompile(`&#x([0-9a-fA-F]+);`)
	var digits strings.Builder
	lastEnd := 0
	for _, loc := range re.FindAllStringSubmatchIndex(htmlStr, -1) {
		for i := lastEnd; i < loc[0]; i++ {
			ch := htmlStr[i]
			if (ch >= '0' && ch <= '9') || ch == '.' {
				digits.WriteByte(ch)
			}
		}
		hexStr := htmlStr[loc[2]:loc[3]]
		cp, _ := strconv.ParseInt(hexStr, 16, 32)
		if digit, ok := mapping[int(cp)]; ok {
			digits.WriteString(digit)
		}
		lastEnd = loc[1]
	}
	for i := lastEnd; i < len(htmlStr); i++ {
		ch := htmlStr[i]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			digits.WriteByte(ch)
		}
	}
	result, err := strconv.ParseFloat(digits.String(), 64)
	if err != nil {
		return 0
	}
	return result
}
