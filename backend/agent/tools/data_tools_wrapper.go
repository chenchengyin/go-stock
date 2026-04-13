package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/random"
	"github.com/go-resty/resty/v2"
	fakeUserAgent "github.com/lib4u/fake-useragent"
	"github.com/tidwall/gjson"
)

type DataToolWrapper struct {
	name        string
	description string
	params      map[string]*schema.ParameterInfo
	handler     func(args string) (string, error)
}

func NewDataToolWrapper(name, description string, params map[string]*schema.ParameterInfo, handler func(args string) (string, error)) *DataToolWrapper {
	return &DataToolWrapper{
		name:        name,
		description: description,
		params:      params,
		handler:     handler,
	}
}

func (t *DataToolWrapper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(t.params),
	}, nil
}

func (t *DataToolWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	logger.SugaredLogger.Infof("Tool %s called with args: %s", t.name, argumentsInJSON)
	return t.handler(argumentsInJSON)
}

func thsResultToMarkdown(res map[string]any, title string) string {
	if convertor.ToString(res["code"]) != "100" {
		return "无符合条件的数据"
	}

	resData, ok := res["data"].(map[string]any)
	if !ok {
		return "无符合条件的数据"
	}
	result, ok := resData["result"].(map[string]any)
	if !ok {
		return "无符合条件的数据"
	}

	dataList, ok := result["dataList"].([]any)
	if !ok {
		return "无符合条件的数据"
	}
	columns, ok := result["columns"].([]any)
	if !ok {
		return "无符合条件的数据"
	}

	headers := map[string]string{}
	for _, v := range columns {
		d := v.(map[string]any)
		colTitle := convertor.ToString(d["title"])
		if dm := convertor.ToString(d["dateMsg"]); dm != "" {
			colTitle += "[" + dm + "]"
		}
		if u := convertor.ToString(d["unit"]); u != "" {
			colTitle += "(" + u + ")"
		}
		headers[d["key"].(string)] = colTitle
	}

	table := &[]map[string]any{}
	for _, v := range dataList {
		d := v.(map[string]any)
		row := map[string]any{}
		for key, colTitle := range headers {
			row[colTitle] = convertor.ToString(d[key])
		}
		*table = append(*table, row)
	}

	jsonData, _ := json.Marshal(*table)
	markdownTable, _ := JSONToMarkdownTable(jsonData)
	return "\r\n### " + title + "：\r\n" + markdownTable + "\r\n"
}

func GetAllDataTools() []tool.BaseTool {
	var tools []tool.BaseTool

	tools = append(tools, NewDataToolWrapper(
		"FilterStocks",
		"根据技术指标或者关注排名或者连涨/连跌跌天数筛选股票。支持多种K线形态和技术指标条件筛选，如MACD金叉、KDJ金叉、均线排列、K线形态，人气，关注排名，连涨/连跌跌天数等。",
		map[string]*schema.ParameterInfo{
			"keyword": {
				Type:     "string",
				Desc:     "股票名称或代码关键词搜索（可选）",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20",
				Required: false,
			},
			"macdGoldenFork": {
				Type:     "boolean",
				Desc:     "MACD金叉",
				Required: false,
			},
			"kdjGoldenFork": {
				Type:     "boolean",
				Desc:     "KDJ金叉",
				Required: false,
			},
			"breakThrough": {
				Type:     "boolean",
				Desc:     "放量突破",
				Required: false,
			},
			"lowFundsInflow": {
				Type:     "boolean",
				Desc:     "低位资金净流入",
				Required: false,
			},
			"highFundsOutflow": {
				Type:     "boolean",
				Desc:     "高位资金净流出",
				Required: false,
			},
			"breakUpMa5Days": {
				Type:     "boolean",
				Desc:     "向上突破5日均线",
				Required: false,
			},
			"longAvgArray": {
				Type:     "boolean",
				Desc:     "均线多头排列",
				Required: false,
			},
			"shortAvgArray": {
				Type:     "boolean",
				Desc:     "均线空头排列",
				Required: false,
			},
			"upperLargeVolume": {
				Type:     "boolean",
				Desc:     "连涨放量",
				Required: false,
			},
			"downNarrowVolume": {
				Type:     "boolean",
				Desc:     "下跌无量",
				Required: false,
			},
			"oneDayangLine": {
				Type:     "boolean",
				Desc:     "一根大阳线",
				Required: false,
			},
			"twoDayangLines": {
				Type:     "boolean",
				Desc:     "两根大阳线",
				Required: false,
			},
			"riseSun": {
				Type:     "boolean",
				Desc:     "旭日东升",
				Required: false,
			},
			"powerFulgun": {
				Type:     "boolean",
				Desc:     "强势多方炮",
				Required: false,
			},
			"restoreJustice": {
				Type:     "boolean",
				Desc:     "拨云见日",
				Required: false,
			},
			"down7Days": {
				Type:     "boolean",
				Desc:     "七仙女下凡(七连阴)",
				Required: false,
			},
			"upper8Days": {
				Type:     "boolean",
				Desc:     "八仙过海(八连阳)",
				Required: false,
			},
			"upper9Days": {
				Type:     "boolean",
				Desc:     "九阳神功(九连阳)",
				Required: false,
			},
			"upper4Days": {
				Type:     "boolean",
				Desc:     "四串阳",
				Required: false,
			},
			"heavenRule": {
				Type:     "boolean",
				Desc:     "天量法则",
				Required: false,
			},
			"upsideVolume": {
				Type:     "boolean",
				Desc:     "放量上攻",
				Required: false,
			},
			"bearishEngulfing": {
				Type:     "boolean",
				Desc:     "穿头破脚",
				Required: false,
			},
			"reversingHammer": {
				Type:     "boolean",
				Desc:     "倒转锤头",
				Required: false,
			},
			"shootingStar": {
				Type:     "boolean",
				Desc:     "射击之星",
				Required: false,
			},
			"eveningStar": {
				Type:     "boolean",
				Desc:     "黄昏之星",
				Required: false,
			},
			"firstDawn": {
				Type:     "boolean",
				Desc:     "曙光初现",
				Required: false,
			},
			"pregnant": {
				Type:     "boolean",
				Desc:     "身怀六甲",
				Required: false,
			},
			"blackCloudTops": {
				Type:     "boolean",
				Desc:     "乌云盖顶",
				Required: false,
			},
			"morningStar": {
				Type:     "boolean",
				Desc:     "早晨之星",
				Required: false,
			},
			"narrowFinish": {
				Type:     "boolean",
				Desc:     "窄幅整理",
				Required: false,
			},
			"uppDays": {
				Type:     "integer",
				Desc:     "人气排名连涨天数：3/5/7天及以上",
				Required: false,
			},
			"concernRank7Days": {
				Type:     "integer",
				Desc:     "7日关注排名：10/50/100名以内",
				Required: false,
			},
			"upNday": {
				Type:     "integer",
				Desc:     "连涨天数：3/5/8天及以上",
				Required: false,
			},
			"downNday": {
				Type:     "integer",
				Desc:     "连跌天数：3/5/8/10/14天及以上",
				Required: false,
			},
		},
		func(args string) (string, error) {
			keyword := gjson.Get(args, "keyword").String()
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 20
			}

			indicators := models.TechnicalIndicators{
				MACDGOLDENFORK:     gjson.Get(args, "macdGoldenFork").Bool(),
				KDJGOLDENFORK:      gjson.Get(args, "kdjGoldenFork").Bool(),
				BREAKTHROUGH:       gjson.Get(args, "breakThrough").Bool(),
				LOWFUNDSINFLOW:     gjson.Get(args, "lowFundsInflow").Bool(),
				HIGHFUNDSOUTFLOW:   gjson.Get(args, "highFundsOutflow").Bool(),
				BREAKUPMA5DAYS:     gjson.Get(args, "breakUpMa5Days").Bool(),
				LONGAVGARRAY:       gjson.Get(args, "longAvgArray").Bool(),
				SHORTAVGARRAY:      gjson.Get(args, "shortAvgArray").Bool(),
				UPPERLARGEVOLUME:   gjson.Get(args, "upperLargeVolume").Bool(),
				DOWNNARROWVOLUME:   gjson.Get(args, "downNarrowVolume").Bool(),
				ONEDAYANGLINE:      gjson.Get(args, "oneDayangLine").Bool(),
				TWODAYANGLINES:     gjson.Get(args, "twoDayangLines").Bool(),
				RISESUN:            gjson.Get(args, "riseSun").Bool(),
				POWERFULGUN:        gjson.Get(args, "powerFulgun").Bool(),
				RESTOREJUSTICE:     gjson.Get(args, "restoreJustice").Bool(),
				DOWN7DAYS:          gjson.Get(args, "down7Days").Bool(),
				UPPER8DAYS:         gjson.Get(args, "upper8Days").Bool(),
				UPPER9DAYS:         gjson.Get(args, "upper9Days").Bool(),
				UPPER4DAYS:         gjson.Get(args, "upper4Days").Bool(),
				HEAVENRULE:         gjson.Get(args, "heavenRule").Bool(),
				UPSIDEVOLUME:       gjson.Get(args, "upsideVolume").Bool(),
				BEARISHENGULFING:   gjson.Get(args, "bearishEngulfing").Bool(),
				REVERSINGHAMMER:    gjson.Get(args, "reversingHammer").Bool(),
				SHOOTINGSTAR:       gjson.Get(args, "shootingStar").Bool(),
				EVENINGSTAR:        gjson.Get(args, "eveningStar").Bool(),
				FIRSTDAWN:          gjson.Get(args, "firstDawn").Bool(),
				PREGNANT:           gjson.Get(args, "pregnant").Bool(),
				BLACKCLOUDTOPS:     gjson.Get(args, "blackCloudTops").Bool(),
				MORNINGSTAR:        gjson.Get(args, "morningStar").Bool(),
				NARROWFINISH:       gjson.Get(args, "narrowFinish").Bool(),
				UPP_DAYS:           int(gjson.Get(args, "uppDays").Int()),
				CONCERN_RANK_7DAYS: int(gjson.Get(args, "concernRank7Days").Int()),
				UPNDAY:             int(gjson.Get(args, "upNday").Int()),
				DOWNNDAY:           int(gjson.Get(args, "downNday").Int()),
			}

			result := data.NewStockDataApi().GetAllStocks(page, pageSize, keyword, indicators)
			if result == nil || len(result.Result.Data) == 0 {
				return "未找到符合条件的股票", nil
			}

			type stockRow struct {
				SECUCODE         string  `md:"股票代码"`
				SECURITYNAMEABBR string  `md:"股票名称"`
				NEWPRICE         float64 `md:"最新价"`
				CHANGERATE       float64 `md:"涨跌幅(%)"`
				HIGHPRICE        float64 `md:"最高价"`
				LOWPRICE         float64 `md:"最低价"`
				VOLUME           string  `md:"成交量"`
				DEALAMOUNT       float64 `md:"成交额"`
				TURNOVERRATE     float64 `md:"换手率(%)"`
				VOLUMERATIO      float64 `md:"量比"`
				INDUSTRY         string  `md:"所属行业"`
			}

			var rows []stockRow
			for _, s := range result.Result.Data {
				newPrice, _ := convertor.ToFloat(s.NEWPRICE)
				changeRate, _ := convertor.ToFloat(s.CHANGERATE)
				highPrice, _ := convertor.ToFloat(s.HIGHPRICE)
				lowPrice, _ := convertor.ToFloat(s.LOWPRICE)
				dealAmount, _ := convertor.ToFloat(s.DEALAMOUNT)
				turnoverRate, _ := convertor.ToFloat(s.TURNOVERRATE)
				volumeRatio, _ := convertor.ToFloat(s.VOLUMERATIO)

				rows = append(rows, stockRow{
					SECUCODE:         s.SECUCODE,
					SECURITYNAMEABBR: s.SECURITYNAMEABBR,
					NEWPRICE:         newPrice,
					CHANGERATE:       changeRate,
					HIGHPRICE:        highPrice,
					LOWPRICE:         lowPrice,
					VOLUME:           convertor.ToString(s.VOLUME),
					DEALAMOUNT:       dealAmount,
					TURNOVERRATE:     turnoverRate,
					VOLUMERATIO:      volumeRatio,
					INDUSTRY:         s.INDUSTRY,
				})
			}

			summary := fmt.Sprintf("共找到 %d 只符合条件的股票，当前显示第 %d 页（每页 %d 条）", result.Result.Count, page, pageSize)
			return summary + "\n\n" + util.MarkdownTableWithTitle("股票筛选结果", rows), nil
		},
	))
	tools = append(tools, NewDataToolWrapper(
		"SearchStockByIndicators",
		"根据自然语言筛选股票。可以使用K线形态、技术指标、财务指标等条件选股。可以查询股票常用的指标，如均线，kdj,rsi,boll，macd等。",
		map[string]*schema.ParameterInfo{
			"words": {
				Type:     "string",
				Desc:     "选股条件描述，支持K线形态、技术指标、财务指标、均线、kdj、rsi、boll、macd等。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			words := gjson.Get(args, "words").String()
			res := data.NewSearchStockApi(words).SearchStock(random.RandInt(50, 120))
			content := thsResultToMarkdown(res, "工具筛选出的相关股票数据")
			return content, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SearchBk",
		"根据自然语言查询板块/概念/指数整体数据。",
		map[string]*schema.ParameterInfo{
			"words": {
				Type:     "string",
				Desc:     "板块/概念/指数查询条件描述。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			words := gjson.Get(args, "words").String()
			res := data.NewSearchStockApi(words).SearchBk(random.RandInt(50, 120))
			content := thsResultToMarkdown(res, "工具筛选出的相关板块/概念数据")
			return content, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SearchETF",
		"根据自然语言查询ETF数据。",
		map[string]*schema.ParameterInfo{
			"words": {
				Type:     "string",
				Desc:     "ETF查询条件描述。",
				Required: true,
			},
		},
		func(args string) (string, error) {
			words := gjson.Get(args, "words").String()
			res := data.NewSearchStockApi(words).SearchETF(random.RandInt(50, 120))
			content := thsResultToMarkdown(res, "工具筛选出的相关ETF数据")
			return content, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"InteractiveAnswer",
		"获取投资者与上市公司互动问答的数据",
		map[string]*schema.ParameterInfo{
			"page": {
				Type:     "string",
				Desc:     "分页号",
				Required: true,
			},
			"pageSize": {
				Type:     "string",
				Desc:     "分页大小",
				Required: true,
			},
			"keyWord": {
				Type:     "string",
				Desc:     "搜索关键词",
				Required: false,
			},
		},
		func(args string) (string, error) {
			page := gjson.Get(args, "page").String()
			pageSize := gjson.Get(args, "pageSize").String()
			keyWord := gjson.Get(args, "keyWord").String()
			pageNo, _ := convertor.ToInt(page)
			if pageNo == 0 {
				pageNo = 1
			}
			pageSizeNum, _ := convertor.ToInt(pageSize)
			if pageSizeNum == 0 {
				pageSizeNum = 50
			}
			datas := data.NewMarketNewsApi().InteractiveAnswer(int(pageNo), int(pageSizeNum), keyWord)
			content := util.MarkdownTableWithTitle("投资互动数据", datas.Results)
			return content, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockResearchReport",
		"获取市场分析师的股票研究报告。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			news := data.NewMarketNewsApi()
			res := news.StockResearchReport(stockCode, 30)
			var md strings.Builder
			for _, a := range res {
				d, ok := a.(map[string]any)
				if !ok {
					continue
				}
				infoCode, _ := d["infoCode"].(string)
				md.WriteString(news.GetIndustryReportInfo(infoCode))
			}
			out := strings.TrimSpace(md.String())
			if out == "" {
				return stockCode + "：未查询到相关研究报告。", nil
			}
			return "### " + stockCode + " 研究报告\r\n" + out, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"HotStrategyTable",
		"获取当前热门选股策略",
		map[string]*schema.ParameterInfo{},
		func(args string) (string, error) {
			result := data.NewSearchStockApi("").HotStrategyTable()
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"HotStockTable",
		"当前热门股票排名",
		map[string]*schema.ParameterInfo{
			"pageSize": {
				Type:     "string",
				Desc:     "分页大小",
				Required: true,
			},
		},
		func(args string) (string, error) {
			pageSize := gjson.Get(args, "pageSize").String()
			pageSizeNum := int64(50)
			if pageSize != "" {
				if d, err := parseInt(pageSize); err == nil {
					pageSizeNum = int64(d)
				}
			}
			res := data.NewMarketNewsApi().XUEQIUHotStock(int(pageSizeNum), "10")
			md := util.MarkdownTableWithTitle("当前热门股票排名", res)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockMoneyData",
		"今日股票资金流入排名",
		map[string]*schema.ParameterInfo{},
		func(args string) (string, error) {
			res := data.NewStockDataApi().GetStockMoneyData()
			md := util.MarkdownTableWithTitle("今日个股资金流向Top50", res.Data.Diff)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetMutualTop10Deal",
		"获取北向资金/南向资金十大成交股数据",
		map[string]*schema.ParameterInfo{
			"mutualType": {
				Type:     "string",
				Desc:     "通道类型：001=沪股通，002=港股通(沪)，003=深股通，004=港股通(深)",
				Required: true,
			},
			"tradeDate": {
				Type:     "string",
				Desc:     "交易日期，格式：YYYY-MM-DD",
				Required: true,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数",
				Required: false,
			},
		},
		func(args string) (string, error) {
			mutualType := gjson.Get(args, "mutualType").String()
			tradeDate := gjson.Get(args, "tradeDate").String()
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if page == 0 {
				page = 1
			}
			if pageSize == 0 {
				pageSize = 10
			}
			result := data.NewStockDataApi().GetMutualTop10Deal(mutualType, tradeDate, page, pageSize)
			title := mutualTypeName(mutualType) + " " + tradeDate
			md := util.MarkdownTableWithTitle(title, result.Result.Data)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockFinancialInfo",
		"获取股票财务报表信息",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			result := data.NewStockDataApi().GetStockFinancialInfo(stockCode)
			md := util.MarkdownTableWithTitle("股票"+stockCode+"财务报表信息", result.Result.Data)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockHolderNum",
		"获取股票股东人数信息",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			result := data.NewStockDataApi().GetStockHolderNum(stockCode)
			md := util.MarkdownTableWithTitle("股票"+stockCode+"股东人数信息", result.Result.Data)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockHistoryMoneyData",
		"获取股票历史资金流向数据",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			result := data.NewStockDataApi().GetStockHistoryMoneyData(stockCode)
			md := util.MarkdownTableWithTitle("股票"+stockCode+"历史资金流向数据", result)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockRZRQInfo",
		"获取股票融资融券信息",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			result := data.NewStockDataApi().GetStockRZRQInfo(stockCode)
			md := util.MarkdownTableWithTitle(stockCode+" 融资融券信息", result.Result.Data)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetIndustryValuation",
		"获取行业/板块平均估值和中值",
		map[string]*schema.ParameterInfo{
			"bkName": {
				Type:     "string",
				Desc:     "行业/板块名称，如：半导体",
				Required: true,
			},
		},
		func(args string) (string, error) {
			bkName := gjson.Get(args, "bkName").String()
			result := data.NewStockDataApi().GetIndustryValuation(bkName)
			md := util.MarkdownTableWithTitle(bkName+"行业估值", result.Result.Data)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetNewsListData",
		"获取新闻资讯",
		map[string]*schema.ParameterInfo{
			"startTime": {
				Type:     "string",
				Desc:     "开始时间（如：2026-02-23 00:00:00）",
				Required: true,
			},
			"keyWord": {
				Type:     "string",
				Desc:     "搜索关键词",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "每页条数",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数",
				Required: false,
			},
		},
		func(args string) (string, error) {
			startTime := gjson.Get(args, "startTime").String()
			keyWord := gjson.Get(args, "keyWord").String()
			limit := int(gjson.Get(args, "limit").Int())
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if pageSize <= 0 {
				pageSize = limit
			}
			if pageSize <= 0 {
				pageSize = 20
			}
			if page < 1 {
				page = 1
			}

			parseTime, err := time.Parse(time.DateTime, startTime)
			if err != nil {
				parseTime = time.Now().Add(-time.Hour * 24)
			}
			list, total := data.NewMarketNewsApi().GetNewsListData(keyWord, parseTime, page, pageSize)

			var md strings.Builder
			md.WriteString("### 最近新闻资讯（共 " + convertor.ToString(total) + " 条，本页 " + convertor.ToString(len(*list)) + " 条）\r\n")
			for _, d := range *list {
				md.WriteString(d.DataTime.Format(time.DateTime) + " " + d.Content + "\r\n")
			}
			return md.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GlobalStockIndexesReadable",
		"获取全球主要指数概览",
		map[string]*schema.ParameterInfo{},
		func(args string) (string, error) {
			result := data.NewMarketNewsApi().GlobalStockIndexesReadable(30)
			if result == "" {
				return "暂无全球指数数据。", nil
			}
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"StockNotice",
		"获取上市公司公告列表",
		map[string]*schema.ParameterInfo{
			"stock_list": {
				Type:     "string",
				Desc:     "股票代码，多只用英文逗号分隔",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockList := gjson.Get(args, "stock_list").String()
			res := data.NewMarketNewsApi().StockNotice(stockList)
			if len(res) == 0 {
				return "未查询到相关上市公司公告。", nil
			}

			type row struct {
				Title      string `md:"公告标题"`
				NoticeDate string `md:"公告日期"`
				ColumnName string `md:"公告类型"`
			}
			var rows []row
			for _, a := range res {
				m, ok := a.(map[string]any)
				if !ok {
					continue
				}
				if m["columns"].([]any) != nil && len(m["columns"].([]any)) > 0 {
					columns := m["columns"].([]any)[0].(map[string]any)
					rows = append(rows, row{
						Title:      convertor.ToString(m["title"]),
						NoticeDate: convertor.ToString(m["notice_date"]),
						ColumnName: convertor.ToString(columns["column_name"]),
					})
				} else {
					rows = append(rows, row{
						Title:      convertor.ToString(m["title"]),
						NoticeDate: convertor.ToString(m["notice_date"]),
					})
				}
			}
			md := util.MarkdownTableWithTitle("上市公司公告", rows)
			return md, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetSecuritiesCompanyOpinion",
		"获取券商/机构的市场分析观点",
		map[string]*schema.ParameterInfo{
			"startDate": {
				Type:     "string",
				Desc:     "开始时间（如：2026-02-23）",
				Required: true,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束时间（如：2026-02-26）",
				Required: true,
			},
		},
		func(args string) (string, error) {
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			res := data.NewMarketNewsApi().GetSecuritiesCompanyOpinion(startDate, endDate)
			var md strings.Builder
			for _, d := range res.Data {
				md.WriteString(d.OpinionData + "\r\n")
			}
			return md.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetCurrentTime",
		"获取当前本地时间及全球市场开盘状态",
		map[string]*schema.ParameterInfo{},
		func(args string) (string, error) {
			now := time.Now().Format("2006-01-02 15:04:05")
			marketStatus := data.NewMarketNewsApi().GlobalStockIndexesReadable(30)
			return "当前本地时间是：" + now + "\n\n" + marketStatus, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetMarketData",
		"获取市场行情数据，包括指数行情、上涨/下跌/涨停/跌停家数、涨跌分布和今日申购信息",
		map[string]*schema.ParameterInfo{},
		func(args string) (string, error) {
			return getMarketDataContent()
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"AiRecommendStocks",
		"获取近期AI分析/推荐股票明细列表",
		map[string]*schema.ParameterInfo{
			"startDate": {
				Type:     "string",
				Desc:     "开始时间",
				Required: true,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束时间",
				Required: true,
			},
			"page": {
				Type:     "string",
				Desc:     "分页号",
				Required: true,
			},
			"pageSize": {
				Type:     "string",
				Desc:     "分页大小",
				Required: true,
			},
			"keyWord": {
				Type:     "string",
				Desc:     "搜索关键词",
				Required: false,
			},
		},
		func(args string) (string, error) {
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			page := gjson.Get(args, "page").String()
			pageSize := gjson.Get(args, "pageSize").String()
			keyWord := gjson.Get(args, "keyWord").String()

			pageNo := int64(1)
			if page != "" {
				if d, err := parseInt(page); err == nil {
					pageNo = int64(d)
				}
			}
			pageSizeNum := int64(50)
			if pageSize != "" {
				if d, err := parseInt(pageSize); err == nil {
					pageSizeNum = int64(d)
				}
			}

			pageData, svcErr := data.NewAiRecommendStocksService().GetAiRecommendStocksList(&models.AiRecommendStocksQuery{
				StartDate: startDate,
				EndDate:   endDate,
				Page:      int(pageNo),
				PageSize:  int(pageSizeNum),
				StockCode: keyWord,
				StockName: keyWord,
				BkName:    keyWord,
			})
			if svcErr != nil {
				return "", svcErr
			}

			var dataExport []models.AiRecommendStocksMdExport
			for _, v := range pageData.List {
				dataExport = append(dataExport, v.ToMdExportStruct())
			}
			content := util.MarkdownTableWithTitle("近期AI分析/推荐股票明细列表", dataExport)
			return content, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetAIAnalysisHistory",
		"查询历史AI分析报告。可以根据股票代码、股票名称、问题关键词、日期范围等条件筛选历史AI分析记录。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码筛选（可选）",
				Required: false,
			},
			"stockName": {
				Type:     "string",
				Desc:     "股票名称筛选（可选）",
				Required: false,
			},
			"question": {
				Type:     "string",
				Desc:     "问题关键词搜索（可选）",
				Required: false,
			},
			"modelName": {
				Type:     "string",
				Desc:     "AI模型名称筛选（可选）",
				Required: false,
			},
			"startDate": {
				Type:     "string",
				Desc:     "开始日期，格式：YYYY-MM-DD（可选）",
				Required: false,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束日期，格式：YYYY-MM-DD（可选）",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			stockName := gjson.Get(args, "stockName").String()
			question := gjson.Get(args, "question").String()
			modelName := gjson.Get(args, "modelName").String()
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())

			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 || pageSize > 50 {
				pageSize = 10
			}

			pageData, svcErr := data.NewAIResponseResultService().GetAIResponseResultList(models.AIResponseResultQuery{
				StockCode: stockCode,
				StockName: stockName,
				Question:  question,
				ModelName: modelName,
				StartDate: startDate,
				EndDate:   endDate,
				Page:      page,
				PageSize:  pageSize,
			})
			if svcErr != nil {
				return "", svcErr
			}

			if pageData == nil || len(pageData.List) == 0 {
				return "未找到符合条件的历史分析报告", nil
			}

			type historyRow struct {
				ID         uint   `md:"ID"`
				StockCode  string `md:"股票代码"`
				StockName  string `md:"股票名称"`
				Question   string `md:"问题"`
				ModelName  string `md:"模型"`
				CreateTime string `md:"创建时间"`
			}

			var rows []historyRow
			for _, item := range pageData.List {
				questionText := item.Question
				if len(questionText) > 50 {
					questionText = questionText[:50] + "..."
				}
				rows = append(rows, historyRow{
					ID:         item.ID,
					StockCode:  item.StockCode,
					StockName:  item.StockName,
					Question:   questionText,
					ModelName:  item.ModelName,
					CreateTime: item.CreatedAt.Format("2006-01-02 15:04:05"),
				})
			}

			summary := fmt.Sprintf("共找到 %d 条历史分析报告，当前第 %d/%d 页", pageData.Total, page, pageData.TotalPages)
			return summary + "\n\n" + util.MarkdownTableWithTitle("历史AI分析报告", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetAIAnalysisDetail",
		"根据ID获取历史AI分析报告的详细内容",
		map[string]*schema.ParameterInfo{
			"id": {
				Type:     "integer",
				Desc:     "分析报告ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			id := uint(gjson.Get(args, "id").Int())
			if id == 0 {
				return "请提供有效的分析报告ID", nil
			}

			var result models.AIResponseResult
			err := db.Dao.First(&result, id).Error
			if err != nil {
				return "未找到该分析报告", nil
			}

			var md strings.Builder
			md.WriteString(fmt.Sprintf("### AI分析报告详情\n\n"))
			md.WriteString(fmt.Sprintf("| 项目 | 内容 |\n| --- | --- |\n"))
			md.WriteString(fmt.Sprintf("| ID | %d |\n", result.ID))
			md.WriteString(fmt.Sprintf("| 股票代码 | %s |\n", result.StockCode))
			md.WriteString(fmt.Sprintf("| 股票名称 | %s |\n", result.StockName))
			md.WriteString(fmt.Sprintf("| 模型 | %s |\n", result.ModelName))
			md.WriteString(fmt.Sprintf("| 创建时间 | %s |\n", result.CreatedAt.Format("2006-01-02 15:04:05")))
			md.WriteString(fmt.Sprintf("\n#### 问题\n\n%s\n", result.Question))
			md.WriteString(fmt.Sprintf("\n#### AI分析结果\n\n%s\n", result.Content))

			return md.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetAIAnalysisContent",
		"根据股票代码获取最新的AI分析报告内容。直接返回该股票最近一次AI分析的完整内容。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：600519.SH、000001.SZ",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			if stockCode == "" {
				return "请提供股票代码", nil
			}

			var result models.AIResponseResult
			err := db.Dao.Where("stock_code = ?", stockCode).Order("created_at DESC").First(&result).Error
			if err != nil {
				return fmt.Sprintf("未找到 %s 的历史分析报告", stockCode), nil
			}

			var md strings.Builder
			md.WriteString(fmt.Sprintf("### %s (%s) AI分析报告\n\n", result.StockName, result.StockCode))
			md.WriteString(fmt.Sprintf("| 项目 | 内容 |\n| --- | --- |\n"))
			md.WriteString(fmt.Sprintf("| 报告ID | %d |\n", result.ID))
			md.WriteString(fmt.Sprintf("| 模型 | %s |\n", result.ModelName))
			md.WriteString(fmt.Sprintf("| 分析时间 | %s |\n", result.CreatedAt.Format("2006-01-02 15:04:05")))
			md.WriteString(fmt.Sprintf("\n#### 问题\n\n%s\n", result.Question))
			md.WriteString(fmt.Sprintf("\n#### AI分析结果\n\n%s\n", result.Content))

			return md.String(), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SendDingDingMessage",
		"发送消息到钉钉机器人",
		map[string]*schema.ParameterInfo{
			"message": {
				Type:     "string",
				Desc:     "要发送的消息内容，支持 Markdown 格式",
				Required: true,
			},
		},
		func(args string) (string, error) {
			message := gjson.Get(args, "message").String()
			if message == "" {
				return "消息内容不能为空", nil
			}
			result := data.NewDingDingAPI().SendToDingDing("通知", message)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SendToDingDing",
		"将指定标题和内容以 Markdown 形式发送到钉钉机器人",
		map[string]*schema.ParameterInfo{
			"title": {
				Type:     "string",
				Desc:     "消息标题，会显示为「go-stock {title}」",
				Required: true,
			},
			"message": {
				Type:     "string",
				Desc:     "消息正文，支持 Markdown 格式，通知内容需尽可能精简",
				Required: true,
			},
		},
		func(args string) (string, error) {
			title := gjson.Get(args, "title").String()
			message := gjson.Get(args, "message").String()
			if title == "" || message == "" {
				return "标题和消息内容不能为空", nil
			}
			result := data.NewDingDingAPI().SendToDingDing(title, message)
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockKLine",
		"获取股票日K线数据。支持一次查询多只。",
		map[string]*schema.ParameterInfo{
			"days": {
				Type:     "string",
				Desc:     "日K数据条数",
				Required: true,
			},
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码（A股：sh,sz开头;港股hk开头,美股：us开头）",
				Required: true,
			},
			"stockCodes": {
				Type:     "array",
				Desc:     "可选，多只股票代码列表",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := gjson.Get(args, "days").String()
			codes := parseStockCodesFromArgs(args, "stockCode")
			toIntDay := 90
			if days != "" {
				if d, err := parseInt(days); err == nil {
					toIntDay = d
				}
			}
			var allResults []map[string]any
			api := data.NewStockDataApi()
			for _, code := range codes {
				var klineData *[]data.KLineData
				if strings.HasPrefix(code, "sz") || strings.HasPrefix(code, "sh") {
					klineData = api.GetKLineData(code, "240", int64(toIntDay))
				} else if strings.HasPrefix(code, "hk") || strings.HasPrefix(code, "us") || strings.HasPrefix(code, "gb_") {
					klineData = api.GetHK_KLineData(code, "day", int64(toIntDay))
				}
				if klineData != nil {
					for _, k := range *klineData {
						allResults = append(allResults, map[string]any{
							"stockCode": code,
							"date":      k.Day,
							"open":      k.Open,
							"high":      k.High,
							"low":       k.Low,
							"close":     k.Close,
							"volume":    k.Volume,
						})
					}
				}
			}
			if len(allResults) == 0 {
				return "未获取到 K 线数据", nil
			}
			jsonData, _ := json.Marshal(allResults)
			markdownTable, _ := data.JSONToMarkdownTable(jsonData)
			return markdownTable, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetEastMoneyKLine",
		"获取股票 K 线数据。支持日/周/月/季/年 K 线及 1/5/15/30/60 分钟线，可选前复权或后复权。股票代码格式：A股 000001.SZ、600000.SH，港股 00700.HK 等。支持一次查询多只。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码。A股如 000001.SZ、600000.SH；港股如 00700.HK。多只时可用英文逗号分隔。",
				Required: true,
			},
			"stockCodes": {
				Type:     "array",
				Desc:     "可选，多只股票代码列表",
				Required: false,
			},
			"kLineType": {
				Type:     "string",
				Desc:     "K 线类型：day/日/101=日K，week/周/102=周K，month/月/103=月K，quarter/季/104=季K，halfYear/半年/105=半年K，year/年/106=年K；分钟线：1/5/15/30/60/120",
				Required: true,
			},
			"adjustFlag": {
				Type:     "string",
				Desc:     "复权类型，仅日K有效：空=不复权，qfq=前复权，hfq=后复权",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "获取 K 线根数",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			kLineType := gjson.Get(args, "kLineType").String()
			adjustFlag := gjson.Get(args, "adjustFlag").String()
			limit := int(gjson.Get(args, "limit").Int())
			if limit <= 0 {
				limit = 60
			}
			codes := parseStockCodesFromArgs(args, "stockCode")
			if stockCode != "" {
				codes = append(codes, stockCode)
			}
			if len(codes) == 0 {
				return "参数 stockCode 或 stockCodes 不能为空", nil
			}
			var results []string
			for _, code := range codes {
				if code == "" {
					continue
				}
				api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
				res := data.EastMoneyKLineSection(api, code, kLineType, adjustFlag, limit)
				results = append(results, res)
			}
			return strings.Join(results, "\n"), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetEastMoneyKLineWithMA",
		"获取股票 K 线数据并带多条均线（SMA，按收盘价计算）。用于技术分析时同时查看 K 线与均线。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码。A股如 000001.SZ、600000.SH；港股如 00700.HK。多只时可用英文逗号分隔。",
				Required: true,
			},
			"stockCodes": {
				Type:     "array",
				Desc:     "可选，多只股票代码列表",
				Required: false,
			},
			"kLineType": {
				Type:     "string",
				Desc:     "K 线类型：day/日/101=日K，week/周/102=周K，month/月/103=月K；分钟线：1/5/15/30/60/120",
				Required: true,
			},
			"limit": {
				Type:     "integer",
				Desc:     "获取 K 线根数",
				Required: false,
			},
			"maPeriods": {
				Type:     "string",
				Desc:     "均线周期，逗号分隔，如 5,10,20,60。不传则默认 5,10,20,60,120",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			kLineType := gjson.Get(args, "kLineType").String()
			limit := int(gjson.Get(args, "limit").Int())
			maPeriodsStr := gjson.Get(args, "maPeriods").String()
			if limit <= 0 {
				limit = 60
			}
			codes := parseStockCodesFromArgs(args, "stockCode")
			if stockCode != "" {
				codes = append(codes, stockCode)
			}
			if len(codes) == 0 {
				return "参数 stockCode 或 stockCodes 不能为空", nil
			}
			var results []string
			for _, code := range codes {
				if code == "" {
					continue
				}
				api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
				res := data.EastMoneyKLineWithMASection(api, code, kLineType, limit, maPeriodsStr)
				results = append(results, res)
			}
			return strings.Join(results, "\n"), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"CreateAiRecommendStocks",
		"创建/保存AI推荐股票记录",
		map[string]*schema.ParameterInfo{
			"modelName": {
				Type:     "string",
				Desc:     "模型名称",
				Required: true,
			},
			"rating": {
				Type:     "string",
				Desc:     "评级(买入/增持/中性/减持/卖出)",
				Required: true,
			},
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH",
				Required: true,
			},
			"stockName": {
				Type:     "string",
				Desc:     "股票名称",
				Required: true,
			},
			"bkName": {
				Type:     "string",
				Desc:     "板块/概念/行业名称",
				Required: true,
			},
			"stockPrice": {
				Type:     "string",
				Desc:     "推荐时股票价格",
				Required: true,
			},
			"recommendReason": {
				Type:     "string",
				Desc:     "推荐理由/驱动因素/逻辑",
				Required: true,
			},
			"recommendBuyPrice": {
				Type:     "string",
				Desc:     "ai建议买入价区间最低价和最高价之间用`-`分隔",
				Required: true,
			},
			"recommendBuyPriceMin": {
				Type:     "number",
				Desc:     "ai建议最低买入价",
				Required: true,
			},
			"recommendBuyPriceMax": {
				Type:     "number",
				Desc:     "ai建议最高买入价",
				Required: true,
			},
			"recommendStopProfitPrice": {
				Type:     "string",
				Desc:     "ai建议止盈价区间最低价和最高价之间用`-`分隔",
				Required: true,
			},
			"recommendStopProfitPriceMin": {
				Type:     "number",
				Desc:     "ai建议最低止盈价",
				Required: true,
			},
			"recommendStopProfitPriceMax": {
				Type:     "number",
				Desc:     "ai建议最高止盈价",
				Required: true,
			},
			"recommendStopLossPrice": {
				Type:     "string",
				Desc:     "ai建议止损价",
				Required: true,
			},
			"riskRemarks": {
				Type:     "string",
				Desc:     "风险提示",
				Required: true,
			},
			"remarks": {
				Type:     "string",
				Desc:     "操作总结/备注",
				Required: true,
			},
		},
		func(args string) (string, error) {
			var recommend models.AiRecommendStocks
			if err := json.Unmarshal([]byte(args), &recommend); err != nil {
				return "", err
			}
			if err := data.NewAiRecommendStocksService().CreateAiRecommendStocks(&recommend); err != nil {
				return "保存股票推荐失败: " + err.Error(), nil
			}
			return "保存股票推荐成功", nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"BatchCreateAiRecommendStocks",
		"批量创建/保存AI推荐股票记录，建议每次批量保存5条记录",
		map[string]*schema.ParameterInfo{
			"stocks": {
				Type:     "array",
				Desc:     "股票推荐列表",
				Required: true,
				ElemInfo: &schema.ParameterInfo{
					Type:     "object",
					Required: true,
					SubParams: map[string]*schema.ParameterInfo{
						"modelName": {
							Type:     "string",
							Desc:     "模型名称",
							Required: true,
						},
						"rating": {
							Type:     "string",
							Desc:     "评级(买入/增持/中性/减持/卖出)",
							Required: true,
						},
						"stockCode": {
							Type:     "string",
							Desc:     "股票代码，如：601138.SH",
							Required: true,
						},
						"stockName": {
							Type:     "string",
							Desc:     "股票名称",
							Required: true,
						},
						"bkName": {
							Type:     "string",
							Desc:     "板块/概念/行业名称",
							Required: true,
						},
						"stockPrice": {
							Type:     "string",
							Desc:     "推荐时股票价格",
							Required: true,
						},
						"recommendReason": {
							Type:     "string",
							Desc:     "推荐理由/驱动因素/逻辑",
							Required: true,
						},
						"recommendBuyPrice": {
							Type:     "string",
							Desc:     "ai建议买入价区间最低价和最高价之间用`-`分隔",
							Required: true,
						},
						"recommendBuyPriceMin": {
							Type:     "number",
							Desc:     "ai建议最低买入价",
							Required: true,
						},
						"recommendBuyPriceMax": {
							Type:     "number",
							Desc:     "ai建议最高买入价",
							Required: true,
						},
						"recommendStopProfitPrice": {
							Type:     "string",
							Desc:     "ai建议止盈价区间最低价和最高价之间用`-`分隔",
							Required: true,
						},
						"recommendStopProfitPriceMin": {
							Type:     "number",
							Desc:     "ai建议最低止盈价",
							Required: true,
						},
						"recommendStopProfitPriceMax": {
							Type:     "number",
							Desc:     "ai建议最高止盈价",
							Required: true,
						},
						"recommendStopLossPrice": {
							Type:     "string",
							Desc:     "ai建议止损价",
							Required: true,
						},
						"riskRemarks": {
							Type:     "string",
							Desc:     "风险提示",
							Required: true,
						},
						"remarks": {
							Type:     "string",
							Desc:     "操作总结/备注",
							Required: true,
						},
					},
				},
			},
		},
		func(args string) (string, error) {
			stocks := gjson.Get(args, "stocks").String()
			var recommends []*models.AiRecommendStocks
			if err := json.Unmarshal([]byte(stocks), &recommends); err != nil {
				return "", err
			}
			if err := data.NewAiRecommendStocksService().BatchCreateAiRecommendStocks(recommends); err != nil {
				return "批量保存股票推荐失败: " + err.Error(), nil
			}
			return "批量保存股票推荐成功，共保存 " + convertor.ToString(len(recommends)) + " 条记录", nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SetTradingPrice",
		"设置股票的预警价位线（开仓价、止盈价、止损价），用于设置股票的买入价格和风险控制参数",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如 000001.SZ、600000.SH",
				Required: true,
			},
			"entryPrice": {
				Type:     "number",
				Desc:     "开仓价/买入价（目标买入价格），0 表示不设置",
				Required: true,
			},
			"takeProfitPrice": {
				Type:     "number",
				Desc:     "止盈价（预期卖出价格），0 表示不设置",
				Required: true,
			},
			"stopLossPrice": {
				Type:     "number",
				Desc:     "止损价（亏损止损价格），0 表示不设置",
				Required: true,
			},
			"costPrice": {
				Type:     "number",
				Desc:     "成本价（持仓成本价格），0 表示不设置",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			entryPrice := gjson.Get(args, "entryPrice").Float()
			takeProfitPrice := gjson.Get(args, "takeProfitPrice").Float()
			stopLossPrice := gjson.Get(args, "stopLossPrice").Float()
			costPrice := gjson.Get(args, "costPrice").Float()
			result := data.NewStockDataApi().SetTradingPrice(entryPrice, takeProfitPrice, stopLossPrice, costPrice, stockCode)
			if result == "设置成功" {
				return fmt.Sprintf("✅ 价位线设置成功！\n\n📈 %s\n💰 开仓价：%.2f\n🎯 止盈价：%.2f\n🛑 止损价：%.2f\n💵 成本价：%.2f", stockCode, entryPrice, takeProfitPrice, stopLossPrice, costPrice), nil
			}
			return "❌ 价位线设置失败：" + result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"SearchFund",
		"搜索基金信息，支持按基金代码或名称模糊搜索",
		map[string]*schema.ParameterInfo{
			"keyword": {
				Type:     "string",
				Desc:     "搜索关键词（基金代码或名称）",
				Required: true,
			},
		},
		func(args string) (string, error) {
			keyword := gjson.Get(args, "keyword").String()
			if keyword == "" {
				return "请输入搜索关键词", nil
			}
			funds := data.NewFundApi().GetFundList(keyword)
			if len(funds) == 0 {
				return "未找到相关基金，请检查关键词是否正确", nil
			}
			type fundRow struct {
				Code        string   `md:"基金代码"`
				Name        string   `md:"基金名称"`
				Type        string   `md:"基金类型"`
				Manager     string   `md:"基金经理"`
				NetGrowth1  *float64 `md:"近1月涨幅"`
				NetGrowth3  *float64 `md:"近3月涨幅"`
				NetGrowth6  *float64 `md:"近6月涨幅"`
				NetGrowth12 *float64 `md:"近1年涨幅"`
			}
			var rows []fundRow
			for _, f := range funds {
				rows = append(rows, fundRow{
					Code:        f.Code,
					Name:        f.Name,
					Type:        f.Type,
					Manager:     f.Manager,
					NetGrowth1:  f.NetGrowth1,
					NetGrowth3:  f.NetGrowth3,
					NetGrowth6:  f.NetGrowth6,
					NetGrowth12: f.NetGrowth12,
				})
			}
			return util.MarkdownTableWithTitle("基金搜索结果", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetFundInfo",
		"获取基金详细信息，包括净值、涨跌幅、评级等",
		map[string]*schema.ParameterInfo{
			"fundCode": {
				Type:     "string",
				Desc:     "基金代码，如 000001",
				Required: true,
			},
		},
		func(args string) (string, error) {
			fundCode := gjson.Get(args, "fundCode").String()
			if fundCode == "" {
				return "请输入基金代码", nil
			}
			fund, err := data.NewFundApi().CrawlFundBasic(fundCode)
			if err != nil || fund.Code == "" {
				return "未找到该基金信息，请检查基金代码是否正确", nil
			}
			type fundDetailRow struct {
				Code           string   `md:"基金代码"`
				Name           string   `md:"基金名称"`
				FullName       string   `md:"基金全称"`
				Type           string   `md:"基金类型"`
				Establishment  string   `md:"成立日期"`
				Scale          string   `md:"最新规模(亿)"`
				Company        string   `md:"基金管理人"`
				Manager        string   `md:"基金经理"`
				Rating         string   `md:"基金评级"`
				TrackingTarget string   `md:"跟踪标的"`
				NetUnitValue   *float64 `md:"单位净值"`
				NetAccumulated *float64 `md:"累计净值"`
				NetGrowth1     *float64 `md:"近1月涨幅(%)"`
				NetGrowth3     *float64 `md:"近3月涨幅(%)"`
				NetGrowth6     *float64 `md:"近6月涨幅(%)"`
				NetGrowth12    *float64 `md:"近1年涨幅(%)"`
				NetGrowthYTD   *float64 `md:"今年来涨幅(%)"`
			}
			row := fundDetailRow{
				Code:           fund.Code,
				Name:           fund.Name,
				FullName:       fund.FullName,
				Type:           fund.Type,
				Establishment:  fund.Establishment,
				Scale:          fund.Scale,
				Company:        fund.Company,
				Manager:        fund.Manager,
				Rating:         fund.Rating,
				TrackingTarget: fund.TrackingTarget,
				NetUnitValue:   fund.NetUnitValue,
				NetAccumulated: fund.NetAccumulated,
				NetGrowth1:     fund.NetGrowth1,
				NetGrowth3:     fund.NetGrowth3,
				NetGrowth6:     fund.NetGrowth6,
				NetGrowth12:    fund.NetGrowth12,
				NetGrowthYTD:   fund.NetGrowthYTD,
			}
			return util.MarkdownTableWithTitle(fund.Name+" ("+fund.Code+") 基金详细信息", row), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetFollowedStocks",
		"获取用户关注/自选的股票列表",
		map[string]*schema.ParameterInfo{
			"groupId": {
				Type:     "integer",
				Desc:     "股票分组ID，不传则返回所有关注/自选的股票",
				Required: false,
			},
		},
		func(args string) (string, error) {
			groupId := int(gjson.Get(args, "groupId").Int())
			var rows []map[string]any
			if groupId > 0 {
				groupStocks := data.NewStockGroupApi(db.Dao).GetGroupStockByGroupId(groupId)
				for _, gs := range groupStocks {
					stockInfo := data.NewStockDataApi().GetFollowedStockByStockCode(gs.StockCode)
					if stockInfo.StockCode != "" {
						rows = append(rows, map[string]any{
							"股票代码": stockInfo.StockCode,
							"股票名称": stockInfo.Name,
							"成本价格": stockInfo.CostPrice,
							"持仓数量": stockInfo.Volume,
						})
					}
				}
			} else {
				list := data.NewStockDataApi().GetFollowList(0)
				if list != nil {
					for _, s := range *list {
						rows = append(rows, map[string]any{
							"股票代码": s.StockCode,
							"股票名称": s.Name,
							"成本价格": s.CostPrice,
							"持仓数量": s.Volume,
						})
					}
				}
			}
			if len(rows) == 0 {
				return "暂无关注/自选的股票", nil
			}
			return util.MarkdownTableWithTitle("关注/自选的股票", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockInfo",
		"获取股票详细信息，包括实时行情、基本信息等",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码(（A股：sh,sz开头;港股hk开头,美股：us开头）)，支持多个股票代码，用逗号分隔",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			if stockCode == "" {
				return "请输入股票代码", nil
			}
			codes := parseStockCodesFromArgs(args, "stockCode")
			if len(codes) == 0 {
				return "请输入股票代码", nil
			}
			var results []string
			for _, code := range codes {
				if code == "" {
					continue
				}
				stockData, err := data.NewStockDataApi().GetStockCodeRealTimeData(code)
				if err != nil || stockData == nil || len(*stockData) == 0 {
					results = append(results, code+"：未找到股票信息")
					continue
				}
				for _, s := range *stockData {
					price, _ := convertor.ToFloat(s.Price)
					prePrice, _ := convertor.ToFloat(s.PrePrice)
					change := price - prePrice
					var pChange float64
					if prePrice > 0 {
						pChange = (price - prePrice) / prePrice * 100
					}
					content := fmt.Sprintf("### %s %s\n\n| 项目 | 值 |\n| --- | --- |\n| 股票代码 | %s |\n| 股票名称 | %s |\n| 当前价格 | %.2f |\n| 涨跌额 | %.2f |\n| 涨跌幅 | %.2f%% |\n| 成交量 | %s手 |\n| 成交额 | %s元 |\n| 今开 | %s |\n| 昨收 | %s |\n| 最高 | %s |\n| 最低 | %s |\n| 时间 | %s %s |",
						s.Name, s.Code,
						s.Code, s.Name, price, change, pChange,
						s.Volume, s.Amount,
						s.Open, s.PreClose, s.High, s.Low,
						s.Date, s.Time)
					results = append(results, content)
				}
			}
			return strings.Join(results, "\n\n"), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetHotStockList",
		"获取雪球热门股票排行榜",
		map[string]*schema.ParameterInfo{
			"marketType": {
				Type:     "string",
				Desc:     "市场类型：全球(10)、沪深(12)、港股(13)、美股(11)",
				Required: false,
			},
			"size": {
				Type:     "integer",
				Desc:     "返回条数，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			marketType := gjson.Get(args, "marketType").String()
			size := int(gjson.Get(args, "size").Int())
			if size <= 0 {
				size = 20
			}
			if marketType == "" {
				marketType = "10"
			}
			hotItems := data.NewMarketNewsApi().XUEQIUHotStock(size, marketType)
			if hotItems == nil || len(*hotItems) == 0 {
				return "暂无热门股票数据", nil
			}
			type hotStockRow struct {
				Rank    int     `md:"排名"`
				Code    string  `md:"股票代码"`
				Name    string  `md:"股票名称"`
				Price   float64 `md:"当前价"`
				Chg     float64 `md:"股价变化"`
				Percent float64 `md:"涨跌幅(%)"`
				Value   float64 `md:"热度"`
			}
			var rows []hotStockRow
			for i, item := range *hotItems {
				rows = append(rows, hotStockRow{
					Rank:    i + 1,
					Code:    item.Code,
					Name:    item.Name,
					Price:   item.Current,
					Chg:     item.Chg,
					Percent: item.Percent,
					Value:   item.Value,
				})
			}
			marketName := map[string]string{
				"10": "全球",
				"12": "沪深",
				"13": "港股",
				"11": "美股",
			}[marketType]
			return util.MarkdownTableWithTitle(marketName+"热门股票排行榜", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetHotEventList",
		"获取雪球热门话题/事件",
		map[string]*schema.ParameterInfo{
			"size": {
				Type:     "integer",
				Desc:     "返回条数，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			size := int(gjson.Get(args, "size").Int())
			if size <= 0 {
				size = 20
			}
			hotEvents := data.NewMarketNewsApi().HotEvent(size)
			if hotEvents == nil || len(*hotEvents) == 0 {
				return "暂无热门话题数据", nil
			}
			type hotEventRow struct {
				Rank    int    `md:"排名"`
				Tag     string `md:"话题标签"`
				Content string `md:"话题内容"`
				Hot     int    `md:"热度"`
			}
			var rows []hotEventRow
			for i, event := range *hotEvents {
				rows = append(rows, hotEventRow{
					Rank:    i + 1,
					Tag:     event.Tag,
					Content: event.Content,
					Hot:     event.Hot,
				})
			}
			return util.MarkdownTableWithTitle("雪球热门话题", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetIndustryMoneyRank",
		"获取行业资金流向排名（按行业分类）",
		map[string]*schema.ParameterInfo{
			"fenlei": {
				Type:     "string",
				Desc:     "行业分类：0=所有行业, 1=行业分类, 2=概念板块, 3=地域板块",
				Required: false,
			},
			"sort": {
				Type:     "string",
				Desc:     "排序字段：netamount=净流入, netbuy=主力净流入, change=涨跌幅",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "返回条数",
				Required: false,
			},
		},
		func(args string) (string, error) {
			fenlei := gjson.Get(args, "fenlei").String()
			sort := gjson.Get(args, "sort").String()
			limit := int(gjson.Get(args, "limit").Int())
			if limit <= 0 {
				limit = 20
			}
			if fenlei == "" {
				fenlei = "1"
			}
			if sort == "" {
				sort = "netamount"
			}
			rankData := data.NewMarketNewsApi().GetIndustryMoneyRankSina(fenlei, sort)
			if len(rankData) == 0 {
				return "暂无行业资金流向数据", nil
			}
			type industryRankRow struct {
				Rank       int     `md:"排名"`
				Name       string  `md:"板块名称"`
				NetAmount  float64 `md:"净流入(万)"`
				NetBuy     float64 `md:"主力净流入(万)"`
				ChangeRate float64 `md:"涨跌幅(%)"`
			}
			var rows []industryRankRow
			for i, item := range rankData {
				if i >= limit {
					break
				}
				netAmount, _ := convertor.ToFloat(item["netamount"])
				netBuy, _ := convertor.ToFloat(item["netbuy"])
				changeRate, _ := convertor.ToFloat(item["change_rate"])
				rows = append(rows, industryRankRow{
					Rank:       i + 1,
					Name:       convertor.ToString(item["name"]),
					NetAmount:  netAmount / 10000,
					NetBuy:     netBuy / 10000,
					ChangeRate: changeRate,
				})
			}
			fenleiName := map[string]string{"0": "所有行业", "1": "行业分类", "2": "概念板块", "3": "地域板块"}[fenlei]
			return util.MarkdownTableWithTitle(fenleiName+"资金流向排名", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetLongTigerList",
		"获取龙虎榜数据（营业部排行榜）",
		map[string]*schema.ParameterInfo{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：2026-03-28",
				Required: true,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			longTigerData := data.NewMarketNewsApi().LongTiger(date)
			if longTigerData == nil || len(*longTigerData) == 0 {
				return "当日暂无龙虎榜数据", nil
			}
			type longTigerRow struct {
				Rank         int     `md:"排名"`
				Code         string  `md:"股票代码"`
				Name         string  `md:"股票名称"`
				ClosePrice   float64 `md:"收盘价"`
				ChangeRate   float64 `md:"涨跌幅(%)"`
				BizNetAmt    float64 `md:"营业部净买入(万)"`
				TurnoverRate float64 `md:"换手率(%)"`
			}
			var rows []longTigerRow
			for i, item := range *longTigerData {
				if i >= 50 {
					break
				}
				closePrice, _ := convertor.ToFloat(item.CLOSEPRICE)
				changeRate, _ := convertor.ToFloat(item.CHANGERATE)
				bizNetAmt, _ := convertor.ToFloat(item.BILLBOARDNETAMT)
				turnoverRate, _ := convertor.ToFloat(item.TURNOVERRATE)
				rows = append(rows, longTigerRow{
					Rank:         i + 1,
					Code:         item.SECURITYCODE,
					Name:         item.SECURITYNAMEABBR,
					ClosePrice:   closePrice,
					ChangeRate:   changeRate,
					BizNetAmt:    bizNetAmt / 10000,
					TurnoverRate: turnoverRate,
				})
			}
			return util.MarkdownTableWithTitle(date+" 龙虎榜数据", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetEconomicData",
		"获取宏观经济数据，包括GDP、CPI、PPI、PMI等",
		map[string]*schema.ParameterInfo{
			"dataType": {
				Type:     "string",
				Desc:     "数据类型：gdp=国内生产总值, cpi=居民消费价格指数, ppi=工业生产者出厂价格指数, pmi=采购经理指数",
				Required: true,
			},
		},
		func(args string) (string, error) {
			dataType := gjson.Get(args, "dataType").String()
			if dataType == "" {
				return "请指定数据类型", nil
			}
			api := data.NewMarketNewsApi()
			var result string
			switch dataType {
			case "gdp":
				gdp := api.GetGDP()
				if gdp == nil || len(gdp.GDPResult.Data) == 0 {
					return "暂无GDP数据", nil
				}
				type gdpRow struct {
					Time           string  `md:"时间"`
					Total          float64 `md:"国内生产总值(亿元)"`
					FirstIndustry  float64 `md:"第一产业(亿元)"`
					SecondIndustry float64 `md:"第二产业(亿元)"`
					ThirdIndustry  float64 `md:"第三产业(亿元)"`
				}
				var rows []gdpRow
				for _, d := range gdp.GDPResult.Data {
					rows = append(rows, gdpRow{
						Time:           d.TIME,
						Total:          d.DOMESTICLPRODUCTBASE / 100000000,
						FirstIndustry:  d.FIRSTPRODUCTBASE / 100000000,
						SecondIndustry: d.SECONDPRODUCTBASE / 100000000,
						ThirdIndustry:  d.THIRDPRODUCTBASE / 100000000,
					})
				}
				result = util.MarkdownTableWithTitle("国内生产总值(GDP)", rows)
			case "cpi":
				cpi := api.GetCPI()
				if cpi == nil || len(cpi.CPIResult.Data) == 0 {
					return "暂无CPI数据", nil
				}
				type cpiRow struct {
					Time         string  `md:"时间"`
					NationalBase float64 `md:"全国当月"`
					NationalSame float64 `md:"全国同比增长(%)"`
					CityBase     float64 `md:"城市当月"`
					RuralBase    float64 `md:"农村当月"`
				}
				var rows []cpiRow
				for _, d := range cpi.CPIResult.Data {
					rows = append(rows, cpiRow{
						Time:         d.TIME,
						NationalBase: d.NATIONALBASE,
						NationalSame: d.NATIONALSAME,
						CityBase:     d.CITYBASE,
						RuralBase:    d.RURALBASE,
					})
				}
				result = util.MarkdownTableWithTitle("居民消费价格指数(CPI)", rows)
			case "ppi":
				ppi := api.GetPPI()
				if ppi == nil || len(ppi.PPIResult.Data) == 0 {
					return "暂无PPI数据", nil
				}
				type ppiRow struct {
					Time  string  `md:"时间"`
					Base  float64 `md:"当月指数"`
					Same  float64 `md:"同比增长(%)"`
					Accum float64 `md:"累计指数"`
				}
				var rows []ppiRow
				for _, d := range ppi.PPIResult.Data {
					rows = append(rows, ppiRow{
						Time:  d.TIME,
						Base:  d.BASE,
						Same:  d.BASESAME,
						Accum: d.BASEACCUMULATE,
					})
				}
				result = util.MarkdownTableWithTitle("工业生产者出厂价格指数(PPI)", rows)
			case "pmi":
				pmi := api.GetPMI()
				if pmi == nil || len(pmi.PMIResult.Data) == 0 {
					return "暂无PMI数据", nil
				}
				type pmiRow struct {
					Time       string  `md:"时间"`
					MakeIndex  float64 `md:"制造业PMI"`
					MakeSame   float64 `md:"制造业同比增长(%)"`
					NMakeIndex float64 `md:"非制造业PMI"`
					NMakeSame  float64 `md:"非制造业同比增长(%)"`
				}
				var rows []pmiRow
				for _, d := range pmi.PMIResult.Data {
					rows = append(rows, pmiRow{
						Time:       d.TIME,
						MakeIndex:  d.MAKEINDEX,
						MakeSame:   d.MAKESAME,
						NMakeIndex: d.NMAKEINDEX,
						NMakeSame:  d.NMAKESAME,
					})
				}
				result = util.MarkdownTableWithTitle("采购经理指数(PMI)", rows)
			default:
				return "不支持的数据类型：" + dataType, nil
			}
			return result, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetInvestCalendar",
		"获取投资日历，包括财报发布、股东大会、IPO等重要日期事件",
		map[string]*schema.ParameterInfo{
			"yearMonth": {
				Type:     "string",
				Desc:     "年月，格式：2026-03",
				Required: false,
			},
		},
		func(args string) (string, error) {
			yearMonth := gjson.Get(args, "yearMonth").String()
			if yearMonth == "" {
				yearMonth = time.Now().Format("2006-01")
			}
			calendarData := data.NewMarketNewsApi().InvestCalendar(yearMonth)
			if len(calendarData) == 0 {
				return "当日暂无投资日历数据", nil
			}
			type calendarRow struct {
				Date      string `md:"日期"`
				Type      string `md:"事件类型"`
				StockCode string `md:"股票代码"`
				StockName string `md:"股票名称"`
				Title     string `md:"事件标题"`
			}
			var rows []calendarRow
			for _, item := range calendarData {
				if m, ok := item.(map[string]any); ok {
					date := convertor.ToString(m["date"])
					eventType := convertor.ToString(m["type"])
					title := convertor.ToString(m["title"])
					stockCode := ""
					stockName := ""
					if stock, ok := m["stock"].(map[string]any); ok {
						stockCode = convertor.ToString(stock["code"])
						stockName = convertor.ToString(stock["name"])
					}
					rows = append(rows, calendarRow{
						Date:      date,
						Type:      eventType,
						StockCode: stockCode,
						StockName: stockName,
						Title:     title,
					})
				}
			}
			return util.MarkdownTableWithTitle(yearMonth+" 投资日历", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockNotice",
		"获取个股公告信息",
		map[string]*schema.ParameterInfo{
			"stockCodes": {
				Type:     "string",
				Desc:     "股票代码列表，逗号分隔，如：600519,000001",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCodes := gjson.Get(args, "stockCodes").String()
			if stockCodes == "" {
				return "请输入股票代码", nil
			}
			noticeData := data.NewMarketNewsApi().StockNotice(stockCodes)
			if len(noticeData) == 0 {
				return "暂无公告数据", nil
			}
			type noticeRow struct {
				NoticeTime string `md:"公告时间"`
				Title      string `md:"公告标题"`
				Code       string `md:"股票代码"`
			}
			var rows []noticeRow
			for _, item := range noticeData {
				if m, ok := item.(map[string]any); ok {
					noticeTime := convertor.ToString(m["notice_time"])
					title := convertor.ToString(m["title"])
					code := convertor.ToString(m["secu_code"])
					rows = append(rows, noticeRow{
						NoticeTime: noticeTime,
						Title:      title,
						Code:       code,
					})
				}
			}
			return util.MarkdownTableWithTitle("个股公告", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockMinuteData",
		"获取股票分时数据（当日分钟级成交量和价格）",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：600519.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			if stockCode == "" {
				return "请输入股票代码", nil
			}
			minuteData, updateTime := data.NewStockDataApi().GetStockMinutePriceData(stockCode)
			if minuteData == nil || len(*minuteData) == 0 {
				return "未获取到分时数据", nil
			}
			type minuteRow struct {
				Time   string  `md:"时间"`
				Price  float64 `md:"价格"`
				Volume float64 `md:"成交量(手)"`
				Amount float64 `md:"成交额(元)"`
			}
			var rows []minuteRow
			for _, m := range *minuteData {
				price, _ := convertor.ToFloat(m.Price)
				volume, _ := convertor.ToFloat(m.Volume)
				amount, _ := convertor.ToFloat(m.Amount)
				rows = append(rows, minuteRow{
					Time:   m.Time,
					Price:  price,
					Volume: volume,
					Amount: amount,
				})
			}
			return fmt.Sprintf("**更新时间**: %s\n\n%s", updateTime, util.MarkdownTableWithTitle(stockCode+" 分时数据", rows)), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockConceptInfo",
		"获取股票的概念板块信息，包括概念名称、成分股数量等",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：600519",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			if stockCode == "" {
				return "请输入股票代码", nil
			}
			conceptInfo := data.NewStockDataApi().GetStockConceptInfo(stockCode)
			if !conceptInfo.Success || len(conceptInfo.Result.Data) == 0 {
				return "未获取到概念板块信息", nil
			}
			type conceptRow struct {
				BoardName string  `md:"概念名称"`
				Desc      string  `md:"概念描述"`
				Yield     float64 `md:"涨跌幅(%)"`
			}
			var rows []conceptRow
			for _, c := range conceptInfo.Result.Data {
				rows = append(rows, conceptRow{
					BoardName: c.BOARDNAME,
					Desc:      c.SELECTEDBOARDREASON,
					Yield:     c.BOARDYIELD,
				})
			}
			return util.MarkdownTableWithTitle(stockCode+" 概念板块", rows), nil
		},
	))

	//tools = append(tools, NewDataToolWrapper(
	//	"WebSearch",
	//	"联网搜索工具，支持搜索最新财经新闻、股票资讯、市场动态等信息。使用此工具获取实时网络数据。",
	//	map[string]*schema.ParameterInfo{
	//		"query": {
	//			Type:     "string",
	//			Desc:     "搜索关键词/问题，如：茅台最新股价、A股今日行情、人工智能板块最新消息",
	//			Required: true,
	//		},
	//		"maxResults": {
	//			Type:     "integer",
	//			Desc:     "最大返回结果数，默认10条",
	//			Required: false,
	//		},
	//	},
	//	func(args string) (string, error) {
	//		defer data.GetBrowserManager().ResetBrowser()
	//		query := gjson.Get(args, "query").String()
	//		maxResults := int(gjson.Get(args, "maxResults").Int())
	//		if query == "" {
	//			return "请输入搜索关键词", nil
	//		}
	//		if maxResults <= 0 {
	//			maxResults = 10
	//		}
	//		searchApi := data.NewWebSearchApi(30)
	//		return searchApi.SearchToMarkdown(query, maxResults), nil
	//	},
	//))

	//tools = append(tools, NewDataToolWrapper(
	//	"SearchParams",
	//	"联网搜索股票技术指标参数、财务参数、API参数等的工具，用于查询指标的计算公式、参数设置和用法说明。",
	//	map[string]*schema.ParameterInfo{
	//		"paramName": {
	//			Type:     "string",
	//			Desc:     "参数/指标名称，如：MACD、RSI、布林带、PE、PB等技术指标或财务指标名称",
	//			Required: true,
	//		},
	//		"searchScope": {
	//			Type:     "string",
	//			Desc:     "搜索范围：technical=技术指标参数, financial=财务指标参数, api=API参数说明",
	//			Required: false,
	//		},
	//	},
	//	func(args string) (string, error) {
	//		defer data.GetBrowserManager().ResetBrowser()
	//		paramName := gjson.Get(args, "paramName").String()
	//		searchScope := gjson.Get(args, "searchScope").String()
	//		if paramName == "" {
	//			return "请输入参数名称", nil
	//		}
	//		var query string
	//		switch searchScope {
	//		case "technical":
	//			query = fmt.Sprintf("股票 %s 指标参数设置 计算公式 使用方法", paramName)
	//		case "financial":
	//			query = fmt.Sprintf("股票 %s 财务指标参数 计算公式 含义", paramName)
	//		case "api":
	//			query = fmt.Sprintf("%s API参数说明 接口文档", paramName)
	//		default:
	//			query = fmt.Sprintf("股票 %s 指标参数 计算公式 使用方法", paramName)
	//		}
	//		searchApi := data.NewWebSearchApi(30)
	//		return searchApi.SearchToMarkdown(query, 5), nil
	//	},
	//))

	tools = append(tools, NewDataToolWrapper(
		"GetStockChanges",
		"获取股票异动数据，包括火箭发射、快速反弹、大笔买入、封涨停板、加速下跌、高台跳水、大笔卖出、封跌停板等异动类型。",
		map[string]*schema.ParameterInfo{
			"changeTypes": {
				Type:     "string",
				Desc:     "异动类型，多个用逗号分隔：火箭发射=8201,快速反弹=8202,大笔买入=8193,封涨停板=4,打开跌停板=32,有大买盘=64,竞价上涨=8207,高开5日线=8209,向上缺口=8211,60日新高=8213,60日大幅上涨=8215,加速下跌=8204,高台跳水=8203,大笔卖出=8194,封跌停板=8,打开涨停板=16,有大卖盘=128,竞价下跌=8208,低开5日线=8210,向下缺口=8212,60日新低=8214,60日大幅下跌=8216。默认查询火箭发射、快速反弹、大笔买入、封涨停板、加速下跌、高台跳水、大笔卖出、封跌停板",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			changeTypesStr := gjson.Get(args, "changeTypes").String()
			pageSize := int(gjson.Get(args, "pageSize").Int())
			if pageSize <= 0 {
				pageSize = 20
			}

			var changeTypes []int
			if changeTypesStr != "" {
				typeMap := map[string]int{
					"火箭发射": 8201, "快速反弹": 8202, "大笔买入": 8193, "封涨停板": 4,
					"打开跌停板": 32, "有大买盘": 64, "竞价上涨": 8207, "高开5日线": 8209,
					"向上缺口": 8211, "60日新高": 8213, "60日大幅上涨": 8215,
					"加速下跌": 8204, "高台跳水": 8203, "大笔卖出": 8194, "封跌停板": 8,
					"打开涨停板": 16, "有大卖盘": 128, "竞价下跌": 8208, "低开5日线": 8210,
					"向下缺口": 8212, "60日新低": 8214, "60日大幅下跌": 8216,
				}
				for _, t := range strings.Split(changeTypesStr, ",") {
					t = strings.TrimSpace(t)
					if code, ok := typeMap[t]; ok {
						changeTypes = append(changeTypes, code)
					} else if code, err := strconv.Atoi(t); err == nil {
						changeTypes = append(changeTypes, code)
					}
				}
			}

			return data.NewStockChangesApi().GetStockChangesReadable(changeTypes, 0, pageSize), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetStockChangeHistoryList",
		"查询股票异动历史记录。可以根据股票代码、股票名称、异动类型、日期范围、成交量、金额、涨跌幅、行业、概念等条件筛选历史异动数据。",
		map[string]*schema.ParameterInfo{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码筛选（可选），支持模糊匹配",
				Required: false,
			},
			"stockName": {
				Type:     "string",
				Desc:     "股票名称筛选（可选），支持模糊匹配",
				Required: false,
			},
			"changeType": {
				Type:     "integer",
				Desc:     "异动类型代码筛选（可选）。完整列表：8201=火箭发射,8202=快速反弹,8193=大笔买入,4=封涨停板,32=打开跌停板,64=有大买盘,8207=竞价上涨,8209=高开5日线,8211=向上缺口,8213=60日新高,8215=60日大幅上涨,8204=加速下跌,8203=高台跳水,8194=大笔卖出,8=封跌停板,16=打开涨停板,128=有大卖盘,8208=竞价下跌,8210=低开5日线,8212=向下缺口,8214=60日新低,8216=60日大幅下跌",
				Required: false,
			},
			"typeName": {
				Type:     "string",
				Desc:     "异动类型名称筛选（可选）。完整列表：火箭发射、快速反弹、大笔买入、封涨停板、打开跌停板、有大买盘、竞价上涨、高开5日线、向上缺口、60日新高、60日大幅上涨、加速下跌、高台跳水、大笔卖出、封跌停板、打开涨停板、有大卖盘、竞价下跌、低开5日线、向下缺口、60日新低、60日大幅下跌",
				Required: false,
			},
			"startDate": {
				Type:     "string",
				Desc:     "开始日期，格式：YYYY-MM-DD（可选）",
				Required: true,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束日期，格式：YYYY-MM-DD（可选）",
				Required: true,
			},
			"startTime": {
				Type:     "string",
				Desc:     "开始时间，格式：HH:MM:SS（可选）",
				Required: true,
			},
			"endTime": {
				Type:     "string",
				Desc:     "结束时间，格式：HH:MM:SS（可选）",
				Required: true,
			},
			"minVolume": {
				Type:     "integer",
				Desc:     "最小成交量筛选（股），如50000表示大于500手（可选）",
				Required: false,
			},
			"minAmount": {
				Type:     "number",
				Desc:     "最小金额筛选（元），如10000000表示大于1000万（可选）",
				Required: false,
			},
			"minChangeRate": {
				Type:     "number",
				Desc:     "最小涨跌幅筛选（%），如5表示涨幅大于5%（可选）",
				Required: false,
			},
			"maxChangeRate": {
				Type:     "number",
				Desc:     "最大涨跌幅筛选（%），如-5表示跌幅小于-5%（可选）",
				Required: false,
			},
			"industry": {
				Type:     "string",
				Desc:     "行业关键词筛选（可选），支持模糊匹配",
				Required: false,
			},
			"concept": {
				Type:     "string",
				Desc:     "概念关键词筛选（可选），支持模糊匹配",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: true,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认20",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			stockName := gjson.Get(args, "stockName").String()
			changeType := int(gjson.Get(args, "changeType").Int())
			typeName := gjson.Get(args, "typeName").String()
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			page := int(gjson.Get(args, "page").Int())
			pageSize := int(gjson.Get(args, "pageSize").Int())
			startTime := gjson.Get(args, "startTime").String()
			endTime := gjson.Get(args, "endTime").String()
			minVolume := gjson.Get(args, "minVolume").Int()
			minAmount := gjson.Get(args, "minAmount").Float()
			minChangeRate := gjson.Get(args, "minChangeRate").Float()
			maxChangeRate := gjson.Get(args, "maxChangeRate").Float()
			industry := gjson.Get(args, "industry").String()
			concept := gjson.Get(args, "concept").String()

			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = 20
			}

			query := models.StockChangeHistoryQuery{
				StockCode:     stockCode,
				StockName:     stockName,
				ChangeType:    changeType,
				TypeName:      typeName,
				StartDate:     startDate,
				EndDate:       endDate,
				Page:          page,
				PageSize:      pageSize,
				StartTime:     startTime,
				EndTime:       endTime,
				MinVolume:     minVolume,
				MinAmount:     minAmount,
				MinChangeRate: minChangeRate,
				MaxChangeRate: maxChangeRate,
				Industry:      industry,
				Concept:       concept,
			}

			pageData, err := data.NewStockChangeHistoryService().GetHistoryList(query)
			if err != nil {
				return "", err
			}

			if pageData == nil || len(pageData.List) == 0 {
				return "未找到符合条件的异动历史记录", nil
			}

			type historyRow struct {
				ChangeTime string  `md:"异动时间"`
				ChangeDate string  `md:"异动日期"`
				StockCode  string  `md:"股票代码"`
				StockName  string  `md:"股票名称"`
				TypeName   string  `md:"异动类型"`
				Volume     int64   `md:"成交量(股)"`
				Price      float64 `md:"价格"`
				ChangeRate float64 `md:"涨跌幅(%)"`
				Amount     float64 `md:"金额"`
				Industry   string  `md:"所属行业"`
				Concept    string  `md:"所属概念"`
			}

			var rows []historyRow
			for _, item := range pageData.List {
				rows = append(rows, historyRow{
					ChangeTime: item.ChangeTime,
					ChangeDate: item.ChangeDate,
					StockCode:  item.StockCode,
					StockName:  item.StockName,
					TypeName:   item.TypeName,
					Volume:     item.Volume,
					Price:      item.Price,
					ChangeRate: item.ChangeRate,
					Amount:     item.Amount,
					Industry:   item.Industry,
					Concept:    item.Concept,
				})
			}

			summary := fmt.Sprintf("共找到 %d 条异动历史记录，当前第 %d/%d 页", pageData.Total, page, pageData.TotalPages)
			return summary + "\n\n" + util.MarkdownTableWithTitle("股票异动历史记录", rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetDailyChangeStats",
		"获取近N日每日异动统计趋势，包括每天的上涨异动数、下跌异动数、封涨停数、封跌停数和总异动数。用于分析市场异动活跃度的变化趋势。",
		map[string]*schema.ParameterInfo{
			"days": {
				Type:     "integer",
				Desc:     "查询天数，如7表示近7日，30表示近30日，默认30",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 30
			}
			result, err := data.NewStockChangeHistoryService().GetDailyChangeStats(days)
			if err != nil {
				return "", err
			}
			if len(result) == 0 {
				return "暂无异动统计数据", nil
			}
			type row struct {
				ChangeDate string `md:"日期"`
				TotalCount int64  `md:"总异动数"`
				UpCount    int64  `md:"上涨异动"`
				DownCount  int64  `md:"下跌异动"`
				LimitUp    int64  `md:"封涨停"`
				LimitDown  int64  `md:"封跌停"`
			}
			var rows []row
			for _, d := range result {
				rows = append(rows, row{
					ChangeDate: d.ChangeDate,
					TotalCount: d.TotalCount,
					UpCount:    d.UpCount,
					DownCount:  d.DownCount,
					LimitUp:    d.LimitUp,
					LimitDown:  d.LimitDown,
				})
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("近%d日每日异动统计", days), rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetChangeRank",
		"获取异动次数排行榜，支持按股票、行业、概念三个维度排名，区分利好异动（封涨停板、火箭发射、快速反弹等）和利空异动（封跌停板、高台跳水、加速下跌等）。",
		map[string]*schema.ParameterInfo{
			"days": {
				Type:     "integer",
				Desc:     "查询天数，1=当日，3=近3日，5=近5日，10=近10日，默认1",
				Required: false,
			},
			"topN": {
				Type:     "integer",
				Desc:     "返回排名前N个，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 1
			}
			topN := int(gjson.Get(args, "topN").Int())
			if topN <= 0 {
				topN = 20
			}
			result, err := data.NewStockChangeHistoryService().GetChangeRank(days, topN)
			if err != nil {
				return "", err
			}
			periodLabel := "当日"
			if days > 1 {
				periodLabel = fmt.Sprintf("近%d日", days)
			}
			var sb strings.Builder
			if len(result.TopStocks) > 0 {
				type row struct {
					Rank      int    `md:"排名"`
					StockName string `md:"股票名称"`
					StockCode string `md:"股票代码"`
					UpCount   int64  `md:"利好异动"`
					DownCount int64  `md:"利空异动"`
					Total     int64  `md:"合计"`
				}
				var rows []row
				for i, d := range result.TopStocks {
					rows = append(rows, row{Rank: i + 1, StockName: d.Name, StockCode: d.Code, UpCount: d.UpCount, DownCount: d.DownCount, Total: d.Count})
				}
				sb.WriteString(util.MarkdownTableWithTitle(periodLabel+"异动次数最多的股票", rows))
				sb.WriteString("\n\n")
			}
			if len(result.TopIndustries) > 0 {
				type row struct {
					Rank      int    `md:"排名"`
					Industry  string `md:"行业"`
					UpCount   int64  `md:"利好异动"`
					DownCount int64  `md:"利空异动"`
					Total     int64  `md:"合计"`
				}
				var rows []row
				for i, d := range result.TopIndustries {
					rows = append(rows, row{Rank: i + 1, Industry: d.Name, UpCount: d.UpCount, DownCount: d.DownCount, Total: d.Count})
				}
				sb.WriteString(util.MarkdownTableWithTitle(periodLabel+"异动次数最多的行业", rows))
				sb.WriteString("\n\n")
			}
			if len(result.TopConcepts) > 0 {
				type row struct {
					Rank      int    `md:"排名"`
					Concept   string `md:"概念"`
					UpCount   int64  `md:"利好异动"`
					DownCount int64  `md:"利空异动"`
					Total     int64  `md:"合计"`
				}
				var rows []row
				for i, d := range result.TopConcepts {
					rows = append(rows, row{Rank: i + 1, Concept: d.Name, UpCount: d.UpCount, DownCount: d.DownCount, Total: d.Count})
				}
				sb.WriteString(util.MarkdownTableWithTitle(periodLabel+"异动次数最多的概念", rows))
			}
			output := sb.String()
			if output == "" {
				return "暂无异动排行数据", nil
			}
			return output, nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetDailyDimensionStats",
		"按维度查询近N日每日异动趋势，支持按股票、行业、概念、异动类型四个维度查询，返回每天的利好异动数、利空异动数和总异动数。用于分析某个股票/行业/概念/异动类型在一段时间内的异动变化趋势。",
		map[string]*schema.ParameterInfo{
			"dimension": {
				Type:     "string",
				Desc:     "查询维度：stock=按股票，industry=按行业，concept=按概念，type=按异动类型",
				Required: true,
			},
			"name": {
				Type:     "string",
				Desc:     "维度名称，如股票名称/代码、行业名称、概念名称、异动类型名称",
				Required: true,
			},
			"days": {
				Type:     "integer",
				Desc:     "查询天数，默认30",
				Required: false,
			},
		},
		func(args string) (string, error) {
			dimension := gjson.Get(args, "dimension").String()
			name := gjson.Get(args, "name").String()
			days := int(gjson.Get(args, "days").Int())
			if dimension == "" || name == "" {
				return "请提供dimension和name参数", nil
			}
			if days <= 0 {
				days = 30
			}
			result, err := data.NewStockChangeHistoryService().GetDailyDimensionStats(dimension, name, days)
			if err != nil {
				return "", err
			}
			if len(result) == 0 {
				return fmt.Sprintf("未找到%s[%s]的异动趋势数据", dimension, name), nil
			}
			type row struct {
				ChangeDate string `md:"日期"`
				UpCount    int64  `md:"利好异动"`
				DownCount  int64  `md:"利空异动"`
				TotalCount int64  `md:"总异动数"`
			}
			var rows []row
			for _, d := range result {
				rows = append(rows, row{
					ChangeDate: d.ChangeDate,
					UpCount:    d.UpCount,
					DownCount:  d.DownCount,
					TotalCount: d.TotalCount,
				})
			}
			dimLabels := map[string]string{"stock": "股票", "industry": "行业", "concept": "概念", "type": "异动类型"}
			title := fmt.Sprintf("%s[%s]近%d日异动趋势", dimLabels[dimension], name, days)
			return util.MarkdownTableWithTitle(title, rows), nil
		},
	))

	tools = append(tools, NewDataToolWrapper(
		"GetTypeStatsByDate",
		"查询某一天的异动类型分布统计，返回该天每种异动类型的利好/利空次数和总次数。用于分析某天的市场异动结构。",
		map[string]*schema.ParameterInfo{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：YYYY-MM-DD，如2025-04-13",
				Required: true,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			if date == "" {
				return "请提供date参数", nil
			}
			result, err := data.NewStockChangeHistoryService().GetTypeStatsByDate(date)
			if err != nil {
				return "", err
			}
			if len(result) == 0 {
				return fmt.Sprintf("未找到%s的异动类型分布数据", date), nil
			}
			type row struct {
				TypeName   string `md:"异动类型"`
				UpCount    int64  `md:"利好异动"`
				DownCount  int64  `md:"利空异动"`
				TotalCount int64  `md:"总次数"`
			}
			var rows []row
			for _, d := range result {
				rows = append(rows, row{
					TypeName:   d.TypeName,
					UpCount:    d.UpCount,
					DownCount:  d.DownCount,
					TotalCount: d.TotalCount,
				})
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("%s异动类型分布", date), rows), nil
		},
	))

	return tools
}

func marketSentiment(upCount, downCount int) string {
	ratio := float64(upCount) / float64(upCount+downCount)
	if ratio > 0.7 {
		return "极度乐观"
	} else if ratio > 0.6 {
		return "乐观"
	} else if ratio > 0.4 {
		return "中性"
	} else if ratio > 0.3 {
		return "悲观"
	} else {
		return "极度悲观"
	}
}

func parseStockCodesFromArgs(args string, mainField string) []string {
	var codes []string
	mainCode := gjson.Get(args, mainField).String()
	if mainCode != "" {
		for _, c := range strings.Split(mainCode, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				codes = append(codes, c)
			}
		}
	}
	codesArr := gjson.Get(args, "stockCodes").Array()
	for _, c := range codesArr {
		code := strings.TrimSpace(c.String())
		if code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}

func parseInt(s string) (int, error) {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result, nil
}

type APIResponse struct {
	Code int     `json:"code"`
	Msg  string  `json:"msg"`
	Data APIData `json:"data"`
}

type APIData struct {
	IndexQuote    []APIIndexQuote `json:"index_quote"`
	UpDownDis     APIUpDownDis    `json:"up_down_dis"`
	PurchaseToday []APIPurchase   `json:"purchase_today"`
}

type APIIndexQuote struct {
	SecuCode string  `json:"secu_code" md:"指数代码"`
	SecuName string  `json:"secu_name" md:"指数名称"`
	LastPx   float64 `json:"last_px" md:"最新价格"`
	Change   float64 `json:"change" md:"涨跌"`
	ChangePx float64 `json:"change_px" md:"涨跌点数"`
	UpNum    int     `json:"up_num" md:"上涨家数"`
	DownNum  int     `json:"down_num" md:"下跌家数"`
	FlatNum  int     `json:"flat_num" md:"平盘家数"`
}

type APIUpDownDis struct {
	UpNum       int     `json:"up_num" md:"涨停家数"`
	DownNum     int     `json:"down_num" md:"跌停家数"`
	AverageRise float64 `json:"average_rise" md:"平均涨幅"`
	RiseNum     int     `json:"rise_num" md:"上涨家数总计"`
	FallNum     int     `json:"fall_num" md:"下跌家数总计"`
	Down10      int     `json:"down_10" md:"跌幅8%~10%家数"`
	Down8       int     `json:"down_8" md:"跌幅6%~8%家数"`
	Down6       int     `json:"down_6" md:"跌幅4%~6%家数"`
	Down4       int     `json:"down_4" md:"跌幅2%~4%家数"`
	Down2       int     `json:"down_2" md:"跌幅0%~2%家数"`
	FlatNum     int     `json:"flat_num" md:"平盘家数"`
	Up2         int     `json:"up_2" md:"涨幅0%~2%家数"`
	Up4         int     `json:"up_4" md:"涨幅2%~4%家数"`
	Up6         int     `json:"up_6" md:"涨幅4%~6%家数"`
	Up8         int     `json:"up_8" md:"涨幅6%~8%家数"`
	Up10        int     `json:"up_10" md:"涨幅8%~10%家数"`
	SuspendNum  int     `json:"suspend_num" md:"停牌家数"`
	Status      bool    `json:"status" md:"状态"`
}

type APIPurchase struct {
	SecuCode     string   `json:"secu_code"`
	SecuName     string   `json:"secu_name"`
	SecuCodeFull string   `json:"SecuCode"`
	IPOPrice     float64  `json:"ipo_price"`
	IPOPE        float64  `json:"ipo_pe"`
	AllotMax     int      `json:"allot_max"`
	LotRate      *float64 `json:"lot_rate"`
}

func getMarketDataContent() (string, error) {
	client := resty.New()
	apiURL := "https://x-quote.cls.cn/quote/index/home?app=CailianpressWeb&os=web&sv=8.4.6"

	uaGen, err := fakeUserAgent.New()
	if err != nil {
		uaGen, _ = fakeUserAgent.New()
	}
	ua := uaGen.GetRandom()

	var apiResp APIResponse
	resp, err := client.R().
		SetHeader("User-Agent", ua).
		SetResult(&apiResp).
		Get(apiURL)

	if err != nil {
		return "", fmt.Errorf("调用API失败: %v", err)
	}

	if resp.StatusCode() != 200 || apiResp.Code != 200 {
		return "", fmt.Errorf("API返回错误: 状态码=%d, 错误信息=%s", resp.StatusCode(), apiResp.Msg)
	}

	content := strings.Builder{}
	content.WriteString("# 市场行情数据\r\n\r\n")

	content.WriteString("## 指数行情\r\n\r\n")
	content.WriteString("| 指数代码 | 指数名称 | 最新价格 | 涨跌(%) | 涨跌点数 | 上涨家数 | 下跌家数 | 平盘家数 |\r\n")
	content.WriteString("|----------|----------|----------|---------|----------|----------|----------|----------|\r\n")
	for _, index := range apiResp.Data.IndexQuote {
		content.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f | %.2f | %d | %d | %d |\r\n",
			index.SecuCode, index.SecuName, index.LastPx, index.Change*100, index.ChangePx,
			index.UpNum, index.DownNum, index.FlatNum))
	}

	content.WriteString("\r\n## 涨跌分布\r\n\r\n")
	content.WriteString("| 涨停家数 | 跌停家数 | 平均涨幅(%) | 上涨家数总计 | 下跌家数总计 |\r\n")
	content.WriteString("|----------|----------|-------------|--------------|--------------|\r\n")
	content.WriteString(fmt.Sprintf("| %d | %d | %.2f | %d | %d |\r\n",
		apiResp.Data.UpDownDis.UpNum, apiResp.Data.UpDownDis.DownNum, apiResp.Data.UpDownDis.AverageRise*100,
		apiResp.Data.UpDownDis.RiseNum, apiResp.Data.UpDownDis.FallNum))

	content.WriteString("\r\n### 跌幅分布\r\n\r\n")
	content.WriteString("| 跌幅8%~10% | 跌幅6%~8% | 跌幅4%~6% | 跌幅2%~4% | 跌幅0%~2% |\r\n")
	content.WriteString("|-----------|-----------|-----------|-----------|-----------|\r\n")
	content.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d |\r\n",
		apiResp.Data.UpDownDis.Down10, apiResp.Data.UpDownDis.Down8, apiResp.Data.UpDownDis.Down6,
		apiResp.Data.UpDownDis.Down4, apiResp.Data.UpDownDis.Down2))

	content.WriteString("\r\n### 涨幅分布\r\n\r\n")
	content.WriteString("| 涨幅0%~2% | 涨幅2%~4% | 涨幅4%~6% | 涨幅6%~8% | 涨幅8%~10% |\r\n")
	content.WriteString("|-----------|-----------|-----------|-----------|------------|\r\n")
	content.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d |\r\n",
		apiResp.Data.UpDownDis.Up2, apiResp.Data.UpDownDis.Up4, apiResp.Data.UpDownDis.Up6,
		apiResp.Data.UpDownDis.Up8, apiResp.Data.UpDownDis.Up10))

	content.WriteString("\r\n### 其他统计\r\n\r\n")
	content.WriteString(fmt.Sprintf("- 平盘家数: %d\r\n", apiResp.Data.UpDownDis.FlatNum))
	content.WriteString(fmt.Sprintf("- 停牌家数: %d\r\n", apiResp.Data.UpDownDis.SuspendNum))

	content.WriteString("\r\n## 今日申购\r\n\r\n")
	if len(apiResp.Data.PurchaseToday) > 0 {
		content.WriteString("| 股票代码 | 股票名称 | 申购价格 | 申购PE | 最大申购数量 | 中签率(%) |\r\n")
		content.WriteString("|----------|----------|----------|--------|--------------|-----------|\r\n")
		for _, purchase := range apiResp.Data.PurchaseToday {
			lotRate := "-"
			if purchase.LotRate != nil {
				lotRate = fmt.Sprintf("%.2f", *purchase.LotRate)
			}
			content.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f | %d | %s |\r\n",
				purchase.SecuCode, purchase.SecuName, purchase.IPOPrice, purchase.IPOPE,
				purchase.AllotMax, lotRate))
		}
	} else {
		content.WriteString("今日无新股申购\r\n")
	}

	logger.SugaredLogger.Debug("%s", content.String())
	return content.String(), nil
}

// mutualTypeName 将 MUTUAL_TYPE 代码翻译为中文名称
// 001=沪股通十大成交股, 002=港股通(沪)十大成交股, 003=深股通十大成交股, 004=港股通(深)十大成交股
func mutualTypeName(code string) string {
	switch code {
	case "001":
		return "沪股通十大成交股"
	case "002":
		return "港股通(沪)十大成交股"
	case "003":
		return "深股通十大成交股"
	case "004":
		return "港股通(深)十大成交股"
	default:
		return code
	}
}
