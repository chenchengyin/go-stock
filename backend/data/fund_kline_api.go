package data

import (
	"encoding/json"
	"fmt"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/go-resty/resty/v2"
	"go-stock/backend/logger"
	"time"
)

type FundKLineApi struct {
	client *resty.Client
	config *SettingConfig
}

func NewFundKLineApi() *FundKLineApi {
	return &FundKLineApi{
		client: SharedHTTPClient,
		config: GetSettingConfig(),
	}
}

func (api *FundKLineApi) GetFundKLine(fundCode, klt string, limit int) *KLineSourceResult {
	if IsOnExchangeFund(fundCode) {
		return api.getOnExchangeFundKLine(fundCode, klt, limit)
	}
	return api.getOffExchangeFundKLine(fundCode, klt, limit)
}

func (api *FundKLineApi) GetFundKLineWithFallback(fundCode, klt string, limit int) *KLineSourceResult {
	result := api.GetFundKLine(fundCode, klt, limit)
	if result != nil && result.Data != nil && len(*result.Data) > 0 {
		return result
	}

	if IsOnExchangeFund(fundCode) {
		logger.SugaredLogger.Warnf("场内基金K线所有数据源失败: code=%s klt=%s, 尝试净值K线", fundCode, klt)
		fallback := api.getOffExchangeFundKLine(fundCode, klt, limit)
		if fallback != nil && fallback.Data != nil && len(*fallback.Data) > 0 {
			fallback.Source = fallback.Source + "(净值合成)"
			return fallback
		}
	}

	if result == nil {
		return &KLineSourceResult{Data: &[]KLineData{}, Source: ""}
	}
	return result
}

func (api *FundKLineApi) getOnExchangeFundKLine(fundCode, klt string, limit int) *KLineSourceResult {
	result := api.fetchKLineFromEastMoney(fundCode, klt, limit)
	if result != nil && result.Data != nil && len(*result.Data) > 0 {
		result.Source = "eastmoney"
		return result
	}
	logger.SugaredLogger.Warnf("东方财富场内基金K线失败，尝试新浪: code=%s klt=%s", fundCode, klt)

	sinaResult := api.fetchKLineFromSina(fundCode, klt, limit)
	if sinaResult != nil && sinaResult.Data != nil && len(*sinaResult.Data) > 0 {
		sinaResult.Source = "sina"
		return sinaResult
	}
	logger.SugaredLogger.Warnf("新浪场内基金K线失败，尝试腾讯: code=%s klt=%s", fundCode, klt)

	tencentResult := api.fetchKLineFromTencent(fundCode, klt, limit)
	if tencentResult != nil && tencentResult.Data != nil && len(*tencentResult.Data) > 0 {
		tencentResult.Source = "tencent"
		return tencentResult
	}
	logger.SugaredLogger.Warnf("腾讯场内基金K线失败，尝试通达信: code=%s klt=%s", fundCode, klt)

	tdxResult := api.fetchKLineFromTdx(fundCode, klt, limit)
	if tdxResult != nil && tdxResult.Data != nil && len(*tdxResult.Data) > 0 {
		tdxResult.Source = "tdx"
		return tdxResult
	}
	logger.SugaredLogger.Warnf("通达信场内基金K线也失败: code=%s klt=%s", fundCode, klt)

	if result != nil {
		result.Source = "eastmoney"
		return result
	}
	return &KLineSourceResult{Data: &[]KLineData{}, Source: ""}
}

func (api *FundKLineApi) getOffExchangeFundKLine(fundCode, klt string, limit int) *KLineSourceResult {
	result := api.fetchOffExchangeKLineFromEastMoney(fundCode, klt, limit)
	if result != nil && result.Data != nil && len(*result.Data) > 0 {
		result.Source = "eastmoney_off"
		return result
	}
	logger.SugaredLogger.Warnf("东方财富场外基金K线失败，尝试天天基金: code=%s klt=%s", fundCode, klt)

	result2 := api.fetchOffExchangeKLineFromLSJZ(fundCode, klt, limit)
	if result2 != nil && result2.Data != nil && len(*result2.Data) > 0 {
		result2.Source = "eastmoney_lsjz"
		return result2
	}

	if result != nil {
		result.Source = "eastmoney_off"
		return result
	}
	return &KLineSourceResult{Data: &[]KLineData{}, Source: ""}
}

func (api *FundKLineApi) fetchKLineFromEastMoney(fundCode, klt string, limit int) *KLineSourceResult {
	secid := fundKLineSecid(fundCode)
	if secid == "" {
		return nil
	}
	eastMoneyApi := NewEastMoneyKLineApi(api.config)
	data := eastMoneyApi.GetKLineDataBefore(secid, klt, "", limit, "20500101")
	return &KLineSourceResult{Data: data}
}

func (api *FundKLineApi) fetchKLineFromSina(fundCode, klt string, limit int) *KLineSourceResult {
	var sinaCode string
	switch fundCode[0:1] {
	case "5", "6":
		sinaCode = "sh" + fundCode
	case "1", "2":
		sinaCode = "sz" + fundCode
	default:
		return nil
	}
	sinaApi := NewSinaKLineApi(api.config)
	data := sinaApi.GetKLineData(sinaCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func (api *FundKLineApi) fetchKLineFromTencent(fundCode, klt string, limit int) *KLineSourceResult {
	var qqCode string
	switch fundCode[0:1] {
	case "5", "6":
		qqCode = "sh" + fundCode
	case "1", "2":
		qqCode = "sz" + fundCode
	default:
		return nil
	}
	tencentApi := NewTencentKLineApi(api.config)
	data := tencentApi.GetKLineData(qqCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func (api *FundKLineApi) fetchKLineFromTdx(fundCode, klt string, limit int) *KLineSourceResult {
	var tdxCode string
	switch fundCode[0:1] {
	case "5", "6":
		tdxCode = "sh" + fundCode
	case "1", "2":
		tdxCode = "sz" + fundCode
	default:
		return nil
	}
	tdxApi := NewTdxKLineApi()
	data := tdxApi.GetKLineData(tdxCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func (api *FundKLineApi) fetchOffExchangeKLineFromEastMoney(fundCode, klt string, limit int) *KLineSourceResult {
	url := fmt.Sprintf("http://api.fund.eastmoney.com/f10/lsjz?fundCode=%s&pageIndex=1&pageSize=%d&startDate=&endDate=&_%d",
		fundCode, limit, time.Now().UnixMilli())
	resp, err := api.client.SetTimeout(time.Duration(api.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Referer", fmt.Sprintf("http://fundf10.eastmoney.com/jjjz_%s.html", fundCode)).
		Get(url)
	if err != nil || resp.StatusCode() != 200 {
		return nil
	}

	var result struct {
		Data struct {
			LSJZList []struct {
				FSRQ  string `json:"FSRQ"`
				DWJZ  string `json:"DWJZ"`
				JZZZL string `json:"JZZZL"`
				LJJZ  string `json:"LJJZ"`
			} `json:"LSJZList"`
			TotalCount int `json:"TotalCount"`
		} `json:"Data"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil
	}

	list := result.Data.LSJZList
	if len(list) == 0 {
		return nil
	}

	data := make([]KLineData, 0, len(list))
	for i := len(list) - 1; i >= 0; i-- {
		item := list[i]
		dwjz, _ := convertor.ToFloat(item.DWJZ)
		jzzzl, _ := convertor.ToFloat(item.JZZZL)
		data = append(data, KLineData{
			Day:           item.FSRQ,
			Open:          item.DWJZ,
			Close:         item.DWJZ,
			High:          item.DWJZ,
			Low:           item.DWJZ,
			Volume:        "0",
			ChangePercent: fmt.Sprintf("%.2f", jzzzl),
			ChangeValue:   "0",
		})
		if i > 0 {
			prevDwjz, _ := convertor.ToFloat(list[i-1].DWJZ)
			if prevDwjz > 0 {
				changeVal := dwjz - prevDwjz
				data[len(data)-1].ChangeValue = fmt.Sprintf("%.4f", changeVal)
			}
		}
	}

	return &KLineSourceResult{Data: &data}
}

func (api *FundKLineApi) fetchOffExchangeKLineFromLSJZ(fundCode, klt string, limit int) *KLineSourceResult {
	var endDate string
	var startDate string
	now := time.Now()
	switch klt {
	case "101":
		startDate = now.AddDate(-1, 0, 0).Format("2006-01-02")
	case "102":
		startDate = now.AddDate(0, -6, 0).Format("2006-01-02")
	case "103":
		startDate = now.AddDate(-3, 0, 0).Format("2006-01-02")
	case "104":
		startDate = now.AddDate(-5, 0, 0).Format("2006-01-02")
	case "105":
		startDate = now.AddDate(-10, 0, 0).Format("2006-01-02")
	default:
		startDate = now.AddDate(-1, 0, 0).Format("2006-01-02")
	}
	endDate = now.Format("2006-01-02")

	url := fmt.Sprintf("http://api.fund.eastmoney.com/f10/lsjz?fundCode=%s&pageIndex=1&pageSize=%d&startDate=%s&endDate=%s&_%d",
		fundCode, limit, startDate, endDate, time.Now().UnixMilli())
	resp, err := api.client.SetTimeout(time.Duration(api.config.CrawlTimeOut)*time.Second).R().
		SetHeader("User-Agent", getRandomUA()).
		SetHeader("Referer", fmt.Sprintf("http://fundf10.eastmoney.com/jjjz_%s.html", fundCode)).
		Get(url)
	if err != nil || resp.StatusCode() != 200 {
		return nil
	}

	var result struct {
		Data struct {
			LSJZList []struct {
				FSRQ  string `json:"FSRQ"`
				DWJZ  string `json:"DWJZ"`
				JZZZL string `json:"JZZZL"`
				LJJZ  string `json:"LJJZ"`
			} `json:"LSJZList"`
			TotalCount int `json:"TotalCount"`
		} `json:"Data"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil
	}

	list := result.Data.LSJZList
	if len(list) == 0 {
		return nil
	}

	data := make([]KLineData, 0, len(list))
	for i := len(list) - 1; i >= 0; i-- {
		item := list[i]
		dwjz, _ := convertor.ToFloat(item.DWJZ)
		jzzzl, _ := convertor.ToFloat(item.JZZZL)
		klineItem := KLineData{
			Day:           item.FSRQ,
			Open:          item.DWJZ,
			Close:         item.DWJZ,
			High:          item.DWJZ,
			Low:           item.DWJZ,
			Volume:        "0",
			ChangePercent: fmt.Sprintf("%.2f", jzzzl),
			ChangeValue:   "0",
		}
		if i > 0 {
			prevDwjz, _ := convertor.ToFloat(list[i-1].DWJZ)
			if prevDwjz > 0 {
				changeVal := dwjz - prevDwjz
				klineItem.ChangeValue = fmt.Sprintf("%.4f", changeVal)
			}
		}
		data = append(data, klineItem)
	}

	return &KLineSourceResult{Data: &data}
}
