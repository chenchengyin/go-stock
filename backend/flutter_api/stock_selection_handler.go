package flutter_api

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"net/http"
)

// ---------------------------------------------------------------------------
// 东财选股测试接口
// ---------------------------------------------------------------------------

// handleStockSelectionTest 处理东财选股测试请求
func handleStockSelectionTest(w http.ResponseWriter, r *http.Request) {
	// 设置 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 获取选股条件，默认为用户的完整条件
	query := r.URL.Query().Get("query")
	if query == "" {
		query = "竞价涨幅在0.01%到3%之间;前7个交易日单日最大涨幅大于9.8%;前一天成交金额大于5亿;流通市值在60亿到8000亿之间;主板"
	}

	logger.SugaredLogger.Infof("东财选股请求: %s", query)

	// 调用东财选股 API
	result := data.NewSearchStockApi(query).SearchStock(50)

	// 记录结果摘要
	if dataMap, ok := result["data"].(map[string]any); ok {
		if total, ok := dataMap["total"].(float64); ok {
			logger.SugaredLogger.Infof("东财选股结果: 共 %d 只股票", int(total))
		}
	}

	WriteJSON(w, result)
}

// TestHandleStockSelectionTest 测试选股功能（内部测试用）
func TestHandleStockSelectionTest() {
	query := "竞价涨幅在0.01%到3%之间;前7个交易日单日最大涨幅大于9.8%;前一天成交金额大于5亿;流通市值在60亿到8000亿之间;主板"
	result := data.NewSearchStockApi(query).SearchStock(50)

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("选股结果:\n%s\n", resultJSON)
}
