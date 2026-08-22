package candlepattern

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

func init() { RegisterGobTypes() }

func RegisterGobTypes() {
	gob.RegisterName("flutter_api.dailyBar", DailyBar{})
	gob.RegisterName("flutter_api.t0Stock", StockMeta{})
	gob.RegisterName("flutter_api.t0DailyCachePayload", dailyCacheGob{})
}

type dailyCacheGob struct {
	TradeDate string
	Stocks    []StockMeta
	Daily     map[string][]DailyBar
}

func ResolveCacheRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if fi, err := os.Stat(filepath.Join(dir, "backend")); err == nil && fi.IsDir() {
				return filepath.Join(dir, "backend", "data", "cache"), nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("project root not found from %s", startDir)
		}
		dir = parent
	}
}

func LoadDailyCache(cacheRoot, tradeDate string) (*DailyCache, error) {
	path := filepath.Join(cacheRoot, "t0", "daily", "t0_daily_cache_"+tradeDate+".gob")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var payload dailyCacheGob
	if err := gob.NewDecoder(f).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode gob %s: %w", path, err)
	}
	return &DailyCache{
		TradeDate: payload.TradeDate,
		Stocks:    payload.Stocks,
		Daily:     payload.Daily,
	}, nil
}
