package flutter_api

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"
)

// Kept separately from daily.gob: refreshing/exporting daily prices must never
// be the only durable copy of the point-in-time selection universe.
type t0PoolArchive struct {
	SchemaVersion int       `json:"schema_version"`
	Date          string    `json:"date"`
	ArchivedAt    string    `json:"archived_at"`
	Source        string    `json:"source"`
	Complete      bool      `json:"complete"`
	Stocks        []t0Stock `json:"stocks"`
}

var t0PoolArchiveMu sync.Mutex
var t0PoolCaptureMu sync.Mutex
var t0PoolFetcher = fetchStockPoolFromSinaStrict
var t0PoolCaptureLastAttempt time.Time

func t0PoolArchivePath(date string) string {
	return filepath.Join(t0CacheRootPath, "t0", "pools", "pool_"+date+".json")
}

func validateT0PoolArchive(a t0PoolArchive, date string) error {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil || parsed.Format("2006-01-02") != date || a.Date != date || a.SchemaVersion != 1 || !a.Complete || len(a.Stocks) == 0 || a.Source == "" {
		return fmt.Errorf("股票池归档日期/版本/完整性无效: %s", date)
	}
	seen := map[string]bool{}
	for _, s := range a.Stocks {
		code := s.ShortCode
		if len(code) != 6 || !(strings.HasPrefix(code, "60") || strings.HasPrefix(code, "00")) || strings.Trim(code, "0123456789") != "" || seen[code] || math.IsNaN(s.MarketCapYi) || math.IsInf(s.MarketCapYi, 0) || s.MarketCapYi < 0 {
			return fmt.Errorf("股票池归档含无效或重复股票: %s %s", date, code)
		}
		seen[code] = true
	}
	return nil
}

func readT0PoolArchive(date string) (*t0PoolArchive, error) {
	if parsed, err := time.Parse("2006-01-02", date); err != nil || parsed.Format("2006-01-02") != date {
		return nil, fmt.Errorf("股票池日期无效: %q", date)
	}
	b, err := os.ReadFile(t0PoolArchivePath(date))
	if err != nil {
		return nil, err
	}
	var a t0PoolArchive
	if err = json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("读取股票池归档 %s: %w", date, err)
	}
	if err = validateT0PoolArchive(a, date); err != nil {
		return nil, err
	}
	return &a, nil
}

func saveT0PoolArchive(date string, stocks []t0Stock, source string) error {
	t0PoolArchiveMu.Lock()
	defer t0PoolArchiveMu.Unlock()
	a := t0PoolArchive{SchemaVersion: 1, Date: date, ArchivedAt: time.Now().In(chinaLocation()).Format(time.RFC3339), Source: source, Complete: true, Stocks: append([]t0Stock(nil), stocks...)}
	if err := validateT0PoolArchive(a, date); err != nil {
		return err
	}
	old, err := readT0PoolArchive(date)
	if err == nil {
		if sameT0StockPool(old.Stocks, stocks) {
			return nil
		}
		if date != time.Now().In(chinaLocation()).Format("2006-01-02") {
			return fmt.Errorf("历史股票池归档冲突，拒绝覆盖: %s", date)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	sort.Slice(a.Stocks, func(i, j int) bool { return a.Stocks[i].ShortCode < a.Stocks[j].ShortCode })
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(t0PoolArchivePath(date))
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".pool-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), t0PoolArchivePath(date))
}

func restoreT0PoolArchives(snapshot *MainboardFloatCap50Gob) map[string]string {
	pools, issues, _ := collectT0PoolSources(*snapshot)
	snapshot.SelectionPools = map[string][]MainboardT0PoolStock{}
	for date, source := range pools {
		if issues[date] != "" {
			continue
		}
		if err := saveT0PoolArchive(date, source.Stocks, source.Source); err != nil {
			issues[date] = err.Error()
			continue
		}
		pool := make([]MainboardT0PoolStock, 0, len(source.Stocks))
		for _, s := range source.Stocks {
			pool = append(pool, MainboardT0PoolStock{Code: s.ShortCode, Name: s.Name, MarketCapYi: s.MarketCapYi})
		}
		snapshot.SelectionPools[date] = pool
	}
	for date, issue := range issues {
		logger.SugaredLogger.Errorf("[股票池留档] 隔离日期 %s: %s", date, issue)
	}
	return issues
}

// Independent of the slow daily-bar refresh. Missing pools are captured even
// when the service first starts after 09:25; nothing is backdated as a live pool.
func runT0PoolArchiveTick(now time.Time) {
	local := now.In(chinaLocation())
	date := local.Format("2006-01-02")
	if isT0WeekendDate(date) || local.Hour()*60+local.Minute() >= 15*60+5 {
		return
	}
	t0PoolCaptureMu.Lock()
	defer t0PoolCaptureMu.Unlock()
	if _, err := readT0PoolArchive(date); err == nil {
		return
	} else if !os.IsNotExist(err) {
		logger.SugaredLogger.Errorf("[股票池留档] %v", err)
		return
	}
	if !t0PoolCaptureLastAttempt.IsZero() && local.Sub(t0PoolCaptureLastAttempt) >= 0 && local.Sub(t0PoolCaptureLastAttempt) < 5*time.Minute {
		return
	}
	t0PoolCaptureLastAttempt = local
	stocks, err := t0PoolFetcher()
	if err == nil {
		err = saveT0PoolArchive(date, stocks, "sina_live_complete")
	}
	if err != nil {
		logger.SugaredLogger.Errorf("[股票池留档] %s 获取或写入失败，5分钟后重试: %v", date, err)
		return
	}
	logger.SugaredLogger.Infof("[股票池留档] %s 已独立保存%d只，日线失败不影响池归档", date, len(stocks))
}
