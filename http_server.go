package main

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// HTTP Server — 为 Flutter 前端提供 REST API + WebSocket 实时推送
// 完全独立于 Wails，不影响原有 Vue 前端。
// ---------------------------------------------------------------------------

var (
	wsClients   = make(map[*websocket.Conn]bool)
	wsClientsMu sync.RWMutex
	upgrader    = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

// HTTP 服务端口，默认 8080
const defaultHTTPServerPort = 8080

// StartHTTPServer 启动 REST API + WebSocket 服务（非阻塞）
func StartHTTPServer() {
	port := defaultHTTPServerPort

	mux := http.NewServeMux()

	// ---- REST API ----
	mux.HandleFunc("/api/news", handleGetNews)                 // GET /api/news?source=财联社电报&limit=50
	mux.HandleFunc("/api/kline", handleGetKLine)               // GET /api/kline?code=600519.SH&klt=101&limit=100
	mux.HandleFunc("/api/global-indexes", handleGlobalIndexes) // GET /api/global-indexes
	mux.HandleFunc("/api/industry-ranks", handleIndustryRanks) // GET /api/industry-ranks?sort=0&limit=150
	mux.HandleFunc("/api/hot-topics", handleHotTopics)         // GET /api/hot-topics?limit=10
	mux.HandleFunc("/api/strategy", handleStrategy)            // GET/POST /api/strategy（策略吧）
	mux.HandleFunc("/api/health", handleHealth)                // GET /api/health

	// ---- WebSocket ----
	mux.HandleFunc("/ws", handleWebSocket)

	addr := fmt.Sprintf(":%d", port)
	logger.SugaredLogger.Infof("HTTP Server (for Flutter) starting on %s", addr)

	// 使用 CORS 中间件包装
	handler := corsMiddleware(mux)

	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.SugaredLogger.Errorf("HTTP Server error: %v", err)
	}
}

// corsMiddleware 添加 CORS 头，允许 Flutter web 跨域访问
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// 预检请求直接返回 200
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// REST API Handlers
// ---------------------------------------------------------------------------

// handleGetNews 获取新闻列表
// GET /api/news?source=财联社电报&limit=50
func handleGetNews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	source := r.URL.Query().Get("source")
	limitStr := r.URL.Query().Get("limit")

	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	news := data.NewMarketNewsApi().GetNewsList(source, limit)
	writeJSON(w, news)
}

// handleGetKLine 获取 K 线数据（多数据源自动 fallback）
// GET /api/kline?code=600519.SH&klt=101&limit=100
// klt: 101=日K, 102=周K, 103=月K, 5=5分钟
func handleGetKLine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	code := r.URL.Query().Get("code")
	klt := r.URL.Query().Get("klt")
	limitStr := r.URL.Query().Get("limit")

	if code == "" {
		writeJSON(w, map[string]string{"error": "code is required"})
		return
	}
	if klt == "" {
		klt = "101" // 默认日K
	}
	limit := 100
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	result := data.FetchKLineWithFallback(code, "", klt, limit, "")
	writeJSON(w, result)
}

// handleGlobalIndexes 获取全球指数
// GET /api/global-indexes
func handleGlobalIndexes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	indexes := data.NewMarketNewsApi().GlobalStockIndexes(30)
	writeJSON(w, indexes)
}

// handleIndustryRanks 获取行业排名（含市值）
// GET /api/industry-ranks?sort=0&limit=150
func handleIndustryRanks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "0"
	}
	limitStr := r.URL.Query().Get("limit")
	limit := 150
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	// 1. 获取行业排名（Tencent）
	rankData := data.NewMarketNewsApi().GetIndustryRank(sort, limit)

	// 2. 获取行业市值（东方财富）
	valuationResp := data.NewStockDataApi().GetAllIndustryValuation()

	// 3. 构建市值映射 map[行业名称]市值(元)
	valMap := make(map[string]float64)
	if valuationResp != nil && valuationResp.Result.Data != nil {
		for _, v := range valuationResp.Result.Data {
			valMap[v.BOARDNAME] = v.MARKETCAPVAG
		}
	}

	// 4. 合并数据：给排名数据中的每个行业补充市值
	if rankItems, ok := rankData["data"].([]any); ok {
		for _, item := range rankItems {
			if m, ok := item.(map[string]any); ok {
				name := ""
				if n, ok := m["bd_name"].(string); ok {
					name = n
				}
				// 尝试按行业名称匹配市值
				if capVal, found := valMap[name]; found {
					m["market_cap"] = capVal
				} else {
					// 模糊匹配：去掉可能的后缀
					for k, v := range valMap {
						if strings.Contains(k, name) || strings.Contains(name, k) {
							m["market_cap"] = v
							break
						}
					}
				}
			}
		}
	}

	writeJSON(w, rankData)
}

// handleHotTopics 获取热门话题
// GET /api/hot-topics?limit=10
func handleHotTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	topics := data.NewMarketNewsApi().HotTopic(limit)
	writeJSON(w, topics)
}

// handleHealth 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{
		"status":  "ok",
		"time":    time.Now().Format(time.RFC3339),
		"service": "go-stock trading API",
	})
}

// ---------------------------------------------------------------------------
// WebSocket — 实时推送（替代 Wails EventsEmit）
// ---------------------------------------------------------------------------

// handleWebSocket 处理 WebSocket 连接
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.SugaredLogger.Errorf("WebSocket upgrade error: %v", err)
		return
	}

	wsClientsMu.Lock()
	wsClients[conn] = true
	wsClientsMu.Unlock()

	logger.SugaredLogger.Infof("WebSocket client connected (total: %d)", len(wsClients))

	// 保持连接：定期发送心跳，断开时清理
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		defer func() {
			wsClientsMu.Lock()
			delete(wsClients, conn)
			wsClientsMu.Unlock()
			conn.Close()
			logger.SugaredLogger.Infof("WebSocket client disconnected (total: %d)", len(wsClients))
		}()

		for {
			select {
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			default:
				// 读取客户端消息（处理关闭帧）
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}
	}()
}

// Broadcast 向所有 WebSocket 客户端广播消息
// 可在任意位置调用，例如 app.go 中的定时任务：
//
//	go Broadcast("newTelegraph", news)
func Broadcast(event string, data interface{}) {
	msg := map[string]interface{}{
		"event": event,
		"data":  data,
		"time":  time.Now().UnixMilli(),
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		logger.SugaredLogger.Errorf("Broadcast marshal error: %v", err)
		return
	}

	wsClientsMu.RLock()
	defer wsClientsMu.RUnlock()

	for conn := range wsClients {
		if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
			logger.SugaredLogger.Warnf("Broadcast write error: %v", err)
			conn.Close()
			delete(wsClients, conn) // 直接删除，后续 ReadMessage 会处理清理
		}
	}
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.SugaredLogger.Errorf("writeJSON error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 策略吧 API Handler
// ---------------------------------------------------------------------------

// handleStrategy 策略吧 API 路由分发
// GET /api/strategy?action=list&page=1&pageSize=20
// GET /api/strategy?action=detail&postId=1
// GET /api/strategy?action=points&userId=xxx
// GET /api/strategy?action=checkin_status&userId=xxx
// GET /api/strategy?action=comments&postId=1
// POST /api/strategy (body JSON)
func handleStrategy(w http.ResponseWriter, r *http.Request) {
	api := data.NewStrategyAPI()

	switch r.Method {
	case http.MethodGet:
		action := r.URL.Query().Get("action")
		switch action {
		case "list":
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
			posts, total := api.GetPosts(page, pageSize)
			writeJSON(w, map[string]interface{}{
				"posts": posts,
				"total": total,
			})

		case "detail":
			postID, _ := strconv.ParseUint(r.URL.Query().Get("postId"), 10, 64)
			post, err := api.GetPostDetail(uint(postID))
			if err != nil {
				writeJSON(w, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, post)

		case "points":
			userID := r.URL.Query().Get("userId")
			u, err := api.GetUserPoints(userID)
			if err != nil {
				writeJSON(w, map[string]interface{}{
					"points":   0,
					"totalIn":  0,
					"totalOut": 0,
				})
				return
			}
			writeJSON(w, u)

		case "checkin_status":
			userID := r.URL.Query().Get("userId")
			checked := api.HasCheckedIn(userID)
			writeJSON(w, map[string]bool{"checkedIn": checked})

		case "comments":
			postID, _ := strconv.ParseUint(r.URL.Query().Get("postId"), 10, 64)
			comments, err := api.GetComments(uint(postID))
			if err != nil {
				writeJSON(w, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, comments)

		case "liked":
			postID, _ := strconv.ParseUint(r.URL.Query().Get("postId"), 10, 64)
			userID := r.URL.Query().Get("userId")
			liked := api.HasLiked(uint(postID), userID)
			viewed := api.HasViewed(uint(postID), userID)
			writeJSON(w, map[string]bool{"liked": liked, "viewed": viewed})

		case "today_reply_points":
			userID := r.URL.Query().Get("userId")
			total := api.GetTodayReplyPoints(userID)
			writeJSON(w, map[string]int64{"todayReplyPoints": total})

		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
		}

	case http.MethodPost:
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		action, _ := req["action"].(string)
		userID, _ := req["userId"].(string)
		nickname, _ := req["nickname"].(string)

		switch action {
		case "create_post":
			title, _ := req["title"].(string)
			content, _ := req["content"].(string)
			imagesRaw, _ := req["images"].([]interface{})
			var images []string
			for _, img := range imagesRaw {
				if s, ok := img.(string); ok {
					images = append(images, s)
				}
			}
			post, err := api.CreatePost(userID, nickname, title, content, images)
			if err != nil {
				writeJSON(w, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, post)

		case "checkin":
			u, ok, err := api.CheckIn(userID, nickname)
			if err != nil {
				writeJSON(w, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, map[string]interface{}{
				"checkedIn": ok,
				"points":    u.Points,
			})

		case "view_post":
			postID, _ := req["postId"].(float64)
			post, deducted, remain, err := api.ViewPost(uint(postID), userID, nickname)
			if err != nil {
				writeJSON(w, map[string]interface{}{"error": err.Error()})
				return
			}
			writeJSON(w, map[string]interface{}{
				"post":     post,
				"deducted": deducted,
				"remain":   remain,
			})

		case "toggle_like":
			postID, _ := req["postId"].(float64)
			isLiked, count, err := api.ToggleLike(uint(postID), userID)
			if err != nil {
				writeJSON(w, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, map[string]interface{}{
				"liked":     isLiked,
				"likeCount": count,
			})

		case "add_comment":
			postID, _ := req["postId"].(float64)
			content, _ := req["content"].(string)
			imagesRaw, _ := req["images"].([]interface{})
			var images []string
			for _, img := range imagesRaw {
				if s, ok := img.(string); ok {
					images = append(images, s)
				}
			}
			var parentID *uint
			if pid, ok := req["parentId"].(float64); ok && pid > 0 {
				p := uint(pid)
				parentID = &p
			}
			var replyToUID, replyToName *string
			if ruid, ok := req["replyToUid"].(string); ok && ruid != "" {
				replyToUID = &ruid
			}
			if rname, ok := req["replyToName"].(string); ok && rname != "" {
				replyToName = &rname
			}

			comment, added, remain, err := api.CreateComment(uint(postID), parentID, userID, nickname, content, images, replyToUID, replyToName)
			if err != nil {
				writeJSON(w, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, map[string]interface{}{
				"comment":     comment,
				"addedPoints": added,
				"remain":      remain,
			})

		case "delete_comment":
			commentID, _ := req["commentId"].(float64)
			err := api.DeleteComment(uint(commentID), userID)
			if err != nil {
				writeJSON(w, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, map[string]string{"status": "ok"})

		case "delete_post":
			postID, _ := req["postId"].(float64)
			err := api.DeletePost(uint(postID), userID)
			if err != nil {
				writeJSON(w, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, map[string]string{"status": "ok"})

		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
