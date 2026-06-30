package flutter_api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"go-stock/backend/data"
)

// handleGetQgqpBid 处理获取 qgqp_b_id 的请求
func handleGetQgqpBid(w http.ResponseWriter, r *http.Request) {
	// 设置 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 使用项目已有的 chromedp 函数获取东财 Cookie
	cookieHeader, err := data.FetchEastMoneyCookiesViaChromedp("", 2*time.Minute, "https://xuangu.eastmoney.com/")
	if err != nil {
		WriteJSON(w, map[string]any{
			"code":    -1,
			"message": fmt.Sprintf("获取Cookie失败: %v", err),
		})
		return
	}

	// cookieHeader 格式: "qgqp_b_id=xxx; other_cookie=yyy; ..."
	// 我们需要解析出 qgqp_b_id
	cookies := strings.Split(cookieHeader, ";")
	for _, cookie := range cookies {
		cookie = strings.TrimSpace(cookie)
		if strings.HasPrefix(cookie, "qgqp_b_id=") {
			qgqpBId := strings.TrimPrefix(cookie, "qgqp_b_id=")
			fmt.Printf("找到 qgqp_b_id: %s\n", qgqpBId)
			WriteJSON(w, map[string]any{
				"code":    1,
				"message": "success",
				"data":    qgqpBId,
			})
			return
		}
	}

	WriteJSON(w, map[string]any{
		"code":    -1,
		"message": "未在Cookie中找到 qgqp_b_id",
	})
}

// GetQgqpBId 获取 East Money 的 qgqp_b_id（内部测试用）
func GetQgqpBId() (string, error) {
	cookieHeader, err := data.FetchEastMoneyCookiesViaChromedp("", 2*time.Minute, "https://xuangu.eastmoney.com/")
	if err != nil {
		return "", fmt.Errorf("获取Cookie失败: %v", err)
	}

	cookies := strings.Split(cookieHeader, ";")
	for _, cookie := range cookies {
		cookie = strings.TrimSpace(cookie)
		if strings.HasPrefix(cookie, "qgqp_b_id=") {
			qgqpBId := strings.TrimPrefix(cookie, "qgqp_b_id=")
			fmt.Printf("找到 qgqp_b_id: %s\n", qgqpBId)
			return qgqpBId, nil
		}
	}

	return "", fmt.Errorf("未在Cookie中找到 qgqp_b_id")
}
