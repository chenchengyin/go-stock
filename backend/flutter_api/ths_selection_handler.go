package flutter_api

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// 同花顺问财免费选股 API（方案C）- 使用 Cookie 认证
// ---------------------------------------------------------------------------

// THSFreeAPI 同花顺免费选股API（方案C）
type THSFreeAPI struct {
	Cookie string
}

type THSFreeResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    []THSStockItem `json:"data"`
	Total   int             `json:"total"`
}

type THSStockItem struct {
	Code     string `json:"code"`      // 股票代码
	Name     string `json:"name"`      // 股票名称
	Price    string `json:"price"`     // 最新价
	Change   string `json:"change"`    // 涨跌额
	ChangePct string `json:"change_pct"` // 涨跌幅%
	Volume   string `json:"volume"`    // 成交量
	Amount   string `json:"amount"`    // 成交额
	Market   string `json:"market"`    // 市场
}

// NewTHSFreeAPI 创建同花顺免费选股API实例
func NewTHSFreeAPI(cookie string) *THSFreeAPI {
	return &THSFreeAPI{Cookie: cookie}
}

// QueryStockSelection 使用自然语言条件查询股票
// query: 自然语言选股条件，如 "竞价涨幅在0.01%到3%之间" 或组合条件
func (api *THSFreeAPI) QueryStockSelection(query string) (*THSFreeResponse, error) {
	if query == "" {
		return nil, fmt.Errorf("选股条件不能为空")
	}

	// 同花顺问财 web API 端点（使用 pywencai 类似的接口）
	// 这个接口使用 Cookie 认证，不需要 API key
	url := "https://www.iwencai.com/stockpick/search"

	// 构造请求
	reqBody := map[string]any{
		"query":      query,
		"typed":      1,
		"header":     "港股",
		"catalogId":  "",
		"searchType": "stock",
		"extflag":    "",
		"isRead":     1,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://www.iwencai.com/")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	if api.Cookie != "" {
		req.Header.Set("Cookie", api.Cookie)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP错误: %d", resp.StatusCode)
	}

	var result THSFreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return &result, nil
}

// QueryStockSelectionSimple 简单的选股查询，返回原始响应
func (api *THSFreeAPI) QueryStockSelectionSimple(query string) map[string]any {
	resp, err := api.QueryStockSelection(query)
	if err != nil {
		return map[string]any{
			"success": false,
			"message": err.Error(),
			"data":    []THSStockItem{},
			"total":   0,
		}
	}
	return map[string]any{
		"success": resp.Success,
		"message": resp.Message,
		"data":    resp.Data,
		"total":   resp.Total,
	}
}

// handleTHSSelectionTest 处理同花顺免费选股测试请求
func handleTHSSelectionTest(w http.ResponseWriter, r *http.Request) {
	// 设置 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 获取选股条件
	query := r.URL.Query().Get("query")
	if query == "" {
		// 使用默认条件
		query = "竞价涨幅在0.01%到3%之间;前7天出现过涨停;前一天成交金额大于5亿;流通市值在60亿到8000亿之间;主板"
	}

	logger.SugaredLogger.Infof("同花顺免费选股请求: %s", query)

	// 从设置中获取 Cookie（如果没有传入）
	cookie := r.URL.Query().Get("cookie")
	if cookie == "" {
		// 使用项目中的同花顺 Cookie（如果有的话）
		cookie = getTHSCookie()
	}

	api := NewTHSFreeAPI(cookie)
	result := api.QueryStockSelectionSimple(query)

	WriteJSON(w, result)
}

// getTHSCookie 从设置中获取同花顺 Cookie
func getTHSCookie() string {
	// TODO: 从设置中获取同花顺 Cookie
	// 目前项目中的同花顺设置可能没有存储 Cookie
	return ""
}
