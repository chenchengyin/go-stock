package data

import (
	"encoding/json"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"
	"math"
	"sort"
	"testing"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/random"
)

func TestSearchStock(t *testing.T) {
	db.Init("../../data/stock.db")

	e := convertor.ToString(math.Floor(float64(9*random.RandFloat(0, 1, 12) + 1)))
	for i := 0; i < 19; i++ {
		e += convertor.ToString(math.Floor(float64(9 * random.RandFloat(0, 1, 12))))
	}
	logger.SugaredLogger.Infof("e:%s", e)

	//res := NewSearchStockApi("量比大于2，基本面优秀，2025年三季报已披露，主力连续3日净流入，非创业板非科创板非ST").SearchStock(20)
	//res := NewSearchStockApi("今日涨幅前5的概念板块").SearchBk(50)
	res := NewSearchStockApi("今日涨幅前15的ETF").SearchETF(50)

	logger.SugaredLogger.Infof("res:%+v", res)
	data := res["data"].(map[string]any)
	result := data["result"].(map[string]any)
	dataList := result["dataList"].([]any)
	columns := result["columns"].([]any)
	headers := map[string]string{}
	for _, v := range columns {
		//logger.SugaredLogger.Infof("v:%+v", v)
		d := v.(map[string]any)
		//logger.SugaredLogger.Infof("key:%s title:%s dateMsg:%s unit:%s", d["key"], d["title"], d["dateMsg"], d["unit"])
		title := convertor.ToString(d["title"])
		if convertor.ToString(d["dateMsg"]) != "" {
			title = title + "[" + convertor.ToString(d["dateMsg"]) + "]"
		}
		if convertor.ToString(d["unit"]) != "" {
			title = title + "(" + convertor.ToString(d["unit"]) + ")"
		}
		headers[d["key"].(string)] = title
	}
	table := &[]map[string]any{}
	for _, v := range dataList {
		//logger.SugaredLogger.Infof("v:%+v", v)
		d := v.(map[string]any)
		tmp := map[string]any{}
		for key, title := range headers {
			//logger.SugaredLogger.Infof("%s:%s", title, convertor.ToString(d[key]))
			tmp[title] = convertor.ToString(d[key])
		}
		*table = append(*table, tmp)
		//logger.SugaredLogger.Infof("--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------")
	}
	jsonData, _ := json.Marshal(*table)
	markdownTable, _ := JSONToMarkdownTable(jsonData)
	logger.SugaredLogger.Infof("markdownTable=\n%s", markdownTable)
}

func TestGetStockFinancialInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewStockDataApi().GetStockFinancialInfo("300390.SZ")
	MD := util.MarkdownTableWithTitle("300390.SZ股票财报信息", res.Result.Data)
	logger.SugaredLogger.Infof("res:\n%s", MD)
}
func TestGetStockHolderNum(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewStockDataApi().GetStockHolderNum("300390.SZ")
	MD := util.MarkdownTableWithTitle("股票股东人数信息", res.Result.Data)
	logger.SugaredLogger.Infof("res:\n%s", MD)
}

func TestSearchStock_CurrentChangeRange(t *testing.T) {
	db.Init("../../data/stock.db")

	query := "当前涨幅在0.01%到3%之间"
	res := NewSearchStockApi(query).SearchStock(50)

	logger.SugaredLogger.Infof("方案A测试 - 查询条件: %s", query)
	logger.SugaredLogger.Infof("方案A测试 - 原始响应: %+v", res)

	code, ok := res["code"].(float64)
	if ok && code != 0 {
		logger.SugaredLogger.Infof("方案A测试 - 接口返回错误, code=%v, message=%v", code, res["message"])
		return
	}

	data, ok := res["data"].(map[string]any)
	if !ok {
		logger.SugaredLogger.Infof("方案A测试 - 响应中无 data 字段")
		return
	}
	result, ok := data["result"].(map[string]any)
	if !ok {
		logger.SugaredLogger.Infof("方案A测试 - 响应中无 result 字段")
		return
	}
	dataList, ok := result["dataList"].([]any)
	if !ok {
		logger.SugaredLogger.Infof("方案A测试 - 响应中无 dataList 字段")
		return
	}

	logger.SugaredLogger.Infof("方案A测试 - 命中股票数量: %d", len(dataList))
	for i, v := range dataList {
		if i >= 10 {
			break
		}
		d := v.(map[string]any)
		logger.SugaredLogger.Infof("方案A测试 - 第%d只: code=%s name=%s", i+1, d["SECURITY_CODE"], d["SECURITY_NAME_ABBR"])
	}
}

func TestSearchStockApi_HotStrategy(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewSearchStockApi("").HotStrategy()
	bytes, err := json.Marshal(res)
	if err != nil {
		return
	}
	strategy := &models.HotStrategy{}
	json.Unmarshal(bytes, strategy)
	for _, data := range strategy.Data {
		data.Chg = mathutil.RoundToFloat(100*data.Chg, 2)
	}
	markdownTable := util.MarkdownTable(strategy.Data)
	logger.SugaredLogger.Infof("res:%s", markdownTable)
	//dataList := res["data"].([]any)
	//for _, v := range dataList {
	//	d := v.(map[string]any)
	//	logger.SugaredLogger.Infof("v:%+v", d)
	//}
}
func TestSearchStockApi_HotStrategyTable(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewSearchStockApi("").StrategySquare()
	logger.SugaredLogger.Infof("res:%+v", res)
}

func TestSearchStock_UserStrategy(t *testing.T) {
	db.Init("../../data/stock.db")

	query := "竞价涨幅在0.01%到3%之间;前7个交易日单日最大涨幅大于9.8%;前一天成交金额大于5亿;流通市值在60亿到8000亿之间;主板"
	res := NewSearchStockApi(query).SearchStock(50)

	logger.SugaredLogger.Infof("用户策略选股 - 查询条件: %s", query)
	logger.SugaredLogger.Infof("用户策略选股 - 原始响应: %+v", res)

	code := res["code"]
	msg := res["msg"]
	logger.SugaredLogger.Infof("用户策略选股 - 接口返回 code=%v msg=%v", code, msg)

	data, ok := res["data"].(map[string]any)
	if !ok {
		t.Fatal("用户策略选股 - 响应中无 data 字段")
	}
	result, ok := data["result"].(map[string]any)
	if !ok {
		t.Fatal("用户策略选股 - 响应中无 result 字段")
	}
	dataList, ok := result["dataList"].([]any)
	if !ok {
		t.Fatal("用户策略选股 - 响应中无 dataList 字段")
	}

	logger.SugaredLogger.Infof("用户策略选股 - 命中股票数量: %d", len(dataList))
	if len(dataList) == 0 {
		t.Fatal("用户策略选股 - 命中股票数量为 0")
	}

	// 按今日涨幅 PCHG 降序排列
	sort.Slice(dataList, func(i, j int) bool {
		di := dataList[i].(map[string]any)
		dj := dataList[j].(map[string]any)
		pi, _ := convertor.ToFloat(di["PCHG"])
		pj, _ := convertor.ToFloat(dj["PCHG"])
		return pi > pj
	})

	for i, v := range dataList {
		d := v.(map[string]any)
		logger.SugaredLogger.Infof("用户策略选股 - 第%d只: code=%s name=%s 今日涨幅=%v", i+1, d["SECURITY_CODE"], d["SECURITY_SHORT_NAME"], d["PCHG"])
	}
}
