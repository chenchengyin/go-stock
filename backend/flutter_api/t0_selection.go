package flutter_api

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// T0 开盘日线选股 — 纯日线动量 + T0 09:25竞价确认
//
// 数据源：
//   - 股票池：新浪 Market_Center API（分页获取全量A股，含 nmc/mktcap）
//   - 日线 K 线：data.FetchKLineWithFallback（东财→新浪→腾讯→通达信）
//   - T0 实时行情：腾讯 qt.gtimg.cn API
//   - 缓存根目录：/tmp/go-stock-cache/t0/{daily,selection}/
//
// 过滤链：
//  1. 主板（60/00 开头）+ 流通市值（优先，否则总市值）50～9000 亿
//  2. 近 7 日收盘涨幅 ≥ 9.89%，或前一交易日涨停破板（最高 ≥ 9.89% 且收盘 < 9.85%）
//  3. 前一交易日成交额 ≥ 5 亿
//  4. 前一交易日收盘价 > MA20（已暂缓，不作为入选条件；函数保留便于恢复）
//  5. T0 开盘涨幅 0.01% ~ 3%
//
// API：
//   GET /api/t0-selection?prewarm=1[&date=]           后台预热日线（进行中立刻返回进度）
//   GET /api/t0-selection[&date=][&save=1]            正式选股（优先读日线缓存；默认归档写一次）
//                                                   若为「今天」且本地时间 < 09:25：自动按预热处理
//   GET /api/t0-selection?archived=1&date=            只读当日选股结果归档
// ─────────────────────────────────────────────────────────────────────────────

const (
	t0MinMarketCapYi    = 50.0
	t0MaxMarketCapYi    = 9000.0
	t0LimitUpCloseRet   = 9.89 // 收盘涨停；昨日破板的最高涨幅门槛
	t0BrokenLimitRet    = 9.85 // 昨日破板：收盘须低于此（最高仍须 ≥ 9.89）
	t0LimitUpMemoryDays = 7
	// t0CacheRoot 仅作最后兜底（相对当前工作目录），正常由 resolveT0CacheRoot 解析为绝对路径
	t0CacheRoot = "backend/data/cache"
)

// t0CacheRootPath 缓存根目录，服务启动时由 initT0CacheRoot 解析为绝对路径；测试可改写为临时目录
var t0CacheRootPath = t0CacheRoot

// isProjectRoot 判断目录是否为 go-stock 项目根：同时含 go.mod 与 backend/
func isProjectRoot(dir string) bool {
	if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil || fi.IsDir() {
		return false
	}
	if fi, err := os.Stat(filepath.Join(dir, "backend")); err != nil || !fi.IsDir() {
		return false
	}
	return true
}

// findProjectRootUpward 从 start 逐级向上查找项目根，找不到返回 ""
func findProjectRootUpward(start string) string {
	if start == "" {
		return ""
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if isProjectRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// resolveT0CacheRoot 解析缓存根目录，优先级：
// 1) 环境变量目录（envDir 非空）→ 绝对路径直接使用
// 2) 从工作目录向上定位项目根 → <root>/backend/data/cache
// 3) 从可执行文件目录向上定位项目根 → <root>/backend/data/cache
// 都失败返回错误，避免静默写到错误目录。
func resolveT0CacheRoot(envDir, cwd, exeDir string) (string, error) {
	if strings.TrimSpace(envDir) != "" {
		abs, err := filepath.Abs(envDir)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	for _, start := range []string{cwd, exeDir} {
		if root := findProjectRootUpward(start); root != "" {
			return filepath.Join(root, "backend", "data", "cache"), nil
		}
	}
	return "", fmt.Errorf("无法定位 go-stock 项目根（含 go.mod 与 backend/），请设置 GO_STOCK_CACHE_DIR")
}

// initT0CacheRoot 在服务启动时确定缓存根目录并创建。
func initT0CacheRoot() error {
	cwd, _ := os.Getwd()
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	root, err := resolveT0CacheRoot(os.Getenv("GO_STOCK_CACHE_DIR"), cwd, exeDir)
	if err != nil {
		return err
	}
	t0CacheRootPath = root
	if err := ensureT0CacheDirs(); err != nil {
		return err
	}
	logger.SugaredLogger.Infof("[T0选股] 缓存根目录: %s", t0CacheRootPath)
	return nil
}

// t0DailyCachePayload 按交易日落盘的股票池 + 日线缓存
type t0DailyCachePayload struct {
	TradeDate string
	Stocks    []t0Stock
	Daily     map[string][]dailyBar
}

// t0SelectionArchive 按日选股结果归档
type t0SelectionArchive struct {
	Date string `json:"date"`
	// SavedAt 首次归档或最近一次覆盖写入时间
	SavedAt string `json:"saved_at"`
	// CloseUpdatedAt 收盘后刷新 T0收盘涨幅 的时间，同时作为当日幂等标记
	CloseUpdatedAt string              `json:"close_updated_at,omitempty"`
	Count          int                 `json:"count"`
	Results        []T0SelectionResult `json:"results"`
}

type t0WarmStatus string

const (
	t0WarmStatusIdle    t0WarmStatus = "idle"
	t0WarmStatusWarming t0WarmStatus = "warming"
	t0WarmStatusReady   t0WarmStatus = "ready"
	t0WarmStatusFailed  t0WarmStatus = "failed"
)

// t0WarmProgress 进程内预热进度（按交易日）
type t0WarmProgress struct {
	Status         t0WarmStatus
	StockCount     int
	DailyFetched   int
	DailyTotal     int
	CandidateCount int
	Err            string
	StartedAt      time.Time
	BackfillDate   string
	BackfillPhase  string // daily | selection | close_refresh
}

var (
	t0WarmMu     sync.Mutex
	t0WarmByDate = map[string]*t0WarmProgress{}
)

func t0DailyCachePath(tradeDate string) string {
	return filepath.Join(t0CacheRootPath, "t0", "daily", "t0_daily_cache_"+tradeDate+".gob")
}

func t0SelectionCachePath(tradeDate string) string {
	return filepath.Join(t0CacheRootPath, "t0", "selection", "t0_selection_"+tradeDate+".json")
}

func ensureT0CacheDirs() error {
	for _, dir := range []string{
		filepath.Join(t0CacheRootPath, "t0", "daily"),
		filepath.Join(t0CacheRootPath, "t0", "selection"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func isT0DailyCacheFilePresent(tradeDate string) bool {
	_, err := os.Stat(t0DailyCachePath(tradeDate))
	return err == nil
}

func getT0WarmProgress(tradeDate string) t0WarmProgress {
	t0WarmMu.Lock()
	defer t0WarmMu.Unlock()
	p, ok := t0WarmByDate[tradeDate]
	if !ok || p == nil {
		return t0WarmProgress{Status: t0WarmStatusIdle}
	}
	return *p
}

func setT0WarmProgressForTest(tradeDate string, p t0WarmProgress) {
	t0WarmMu.Lock()
	defer t0WarmMu.Unlock()
	cp := p
	t0WarmByDate[tradeDate] = &cp
}

func updateT0WarmProgress(tradeDate string, fn func(p *t0WarmProgress)) {
	t0WarmMu.Lock()
	defer t0WarmMu.Unlock()
	p, ok := t0WarmByDate[tradeDate]
	if !ok || p == nil {
		p = &t0WarmProgress{Status: t0WarmStatusIdle}
		t0WarmByDate[tradeDate] = p
	}
	fn(p)
}

func shouldReturnWarmingForSelection(tradeDate string) bool {
	return !isT0DailyCacheFilePresent(tradeDate) && getT0WarmProgress(tradeDate).Status == t0WarmStatusWarming
}

func loadT0DailyCache(tradeDate string) (*t0DailyCachePayload, bool) {
	_ = ensureT0CacheDirs()
	path := t0DailyCachePath(tradeDate)
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	var payload t0DailyCachePayload
	if err := gob.NewDecoder(f).Decode(&payload); err != nil {
		logger.SugaredLogger.Warnf("[T0选股] 日线缓存解码失败 %s: %v", path, err)
		return nil, false
	}
	if payload.TradeDate != tradeDate || len(payload.Stocks) == 0 || len(payload.Daily) == 0 {
		return nil, false
	}
	return &payload, true
}

func saveT0DailyCache(tradeDate string, stocks []t0Stock, daily map[string][]dailyBar) error {
	if err := ensureT0CacheDirs(); err != nil {
		return err
	}
	payload := t0DailyCachePayload{
		TradeDate: tradeDate,
		Stocks:    stocks,
		Daily:     daily,
	}
	path := t0DailyCachePath(tradeDate)
	tmp := path + ".tmp"

	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	encErr := gob.NewEncoder(f).Encode(&payload)
	closeErr := f.Close()
	if encErr != nil {
		_ = os.Remove(tmp)
		return encErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	logger.SugaredLogger.Infof("[T0选股] 日线缓存已写入: %s (股票%d只, 日线%d只)",
		path, len(stocks), len(daily))
	return nil
}

func saveT0SelectionArchive(tradeDate string, results []T0SelectionResult, force bool) error {
	return saveT0SelectionArchiveFull(&t0SelectionArchive{
		Date:    tradeDate,
		SavedAt: time.Now().Format(time.RFC3339),
		Count:   len(results),
		Results: results,
	}, force)
}

// saveT0SelectionArchiveFull 写入完整归档（含 CloseUpdatedAt），force=false 且文件已存在时跳过。
func saveT0SelectionArchiveFull(a *t0SelectionArchive, force bool) error {
	if err := ensureT0CacheDirs(); err != nil {
		return err
	}
	path := t0SelectionCachePath(a.Date)
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadT0SelectionArchive(tradeDate string) (*t0SelectionArchive, bool) {
	data, err := os.ReadFile(t0SelectionCachePath(tradeDate))
	if err != nil {
		return nil, false
	}
	var a t0SelectionArchive
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, false
	}
	return &a, true
}

// sortT0ResultsForClient 返回用于客户端展示的排序副本：
// 1) blue；2) orange；3) green；4) 涨停破板；5) 前一天跌停；
// 6) 前一天大阴线；7) 其他结果。组内保留有标记优先和 T0 开盘涨幅降序，稳定排序。
// 不修改入参切片与磁盘归档。
func sortT0ResultsForClient(results []T0SelectionResult) []T0SelectionResult {
	sorted := make([]T0SelectionResult, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool {
		ri := t0DisplaySortRank(sorted[i])
		rj := t0DisplaySortRank(sorted[j])
		if ri != rj {
			return ri < rj
		}
		ti := sorted[i].Tag != ""
		tj := sorted[j].Tag != ""
		if ti != tj {
			return ti
		}
		return sorted[i].OpenGap > sorted[j].OpenGap
	})
	return sorted
}

func t0DisplaySortRank(result T0SelectionResult) int {
	switch result.BuySignal {
	case BuySignalBlue:
		return 0
	case BuySignalOrange:
		return 1
	case BuySignalGreen:
		return 2
	}

	switch result.Tag {
	case "涨停破板":
		return 3
	case "前一天跌停":
		return 4
	case "前一天大阴线":
		return 5
	default:
		return 6
	}
}

// listSelectionArchiveDates 扫描 selection 目录，返回所有有效归档日期（降序）。
func listSelectionArchiveDates() []string {
	dir := filepath.Dir(t0SelectionCachePath("1970-01-01"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	const prefix = "t0_selection_"
	const suffix = ".json"
	dates := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		d := name[len(prefix) : len(name)-len(suffix)]
		if _, perr := time.Parse("2006-01-02", d); perr != nil {
			continue
		}
		if _, ok := loadT0SelectionArchive(d); !ok {
			continue
		}
		dates = append(dates, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	return dates
}

// findLatestSelectionArchiveBefore 在 selection 目录里找日期严格早于 tradeDate 的最新有效归档。
// 文件名形如 t0_selection_YYYY-MM-DD.json；损坏或非法文件跳过。
func findLatestSelectionArchiveBefore(tradeDate string) (*t0SelectionArchive, bool) {
	dir := filepath.Dir(t0SelectionCachePath(tradeDate))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}

	const prefix = "t0_selection_"
	const suffix = ".json"
	dates := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		d := name[len(prefix) : len(name)-len(suffix)]
		if _, perr := time.Parse("2006-01-02", d); perr != nil {
			continue
		}
		if d >= tradeDate {
			continue
		}
		dates = append(dates, d)
	}
	if len(dates) == 0 {
		return nil, false
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	for _, d := range dates {
		if a, ok := loadT0SelectionArchive(d); ok {
			return a, true
		}
	}
	return nil, false
}

// loadOrFetchT0Daily 优先读磁盘缓存；未命中则拉股票池+日线并写入。
func loadOrFetchT0Daily(tradeDate string) (stocks []t0Stock, daily map[string][]dailyBar, fromCache bool, err error) {
	if cached, ok := loadT0DailyCache(tradeDate); ok {
		logger.SugaredLogger.Infof("[T0选股] 日线缓存命中: %s (股票%d只, 日线%d只)",
			tradeDate, len(cached.Stocks), len(cached.Daily))
		return cached.Stocks, cached.Daily, true, nil
	}

	t1 := time.Now()
	stocks = fetchStockPoolFromSina()
	logger.SugaredLogger.Infof("[T0选股] 股票池(主板+市值): %d只 (%.1fs)", len(stocks), time.Since(t1).Seconds())
	if len(stocks) == 0 {
		return nil, nil, false, fmt.Errorf("股票池为空")
	}

	t2 := time.Now()
	daily = fetchAllDailyKLine(stocks, tradeDate, nil)
	logger.SugaredLogger.Infof("[T0选股] 日线获取成功: %d只 (%.1fs)", len(daily), time.Since(t2).Seconds())
	if len(daily) == 0 {
		return nil, nil, false, fmt.Errorf("日线获取为空")
	}

	if saveErr := saveT0DailyCache(tradeDate, stocks, daily); saveErr != nil {
		logger.SugaredLogger.Warnf("[T0选股] 日线缓存写入失败: %v", saveErr)
	}
	return stocks, daily, false, nil
}

// tryStartT0Prewarm 尝试启动后台预热。已在 warming 或文件已就绪时不重复启动。
func tryStartT0Prewarm(tradeDate string) (started bool, progress t0WarmProgress) {
	t0WarmMu.Lock()
	if isT0DailyCacheFilePresent(tradeDate) {
		p := t0WarmByDate[tradeDate]
		if p == nil {
			p = &t0WarmProgress{}
			t0WarmByDate[tradeDate] = p
		}
		p.Status = t0WarmStatusReady
		out := *p
		t0WarmMu.Unlock()
		return false, out
	}
	if p, ok := t0WarmByDate[tradeDate]; ok && p != nil && p.Status == t0WarmStatusWarming {
		out := *p
		t0WarmMu.Unlock()
		return false, out
	}
	now := time.Now()
	t0WarmByDate[tradeDate] = &t0WarmProgress{
		Status:    t0WarmStatusWarming,
		StartedAt: now,
	}
	out := *t0WarmByDate[tradeDate]
	t0WarmMu.Unlock()

	go runT0PrewarmJob(tradeDate)
	return true, out
}

func runT0PrewarmJob(tradeDate string) {
	logger.SugaredLogger.Infof("========== T0 日线预热(后台) | 基准日: %s ==========", tradeDate)
	defer func() {
		if r := recover(); r != nil {
			updateT0WarmProgress(tradeDate, func(p *t0WarmProgress) {
				p.Status = t0WarmStatusFailed
				p.Err = fmt.Sprintf("panic: %v", r)
			})
		}
	}()

	stocks := fetchStockPoolFromSina()
	updateT0WarmProgress(tradeDate, func(p *t0WarmProgress) {
		p.StockCount = len(stocks)
		p.DailyTotal = len(stocks)
		p.DailyFetched = 0
	})
	if len(stocks) == 0 {
		updateT0WarmProgress(tradeDate, func(p *t0WarmProgress) {
			p.Status = t0WarmStatusFailed
			p.Err = "股票池为空"
		})
		return
	}

	daily := fetchAllDailyKLine(stocks, tradeDate, func() {
		updateT0WarmProgress(tradeDate, func(p *t0WarmProgress) {
			p.DailyFetched++
		})
	})
	if len(daily) == 0 {
		updateT0WarmProgress(tradeDate, func(p *t0WarmProgress) {
			p.Status = t0WarmStatusFailed
			p.Err = "日线获取为空"
		})
		return
	}

	if err := saveT0DailyCache(tradeDate, stocks, daily); err != nil {
		updateT0WarmProgress(tradeDate, func(p *t0WarmProgress) {
			p.Status = t0WarmStatusFailed
			p.Err = err.Error()
		})
		return
	}

	hist := make(map[string][]dailyBar, len(daily))
	for sc, bars := range daily {
		hist[sc] = histBarsBeforeTradeDate(bars, tradeDate)
	}
	step1 := filterLimitUpRecent(stocks, hist, t0LimitUpMemoryDays, t0LimitUpCloseRet)
	step2 := filterTurnover(step1, hist, 5.0)
	updateT0WarmProgress(tradeDate, func(p *t0WarmProgress) {
		p.Status = t0WarmStatusReady
		p.StockCount = len(stocks)
		p.DailyFetched = len(daily)
		p.DailyTotal = len(stocks)
		p.CandidateCount = len(step2)
		p.Err = ""
	})
	logger.SugaredLogger.Infof("[T0选股] 后台预热完成: 股票%d 日线%d 候选%d",
		len(stocks), len(daily), len(step2))
}

func buildPrewarmReadyResponse(tradeDate string) map[string]interface{} {
	return buildPrewarmReadyResponseAt(tradeDate, time.Now())
}

func buildPrewarmReadyResponseAt(tradeDate string, now time.Time) map[string]interface{} {
	tStart := time.Now()
	stocks, daily, ok := []t0Stock(nil), map[string][]dailyBar(nil), false
	if cached, hit := loadT0DailyCache(tradeDate); hit {
		stocks, daily, ok = cached.Stocks, cached.Daily, true
	}
	hist := make(map[string][]dailyBar, len(daily))
	if ok {
		for sc, bars := range daily {
			hist[sc] = histBarsBeforeTradeDate(bars, tradeDate)
		}
	}
	var step2 []t0Stock
	candidateCount := 0
	if ok {
		step1 := filterLimitUpRecent(stocks, hist, t0LimitUpMemoryDays, t0LimitUpCloseRet)
		step2 = filterTurnover(step1, hist, 5.0)
		candidateCount = len(step2)
	}
	prog := getT0WarmProgress(tradeDate)
	if prog.CandidateCount > 0 {
		candidateCount = prog.CandidateCount
	}
	resp := map[string]interface{}{
		"date":            tradeDate,
		"prewarm":         true,
		"status":          string(t0WarmStatusReady),
		"stock_count":     len(stocks),
		"daily_count":     len(daily),
		"daily_fetched":   len(daily),
		"daily_total":     len(stocks),
		"candidate_count": candidateCount,
		"cache_hit":       true,
		"elapsed_sec":     round2(time.Since(tStart).Seconds()),
	}

	// 凌晨窗口内：附带最近历史归档，供前端直接展示前一交易日结果；不附当日 candidates
	if isPreopenPrevResultWindow(now, tradeDate) {
		if a, found := findLatestSelectionArchiveBefore(tradeDate); found {
			resp["historical"] = true
			resp["display_date"] = a.Date
			resp["results"] = sortT0ResultsForClient(a.Results)
		}
		return resp
	}

	if ok {
		cands := assembleT0CandidateResults(tradeDate, step2, hist)
		cands = sortT0ResultsForClient(cands)
		resp["candidates"] = cands
		resp["phase"] = "candidates"
		resp["candidate_count"] = len(cands)
	}
	return resp
}

// buildT0CandidateList 从当日日线缓存构建竞价前候选（涨停记忆+成交额），不含开盘涨幅过滤。
func buildT0CandidateList(tradeDate string) ([]T0SelectionResult, bool) {
	cached, ok := loadT0DailyCache(tradeDate)
	if !ok || cached == nil {
		return nil, false
	}
	hist := make(map[string][]dailyBar, len(cached.Daily))
	for sc, bars := range cached.Daily {
		hist[sc] = histBarsBeforeTradeDate(bars, tradeDate)
	}
	step1 := filterLimitUpRecent(cached.Stocks, hist, t0LimitUpMemoryDays, t0LimitUpCloseRet)
	step2 := filterTurnover(step1, hist, 5.0)
	return assembleT0CandidateResults(tradeDate, step2, hist), true
}

// assembleT0CandidateResults 把过滤后的股票编成预览条目；开盘/收盘涨幅固定为 0，留给客户端行情覆盖。
func assembleT0CandidateResults(tradeDate string, stocks []t0Stock, histCache map[string][]dailyBar) []T0SelectionResult {
	out := make([]T0SelectionResult, 0, len(stocks))
	for _, s := range stocks {
		hist := histCache[s.ShortCode]
		if len(hist) == 0 {
			continue
		}
		prevClose := hist[len(hist)-1].Close
		prevAmountYi := hist[len(hist)-1].AmountYi
		var prevRet float64
		if len(hist) >= 2 && hist[len(hist)-2].Close != 0 {
			prevRet = (hist[len(hist)-1].Close - hist[len(hist)-2].Close) / hist[len(hist)-2].Close * 100
		}

		limitUpInfo := formatLimitUpDates(collectLimitUpMemoryDates(hist, t0LimitUpMemoryDays, t0LimitUpCloseRet))

		tag := ""
		if highRet, openRet, prevDayCloseRet, ok := prevDayRetsFromHist(hist); ok {
			tag = pickPrevDayTag(highRet, openRet, prevDayCloseRet)
		}

		marketSuffix := ".XSHE"
		if strings.HasPrefix(s.Code, "sh") {
			marketSuffix = ".XSHG"
		}
		out = append(out, T0SelectionResult{
			Time:         tradeDate,
			OpenGap:      0,
			CloseRet:     0,
			LimitUpDates: limitUpInfo,
			MA20:         round2(calcMA20(hist)),
			AmountYi:     round2(prevAmountYi),
			StockCode:    s.ShortCode + marketSuffix,
			StockName:    s.Name,
			PrevClose:    round2(prevClose),
			PrevCloseRet: round2(prevRet),
			Tag:          tag,
		})
		enrichResultWithPattern(&out[len(out)-1], hist)
	}
	return out
}

func buildPrewarmProgressResponse(tradeDate string, prog t0WarmProgress) map[string]interface{} {
	elapsed := 0.0
	if !prog.StartedAt.IsZero() {
		elapsed = time.Since(prog.StartedAt).Seconds()
	}
	resp := map[string]interface{}{
		"date":            tradeDate,
		"prewarm":         true,
		"status":          string(prog.Status),
		"stock_count":     prog.StockCount,
		"daily_fetched":   prog.DailyFetched,
		"daily_total":     prog.DailyTotal,
		"daily_count":     prog.DailyFetched,
		"candidate_count": prog.CandidateCount,
		"cache_hit":       false,
		"elapsed_sec":     round2(elapsed),
	}
	if prog.Status == t0WarmStatusFailed && prog.Err != "" {
		resp["error"] = prog.Err
	}
	if prog.BackfillDate != "" {
		resp["backfill_date"] = prog.BackfillDate
	}
	if prog.BackfillPhase != "" {
		resp["backfill_phase"] = prog.BackfillPhase
	}
	return resp
}

// ── 数据结构 ─────────────────────────────────────────────────────────────────

// dailyBar 内部使用的日线数据
type dailyBar struct {
	Date     string
	Open     float64
	Close    float64
	High     float64
	Low      float64
	Volume   float64 // 成交量(股)
	AmountYi float64 // 成交额(亿元)
}

// t0Stock 股票基础信息
type t0Stock struct {
	Code        string // sh.600000 / sz.000001
	ShortCode   string // 600000 / 000001
	Name        string
	MarketCapYi float64 // 市值(亿)，优先流通市值
}

// t0Realtime T0 实时行情
type t0Realtime struct {
	Open      float64
	Close     float64 // 当前价
	PrevClose float64
}

// T0SelectionResult 最终选股结果
type T0SelectionResult struct {
	Time         string  `json:"时间"`
	OpenGap      float64 `json:"T0开盘涨幅(%)"`
	CloseRet     float64 `json:"T0收盘涨幅(%)"`
	LimitUpDates string  `json:"涨停日期"`
	MA20         float64 `json:"MA20"`
	AmountYi     float64 `json:"成交额(亿)"`
	StockCode    string  `json:"股票代码"` // 如 600000.XSHG
	StockName    string  `json:"股票名称"`
	PrevClose    float64 `json:"前一交易日收盘"`
	PrevCloseRet float64 `json:"前一交易日收盘涨幅(%)"`
	Tag          string  `json:"标记"`
	Pattern          string  `json:"形态"`
	PatternT0N       int     `json:"形态样本数"`
	PatternWinPct    float64 `json:"形态达标率(%)"`
	PatternFailPct   float64 `json:"形态真亏率(%)"`
	BuySignal        string  `json:"买入信号"`
}

// t0CloseRefreshStartHM 收盘后刷新归档收盘涨幅的最早时分（含）：15:05
const t0CloseRefreshStartHM = 15*60 + 5

// t0ShortCodeFromResultCode 从 "600188.XSHG" 取出 "600188"
func t0ShortCodeFromResultCode(stockCode string) string {
	if i := strings.IndexByte(stockCode, '.'); i > 0 {
		return stockCode[:i]
	}
	return stockCode
}

// shouldRefreshSelectionClose 判断是否该刷新当日归档的收盘涨幅：
// 周一到周五、上海时区 15:05 之后，且当日尚未刷新过。
func shouldRefreshSelectionClose(now time.Time, closeUpdatedAt string) bool {
	if strings.TrimSpace(closeUpdatedAt) != "" {
		return false
	}
	local := now.In(chinaLocation())
	switch local.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	return local.Hour()*60+local.Minute() >= t0CloseRefreshStartHM
}

// patchSelectionCloseRets 用行情覆盖归档结果里的 T0收盘涨幅，其余字段一律不动。
// 行情缺失、现价<=0 或前收无法确定时保留原值，避免把有效数据写成 0。
func patchSelectionCloseRets(results []T0SelectionResult, quotes map[string]t0Realtime) (updated, kept int) {
	for i := range results {
		rt, ok := quotes[t0ShortCodeFromResultCode(results[i].StockCode)]
		if !ok || rt.Close <= 0 {
			kept++
			continue
		}
		prev := rt.PrevClose
		if prev <= 0 {
			prev = results[i].PrevClose
		}
		if prev <= 0 {
			kept++
			continue
		}
		results[i].CloseRet = round2((rt.Close - prev) / prev * 100)
		updated++
	}
	return updated, kept
}

// resultsToT0Stocks 把归档结果还原成行情查询所需的最小股票信息。
func resultsToT0Stocks(results []T0SelectionResult) []t0Stock {
	out := make([]t0Stock, 0, len(results))
	for _, r := range results {
		sc := t0ShortCodeFromResultCode(r.StockCode)
		prefix := "sz."
		if strings.HasSuffix(r.StockCode, ".XSHG") {
			prefix = "sh."
		}
		out = append(out, t0Stock{Code: prefix + sc, ShortCode: sc, Name: r.StockName})
	}
	return out
}

// refreshSelectionCloseRet 收盘后只刷新归档中的 T0收盘涨幅。
// 不重跑选股、不触碰日线缓存，名单与其余字段保持原样。
// force=true 时忽略「时间未到 / 已刷新」短路，用于手动修正历史归档。
func refreshSelectionCloseRet(tradeDate string, force bool) (map[string]any, error) {
	a, ok := loadT0SelectionArchive(tradeDate)
	if !ok || a == nil {
		return nil, fmt.Errorf("该日无选股归档")
	}
	if !force && !shouldRefreshSelectionClose(time.Now(), a.CloseUpdatedAt) {
		return map[string]any{
			"date":             a.Date,
			"skipped":          true,
			"reason":           "not_due_or_already_updated",
			"close_updated_at": a.CloseUpdatedAt,
			"count":            a.Count,
		}, nil
	}

	quotes := fetchT0Realtime(resultsToT0Stocks(a.Results))
	updated, kept := patchSelectionCloseRets(a.Results, quotes)

	now := time.Now().In(chinaLocation()).Format(time.RFC3339)
	a.SavedAt = now
	a.CloseUpdatedAt = now
	a.Count = len(a.Results)
	if err := saveT0SelectionArchiveFull(a, true); err != nil {
		return nil, err
	}

	return map[string]any{
		"date":             tradeDate,
		"count":            a.Count,
		"updated":          updated,
		"kept":             kept,
		"close_updated_at": now,
		"results":          sortT0ResultsForClient(a.Results),
	}, nil
}

// patchSelectionTags 用日线缓存重算归档结果的前日标记，其余字段不动。
// 日线缺失或历史不足两根时记入 missing 并留空标记。
func patchSelectionTags(results []T0SelectionResult, daily map[string][]dailyBar, tradeDate string) (tagged, missing int) {
	for i := range results {
		hist := histBarsBeforeTradeDate(daily[t0ShortCodeFromResultCode(results[i].StockCode)], tradeDate)
		highRet, openRet, closeRet, ok := prevDayRetsFromHist(hist)
		if !ok {
			results[i].Tag = ""
			missing++
			continue
		}
		results[i].Tag = pickPrevDayTag(highRet, openRet, closeRet)
		if results[i].Tag != "" {
			tagged++
		}
	}
	return tagged, missing
}

// refreshSelectionTags 为早于标记功能生成的历史归档补算前日标记。
// 只依赖前一交易日及更早的 K 线，故不受当日日线是否完整影响。
func refreshSelectionTags(tradeDate string) (map[string]any, error) {
	a, ok := loadT0SelectionArchive(tradeDate)
	if !ok || a == nil {
		return nil, fmt.Errorf("该日无选股归档")
	}
	cache, ok := loadT0DailyCache(tradeDate)
	if !ok || cache == nil {
		return nil, fmt.Errorf("该日无日线缓存，无法重算标记")
	}

	tagged, missing := patchSelectionTags(a.Results, cache.Daily, tradeDate)
	a.SavedAt = time.Now().In(chinaLocation()).Format(time.RFC3339)
	a.Count = len(a.Results)
	if err := saveT0SelectionArchiveFull(a, true); err != nil {
		return nil, err
	}

	return map[string]any{
		"date":    tradeDate,
		"count":   a.Count,
		"tagged":  tagged,
		"missing": missing,
		"results": sortT0ResultsForClient(a.Results),
	}, nil
}

// runT0CloseRefreshTick 定时任务入口：交易日 15:05 后把当日归档刷成真收盘涨幅，每天一次。
func runT0CloseRefreshTick(now time.Time) {
	tradeDate := now.In(chinaLocation()).Format("2006-01-02")
	a, ok := loadT0SelectionArchive(tradeDate)
	if !ok || a == nil {
		return
	}
	if !shouldRefreshSelectionClose(now, a.CloseUpdatedAt) {
		return
	}
	out, err := refreshSelectionCloseRet(tradeDate, false)
	if err != nil {
		logger.SugaredLogger.Warnf("[定时任务] T0归档收盘涨幅刷新失败 %s: %v", tradeDate, err)
		return
	}
	if skipped, _ := out["skipped"].(bool); skipped {
		return
	}
	logger.SugaredLogger.Infof("[定时任务] T0归档收盘涨幅已刷新: %s updated=%v kept=%v",
		tradeDate, out["updated"], out["kept"])
}

// prevDayRetsFromHist 由开盘前历史日线算出前一交易日的最高/开盘/收盘涨幅（相对再前一日收盘）。
// hist 必须已剔除选股当日 K，故 hist[-1] 即前一交易日。
func prevDayRetsFromHist(hist []dailyBar) (highRet, openRet, closeRet float64, ok bool) {
	if len(hist) < 2 {
		return 0, 0, 0, false
	}
	base := hist[len(hist)-2].Close
	if base == 0 {
		return 0, 0, 0, false
	}
	prev := hist[len(hist)-1]
	highRet = (prev.High - base) / base * 100
	openRet = (prev.Open - base) / base * 100
	closeRet = (prev.Close - base) / base * 100
	return highRet, openRet, closeRet, true
}

// pickPrevDayTag 依据前一交易日涨跌结构给出唯一展示标记。
// 优先级：涨停破板 > 前一天跌停 > 前一天大阴线；同时破板且跌停时不打标记。
// 大阴线：开盘涨幅−收盘涨幅≥4，且收盘涨幅≥−2%（跌超 2% 不在关注范围）。
func pickPrevDayTag(highRet, openRet, closeRet float64) string {
	brokenLimitUp := highRet >= t0LimitUpCloseRet && closeRet < t0BrokenLimitRet
	limitDown := closeRet <= -9.9
	bigYin := openRet-closeRet >= 4.0 && closeRet >= -2.0

	if brokenLimitUp && limitDown {
		return ""
	}
	if brokenLimitUp {
		return "涨停破板"
	}
	if limitDown {
		return "前一天跌停"
	}
	if bigYin {
		return "前一天大阴线"
	}
	return ""
}

// ── 股票池获取（新浪 API） ──────────────────────────────────────────────────

// sinaFlexibleFloat 兼容新浪 JSON 中 number / string 两种市值字段
type sinaFlexibleFloat float64

func (f *sinaFlexibleFloat) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = 0
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			*f = 0
			return nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		*f = sinaFlexibleFloat(v)
		return nil
	}
	v, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return err
	}
	*f = sinaFlexibleFloat(v)
	return nil
}

// marketCapYiFromSina 新浪 mktcap/nmc 单位为万元 → 亿元
func marketCapYiFromSina(nmc, mktcap float64) (float64, bool) {
	raw := nmc
	if raw <= 0 {
		raw = mktcap
	}
	if raw <= 0 {
		return 0, false
	}
	return raw / 10000, true
}

func fetchStockPoolFromSina() []t0Stock {
	const baseURL = "http://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/" +
		"Market_Center.getHQNodeData?page=%d&num=100&sort=code&asc=1" +
		"&node=hs_a&symbol=&_s_r_a=page"

	var allStocks []t0Stock
	mainBoardCount := 0

	for page := 1; page <= 100; page++ {
		url := fmt.Sprintf(baseURL, page)
		resp, err := data.SharedHTTPClient.R().
			SetHeader("User-Agent", "Mozilla/5.0").
			Get(url)
		if err != nil || resp.StatusCode() != 200 {
			break
		}

		var items []struct {
			Code   string            `json:"code"`
			Name   string            `json:"name"`
			Nmc    sinaFlexibleFloat `json:"nmc"`    // 流通市值(万元)
			Mktcap sinaFlexibleFloat `json:"mktcap"` // 总市值(万元)
		}
		if err := json.Unmarshal(resp.Body(), &items); err != nil {
			break
		}
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			code := item.Code
			var prefix string
			if strings.HasPrefix(code, "60") {
				prefix = "sh."
			} else if strings.HasPrefix(code, "00") {
				prefix = "sz."
			} else {
				continue
			}
			mainBoardCount++

			capYi, ok := marketCapYiFromSina(float64(item.Nmc), float64(item.Mktcap))
			if !ok || capYi < t0MinMarketCapYi || capYi > t0MaxMarketCapYi {
				continue
			}

			allStocks = append(allStocks, t0Stock{
				Code:        prefix + code,
				ShortCode:   code,
				Name:        item.Name,
				MarketCapYi: capYi,
			})
		}

		if len(items) < 100 {
			break
		}
	}

	logger.SugaredLogger.Infof("[T0选股] 过滤1(主板+市值%.0f~%.0f亿): 主板%d只 -> 通过%d只",
		t0MinMarketCapYi, t0MaxMarketCapYi, mainBoardCount, len(allStocks))
	return allStocks
}

// ── 日线 K 线获取 ──────────────────────────────────────────────────────────

// parseKLineToDailyBar 将 KLineData 转为内部 dailyBar
func parseKLineToDailyBar(kd data.KLineData) (dailyBar, bool) {
	var bar dailyBar
	bar.Date = kd.Day

	parse := func(s string) (float64, bool) {
		v, err := strconv.ParseFloat(s, 64)
		return v, err == nil
	}

	var ok bool
	bar.Open, ok = parse(kd.Open)
	if !ok {
		return bar, false
	}
	bar.Close, ok = parse(kd.Close)
	if !ok {
		return bar, false
	}
	bar.High, ok = parse(kd.High)
	if !ok {
		return bar, false
	}
	bar.Low, ok = parse(kd.Low)
	if !ok {
		return bar, false
	}
	bar.Volume, ok = parse(kd.Volume)
	if !ok {
		return bar, false
	}
	// 成交额(亿元) = 成交量 * 收盘价 / 1e8
	bar.AmountYi = bar.Volume * bar.Close / 1e8
	return bar, true
}

// fetchDailyKLine 获取单只股票日线，返回按期排序的 bar 列表。
// 有 endDate 时多取一些（60 根），避免新浪/腾讯回退只有「近端」窗口时，
// 截断到较早历史日后条数不足（MA20 / 涨停记忆）。
func fetchDailyKLine(shortCode string, endDate string) []dailyBar {
	limit := 30
	if strings.TrimSpace(endDate) != "" {
		limit = 60
	}
	return fetchDailyKLineWithLimit(shortCode, endDate, limit)
}

func fetchDailyKLineWithLimit(shortCode string, endDate string, limit int) []dailyBar {
	if limit <= 0 {
		limit = 30
	}
	result := data.FetchKLineWithFallback(shortCode, "", "101", limit, endDate)
	if result == nil || result.Data == nil {
		return nil
	}

	var bars []dailyBar
	for _, kd := range *result.Data {
		if bar, ok := parseKLineToDailyBar(kd); ok {
			bars = append(bars, bar)
		}
	}

	// 确保按期排序
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].Date < bars[j].Date
	})

	// 截断到 endDate
	if endDate != "" {
		cut := -1
		for i, b := range bars {
			if b.Date > endDate {
				cut = i
				break
			}
		}
		if cut >= 0 {
			bars = bars[:cut]
		}
	}

	return bars
}

// fetchAllDailyKLine 并发获取所有股票的日线；onOneDone 在每只处理完成后回调（成功或失败都计一次进度）
func fetchAllDailyKLine(stocks []t0Stock, endDate string, onOneDone func()) map[string][]dailyBar {
	cache := make(map[string][]dailyBar)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 20) // 20 并发

	for _, s := range stocks {
		wg.Add(1)
		go func(stock t0Stock) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			bars := fetchDailyKLine(stock.ShortCode, endDate)
			if len(bars) >= 2 {
				mu.Lock()
				cache[stock.ShortCode] = bars
				mu.Unlock()
			}
			if onOneDone != nil {
				onOneDone()
			}
		}(s)
	}
	wg.Wait()
	return cache
}

// ── T0 实时行情（腾讯 API） ─────────────────────────────────────────────────

func fetchT0Realtime(stocks []t0Stock) map[string]t0Realtime {
	if len(stocks) == 0 {
		return nil
	}

	// 构建请求符号：sh600000,sz000001
	var symbols []string
	codeToSC := make(map[string]string) // sym -> shortCode
	for _, s := range stocks {
		var prefix string
		if strings.HasPrefix(s.Code, "sh") {
			prefix = "sh"
		} else {
			prefix = "sz"
		}
		sym := prefix + s.ShortCode
		symbols = append(symbols, sym)
		codeToSC[sym] = s.ShortCode
	}

	// 分批请求（腾讯 API 单次支持多个符号，但不宜太多）
	result := make(map[string]t0Realtime)
	batchSize := 100

	for i := 0; i < len(symbols); i += batchSize {
		end := i + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batch := symbols[i:end]
		url := "http://qt.gtimg.cn/q=" + strings.Join(batch, ",")

		resp, err := data.SharedHTTPClient.R().
			SetHeader("User-Agent", "Mozilla/5.0").
			Get(url)
		if err != nil {
			continue
		}

		text := resp.String()
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// 格式: v_sh600000="1~平安银行~000001~..."
			parts := strings.Split(line, "~")
			if len(parts) < 6 {
				continue
			}
			eqPos := strings.Index(parts[0], `="`)
			if eqPos < 3 {
				continue
			}
			sym := parts[0][2:eqPos]
			sc, ok := codeToSC[sym]
			if !ok {
				continue
			}

			parse := func(s string) float64 {
				v, _ := strconv.ParseFloat(s, 64)
				return v
			}

			rt := t0Realtime{
				Close:     parse(parts[3]),
				PrevClose: parse(parts[4]),
				Open:      parse(parts[5]),
			}
			// 腾讯API：当前价=0时使用开盘价
			if rt.Close == 0 && rt.Open > 0 {
				rt.Close = rt.Open
			}
			result[sc] = rt
		}
	}

	return result
}

func barCloseHighRet(prevClose float64, bar dailyBar) (closeRet, highRet float64, ok bool) {
	if prevClose == 0 {
		return 0, 0, false
	}
	closeRet = (bar.Close - prevClose) / prevClose * 100
	highRet = (bar.High - prevClose) / prevClose * 100
	return closeRet, highRet, true
}

func isCloseLimitUpDay(prevClose float64, bar dailyBar, closeThreshold float64) bool {
	closeRet, _, ok := barCloseHighRet(prevClose, bar)
	return ok && closeRet >= closeThreshold
}

func isBrokenLimitUpDay(prevClose float64, bar dailyBar) bool {
	closeRet, highRet, ok := barCloseHighRet(prevClose, bar)
	return ok && highRet >= t0LimitUpCloseRet && closeRet < t0BrokenLimitRet
}

func isLimitUpMemoryDay(prevClose float64, bar dailyBar, closeThreshold float64) bool {
	return isCloseLimitUpDay(prevClose, bar, closeThreshold) || isBrokenLimitUpDay(prevClose, bar)
}

func limitUpMemoryTail(hist []dailyBar, days int) []dailyBar {
	if days < 1 || len(hist) < 2 {
		return nil
	}
	need := days + 1
	if len(hist) <= need {
		return hist
	}
	return hist[len(hist)-need:]
}

func hasLimitUpMemory(hist []dailyBar, days int, closeThreshold float64) bool {
	tail := limitUpMemoryTail(hist, days)
	last := len(tail) - 1
	for i := 1; i < len(tail); i++ {
		prev, bar := tail[i-1].Close, tail[i]
		if isCloseLimitUpDay(prev, bar, closeThreshold) {
			return true
		}
		if i == last && isBrokenLimitUpDay(prev, bar) {
			return true
		}
	}
	return false
}

func collectLimitUpMemoryDates(hist []dailyBar, days int, closeThreshold float64) []string {
	tail := limitUpMemoryTail(hist, days)
	last := len(tail) - 1
	var dates []string
	for i := 1; i < len(tail); i++ {
		prev, bar := tail[i-1].Close, tail[i]
		if isCloseLimitUpDay(prev, bar, closeThreshold) {
			dates = append(dates, bar.Date)
			continue
		}
		if i == last && isBrokenLimitUpDay(prev, bar) {
			dates = append(dates, bar.Date)
		}
	}
	return dates
}

func formatLimitUpDates(dates []string) string {
	if len(dates) == 0 {
		return "-"
	}
	start := 0
	if len(dates) > 3 {
		start = len(dates) - 3
	}
	return strings.Join(dates[start:], ", ")
}

// ── 过滤层 ──────────────────────────────────────────────────────────────────

// filterLimitUpRecent 过滤2：近 N 日收盘涨停（≥ threshold%），或前一交易日涨停破板
func filterLimitUpRecent(stocks []t0Stock, cache map[string][]dailyBar, days int, threshold float64) []t0Stock {
	logger.SugaredLogger.Infof("[T0选股] 过滤2(涨停记忆): %d只 -> 检查近%d日收盘≥%.2f%%或昨日涨停破板", len(stocks), days, threshold)

	var result []t0Stock
	for _, s := range stocks {
		if hasLimitUpMemory(cache[s.ShortCode], days, threshold) {
			result = append(result, s)
		}
	}
	logger.SugaredLogger.Infof("[T0选股] 过滤2通过: %d只", len(result))
	return result
}

// filterTurnover 过滤3：前一交易日成交额 ≥ minTurnover 亿
func filterTurnover(stocks []t0Stock, cache map[string][]dailyBar, minTurnover float64) []t0Stock {
	logger.SugaredLogger.Infof("[T0选股] 过滤3(成交额≥%.1f亿): %d只", minTurnover, len(stocks))

	var result []t0Stock
	for _, s := range stocks {
		bars := cache[s.ShortCode]
		if len(bars) == 0 {
			continue
		}
		last := bars[len(bars)-1]
		if last.AmountYi >= minTurnover {
			result = append(result, s)
		}
	}
	logger.SugaredLogger.Infof("[T0选股] 过滤3通过: %d只", len(result))
	return result
}

// calcMA20 计算最近 20 日收盘均价；不足 20 根返回 0
func calcMA20(bars []dailyBar) float64 {
	if len(bars) < 20 {
		return 0
	}
	tail := bars[len(bars)-20:]
	var sum float64
	for _, b := range tail {
		sum += b.Close
	}
	return sum / 20
}

// filterMA20Above 过滤4：前一交易日收盘价 > MA20
// 当前主链已暂缓调用，保留函数便于后续重新启用。
func filterMA20Above(stocks []t0Stock, cache map[string][]dailyBar) ([]t0Stock, map[string]float64) {
	logger.SugaredLogger.Infof("[T0选股] 过滤4(收盘>MA20): %d只", len(stocks))

	var result []t0Stock
	ma20Cache := make(map[string]float64)
	for _, s := range stocks {
		bars := cache[s.ShortCode]
		ma20 := calcMA20(bars)
		if ma20 == 0 {
			continue
		}
		prevClose := bars[len(bars)-1].Close
		if prevClose > ma20 {
			result = append(result, s)
			ma20Cache[s.ShortCode] = ma20
		}
	}
	logger.SugaredLogger.Infof("[T0选股] 过滤4通过: %d只", len(result))
	return result, ma20Cache
}

// filterOpenGap 过滤5：T0 竞价开盘涨幅 0.01% ~ 3%（用开盘价 Open，非现价）
func filterOpenGap(stocks []t0Stock, auction map[string]t0Realtime,
	minGap, maxGap float64) ([]t0Stock, map[string]float64) {

	logger.SugaredLogger.Infof("[T0选股] 过滤5(T0竞价开盘涨幅%.2f%%~%.1f%%): %d只", minGap, maxGap, len(stocks))

	var result []t0Stock
	gapCache := make(map[string]float64)
	for _, s := range stocks {
		rt, ok := auction[s.ShortCode]
		if !ok || rt.Open == 0 || rt.PrevClose == 0 {
			continue
		}
		gap := (rt.Open - rt.PrevClose) / rt.PrevClose * 100
		if gap >= minGap && gap <= maxGap {
			result = append(result, s)
			gapCache[s.ShortCode] = gap
		}
	}
	logger.SugaredLogger.Infof("[T0选股] 过滤5通过: %d只", len(result))
	return result, gapCache
}

// histBarsBeforeTradeDate 选股用的「开盘前」日线：若缓存已含 tradeDate 当日K线则剔除，
// 保证涨停记忆/成交额/MA20 都基于前一交易日及更早数据。
func histBarsBeforeTradeDate(bars []dailyBar, tradeDate string) []dailyBar {
	if len(bars) == 0 {
		return bars
	}
	if bars[len(bars)-1].Date == tradeDate {
		return bars[:len(bars)-1]
	}
	return bars
}

// buildT0AuctionQuotes 构建竞价开盘价口径：
//   - 历史回测（tradeDate < 今天）：日线含当日 K → 用当日 Open 作竞价开盘
//   - 当天实盘（tradeDate == 今天）：一律腾讯 Open（09:25 后即竞价价），避免未收盘日线 Open 干扰
func buildT0AuctionQuotes(tradeDate string, stocks []t0Stock, dailyCache map[string][]dailyBar) (map[string]t0Realtime, string) {
	today := time.Now().In(chinaLocation()).Format("2006-01-02")
	useLiveOnly := tradeDate == today || tradeDate > today

	fromDaily := 0
	needLive := make([]t0Stock, 0, len(stocks))
	out := make(map[string]t0Realtime, len(stocks))

	for _, s := range stocks {
		bars := dailyCache[s.ShortCode]
		if !useLiveOnly && len(bars) >= 2 && bars[len(bars)-1].Date == tradeDate && bars[len(bars)-1].Open > 0 {
			last := bars[len(bars)-1]
			prev := bars[len(bars)-2]
			out[s.ShortCode] = t0Realtime{
				Open:      last.Open,
				Close:     last.Close,
				PrevClose: prev.Close,
			}
			fromDaily++
			continue
		}
		needLive = append(needLive, s)
	}

	source := "日线开盘"
	if len(needLive) > 0 {
		live := fetchT0Realtime(needLive)
		for sc, rt := range live {
			out[sc] = rt
		}
		if fromDaily == 0 {
			source = "腾讯竞价开盘"
		} else {
			source = fmt.Sprintf("混合(日线%d/腾讯%d)", fromDaily, len(live))
		}
	}
	return out, source
}

// ── 主函数 ──────────────────────────────────────────────────────────────────

func normalizeT0TradeDate(tradeDate string) (string, error) {
	if tradeDate == "" {
		tradeDate = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", tradeDate); err != nil {
		return "", fmt.Errorf("日期格式错误: %s (需为 2006-01-02)", tradeDate)
	}
	return tradeDate, nil
}

// RunT0Selection 执行完整 T0 选股链
// tradeDate: 交易日 "2006-01-02"，空字符串 = 今天
func RunT0Selection(tradeDate string) ([]T0SelectionResult, error) {
	tStart := time.Now()

	tradeDate, err := normalizeT0TradeDate(tradeDate)
	if err != nil {
		return nil, err
	}

	logger.SugaredLogger.Infof("========== T0 开盘日线选股 | 基准日: %s ==========", tradeDate)

	// ── 1+2. 股票池 + 日线（优先磁盘缓存）──
	t12 := time.Now()
	allStocks, dailyCache, fromCache, err := loadOrFetchT0Daily(tradeDate)
	if err != nil {
		return nil, err
	}
	logger.SugaredLogger.Infof("[T0选股] [1-2/5] 股票池+日线就绪: 股票%d 日线%d 缓存命中=%v (%.1fs)",
		len(allStocks), len(dailyCache), fromCache, time.Since(t12).Seconds())

	// 开盘前历史日线（剔除 tradeDate 当日K，若有）
	histCache := make(map[string][]dailyBar, len(dailyCache))
	for sc, bars := range dailyCache {
		histCache[sc] = histBarsBeforeTradeDate(bars, tradeDate)
	}

	// ── 3. 日线过滤（基于前一交易日及更早；MA20 门闸已暂缓）──
	t3 := time.Now()
	step1 := filterLimitUpRecent(allStocks, histCache, t0LimitUpMemoryDays, t0LimitUpCloseRet)
	step2 := filterTurnover(step1, histCache, 5.0)
	if len(step2) == 0 {
		logger.SugaredLogger.Infof("[T0选股] 成交额过滤后无股票，总耗时: %.1fs", time.Since(tStart).Seconds())
		return nil, fmt.Errorf("成交额过滤后无股票")
	}
	logger.SugaredLogger.Infof("[T0选股] [3/5] 日线过滤完成: %d -> %d只 (MA20门闸已暂缓) (%.1fs)",
		len(allStocks), len(step2), time.Since(t3).Seconds())

	// ── 4. 竞价开盘价（历史用当日日线 Open；当日盘中用腾讯 Open）──
	t4 := time.Now()
	auction, auctionSrc := buildT0AuctionQuotes(tradeDate, step2, dailyCache)
	logger.SugaredLogger.Infof("[T0选股] [4/5] 竞价开盘价就绪: %d只 来源=%s (%.1fs)",
		len(auction), auctionSrc, time.Since(t4).Seconds())

	// ── 5. T0 竞价开盘涨幅过滤 ──
	t5 := time.Now()
	step4, gapCache := filterOpenGap(step2, auction, 0.01, 3.0)
	logger.SugaredLogger.Infof("[T0选股] [5/5] T0竞价开盘过滤: %d -> %d只 (%.1fs)",
		len(step2), len(step4), time.Since(t5).Seconds())

	if len(step4) == 0 {
		logger.SugaredLogger.Infof("[T0选股] T0开盘过滤后无股票，总耗时: %.1fs", time.Since(tStart).Seconds())
		return nil, fmt.Errorf("T0开盘过滤后无股票")
	}

	// ── 6. 组装结果 ──
	t6 := time.Now()
	var results []T0SelectionResult
	for _, s := range step4 {
		hist := histCache[s.ShortCode]
		full := dailyCache[s.ShortCode]
		openGap := gapCache[s.ShortCode]
		ma20 := calcMA20(hist)
		rt := auction[s.ShortCode]

		if len(hist) == 0 {
			continue
		}
		prevClose := hist[len(hist)-1].Close
		if rt.PrevClose > 0 {
			prevClose = rt.PrevClose
		}
		prevAmountYi := hist[len(hist)-1].AmountYi

		var prevRet float64
		if len(hist) >= 2 && hist[len(hist)-2].Close != 0 {
			prevRet = (hist[len(hist)-1].Close - hist[len(hist)-2].Close) / hist[len(hist)-2].Close * 100
		}

		// 收盘涨幅：回测用当日收盘；盘中用现价（若有）
		var t0CloseRet float64
		closePx := rt.Close
		if closePx == 0 && len(full) > 0 && full[len(full)-1].Date == tradeDate {
			closePx = full[len(full)-1].Close
		}
		if closePx != 0 && prevClose != 0 {
			t0CloseRet = (closePx - prevClose) / prevClose * 100
		}

		limitUpInfo := formatLimitUpDates(collectLimitUpMemoryDates(hist, t0LimitUpMemoryDays, t0LimitUpCloseRet))

		tag := ""
		if highRet, openRet, prevDayCloseRet, ok := prevDayRetsFromHist(hist); ok {
			tag = pickPrevDayTag(highRet, openRet, prevDayCloseRet)
		}

		var marketSuffix string
		if strings.HasPrefix(s.Code, "sh") {
			marketSuffix = ".XSHG"
		} else {
			marketSuffix = ".XSHE"
		}
		userCode := s.ShortCode + marketSuffix

		results = append(results, T0SelectionResult{
			Time:         tradeDate,
			OpenGap:      round2(openGap),
			CloseRet:     round2(t0CloseRet),
			LimitUpDates: limitUpInfo,
			MA20:         round2(ma20),
			AmountYi:     round2(prevAmountYi),
			StockCode:    userCode,
			StockName:    s.Name,
			PrevClose:    round2(prevClose),
			PrevCloseRet: round2(prevRet),
			Tag:          tag,
		})
		enrichResultWithPattern(&results[len(results)-1], hist)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].OpenGap > results[j].OpenGap
	})

	logger.SugaredLogger.Infof("[T0选股] 结果组装完成: %d只 (%.1fs)",
		len(results), time.Since(t6).Seconds())
	logger.SugaredLogger.Infof("[T0选股] 总耗时: %.1fs", time.Since(tStart).Seconds())

	return results, nil
}

func round2(v float64) float64 {
	// math.Round 对负数同样按绝对值四舍五入；早期的 int(v*100+0.5) 会向零截断，
	// 导致所有下跌幅度偏小 0.01。
	return math.Round(v*100) / 100
}

// ── API Handler ─────────────────────────────────────────────────────────────

func isTruthyQuery(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}

// t0AuctionCutoffHM 竞价确认可用的最早时分（含）：09:25
const t0AuctionCutoffHM = 9*60 + 25

func chinaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

// t0AutoPrewarmEndHM 主动预热窗口结束时分（不含）：09:00
const t0AutoPrewarmEndHM = 9 * 60

// shouldAutoPrewarmT0 判断 now 是否落在「周一到周五 00:00~09:00（上海时区）」主动预热窗口。
// 09:00 之后不再主动拉取，交给请求触发的预热逻辑，避免与竞价确认窗口抢资源。
func shouldAutoPrewarmT0(now time.Time) bool {
	local := now.In(chinaLocation())
	switch local.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	return local.Hour()*60+local.Minute() < t0AutoPrewarmEndHM
}

// isPreopenPrevResultWindow 判断 now 是否处于「上海时区当天 00:00~09:00」且 tradeDate 即当天。
// 命中时主板策略可展示前一交易日归档；09:00 起交回等待流程。
func isPreopenPrevResultWindow(now time.Time, tradeDate string) bool {
	local := now.In(chinaLocation())
	if local.Format("2006-01-02") != tradeDate {
		return false
	}
	return local.Hour()*60+local.Minute() < t0AutoPrewarmEndHM
}

// isBeforeT0AuctionCutoff 判断 now 是否仍早于当日 09:25（上海时区）。
// 仅当 tradeDate 等于「上海时区的今天」时，预竞价窗口才生效。
func isBeforeT0AuctionCutoff(now time.Time, tradeDate string) bool {
	loc := chinaLocation()
	local := now.In(loc)
	today := local.Format("2006-01-02")
	if tradeDate != today {
		return false
	}
	minutes := local.Hour()*60 + local.Minute()
	return minutes < t0AuctionCutoffHM
}

func writeT0PrewarmHTTP(w http.ResponseWriter, tradeDate string) {
	now := time.Now()
	ensurePrevTradingDayBackfillStarted(tradeDate, now)
	if isPrevDayBackfillInProgress(tradeDate) {
		syncBackfillDailyProgress(tradeDate, getT0WarmProgress(tradeDate).BackfillDate)
		WriteJSON(w, buildPrewarmProgressResponse(tradeDate, getT0WarmProgress(tradeDate)))
		return
	}
	if isT0DailyCacheFilePresent(tradeDate) {
		WriteJSON(w, buildPrewarmReadyResponse(tradeDate))
		return
	}
	tryStartT0Prewarm(tradeDate)
	if isT0DailyCacheFilePresent(tradeDate) {
		WriteJSON(w, buildPrewarmReadyResponse(tradeDate))
		return
	}
	WriteJSON(w, buildPrewarmProgressResponse(tradeDate, getT0WarmProgress(tradeDate)))
}

// handleT0Selection 处理 /api/t0-selection 请求
func handleT0Selection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tradeDate := r.URL.Query().Get("date")
	if tradeDate == "" {
		tradeDate = time.Now().In(chinaLocation()).Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", tradeDate); err != nil {
		WriteJSON(w, map[string]interface{}{
			"error": fmt.Sprintf("日期格式错误: %s (需为 2006-01-02)", tradeDate),
			"date":  tradeDate,
		})
		return
	}

	q := r.URL.Query()

	// list_dates：返回所有有效选股归档日期（降序）
	if isTruthyQuery(q.Get("list_dates")) {
		WriteJSON(w, map[string]interface{}{
			"dates": listSelectionArchiveDates(),
		})
		return
	}

	// archived 优先
	if isTruthyQuery(q.Get("archived")) {
		a, ok := loadT0SelectionArchive(tradeDate)
		if !ok {
			WriteJSON(w, map[string]interface{}{
				"error":    "该日无选股归档",
				"date":     tradeDate,
				"archived": true,
				"count":    0,
			})
			return
		}
		WriteJSON(w, map[string]interface{}{
			"date":             a.Date,
			"archived":         true,
			"saved_at":         a.SavedAt,
			"close_updated_at": a.CloseUpdatedAt,
			"count":            a.Count,
			"results":          sortT0ResultsForClient(enrichArchivedResults(tradeDate, a.Results)),
		})
		return
	}

	// refresh_tags：用日线缓存补算历史归档的前日标记
	if isTruthyQuery(q.Get("refresh_tags")) {
		out, err := refreshSelectionTags(tradeDate)
		if err != nil {
			WriteJSON(w, map[string]interface{}{
				"error": err.Error(),
				"date":  tradeDate,
			})
			return
		}
		WriteJSON(w, out)
		return
	}

	// refresh_close：只刷新归档中的 T0收盘涨幅，手动调用一律强制执行
	if isTruthyQuery(q.Get("refresh_close")) {
		out, err := refreshSelectionCloseRet(tradeDate, true)
		if err != nil {
			WriteJSON(w, map[string]interface{}{
				"error": err.Error(),
				"date":  tradeDate,
			})
			return
		}
		WriteJSON(w, out)
		return
	}

	explicitPrewarm := isTruthyQuery(q.Get("prewarm"))
	// 凌晨～09:25：同一正式选股 URL 自动走预热，避免竞价未出时误选股；
	// 已有日线缓存则返回 ready，正在预热则返回 warming 进度。
	autoPrewarm := !explicitPrewarm && isBeforeT0AuctionCutoff(time.Now(), tradeDate)

	if explicitPrewarm || autoPrewarm {
		writeT0PrewarmHTTP(w, tradeDate)
		return
	}

	if shouldReturnWarmingForSelection(tradeDate) {
		WriteJSON(w, map[string]interface{}{
			"date":    tradeDate,
			"status":  string(t0WarmStatusWarming),
			"count":   0,
			"results": []T0SelectionResult{},
		})
		return
	}

	forceSave := isTruthyQuery(q.Get("save"))
	results, err := RunT0Selection(tradeDate)
	if err != nil {
		WriteJSON(w, map[string]interface{}{
			"error":   err.Error(),
			"date":    tradeDate,
			"count":   0,
			"results": []T0SelectionResult{},
		})
		return
	}

	if saveErr := saveT0SelectionArchive(tradeDate, results, forceSave); saveErr != nil {
		logger.SugaredLogger.Warnf("[T0选股] 结果归档写入失败: %v", saveErr)
	}

	WriteJSON(w, map[string]interface{}{
		"date":    tradeDate,
		"count":   len(results),
		"results": sortT0ResultsForClient(results),
	})
}
