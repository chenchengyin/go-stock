package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// HTTP Server — 为 Flutter 前端提供 REST API + WebSocket 实时推送
// ---------------------------------------------------------------------------

var (
	wsClients   = make(map[*websocket.Conn]bool)
	wsClientsMu sync.RWMutex
	upgrader    = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

const defaultHTTPServerPort = 8080

// Start 启动 REST API + WebSocket 服务（阻塞）
func Start() {
	port := defaultHTTPServerPort
	mux := http.NewServeMux()

	mux.HandleFunc("/api/news", handleGetNews)
	mux.HandleFunc("/api/kline", handleGetKLine)
	mux.HandleFunc("/api/global-indexes", handleGlobalIndexes)
	mux.HandleFunc("/api/industry-ranks", handleIndustryRanks)
	mux.HandleFunc("/api/hot-topics", handleHotTopics)
	mux.HandleFunc("/api/stock-changes", handleStockChanges)
	mux.HandleFunc("/api/follow", handleFollow)
	mux.HandleFunc("/api/unfollow", handleUnfollow)
	mux.HandleFunc("/api/follow-list", handleGetFollowList)
	mux.HandleFunc("/api/stock-search", handleStockSearch)
	mux.HandleFunc("/api/stock-realtime", handleStockRealtime)
	mux.HandleFunc("/api/strategy", handleStrategy)
	mux.HandleFunc("/api/upload", handleFileUpload)
	mux.HandleFunc("/api/health", handleHealth)

	uploadDir := filepath.Join("data", "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		logger.SugaredLogger.Errorf("创建上传目录失败: %v", err)
	}
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))

	mux.HandleFunc("/ws", handleWebSocket)

	addr := fmt.Sprintf(":%d", port)
	logger.SugaredLogger.Infof("HTTP Server (for Flutter) starting on %s", addr)

	handler := corsMiddleware(mux)

	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.SugaredLogger.Errorf("HTTP Server error: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.SugaredLogger.Errorf("writeJSON error: %v", err)
	}
}

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
		klt = "101"
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

func handleGlobalIndexes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	indexes := data.NewMarketNewsApi().GlobalStockIndexes(30)
	writeJSON(w, indexes)
}

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
	rankData := data.NewMarketNewsApi().GetIndustryRank(sort, limit)
	valuationResp := data.NewStockDataApi().GetAllIndustryValuation()
	valMap := make(map[string]float64)
	if valuationResp != nil && valuationResp.Result.Data != nil {
		for _, v := range valuationResp.Result.Data {
			valMap[v.BOARDNAME] = v.MARKETCAPVAG
		}
	}
	if rankItems, ok := rankData["data"].([]any); ok {
		for _, item := range rankItems {
			if m, ok := item.(map[string]any); ok {
				name := ""
				if n, ok := m["bd_name"].(string); ok {
					name = n
				}
				if capVal, found := valMap[name]; found {
					m["market_cap"] = capVal
				} else {
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

func handleStockChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	codes := r.URL.Query().Get("codes")
	if codes == "" {
		writeJSON(w, map[string]interface{}{"data": []interface{}{}, "totalCount": 0})
		return
	}
	service := data.NewStockChangeHistoryService()
	query := data.StockChangeCodesQuery{
		StockCodes: strings.Split(codes, ","),
		PageSize:   100,
	}
	result, err := service.GetLatestByStockCodes(query)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, result)
}

func handleFollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		StockCode string `json:"stockCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]string{"result": "参数错误"})
		return
	}
	if req.StockCode == "" {
		writeJSON(w, map[string]string{"result": "股票代码不能为空"})
		return
	}
	result := data.NewStockDataApi().Follow(req.StockCode)
	writeJSON(w, map[string]string{"result": result})
}

func handleUnfollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		StockCode string `json:"stockCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]string{"result": "参数错误"})
		return
	}
	if req.StockCode == "" {
		writeJSON(w, map[string]string{"result": "股票代码不能为空"})
		return
	}
	result := data.NewStockDataApi().UnFollow(req.StockCode)
	writeJSON(w, map[string]string{"result": result})
}

func handleGetFollowList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	groupId, _ := strconv.Atoi(r.URL.Query().Get("groupId"))
	result := data.NewStockDataApi().GetFollowList(groupId)
	type FollowItem struct {
		StockCode          string  `json:"stockCode"`
		Name               string  `json:"name"`
		Price              float64 `json:"price"`
		ChangePercent      float64 `json:"changePercent"`
		AlarmChangePercent float64 `json:"alarmChangePercent"`
		Time               string  `json:"time"`
	}
	var items []FollowItem
	if result != nil {
		for _, s := range *result {
			items = append(items, FollowItem{
				StockCode:          s.StockCode,
				Name:               s.Name,
				Price:              s.Price,
				ChangePercent:      s.ChangePercent,
				AlarmChangePercent: s.AlarmChangePercent,
				Time:               s.Time.Format("2006-01-02 15:04:05"),
			})
		}
	}
	writeJSON(w, items)
}

func handleStockRealtime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	codesParam := r.URL.Query().Get("codes")
	if codesParam == "" {
		writeJSON(w, []map[string]any{})
		return
	}
	codes := strings.Split(codesParam, ",")
	result, err := data.NewStockDataApi().GetStockCodeRealTimeData(codes...)
	if err != nil || result == nil {
		writeJSON(w, []map[string]any{})
		return
	}
	type RealtimeItem struct {
		Code          string  `json:"code"`
		Name          string  `json:"name"`
		Price         float64 `json:"price"`
		ChangePercent float64 `json:"changePercent"`
		Volume        int64   `json:"volume"`
		Amount        float64 `json:"amount"`
	}
	var items []RealtimeItem
	for _, s := range *result {
		price, _ := strconv.ParseFloat(s.Price, 64)
		volume, _ := strconv.ParseFloat(s.Volume, 64)
		amount, _ := strconv.ParseFloat(s.Amount, 64)
		items = append(items, RealtimeItem{
			Code:          s.Code,
			Name:          s.Name,
			Price:         price,
			ChangePercent: s.ChangePercent,
			Volume:        int64(volume),
			Amount:        amount,
		})
	}
	writeJSON(w, items)
}

func handleStockSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		writeJSON(w, []map[string]string{})
		return
	}
	result := data.NewStockDataApi().GetStockList(keyword)
	type SearchItem struct {
		StockCode string `json:"stockCode"`
		Name      string `json:"name"`
		Market    string `json:"market"`
	}
	var items []SearchItem
	for _, s := range result {
		items = append(items, SearchItem{
			StockCode: data.ConvertTushareCodeToStockCode(s.TsCode),
			Name:      s.Name,
			Market:    s.Market,
		})
	}
	writeJSON(w, items)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.SugaredLogger.Errorf("WebSocket upgrade error: %v", err)
		return
	}
	wsClientsMu.Lock()
	wsClients[conn] = true
	wsClientsMu.Unlock()
	logger.SugaredLogger.Infof("WebSocket client connected: %s", conn.RemoteAddr())
	go func() {
		defer func() {
			wsClientsMu.Lock()
			delete(wsClients, conn)
			wsClientsMu.Unlock()
			conn.Close()
			logger.SugaredLogger.Infof("WebSocket client disconnected: %s", conn.RemoteAddr())
		}()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

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
			delete(wsClients, conn)
		}
	}
}

func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "文件太大或格式错误", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "读取文件失败", http.StatusBadRequest)
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	if !allowed[ext] {
		http.Error(w, "不支持的文件格式", http.StatusBadRequest)
		return
	}
	randBytes := make([]byte, 8)
	rand.Read(randBytes)
	filename := hex.EncodeToString(randBytes) + ext
	uploadDir := filepath.Join("data", "uploads")
	os.MkdirAll(uploadDir, 0755)
	dst, err := os.Create(filepath.Join(uploadDir, filename))
	if err != nil {
		http.Error(w, "保存文件失败", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "写入文件失败", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{
		"url": fmt.Sprintf("http://localhost:%d/uploads/%s", defaultHTTPServerPort, filename),
	})
}

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
			writeJSON(w, map[string]interface{}{"posts": posts, "total": total})
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
				writeJSON(w, map[string]interface{}{"points": 0, "totalIn": 0, "totalOut": 0})
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
			writeJSON(w, map[string]interface{}{"checkedIn": ok, "points": u.Points})
		case "view_post":
			postID, _ := req["postId"].(float64)
			post, deducted, remain, err := api.ViewPost(uint(postID), userID, nickname)
			if err != nil {
				writeJSON(w, map[string]interface{}{"error": err.Error()})
				return
			}
			writeJSON(w, map[string]interface{}{"post": post, "deducted": deducted, "remain": remain})
		case "toggle_like":
			postID, _ := req["postId"].(float64)
			isLiked, count, err := api.ToggleLike(uint(postID), userID)
			if err != nil {
				writeJSON(w, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, map[string]interface{}{"liked": isLiked, "likeCount": count})
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
			writeJSON(w, map[string]interface{}{"comment": comment, "addedPoints": added, "remain": remain})
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
