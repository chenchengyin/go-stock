package flutter_api

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const t0ArchiveBackfillDailyLimit = 800

// T0ArchiveBackfillOptions 控制历史选股归档的字段补全。
type T0ArchiveBackfillOptions struct {
	Limit       int
	Reverse     bool
	Concurrency int
}

// T0ArchiveBackfillReport 是一次补全批次的执行结果。
type T0ArchiveBackfillReport struct {
	SelectedArchives  int
	CompletedArchives int
	FailedArchives    int
	EnrichedResults   int
	MissingDailyData  int
	FirstDate         string
	LastDate          string
}

type t0ArchiveBackfillItem struct {
	Date    string
	Archive *t0SelectionArchive
}

// BackfillT0SelectionArchives 按归档日期顺序补全缺少“买入信号”的历史 JSON。
//
// 每个归档只查询该归档已经入选的股票，不会重新拉取完整股票池；已有的
// t0 daily 缓存会优先复用，缓存缺少个股时才按个股补拉日线。补全成功后用
// 临时文件 + rename 原子覆盖归档，避免留下半个 JSON。
func BackfillT0SelectionArchives(cacheDir string, options T0ArchiveBackfillOptions) (T0ArchiveBackfillReport, error) {
	if strings.TrimSpace(cacheDir) != "" {
		abs, err := filepath.Abs(cacheDir)
		if err != nil {
			return T0ArchiveBackfillReport{}, err
		}
		t0CacheRootPath = abs
	}
	if err := ensureT0CacheDirs(); err != nil {
		return T0ArchiveBackfillReport{}, err
	}

	items, err := incompleteT0ArchiveItems(options.Reverse, options.Limit)
	if err != nil {
		return T0ArchiveBackfillReport{}, err
	}
	report := T0ArchiveBackfillReport{SelectedArchives: len(items)}
	if len(items) == 0 {
		return report, nil
	}
	report.FirstDate = items[0].Date
	report.LastDate = items[len(items)-1].Date

	concurrency := options.Concurrency
	if concurrency <= 0 {
		concurrency = 20
	}
	for _, item := range items {
		enriched, missing, err := backfillT0Archive(item, concurrency)
		report.EnrichedResults += enriched
		report.MissingDailyData += missing
		if err != nil {
			report.FailedArchives++
			continue
		}
		report.CompletedArchives++
	}
	return report, nil
}

func incompleteT0ArchiveItems(reverse bool, limit int) ([]t0ArchiveBackfillItem, error) {
	dates := listSelectionArchiveDates()
	if !reverse {
		sort.Strings(dates)
	}

	items := make([]t0ArchiveBackfillItem, 0, len(dates))
	for _, date := range dates {
		a, ok := loadT0SelectionArchive(date)
		if !ok || !t0ArchiveNeedsBackfill(a) {
			continue
		}
		items = append(items, t0ArchiveBackfillItem{Date: date, Archive: a})
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items, nil
}

func t0ArchiveNeedsBackfill(a *t0SelectionArchive) bool {
	if a == nil {
		return false
	}
	for _, result := range a.Results {
		if strings.TrimSpace(result.BuySignal) == "" {
			return true
		}
	}
	return false
}

func backfillT0Archive(item t0ArchiveBackfillItem, concurrency int) (int, int, error) {
	a := item.Archive
	if a == nil {
		return 0, 0, fmt.Errorf("归档为空: %s", item.Date)
	}

	shortCodes := make(map[string]struct{})
	for _, result := range a.Results {
		if strings.TrimSpace(result.BuySignal) == "" {
			shortCodes[t0ShortCodeFromResultCode(result.StockCode)] = struct{}{}
		}
	}
	daily, missing := loadT0ArchiveDaily(item.Date, shortCodes, concurrency)
	if missing > 0 {
		return 0, missing, fmt.Errorf("%s 缺少 %d 只股票日线", item.Date, missing)
	}

	enriched := 0
	for i := range a.Results {
		if strings.TrimSpace(a.Results[i].BuySignal) != "" {
			continue
		}
		sc := t0ShortCodeFromResultCode(a.Results[i].StockCode)
		bars, ok := daily[sc]
		if !ok || len(bars) == 0 {
			return enriched, 1, fmt.Errorf("%s %s 无日线", item.Date, sc)
		}
		enrichResultWithPattern(&a.Results[i], histBarsBeforeTradeDate(bars, item.Date))
		if strings.TrimSpace(a.Results[i].BuySignal) == "" {
			return enriched, 1, fmt.Errorf("%s %s 补全后仍无买入信号", item.Date, sc)
		}
		enriched++
	}

	if err := saveT0SelectionArchiveFull(a, true); err != nil {
		return enriched, 0, fmt.Errorf("%s 写入失败: %w", item.Date, err)
	}
	return enriched, 0, nil
}

func loadT0ArchiveDaily(tradeDate string, shortCodes map[string]struct{}, concurrency int) (map[string][]dailyBar, int) {
	daily := make(map[string][]dailyBar, len(shortCodes))
	if cached, ok := loadT0DailyCacheForBackfill(tradeDate); ok {
		for sc := range shortCodes {
			if bars, found := cached.Daily[sc]; found && len(bars) > 0 {
				daily[sc] = bars
			}
		}
	}

	missingCodes := make([]string, 0)
	for sc := range shortCodes {
		if _, ok := daily[sc]; !ok {
			missingCodes = append(missingCodes, sc)
		}
	}
	if len(missingCodes) == 0 {
		return daily, 0
	}

	if concurrency <= 0 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, sc := range missingCodes {
		wg.Add(1)
		go func(shortCode string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// 2025 年归档已经超出近 60 根日线窗口；补全任务取更长窗口，
			// 再由 histBarsBeforeTradeDate 截取归档日期之前的历史。
			bars := fetchDailyKLineWithLimitSafely(shortCode)
			if len(bars) < 2 {
				return
			}
			mu.Lock()
			daily[shortCode] = bars
			mu.Unlock()
		}(sc)
	}
	wg.Wait()

	missing := 0
	for sc := range shortCodes {
		if _, ok := daily[sc]; !ok {
			missing++
		}
	}
	return daily, missing
}

func fetchDailyKLineWithLimitSafely(shortCode string) (bars []dailyBar) {
	defer func() {
		if recover() != nil {
			bars = nil
		}
	}()
	return fetchDailyKLineWithLimit(shortCode, "", t0ArchiveBackfillDailyLimit)
}

// loadT0DailyCacheForBackfill 允许一次性归档补全读取旧版日线缓存。
// 旧缓存可能没有 StockPoolComplete 标记，但其中的个股日线仍可用于计算
// 已存在归档的形态；正式选股链路仍继续使用严格的 loadT0DailyCache。
func loadT0DailyCacheForBackfill(tradeDate string) (*t0DailyCachePayload, bool) {
	path := t0DailyCachePath(tradeDate)
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	var payload t0DailyCachePayload
	if err := gob.NewDecoder(f).Decode(&payload); err != nil {
		return nil, false
	}
	if payload.TradeDate != "" && payload.TradeDate != tradeDate {
		return nil, false
	}
	if len(payload.Daily) == 0 {
		return nil, false
	}
	return &payload, true
}

// T0ArchiveBackfillDateRange 返回指定批次的日期范围，供一次性命令输出使用。
func T0ArchiveBackfillDateRange(report T0ArchiveBackfillReport) (string, string) {
	if report.FirstDate == "" || report.LastDate == "" {
		return "", ""
	}
	return report.FirstDate, report.LastDate
}

// T0ArchiveBackfillNow 保留统一的日期格式入口，避免一次性命令依赖本地时区。
func T0ArchiveBackfillNow() string {
	return time.Now().In(chinaLocation()).Format(time.RFC3339)
}
