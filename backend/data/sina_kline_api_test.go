package data

import (
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"testing"
)

func init() {
	db.Init("../../data/stock.db")
}

func TestSinaKLineApi_GetDayKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewSinaKLineApi(config)
	kLines := api.GetKLineData("600519.SH", "101", 10)
	logger.SugaredLogger.Infof("Sina 日K: 获取到 %d 条数据", len(*kLines))
	if len(*kLines) == 0 {
		t.Error("Sina 日K数据为空")
		return
	}
	first := (*kLines)[0]
	logger.SugaredLogger.Infof("Sina 日K首条: day=%s open=%s close=%s high=%s low=%s vol=%s",
		first.Day, first.Open, first.Close, first.High, first.Low, first.Volume)
}

func TestSinaKLineApi_Get5MinKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewSinaKLineApi(config)
	kLines := api.GetKLineData("000001.SZ", "5", 10)
	logger.SugaredLogger.Infof("Sina 5分钟K: 获取到 %d 条数据", len(*kLines))
	if len(*kLines) == 0 {
		t.Error("Sina 5分钟K数据为空")
	}
}

func TestTencentKLineApi_GetDayKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewTencentKLineApi(config)
	kLines := api.GetKLineData("600519.SH", "101", 10)
	logger.SugaredLogger.Infof("Tencent 日K: 获取到 %d 条数据", len(*kLines))
	if len(*kLines) == 0 {
		t.Error("Tencent 日K数据为空")
		return
	}
	first := (*kLines)[0]
	logger.SugaredLogger.Infof("Tencent 日K首条: day=%s open=%s close=%s high=%s low=%s vol=%s",
		first.Day, first.Open, first.Close, first.High, first.Low, first.Volume)
}

func TestTencentKLineApi_GetWeekKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewTencentKLineApi(config)
	kLines := api.GetKLineData("600519.SH", "102", 10)
	logger.SugaredLogger.Infof("Tencent 周K: 获取到 %d 条数据", len(*kLines))
	if len(*kLines) == 0 {
		t.Error("Tencent 周K数据为空")
	}
}

func TestFetchKLineWithFallback(t *testing.T) {
	result := FetchKLineWithFallback("600519.SH", "贵州茅台", "101", 10, "")
	logger.SugaredLogger.Infof("Fallback 日K: source=%s, 数据条数=%d", result.Source, len(*result.Data))
	if len(*result.Data) == 0 {
		t.Error("Fallback 日K数据为空")
	}
	if result.Source == "" {
		t.Error("Fallback 未识别数据源")
	}
}
