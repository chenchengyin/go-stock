package flutter_api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-stock/backend/agent"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"io"
	mrand "math/rand"
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
// 自动建表
// ---------------------------------------------------------------------------

func AutoMigrate() {
	db.Dao.AutoMigrate(
		&StrategyUser{},
		&StrategyPost{},
		&StrategyComment{},
		&StrategyLike{},
		&StrategyCheckIn{},
		&StrategyPointsLog{},
	)
	// Telegraph 表由原项目管理，不在 flutter_api 层修改模型
	// 仅通过 SQL 迁移补充必要的索引和字段
	runTelegraphMigrations()
}

// runTelegraphMigrations 执行 Telegraph 表的 SQL 迁移（避免修改原模型）
func runTelegraphMigrations() {
	// 添加 ai_opinion 字段（如不存在）
	db.Dao.Exec("ALTER TABLE telegraphs ADD COLUMN IF NOT EXISTS ai_opinion text;")

	// 添加复合唯一索引防止重复数据（如不存在）
	// 索名：idx_telegraph_unique，包含 source+title+data_time
	db.Dao.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_telegraph_unique
		ON telegraphs (source, title, data_time)
		WHERE title IS NOT NULL AND data_time IS NOT NULL;
	`)
}

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
	mux.HandleFunc("/api/news/domestic", handleGetDomesticNews)
	mux.HandleFunc("/api/news/clean-duplicate", handleCleanDuplicateNews)
	mux.HandleFunc("/api/kline", handleGetKLine)
	mux.HandleFunc("/api/global-indexes", handleGlobalIndexes)
	mux.HandleFunc("/api/industry-ranks", handleIndustryRanks)
	mux.HandleFunc("/api/hot-topics", handleHotTopics)
	mux.HandleFunc("/api/stock-changes", handleStockChanges)
	mux.HandleFunc("/api/stock-changes/save", handleStockChangesSave)
	mux.HandleFunc("/api/follow", handleFollow)
	mux.HandleFunc("/api/unfollow", handleUnfollow)
	mux.HandleFunc("/api/follow-list", handleGetFollowList)
	mux.HandleFunc("/api/stock-search", handleStockSearch)
	mux.HandleFunc("/api/stock-realtime", handleStockRealtime)
	mux.HandleFunc("/api/strategy", handleStrategy)
	mux.HandleFunc("/api/upload", handleFileUpload)
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/stock-selection-test", handleStockSelectionTest)
	mux.HandleFunc("/api/ths-selection-test", handleTHSSelectionTest)
	mux.HandleFunc("/api/get-qgqp-bid", handleGetQgqpBid)

	uploadDir := filepath.Join("data", "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		logger.SugaredLogger.Errorf("创建上传目录失败: %v", err)
	}
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))

	mux.HandleFunc("/ws", handleWebSocket)

	// 启动定时新闻抓取（每60秒抓一次财联社和新浪新闻）
	go func() {
		newsApi := data.NewMarketNewsApi()
		logger.SugaredLogger.Info("[定时任务] 启动新闻抓取...")
		newsApi.TelegraphList(30)
		newsApi.GetSinaNews(30)
		logger.SugaredLogger.Info("[定时任务] 新闻抓取完成")
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			logger.SugaredLogger.Info("[定时任务] 开始抓取新闻...")
			newsApi.TelegraphList(30)
			newsApi.GetSinaNews(30)
			logger.SugaredLogger.Info("[定时任务] 新闻抓取完成")
		}
	}()

	go func() {
		cronApi := agent.NewCronTaskApi()
		if !cronApi.ExistsByTaskType("stock_change_save") {
			task := &models.CronTask{
				Name:        "异动数据保存",
				CronExpr:    "0 */1 * * * *",
				TaskType:    "stock_change_save",
				Enable:      true,
				Status:      "active",
				Description: "每分钟自动保存A股异动数据（火箭发射、快速反弹、大笔买入、封涨停板等），交易时间外自动跳过",
			}
			err := cronApi.Create(task)
			if err != nil {
				logger.SugaredLogger.Errorf("自动创建异动数据保存任务失败：%v", err)
			} else {
				logger.SugaredLogger.Info("已自动创建异动数据保存定时任务")
			}
		}

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if !isTradingTime() {
				continue
			}

			intervalSec := int64(60)
			var settings data.Settings
			if err := db.Dao.First(&settings).Error; err == nil && settings.StockChangeIntervalSec > 0 {
				intervalSec = settings.StockChangeIntervalSec
			}

			randomDelay := time.Duration(mrand.Int63n(intervalSec/2)) * time.Second
			if randomDelay > 0 {
				time.Sleep(randomDelay)
			}

			api := data.NewStockChangesApi()
			changeTypes := []int{
				8201, 8202, 8193, 4, 32, 64, 8207, 8209, 8211, 8213, 8215,
				8204, 8203, 8194, 8, 16, 128, 8208, 8210, 8212, 8214, 8216,
			}
			result := api.GetStockChanges(changeTypes, 0, 500)
			if result == nil || len(result.Data) == 0 {
				continue
			}

			savedCount, err := data.NewStockChangeHistoryService().SaveStockChangesWithDedup(result.Data)
			if err != nil {
				logger.SugaredLogger.Errorf("保存异动数据失败：%v", err)
			} else {
				logger.SugaredLogger.Infof("成功保存 %d 条异动数据（去重后）", savedCount)
			}
		}
	}()

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

// WriteJSON 写入 JSON 响应
func WriteJSON(w http.ResponseWriter, v interface{}) {
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
	// 使用 NewsWrapper 在 flutter_api 层做去重
	news := NewNewsWrapper().GetNewsList(source, limit)
	WriteJSON(w, news)
}

// handleGetDomesticNews 获取国内新闻（财联社+新浪，按时间倒序）
func handleGetDomesticNews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}
	// 使用 NewsWrapper 在 flutter_api 层做去重
	news := NewNewsWrapper().GetDomesticNews(limit)
	WriteJSON(w, news)
}

// handleCleanDuplicateNews 清理重复新闻数据
func handleCleanDuplicateNews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 使用 SQL 删除重复数据，保留 ID 最小的记录
	// 对于有标题的记录，按 source+title+data_time 去重
	// 表名是 telegraph_list（GORM 默认表名）
	result := db.Dao.Exec(`
		DELETE FROM telegraph_list 
		WHERE id NOT IN (
			SELECT MIN(id) 
			FROM telegraph_list 
			WHERE title != '' 
			GROUP BY source, title, data_time
		) 
		AND title != ''
	`)

	if result.Error != nil {
		http.Error(w, "Clean duplicate error: "+result.Error.Error(), http.StatusInternalServerError)
		return
	}

	WriteJSON(w, map[string]interface{}{
		"deleted": result.RowsAffected,
		"message": "Duplicate news cleaned successfully",
	})
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
		WriteJSON(w, map[string]string{"error": "code is required"})
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
	WriteJSON(w, result)
}

func handleGlobalIndexes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	indexes := data.NewMarketNewsApi().GlobalStockIndexes(30)
	WriteJSON(w, indexes)
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
	valuationResp := getAllIndustryValuation()
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
	WriteJSON(w, rankData)
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
	WriteJSON(w, topics)
}

func handleStockChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rawCodes := r.URL.Query().Get("codes")

	service := data.NewStockChangeHistoryService()

	// codes 为空 → 返回今天全部异动
	if rawCodes == "" {
		result, err := service.GetLatestByStockCodes(data.StockChangeCodesQuery{
			Date:     time.Now().Format("2006-01-02"),
			PageSize: 10000,
		})
		if err != nil {
			WriteJSON(w, map[string]string{"error": err.Error()})
			return
		}
		WriteJSON(w, result)
		return
	}

	// 去掉市场前缀（sh/sz/bj），数据库存的只有纯数字代码
	parts := strings.Split(rawCodes, ",")
	cleanCodes := make([]string, len(parts))
	for i, c := range parts {
		cleanCodes[i] = strings.TrimLeft(c, "shszbjSHZSBJ")
	}
	query := data.StockChangeCodesQuery{
		StockCodes: cleanCodes,
		PageSize:   100,
	}
	result, err := service.GetLatestByStockCodes(query)
	if err != nil {
		WriteJSON(w, map[string]string{"error": err.Error()})
		return
	}
	WriteJSON(w, result)
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
		WriteJSON(w, map[string]string{"result": "参数错误"})
		return
	}
	if req.StockCode == "" {
		WriteJSON(w, map[string]string{"result": "股票代码不能为空"})
		return
	}
	result := data.NewStockDataApi().Follow(req.StockCode)
	WriteJSON(w, map[string]string{"result": result})
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
		WriteJSON(w, map[string]string{"result": "参数错误"})
		return
	}
	if req.StockCode == "" {
		WriteJSON(w, map[string]string{"result": "股票代码不能为空"})
		return
	}
	result := data.NewStockDataApi().UnFollow(req.StockCode)
	WriteJSON(w, map[string]string{"result": result})
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
	WriteJSON(w, items)
}

func handleStockRealtime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	codesParam := r.URL.Query().Get("codes")
	if codesParam == "" {
		WriteJSON(w, []map[string]any{})
		return
	}
	codes := strings.Split(codesParam, ",")
	result, err := data.NewStockDataApi().GetStockCodeRealTimeData(codes...)
	if err != nil || result == nil {
		WriteJSON(w, []map[string]any{})
		return
	}
	type RealtimeItem struct {
		Code          string  `json:"code"`
		Name          string  `json:"name"`
		Price         float64 `json:"price"`
		ChangePercent float64 `json:"changePercent"`
		Volume        int64   `json:"volume"`
		Amount        float64 `json:"amount"`
		Open          float64 `json:"open"`
		PreClose      float64 `json:"preClose"`
		High          float64 `json:"high"`
		Low           float64 `json:"low"`
	}
	var items []RealtimeItem
	for _, s := range *result {
		price, _ := strconv.ParseFloat(s.Price, 64)
		volume, _ := strconv.ParseFloat(s.Volume, 64)
		amount, _ := strconv.ParseFloat(s.Amount, 64)
		open, _ := strconv.ParseFloat(s.Open, 64)
		preClose, _ := strconv.ParseFloat(s.PreClose, 64)
		high, _ := strconv.ParseFloat(s.High, 64)
		low, _ := strconv.ParseFloat(s.Low, 64)
		changePct := 0.0
		if preClose > 0 {
			changePct = (price - preClose) / preClose * 100
		}
		items = append(items, RealtimeItem{
			Code:          s.Code,
			Name:          s.Name,
			Price:         price,
			ChangePercent: changePct,
			Volume:        int64(volume),
			Amount:        amount,
			Open:          open,
			PreClose:      preClose,
			High:          high,
			Low:           low,
		})
	}
	WriteJSON(w, items)
}

func handleStockChangesSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.SugaredLogger.Info("[手动触发] 开始保存异动数据...")

	api := data.NewStockChangesApi()
	changeTypes := []int{
		8201, 8202, 8193, 4, 32, 64, 8207, 8209, 8211, 8213, 8215,
		8204, 8203, 8194, 8, 16, 128, 8208, 8210, 8212, 8214, 8216,
	}
	result := api.GetStockChanges(changeTypes, 0, 500)
	if result == nil || len(result.Data) == 0 {
		logger.SugaredLogger.Info("[手动触发] 没有获取到异动数据")
		WriteJSON(w, map[string]any{"success": true, "savedCount": 0, "message": "没有获取到异动数据"})
		return
	}

	savedCount, err := data.NewStockChangeHistoryService().SaveStockChangesWithDedup(result.Data)
	if err != nil {
		logger.SugaredLogger.Errorf("[手动触发] 保存异动数据失败：%v", err)
		WriteJSON(w, map[string]any{"success": false, "message": err.Error()})
		return
	}

	logger.SugaredLogger.Infof("[手动触发] 成功保存 %d 条异动数据（去重后）", savedCount)
	WriteJSON(w, map[string]any{"success": true, "savedCount": savedCount, "totalFetched": len(result.Data)})
}

func handleStockSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		WriteJSON(w, []map[string]string{})
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
	WriteJSON(w, items)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, map[string]string{"status": "ok"})
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
	WriteJSON(w, map[string]string{
		"url": fmt.Sprintf("http://localhost:%d/uploads/%s", defaultHTTPServerPort, filename),
	})
}

func handleStrategy(w http.ResponseWriter, r *http.Request) {
	api := NewStrategyAPI()
	switch r.Method {
	case http.MethodGet:
		action := r.URL.Query().Get("action")
		switch action {
		case "list":
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
			posts, total := api.GetPosts(page, pageSize)
			WriteJSON(w, map[string]interface{}{"posts": posts, "total": total})
		case "detail":
			postID, _ := strconv.ParseUint(r.URL.Query().Get("postId"), 10, 64)
			post, err := api.GetPostDetail(uint(postID))
			if err != nil {
				WriteJSON(w, map[string]string{"error": err.Error()})
				return
			}
			WriteJSON(w, post)
		case "points":
			userID := r.URL.Query().Get("userId")
			u, err := api.GetUserPoints(userID)
			if err != nil {
				WriteJSON(w, map[string]interface{}{"points": 0, "totalIn": 0, "totalOut": 0})
				return
			}
			WriteJSON(w, u)
		case "checkin_status":
			userID := r.URL.Query().Get("userId")
			checked := api.HasCheckedIn(userID)
			WriteJSON(w, map[string]bool{"checkedIn": checked})
		case "comments":
			postID, _ := strconv.ParseUint(r.URL.Query().Get("postId"), 10, 64)
			comments, err := api.GetComments(uint(postID))
			if err != nil {
				WriteJSON(w, map[string]string{"error": err.Error()})
				return
			}
			WriteJSON(w, comments)
		case "liked":
			postID, _ := strconv.ParseUint(r.URL.Query().Get("postId"), 10, 64)
			userID := r.URL.Query().Get("userId")
			liked := api.HasLiked(uint(postID), userID)
			viewed := api.HasViewed(uint(postID), userID)
			WriteJSON(w, map[string]bool{"liked": liked, "viewed": viewed})
		case "today_reply_points":
			userID := r.URL.Query().Get("userId")
			total := api.GetTodayReplyPoints(userID)
			WriteJSON(w, map[string]int64{"todayReplyPoints": total})
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
				WriteJSON(w, map[string]string{"error": err.Error()})
				return
			}
			WriteJSON(w, post)
		case "checkin":
			u, ok, err := api.CheckIn(userID, nickname)
			if err != nil {
				WriteJSON(w, map[string]string{"error": err.Error()})
				return
			}
			WriteJSON(w, map[string]interface{}{"checkedIn": ok, "points": u.Points})
		case "view_post":
			postID, _ := req["postId"].(float64)
			post, deducted, remain, err := api.ViewPost(uint(postID), userID, nickname)
			if err != nil {
				WriteJSON(w, map[string]interface{}{"error": err.Error()})
				return
			}
			WriteJSON(w, map[string]interface{}{"post": post, "deducted": deducted, "remain": remain})
		case "toggle_like":
			postID, _ := req["postId"].(float64)
			isLiked, count, err := api.ToggleLike(uint(postID), userID)
			if err != nil {
				WriteJSON(w, map[string]string{"error": err.Error()})
				return
			}
			WriteJSON(w, map[string]interface{}{"liked": isLiked, "likeCount": count})
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
				WriteJSON(w, map[string]string{"error": err.Error()})
				return
			}
			WriteJSON(w, map[string]interface{}{"comment": comment, "addedPoints": added, "remain": remain})
		case "delete_comment":
			commentID, _ := req["commentId"].(float64)
			err := api.DeleteComment(uint(commentID), userID)
			if err != nil {
				WriteJSON(w, map[string]string{"error": err.Error()})
				return
			}
			WriteJSON(w, map[string]string{"status": "ok"})
		case "delete_post":
			postID, _ := req["postId"].(float64)
			err := api.DeletePost(uint(postID), userID)
			if err != nil {
				WriteJSON(w, map[string]string{"error": err.Error()})
				return
			}
			WriteJSON(w, map[string]string{"status": "ok"})
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

// loMap 替代 github.com/samber/lo.Map 简化依赖
func loMap[T any, R any](collection []T, iteratee func(T, int) R) []R {
	result := make([]R, len(collection))
	for i, item := range collection {
		result[i] = iteratee(item, i)
	}
	return result
}

// getAllIndustryValuation 获取所有行业市值数据（替代 data.NewStockDataApi().GetAllIndustryValuation）
func getAllIndustryValuation() *IndustryValuationResp {
	url := "https://datacenter-web.eastmoney.com/api/data/v1/get?callback=data&reportName=RPT_VALUEINDUSTRY_STA&columns=ALL&quoteColumns=&source=WEB&client=WEB&pageNumber=1&pageSize=500&_=" + strconv.Itoa(time.Now().Nanosecond())
	client := data.CreateHTTPClientWithTimeout(30 * time.Second)
	resp, err := client.R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("getAllIndustryValuation err:%v", err)
		return nil
	}
	body := string(resp.Body())
	// 提取 JSON: data({...})
	start := strings.Index(body, "(")
	end := strings.LastIndex(body, ")")
	if start == -1 || end == -1 || end <= start {
		logger.SugaredLogger.Errorf("getAllIndustryValuation parse error: no brackets")
		return nil
	}
	result := &IndustryValuationResp{}
	if err := json.Unmarshal([]byte(body[start+1:end]), result); err != nil {
		logger.SugaredLogger.Errorf("getAllIndustryValuation unmarshal err:%v", err)
		return nil
	}
	return result
}

// IndustryValuationResp 行业估值响应
type IndustryValuationResp struct {
	Result IndustryValuationData `json:"result"`
}

type IndustryValuationData struct {
	Data []IndustryValuationItem `json:"data"`
}

type IndustryValuationItem struct {
	BOARDNAME    string  `json:"BOARDNAME"`
	MARKETCAPVAG float64 `json:"MARKETCAPVAG"`
}

func isTradingTime() bool {
	now := time.Now()
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}

	hour, minute := now.Hour(), now.Minute()
	currentTime := hour*100 + minute

	morningStart := 915
	morningEnd := 1130
	afternoonStart := 1300
	afternoonEnd := 1500

	isMorning := currentTime >= morningStart && currentTime <= morningEnd
	isAfternoon := currentTime >= afternoonStart && currentTime <= afternoonEnd

	return isMorning || isAfternoon
}
