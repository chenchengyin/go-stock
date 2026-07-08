package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"
	"time"

	"github.com/duke-git/lancet/v2/mathutil"
)

// @Author spark
// @Date 2025/6/28 21:02
// @Desc
// -----------------------------------------------------------------------------------
type SearchStockApi struct {
	words string
}

func NewSearchStockApi(words string) *SearchStockApi {
	return &SearchStockApi{words: words}
}
func (s SearchStockApi) SearchStock(pageSize int) map[string]any {
	qgqpBId := NewSettingsApi().Config.QgqpBId
	if qgqpBId == "" {
		return map[string]any{
			"code":    -1,
			"message": "请先获取东财用户标识（qgqp_b_id）：打开浏览器,访问东财网站，按F12打开开发人员工具-》网络面板，随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。保存到设置中的东财唯一标识输入框",
		}
	}
	url := "https://np-tjxg-g.eastmoney.com/api/smart-tag/stock/v3/pw/search-code"
	bodyMap := map[string]any{
		"keyWord":                s.words,
		"pageSize":               pageSize,
		"pageNo":                 1,
		"fingerprint":            qgqpBId,
		"gids":                   []any{},
		"matchWord":              "",
		"timestamp":              fmt.Sprintf("%d", time.Now().Unix()),
		"shareToGuba":            false,
		"requestId":              "",
		"needCorrect":            true,
		"removedConditionIdList": []any{},
		"xcId":                   "",
		"ownSelectAll":           false,
		"dxInfo":                 []any{},
		"extraCondition":         "",
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchStock-marshal-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	logger.SugaredLogger.Infof("SearchStock-req: keyWord=%s pageSize=%d", s.words, pageSize)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-tjxg-g.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0").
		SetHeader("Content-Type", "application/json").
		SetBody(bodyBytes).Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchStock-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	count := 0
	if data, ok := respMap["data"].(map[string]any); ok {
		if result, ok := data["result"].(map[string]any); ok {
			if list, ok := result["dataList"].([]any); ok {
				count = len(list)
			}
		}
	}
	logger.SugaredLogger.Infof("SearchStock-resp: code=%v msg=%v count=%d", respMap["code"], respMap["msg"], count)
	return respMap
}

func (s SearchStockApi) SearchBk(pageSize int) map[string]any {
	url := "https://np-tjxg-b.eastmoney.com/api/smart-tag/bkc/v3/pw/search-code"
	qgqpBId := NewSettingsApi().Config.QgqpBId
	if qgqpBId == "" {
		return map[string]any{
			"code":    -1,
			"message": "请先获取东财用户标识（qgqp_b_id）：打开浏览器,访问东财网站，按F12打开开发人员工具-》网络面板，随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。保存到设置中的东财唯一标识输入框",
		}
	}
	bodyMap := map[string]any{
		"keyWord":                s.words,
		"pageSize":               pageSize,
		"pageNo":                 1,
		"fingerprint":            qgqpBId,
		"gids":                   []any{},
		"matchWord":              "",
		"timestamp":              fmt.Sprintf("%d", time.Now().Unix()),
		"shareToGuba":            false,
		"requestId":              "",
		"needCorrect":            true,
		"removedConditionIdList": []any{},
		"xcId":                   "",
		"ownSelectAll":           false,
		"dxInfo":                 []any{},
		"extraCondition":         "",
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchBk-marshal-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	logger.SugaredLogger.Infof("SearchBk-req: keyWord=%s pageSize=%d", s.words, pageSize)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-tjxg-g.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0").
		SetHeader("Content-Type", "application/json").
		SetBody(bodyBytes).Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchStock-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	count := 0
	if data, ok := respMap["data"].(map[string]any); ok {
		if result, ok := data["result"].(map[string]any); ok {
			if list, ok := result["dataList"].([]any); ok {
				count = len(list)
			}
		}
	}
	logger.SugaredLogger.Infof("SearchBk-resp: code=%v msg=%v count=%d", respMap["code"], respMap["msg"], count)
	return respMap
}

func (s SearchStockApi) SearchETF(pageSize int) map[string]any {
	url := "https://np-tjxg-b.eastmoney.com/api/smart-tag/etf/v3/pw/search-code"
	qgqpBId := NewSettingsApi().Config.QgqpBId
	if qgqpBId == "" {
		return map[string]any{
			"code":    -1,
			"message": "请先获取东财用户标识（qgqp_b_id）：打开浏览器,访问东财网站，按F12打开开发人员工具-》网络面板，随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。保存到设置中的东财唯一标识输入框",
		}
	}
	bodyMap := map[string]any{
		"keyWord":                s.words,
		"pageSize":               pageSize,
		"pageNo":                 1,
		"fingerprint":            qgqpBId,
		"gids":                   []any{},
		"matchWord":              "",
		"timestamp":              fmt.Sprintf("%d", time.Now().Unix()),
		"shareToGuba":            false,
		"requestId":              "",
		"needCorrect":            true,
		"removedConditionIdList": []any{},
		"xcId":                   "",
		"ownSelectAll":           false,
		"dxInfo":                 []any{},
		"extraCondition":         "",
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchETF-marshal-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	logger.SugaredLogger.Infof("SearchETF-req: keyWord=%s pageSize=%d", s.words, pageSize)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-tjxg-g.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0").
		SetHeader("Content-Type", "application/json").
		SetBody(bodyBytes).Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchETF-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	count := 0
	if data, ok := respMap["data"].(map[string]any); ok {
		if result, ok := data["result"].(map[string]any); ok {
			if list, ok := result["dataList"].([]any); ok {
				count = len(list)
			}
		}
	}
	logger.SugaredLogger.Infof("SearchETF-resp: code=%v msg=%v count=%d", respMap["code"], respMap["msg"], count)
	return respMap
}

func (s SearchStockApi) HotStrategy() map[string]any {
	url := fmt.Sprintf("https://np-ipick.eastmoney.com/recommend/stock/heat/ranking?count=20&trace=%d&client=web&biz=web_smart_tag", time.Now().Unix())
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-ipick.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("HotStrategy-err:%+v", err)
		return map[string]any{}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	return respMap
}

func (s SearchStockApi) HotStrategyTable() string {
	markdownTable := ""
	res := s.HotStrategy()
	bytes, _ := json.Marshal(res)
	strategy := &models.HotStrategy{}
	json.Unmarshal(bytes, strategy)
	for _, data := range strategy.Data {
		data.Chg = mathutil.RoundToFloat(100*data.Chg, 2)
	}
	markdownTable = util.MarkdownTableWithTitle("当前热门选股策略", strategy.Data)
	return markdownTable
}

func (s SearchStockApi) StrategySquare() map[string]any {
	//https://backtest.10jqka.com.cn/strategysquare/list?order=desc&page=1&pageNum=10&sortType=hot&keyword=
	url := "https://backtest.10jqka.com.cn/strategysquare/list?order=desc&page=1&pageNum=10&sortType=hot&keyword="
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "backtest.10jqka.com.cn").
		SetHeader("Origin", "https://backtest.10jqka.com.cn").
		SetHeader("Referer", "https://backtest.10jqka.com.cn/strategysquare/list").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("StrategySquare-err:%+v", err)
		return map[string]any{}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap
}
