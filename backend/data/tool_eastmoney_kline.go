package data

import (
	"encoding/json"
	"go-stock/backend/logger"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("GetEastMoneyKLine", handleGetEastMoneyKLine)
}

// normalizeKLineType 将前端/自然语言 K 线类型转为东方财富 API 参数
func normalizeKLineType(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "day", "日", "101", "日k", "日k线":
		return "101"
	case "week", "周", "102", "周k", "周k线":
		return "102"
	case "month", "月", "103", "月k", "月k线":
		return "103"
	case "quarter", "季", "104", "季k", "季k线":
		return "104"
	case "year", "年", "105", "年k", "年k线":
		return "105"
	case "1", "1min", "1分钟":
		return "1"
	case "5", "5min", "5分钟":
		return "5"
	case "15", "15min", "15分钟":
		return "15"
	case "30", "30min", "30分钟":
		return "30"
	case "60", "60min", "60分钟":
		return "60"
	default:
		return s
	}
}

func handleGetEastMoneyKLine(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	stockCode := gjson.Get(funcArguments, "stockCode").String()
	kLineType := gjson.Get(funcArguments, "kLineType").String()
	adjustFlag := gjson.Get(funcArguments, "adjustFlag").String()
	limit := gjson.Get(funcArguments, "limit").Int()
	if limit <= 0 {
		limit = 60
	}

	ctx.Ch <- map[string]any{
		"code":     1,
		"question": ctx.Question,
		"chatId":   ctx.StreamResponseID,
		"model":    ctx.Model,
		"content":  "\r\n```\r\n开始调用工具：GetEastMoneyKLine，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":     time.Now().Format(time.DateTime),
	}

	api := NewEastMoneyKLineApi(GetSettingConfig())
	if !api.ValidateStockCode(stockCode) {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "股票代码无效，请使用正确格式（如 000001.SZ、600000.SH、00700.HK）。")
		return nil
	}

	kType := normalizeKLineType(kLineType)
	var list *[]KLineData
	if adjustFlag != "" && (kType == "101" || kType == "day") {
		adj := strings.TrimSpace(strings.ToLower(adjustFlag))
		if adj != "qfq" && adj != "hfq" {
			adj = "qfq"
		}
		list = api.GetAdjustedKLine(stockCode, adj, int(limit))
	} else {
		list = api.GetKLineData(stockCode, kType, strings.TrimSpace(adjustFlag), int(limit))
	}

	if list == nil || len(*list) == 0 {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
			ctx.CurrentCallID, ctx.FuncName, funcArguments, "未获取到 K 线数据，请检查股票代码与类型。")
		return nil
	}

	rows := make([]map[string]any, 0, len(*list))
	for _, k := range *list {
		vol, _ := convertor.ToFloat(k.Volume)
		rows = append(rows, map[string]any{
			"日期":      k.Day,
			"开盘价":     k.Open,
			"收盘价":     k.Close,
			"最高价":     k.High,
			"最低价":     k.Low,
			"成交量(万手)": vol / 10000 / 100,
			"涨跌幅(%)":  k.ChangePercent,
			"涨跌额":     k.ChangeValue,
			"振幅(%)":   k.Amplitude,
			"换手率(%)":  k.TurnoverRate,
		})
	}
	jsonData, _ := json.Marshal(rows)
	markdownTable, err := JSONToMarkdownTable(jsonData)
	if err != nil {
		markdownTable = string(jsonData)
	}
	typeLabel := kLineType
	if typeLabel == "" {
		typeLabel = kType
	}
	res := "\r\n### " + stockCode + " " + typeLabel + " K线（共 " + convertor.ToString(len(*list)) + " 条）\r\n" + markdownTable + "\r\n"
	logger.SugaredLogger.Infof("GetEastMoneyKLine: %s %s -> %d 条", stockCode, kType, len(*list))
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, res)
	return nil
}
