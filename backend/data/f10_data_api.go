package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"strconv"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
)

const emF10BaseURL = "https://datacenter.eastmoney.com/securities/api/data/v1/get"

func (receiver StockDataApi) f10Request(url string, result any) error {
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter.eastmoney.com").
		SetHeader("Referer", "https://emweb.securities.eastmoney.com/").
		SetHeader("Origin", "https://emweb.securities.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0").
		Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode())
	}
	if err := json.Unmarshal(resp.Body(), result); err != nil {
		return fmt.Errorf("parse failed: %v", err)
	}
	return nil
}

func normalizeF10Code(stockCode string) string {
	if strutil.ContainsAny(stockCode, []string{"."}) {
		return stockCode
	}
	converted := ConvertStockCodeToTushareCode(stockCode)
	if strutil.ContainsAny(converted, []string{"."}) {
		return converted
	}
	code := RemoveAllNonDigitChar(stockCode)
	if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "9") {
		return code + ".SH"
	}
	if strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") {
		return code + ".SZ"
	}
	if strings.HasPrefix(code, "4") || strings.HasPrefix(code, "8") {
		return code + ".BJ"
	}
	if strings.HasPrefix(code, "5") {
		return code + ".SH"
	}
	return code + ".SZ"
}

type F10GenericResp struct {
	Version string     `json:"version"`
	Result  *F10Result `json:"result"`
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Code    int        `json:"code"`
}

type F10Result struct {
	Count int              `json:"count"`
	Data  []map[string]any `json:"data"`
}

func (receiver StockDataApi) GetStockLatestFinance(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_PCF10_FINANCEMAINFINADATA&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CREPORT_DATE%2CREPORT_TYPE%2CEPSJB%2CEPSKCJB%2CEPSXS%2CBPS%2CMGZBGJ%2CMGWFPLR%2CMGJYXJJE%2CTOTAL_OPERATEINCOME%2CTOTAL_OPERATEINCOME_LAST%2CPARENT_NETPROFIT%2CPARENT_NETPROFIT_LAST%2CKCFJCXSYJLR%2CKCFJCXSYJLR_LAST%2CROEJQ%2CROEJQ_LAST%2CXSMLL%2CXSMLL_LAST%2CZCFZL%2CZCFZL_LAST%2CYYZSRGDHBZC_LAST%2CYYZSRGDHBZC%2CNETPROFITRPHBZC%2CNETPROFITRPHBZC_LAST%2CKFJLRGDHBZC%2CKFJLRGDHBZC_LAST%2CTOTALOPERATEREVETZ%2CTOTALOPERATEREVETZ_LAST%2CPARENTNETPROFITTZ%2CPARENTNETPROFITTZ_LAST%2CKCFJCXSYJLRTZ%2CKCFJCXSYJLRTZ_LAST%2CTOTAL_SHARE%2CFREE_SHARE%2CEPSJB_PL%2CBPS_PL%2CFORMERNAME&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&sortTypes=-1&sortColumns=REPORT_DATE&pageNumber=1&pageSize=1&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockQtrMainFinance(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_F10_QTR_MAINFINADATA&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CORG_CODE%2CREPORT_DATE%2CEPSJB%2CBPS%2CPER_CAPITAL_RESERVE%2CPER_UNASSIGN_PROFIT%2CPER_NETCASH%2CTOTALOPERATEREVE%2CGROSS_PROFIT%2CPARENTNETPROFIT%2CDEDU_PARENT_PROFIT%2CTOTALOPERATEREVETZ%2CPARENTNETPROFITTZ%2CDPNP_YOY_RATIO%2CYYZSRGDHBZC%2CNETPROFITRPHBZC%2CKFJLRGDHBZC%2CROE_DILUTED%2CJROA%2CNET_PROFIT_RATIO%2CGROSS_PROFIT_RATIO&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=9&sortTypes=-1&sortColumns=REPORT_DATE&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockOrgPredict(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_HSF10_RES_ORGPREDICT&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CPUBLISH_DATE%2CORG_CODE%2CORG_NAME_ABBR%2CYEAR1%2CYEAR_MARK1%2CEPS1%2CPE1%2CYEAR2%2CYEAR_MARK2%2CEPS2%2CPE2%2CYEAR3%2CYEAR_MARK3%2CEPS3%2CPE3%2CYEAR4%2CYEAR_MARK4%2CEPS4%2CPE4&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=200&sortTypes=&sortColumns=&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockPredictSummary(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_HSF10_RESPREDICT_STATISTICS&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CYEAR%2CYEAR_MARK%2CEPS%2CEPS_RATIO%2CPE&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=200&sortTypes=1&sortColumns=RANK&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockValuationPercentile(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_STOCKVALUATIONTANTILE&columns=SECUCODE%2CSTATISTICS_CYCLE%2CINDEX_TYPE%2CPERCENTILE_THIRTY%2CPERCENTILE_FIFTY%2CPERCENTILE_SEVENTY&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)(INDEX_TYPE%3D%221%22)(STATISTICS_CYCLE%3D%223%22)&pageNumber=1&pageSize=&sortTypes=&sortColumns=&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockMarginTrading(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_MARGIN_STATISTICS_STOCKS&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CTRADE_DATE%2CFIN_BUY_AMT%2CFIN_REPAY_AMT%2CFIN_BALANCE%2CLOAN_SELL_VOL%2CLOAN_REPAY_VOL%2CLOAN_BALANCE&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=10&sortTypes=-1&sortColumns=TRADE_DATE&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockBlockTrade(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_DATA_BLOCKTRADE&columns=SECUCODE%2CSECURITY_INNER_CODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CSECURITY_TYPE%2CSECURITY_TYPE_WEB%2CTRADE_DATE%2CDEAL_PRICE%2CPREMIUM_RATIO%2CDEAL_VOLUME%2CDEAL_AMT%2CBUYER_NAME%2CSELLER_NAME%2CDAILY_RANK%2CCLOSE_PRICE%2CTRADE_UNIT%2CTURNOVER_RATE%2CCHANGE_RATE%2CCHANGE_RATE_1DAYS%2CCHANGE_RATE_5DAYS%2CBUYER_CODE%2CSELLER_CODE%2CPREMIUM_TURNOVER%2CDISCOUNT_TURNOVER%2CUNLIMITED_A_SHARES%2CTRADE_MARKET_OLD&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=10&sortTypes=-1&sortColumns=TRADE_DATE&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockHolderTrend(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_CUSTOM_DMSK_TREND&columns=ALL&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)(INDICATORTYPE%3D1)(DATETYPE%3D3)&pageNumber=1&pageSize=&sortTypes=1&sortColumns=TRADE_DATE&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockBillboard(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_BILLBOARD_DAILYDETAILS&columns=SECURITY_CODE%2CSECUCODE%2CTRADE_DATE%2CEXPLANATION%2CTOTAL_BUY%2CTOTAL_SELL%2CTOTAL_BUYRIOTOP%2CTOTAL_SELLRIOTOP&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=5&sortTypes=-1%2C-1&sortColumns=TRADE_DATE%2CEXPLANATION&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockOperationDeptTrade(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_OPERATEDEPT_TRADE&columns=TRADE_DATE%2CEXPLANATION%2COPERATEDEPT_NAME%2CBUY_AMT_REAL%2CBUY_RATIO%2CSELL_AMT_REAL%2CSELL_RATIO&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)(TRADE_DIRECTION%3D%220%22)&pageNumber=1&pageSize=15&sortTypes=-1%2C-1%2C1&sortColumns=TRADE_DATE%2CEXPLANATION%2CRANK&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func f10GenericToMarkdown(title string, resp *F10GenericResp) string {
	if resp == nil || resp.Result == nil || len(resp.Result.Data) == 0 {
		return fmt.Sprintf("## %s\n\n暂无数据", title)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s\n\n", title))

	data := resp.Result.Data
	if len(data) == 0 {
		sb.WriteString("暂无数据\n")
		return sb.String()
	}

	cols := make([]string, 0)
	colSet := make(map[string]bool)
	for _, row := range data {
		for k := range row {
			if !colSet[k] {
				colSet[k] = true
				cols = append(cols, k)
			}
		}
	}

	sb.WriteString("| ")
	for _, c := range cols {
		sb.WriteString(c + " | ")
	}
	sb.WriteString("\n| ")
	for range cols {
		sb.WriteString("--- | ")
	}
	sb.WriteString("\n")

	for _, row := range data {
		sb.WriteString("| ")
		for _, c := range cols {
			v := row[c]
			if v == nil {
				sb.WriteString("- | ")
			} else {
				switch val := v.(type) {
				case float64:
					if val == float64(int64(val)) {
						sb.WriteString(strconv.FormatInt(int64(val), 10) + " | ")
					} else {
						sb.WriteString(fmt.Sprintf("%.4f", val) + " | ")
					}
				case string:
					sb.WriteString(val + " | ")
				default:
					sb.WriteString(fmt.Sprintf("%v", v) + " | ")
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (receiver StockDataApi) GetStockLatestFinanceToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockLatestFinance(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取最新财务数据失败: %v", err)
		return fmt.Sprintf("获取最新财务数据失败: %v", err)
	}
	name := ""
	if len(resp.Result.Data) > 0 {
		if n, ok := resp.Result.Data[0]["SECURITY_NAME_ABBR"].(string); ok {
			name = n
		}
	}
	return f10GenericToMarkdown(name+" 最新财务主要数据", resp)
}

func (receiver StockDataApi) GetStockQtrMainFinanceToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockQtrMainFinance(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取季度财务指标失败: %v", err)
		return fmt.Sprintf("获取季度财务指标失败: %v", err)
	}
	return f10GenericToMarkdown("季度主要财务指标", resp)
}

func (receiver StockDataApi) GetStockOrgPredictToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockOrgPredict(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取机构预测失败: %v", err)
		return fmt.Sprintf("获取机构预测失败: %v", err)
	}
	return f10GenericToMarkdown("机构预测明细", resp)
}

func (receiver StockDataApi) GetStockPredictSummaryToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockPredictSummary(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取机构预测汇总失败: %v", err)
		return fmt.Sprintf("获取机构预测汇总失败: %v", err)
	}
	return f10GenericToMarkdown("机构预测汇总", resp)
}

func (receiver StockDataApi) GetStockValuationPercentileToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockValuationPercentile(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取估值百分位失败: %v", err)
		return fmt.Sprintf("获取估值百分位失败: %v", err)
	}
	return f10GenericToMarkdown("估值百分位", resp)
}

func (receiver StockDataApi) GetStockMarginTradingToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockMarginTrading(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取融资融券数据失败: %v", err)
		return fmt.Sprintf("获取融资融券数据失败: %v", err)
	}
	return f10GenericToMarkdown("融资融券数据", resp)
}

func (receiver StockDataApi) GetStockBlockTradeToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockBlockTrade(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取大宗交易数据失败: %v", err)
		return fmt.Sprintf("获取大宗交易数据失败: %v", err)
	}
	return f10GenericToMarkdown("大宗交易数据", resp)
}

func (receiver StockDataApi) GetStockHolderTrendToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockHolderTrend(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取户均持股趋势失败: %v", err)
		return fmt.Sprintf("获取户均持股趋势失败: %v", err)
	}
	return f10GenericToMarkdown("户均持股趋势", resp)
}

func (receiver StockDataApi) GetStockBillboardToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockBillboard(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取龙虎榜数据失败: %v", err)
		return fmt.Sprintf("获取龙虎榜数据失败: %v", err)
	}
	return f10GenericToMarkdown("龙虎榜数据", resp)
}

func (receiver StockDataApi) GetStockOperationDeptTradeToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockOperationDeptTrade(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取营业部买卖明细失败: %v", err)
		return fmt.Sprintf("获取营业部买卖明细失败: %v", err)
	}
	return f10GenericToMarkdown("营业部买卖明细", resp)
}
