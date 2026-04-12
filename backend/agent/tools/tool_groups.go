package tools

import (
	"strings"

	"github.com/cloudwego/eino/components/tool"
)

type ToolGroup string

const (
	GroupBase          ToolGroup = "base"
	GroupStockAnalysis ToolGroup = "stock_analysis"
	GroupMarket        ToolGroup = "market"
	GroupScreening     ToolGroup = "screening"
	GroupMoneyFlow     ToolGroup = "money_flow"
	GroupNewsResearch  ToolGroup = "news_research"
	GroupAIAnalysis    ToolGroup = "ai_analysis"
	GroupOperations    ToolGroup = "operations"
)

var toolGroupMap = map[string]ToolGroup{
	"QueryStockCodeInfo": GroupBase,
	"QueryBKDictInfo":    GroupBase,
	"GetCurrentTime":     GroupBase,
	"GetFollowedStocks":  GroupBase,
	"GetHolidayInfo":     GroupBase,
	"GetHolidayYear":     GroupBase,
	"GetHolidayBatch":    GroupBase,
	"IsTradingDay":       GroupBase,
	"GetNextTradingDay":  GroupBase,

	"GetStockInfo":            GroupStockAnalysis,
	"GetStockKLine":           GroupStockAnalysis,
	"GetEastMoneyKLine":       GroupStockAnalysis,
	"GetEastMoneyKLineWithMA": GroupStockAnalysis,
	"GetStockMinuteData":      GroupStockAnalysis,
	"GetStockFinancialInfo":   GroupStockAnalysis,
	"GetStockHolderNum":       GroupStockAnalysis,
	"GetStockRZRQInfo":        GroupStockAnalysis,
	"GetStockConceptInfo":     GroupStockAnalysis,

	"GetMarketData":              GroupMarket,
	"GlobalStockIndexesReadable": GroupMarket,
	"GetStockChanges":            GroupMarket,
	"GetStockChangeHistoryList":  GroupMarket,

	"FilterStocks":            GroupScreening,
	"SearchStockByIndicators": GroupScreening,
	"SearchBk":                GroupScreening,
	"SearchETF":               GroupScreening,
	"HotStrategyTable":        GroupScreening,
	"HotStockTable":           GroupScreening,

	"GetStockMoneyData":        GroupMoneyFlow,
	"GetMutualTop10Deal":       GroupMoneyFlow,
	"GetStockHistoryMoneyData": GroupMoneyFlow,
	"GetIndustryMoneyRank":     GroupMoneyFlow,

	"QueryStockNewsTool":          GroupNewsResearch,
	"GetNewsListData":             GroupNewsResearch,
	"GetStockResearchReport":      GroupNewsResearch,
	"GetIndustryResearchReport":   GroupNewsResearch,
	"GetSecuritiesCompanyOpinion": GroupNewsResearch,
	"StockNotice":                 GroupNewsResearch,
	"GetStockNotice":              GroupNewsResearch,
	"InteractiveAnswer":           GroupNewsResearch,
	"GetInvestCalendar":           GroupNewsResearch,
	"GetLongTigerList":            GroupNewsResearch,
	"GetHotStockList":             GroupNewsResearch,
	"GetHotEventList":             GroupNewsResearch,

	"AiRecommendStocks":            GroupAIAnalysis,
	"GetAIAnalysisHistory":         GroupAIAnalysis,
	"GetAIAnalysisDetail":          GroupAIAnalysis,
	"GetAIAnalysisContent":         GroupAIAnalysis,
	"CreateAiRecommendStocks":      GroupAIAnalysis,
	"BatchCreateAiRecommendStocks": GroupAIAnalysis,

	"SetTradingPrice":     GroupOperations,
	"SendDingDingMessage": GroupOperations,
	"SendToDingDing":      GroupOperations,
	"SearchFund":          GroupOperations,
	"GetFundInfo":         GroupOperations,
	"GetEconomicData":     GroupOperations,
}

type groupKeywords struct {
	group    ToolGroup
	keywords []string
}

var groupKeywordsList = []groupKeywords{
	{GroupStockAnalysis, []string{
		"股票", "股价", "行情", "K线", "k线", "日K", "周K", "月K", "分钟线",
		"分时", "实时", "价格", "涨跌", "成交量", "成交额",
		"财务", "报表", "营收", "利润", "ROE", "PE", "PB", "EPS",
		"股东", "持股", "融资融券", "融券", "融资", "杠杆",
		"概念", "板块归属", "所属概念",
		"分析", "诊断", "评估", "估值",
		"技术面", "基本面", "MACD", "KDJ", "RSI", "布林", "BOLL",
		"均线", "MA5", "MA10", "MA20", "MA60", "MA120",
		"前复权", "后复权", "复权",
	}},
	{GroupMarket, []string{
		"大盘", "市场", "指数", "行情", "涨跌分布", "涨停", "跌停",
		"上涨家数", "下跌家数", "申购", "新股",
		"异动", "火箭发射", "快速反弹", "大笔买入", "封涨停",
		"加速下跌", "高台跳水", "大笔卖出", "封跌停",
		"全球指数", "道琼斯", "纳斯达克", "标普", "恒生", "日经",
	}},
	{GroupScreening, []string{
		"筛选", "选股", "过滤", "条件选股", "指标选股",
		"形态选股", "MACD金叉", "KDJ金叉", "放量突破",
		"连涨", "连跌", "多头排列", "空头排列",
		"板块", "概念", "ETF",
		"热门策略", "热门股票", "策略",
	}},
	{GroupMoneyFlow, []string{
		"资金", "流入", "流出", "净流入", "净流出",
		"北向", "南向", "沪股通", "深股通", "港股通",
		"主力", "机构", "外资",
		"行业资金", "板块资金",
	}},
	{GroupNewsResearch, []string{
		"新闻", "资讯", "消息", "公告", "研报", "研究报告",
		"券商", "机构观点", "分析师", "评级",
		"互动", "问答", "投资者互动",
		"日历", "财报日", "股东大会", "IPO",
		"龙虎榜", "营业部",
		"热门话题", "热点事件", "雪球",
	}},
	{GroupAIAnalysis, []string{
		"AI分析", "AI推荐", "历史分析", "分析报告",
		"推荐股票", "买入评级", "增持", "减持",
		"止盈", "止损", "买入价", "目标价",
	}},
	{GroupOperations, []string{
		"预警", "价位", "开仓", "止盈价", "止损价", "成本价",
		"钉钉", "通知", "推送", "发送消息",
		"基金", "基金代码", "基金名称", "净值",
		"GDP", "CPI", "PPI", "PMI", "宏观经济",
	}},
}

func ClassifyQuestion(question string) map[ToolGroup]bool {
	matched := map[ToolGroup]bool{
		GroupBase: true,
	}

	lowerQ := strings.ToLower(question)

	for _, gk := range groupKeywordsList {
		for _, kw := range gk.keywords {
			if strings.Contains(lowerQ, strings.ToLower(kw)) {
				matched[gk.group] = true
				break
			}
		}
	}

	if len(matched) <= 1 {
		for _, g := range []ToolGroup{
			GroupStockAnalysis, GroupMarket, GroupNewsResearch,
		} {
			matched[g] = true
		}
	}

	return matched
}

func FilterToolsByGroups(allTools []tool.BaseTool, groups map[ToolGroup]bool) []tool.BaseTool {
	var filtered []tool.BaseTool
	for _, t := range allTools {
		info, err := t.Info(nil)
		if err != nil {
			filtered = append(filtered, t)
			continue
		}
		group, exists := toolGroupMap[info.Name]
		if !exists || groups[group] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
