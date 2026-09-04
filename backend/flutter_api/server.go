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
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	moneyDataCache     = make(map[string]models.StockMoneyDataDiff)
	moneyDataCacheTime time.Time
	moneyDataMutex     sync.RWMutex
	moneyDataCacheTTL  = 5 * time.Minute
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
		&data.StockBasic{},
		&data.FollowedStock{},
		&models.MarketStatistic{},
		&models.T0PatternStat{},
		&models.T0PatternConfig{},
	)
	// Telegraph 表由原项目管理，不在 flutter_api 层修改模型
	// 仅通过 SQL 迁移补充必要的索引和字段
	runTelegraphMigrations()
}

// runTelegraphMigrations 执行 Telegraph 表的 SQL 迁移
func runTelegraphMigrations() {
	// 检查 ai_opinion 列是否存在，不存在则添加
	var colCount int64
	db.Dao.Raw("SELECT COUNT(*) as cnt FROM pragma_table_info WHERE name='telegraph_list' AND tbl_name='telegraph_list' AND type='ai_opinion'").Scan(&colCount)
	// 如果查询失败或返回0，直接用 Exec 尝试添加（SQLite 会忽略已存在的列错误）
	if colCount == 0 {
		db.Dao.Exec("ALTER TABLE telegraph_list ADD COLUMN ai_opinion text;")
	}

	// 添加复合唯一索引防止重复数据（如不存在）
	db.Dao.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_telegraph_unique
		ON telegraph_list (source, title, data_time)
		WHERE title IS NOT NULL AND data_time IS NOT NULL;
	`)
}

// ---------------------------------------------------------------------------
// HTTP Server — 为 Flutter 前端提供 REST API + WebSocket 实时推送
// ---------------------------------------------------------------------------

var (
	wsClients   = make(map[*websocket.Conn]*wsClient)
	wsClientsMu sync.RWMutex
	upgrader    = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

type wsClient struct {
	conn      *websocket.Conn
	userID    string
	sessionID string
	writeMu   sync.Mutex
	closeOnce sync.Once

	beforeWrite func()
}

const (
	defaultHTTPServerPort = 8080
	webSocketWriteTimeout = 5 * time.Second
)

func (client *wsClient) writeMessage(messageType int, data []byte) error {
	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	if client.beforeWrite != nil {
		client.beforeWrite()
	}
	if err := client.conn.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout)); err != nil {
		return err
	}
	return client.conn.WriteMessage(messageType, data)
}

func (client *wsClient) close(code int, reason string) {
	client.closeOnce.Do(func() {
		client.writeMu.Lock()
		defer client.writeMu.Unlock()
		if client.beforeWrite != nil {
			client.beforeWrite()
		}
		deadline := time.Now().Add(webSocketWriteTimeout)
		_ = client.conn.SetWriteDeadline(deadline)
		_ = client.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(code, reason),
			deadline,
		)
		_ = client.conn.Close()
	})
}

// Start 启动 REST API + WebSocket 服务（阻塞）
func Start() {
	port := defaultHTTPServerPort

	// 解析并校验 T0 缓存根目录（gob/json 固定写入项目内 backend/data/cache），
	// 无法定位时直接终止，避免静默写到错误目录。
	if err := initT0CacheRoot(); err != nil {
		logger.SugaredLogger.Fatalf("[T0选股] 初始化缓存目录失败：%v", err)
	}

	if err := MigrateAuthTables(db.Dao); err != nil {
		logger.SugaredLogger.Errorf("认证表迁移失败: %v", err)
		return
	}

	authService := NewAuthService(db.Dao, func(userID, _ string, revokedSessionIDs []string) {
		closeWebSocketsForSessions(userID, revokedSessionIDs)
	})

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

	// 启动市场统计快照采集。用于支撑超短情绪盘中曲线：上涨/下跌家数、红盘率、涨跌停等。
	go func() {
		marketApi := data.NewMarketStatisticApi()
		if captured, err := captureMarketStatisticSnapshot(isTradingTime(), marketApi.FetchAndSave); captured {
			if err != nil {
				logger.SugaredLogger.Errorf("[定时任务] 市场统计快照采集失败：%v", err)
			} else {
				logger.SugaredLogger.Info("[定时任务] 市场统计快照采集完成")
			}
		}

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			captured, err := captureMarketStatisticSnapshot(isTradingTime(), marketApi.FetchAndSave)
			if !captured {
				continue
			}
			if err != nil {
				logger.SugaredLogger.Errorf("[定时任务] 市场统计快照采集失败：%v", err)
			} else {
				logger.SugaredLogger.Info("[定时任务] 市场统计快照采集完成")
			}
		}
	}()

	// 交易日 00:00~09:00 主动预热 T0 日线，避免早盘前无人访问时才开始拉数据。
	go func() {
		runT0AutoPrewarmTick(time.Now())

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for tick := range ticker.C {
			runT0AutoPrewarmTick(tick)
		}
	}()

	// 交易日 15:05 后把当日选股归档的 T0收盘涨幅 刷成真实收盘值，每天只刷一次。
	go func() {
		runT0CloseRefreshTick(time.Now())

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for tick := range ticker.C {
			runT0CloseRefreshTick(tick)
		}
	}()

	addr := fmt.Sprintf(":%d", port)
	logger.SugaredLogger.Infof("HTTP Server (for Flutter) starting on %s", addr)

	handler := newHTTPHandler(authService)

	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.SugaredLogger.Errorf("HTTP Server error: %v", err)
	}
}

type serverHandlerOverrides struct {
	News       http.HandlerFunc
	FollowList http.HandlerFunc
	Upload     http.HandlerFunc
	WebSocket  http.HandlerFunc
}

func newHTTPHandler(authService *AuthService, overrides ...serverHandlerOverrides) http.Handler {
	mux := http.NewServeMux()
	userDataHandler := userDataHTTPHandler{service: NewUserDataService(authService.dao)}
	newsHandler := handleGetNews
	followListHandler := userDataHandler.handleGetFollowList
	uploadHandler := handleFileUpload
	webSocketHandler := newWebSocketHandler(authService)
	if len(overrides) > 0 {
		if overrides[0].News != nil {
			newsHandler = overrides[0].News
		}
		if overrides[0].FollowList != nil {
			followListHandler = overrides[0].FollowList
		}
		if overrides[0].Upload != nil {
			uploadHandler = overrides[0].Upload
		}
		if overrides[0].WebSocket != nil {
			webSocketHandler = overrides[0].WebSocket
		}
	}

	adminService := NewAdminService(authService.dao, func(userID string, sessionIDs []string) {
		closeWebSocketsForSessions(userID, sessionIDs)
	})
	moduleService := NewModuleService(authService.dao)
	mux.Handle("/api/admin/", NewAdminHTTPHandler(adminService, moduleService))
	mux.Handle("/api/auth/", NewAuthHTTPHandler(authService, moduleService))
	mux.HandleFunc("/api/news", newsHandler)
	mux.HandleFunc("/api/news/domestic", handleGetDomesticNews)
	mux.HandleFunc("/api/news/clean-duplicate", handleCleanDuplicateNews)
	mux.HandleFunc("/api/kline", handleGetKLine)
	mux.HandleFunc("/api/global-indexes", handleGlobalIndexes)
	mux.HandleFunc("/api/industry-ranks", handleIndustryRanks)
	mux.HandleFunc("/api/hot-topics", handleHotTopics)
	mux.HandleFunc("/api/stock-changes", handleStockChanges)
	mux.HandleFunc("/api/stock-changes/save", handleStockChangesSave)
	mux.HandleFunc("/api/short-term-emotion", handleShortTermEmotion)
	mux.HandleFunc("/api/follow", userDataHandler.handleFollow)
	mux.HandleFunc("/api/unfollow", userDataHandler.handleUnfollow)
	mux.HandleFunc("/api/follow-list", followListHandler)
	mux.HandleFunc("/api/stock-search", handleStockSearch)
	mux.HandleFunc("/api/stock-realtime", handleStockRealtime)
	mux.HandleFunc("/api/strategy", handleStrategy)
	mux.HandleFunc("/api/upload", uploadHandler)
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/debug-money", handleDebugMoney)
	mux.HandleFunc("/api/stock-selection-test", handleStockSelectionTest)
	mux.HandleFunc("/api/ths-selection-test", handleTHSSelectionTest)
	mux.HandleFunc("/api/get-qgqp-bid", handleGetQgqpBid)
	mux.Handle("/api/t0-selection", newT0SelectionHandler(moduleService))

	uploadDir := filepath.Join("data", "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		logger.SugaredLogger.Errorf("创建上传目录失败: %v", err)
	}
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))
	mux.HandleFunc("/ws", webSocketHandler)

	// 托管 Flutter web 静态产物：根路径提供页面，未知路由回退 index.html（SPA）。
	// 缺产物时仅告警，服务继续以 API-only 模式运行。
	if webRoot, err := resolveFlutterWebRoot(); err != nil {
		logger.SugaredLogger.Warnf("[Web] 未托管前端页面：%v", err)
	} else {
		logger.SugaredLogger.Infof("[Web] 托管 Flutter 页面目录: %s", webRoot)
		mux.Handle("/", spaFileServer(webRoot))
	}

	return corsMiddleware(webSocketAccessTokenFallback(RequireAuth(authService, mux)))
}

func webSocketAccessTokenFallback(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws" && bearerTokenFromRequest(r) == "" {
			if token := strings.TrimSpace(r.URL.Query().Get("access_token")); token != "" {
				requestWithToken := r.Clone(r.Context())
				requestWithToken.Header = r.Header.Clone()
				requestWithToken.Header.Set("Authorization", "Bearer "+token)
				r = requestWithToken
			}
		}
		next.ServeHTTP(w, r)
	})
}

// resolveFlutterWebRoot 解析 Flutter web 静态产物目录：
// 优先 GO_STOCK_WEB_DIR，否则项目根下 trading_app/build/web；
// 目录不存在或无 index.html 返回错误。
func resolveFlutterWebRoot() (string, error) {
	candidates := []string{}
	if env := strings.TrimSpace(os.Getenv("GO_STOCK_WEB_DIR")); env != "" {
		candidates = append(candidates, env)
	}
	cwd, _ := os.Getwd()
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	for _, start := range []string{cwd, exeDir} {
		if root := findProjectRootUpward(start); root != "" {
			candidates = append(candidates, filepath.Join(root, "trading_app", "build", "web"))
		}
	}
	for _, dir := range candidates {
		if fi, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && !fi.IsDir() {
			abs, err := filepath.Abs(dir)
			if err != nil {
				return "", err
			}
			return abs, nil
		}
	}
	return "", fmt.Errorf("未找到 Flutter web 产物（需含 index.html），可设置 GO_STOCK_WEB_DIR 指定")
}

// spaFileServer 返回一个静态文件处理器：命中文件直接返回，
// 未命中且非 API 资源时回退 index.html，支持前端路由刷新。
func spaFileServer(webRoot string) http.Handler {
	fs := http.FileServer(http.Dir(webRoot))
	indexPath := filepath.Join(webRoot, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean(r.URL.Path)
		full := filepath.Join(webRoot, clean)
		if fi, err := os.Stat(full); err == nil && !fi.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}

func runT0AutoPrewarmTick(now time.Time) {
	if !shouldAutoPrewarmT0(now) {
		return
	}
	tradeDate := now.In(chinaLocation()).Format("2006-01-02")
	ensurePrevTradingDayBackfillStarted(tradeDate, now)
	if isPrevDayBackfillInProgress(tradeDate) {
		return
	}
	if started, _ := tryStartT0Prewarm(tradeDate); started {
		logger.SugaredLogger.Infof("[定时任务] T0 日线主动预热已启动: %s", tradeDate)
	}
}

func captureMarketStatisticSnapshot(isTrading bool, fetch func() error) (bool, error) {
	if !isTrading {
		return false, nil
	}
	return true, fetch()
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
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

// WriteJSONStatus commits a JSON content type before the response status.
func WriteJSONStatus(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	WriteJSON(w, v)
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

type userDataHTTPHandler struct {
	service *UserDataService
}

func (h userDataHTTPHandler) handleFollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		WriteAuthError(w, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证"))
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
	result, err := h.service.Follow(r.Context(), principal.UserID, req.StockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("关注股票失败: %v", err)
	}
	// 裸代码（无 sh/sz 前缀）重试：尝试 sz 前缀再试，失败再试 sh 前缀
	if result == "关注失败" && isBareCode(req.StockCode) {
		result, err = h.service.Follow(r.Context(), principal.UserID, "sz"+req.StockCode)
		if err != nil {
			logger.SugaredLogger.Errorf("关注股票失败: %v", err)
		}
	}
	if result == "关注失败" && isBareCode(req.StockCode) {
		result, err = h.service.Follow(r.Context(), principal.UserID, "sh"+req.StockCode)
		if err != nil {
			logger.SugaredLogger.Errorf("关注股票失败: %v", err)
		}
	}
	WriteJSON(w, map[string]string{"result": result})
}

// isBareCode 判断股票代码是否为裸代码（无 sh/sz/hk/us 前缀，纯数字）
func isBareCode(code string) bool {
	lower := strings.ToLower(code)
	if strings.HasPrefix(lower, "sh") || strings.HasPrefix(lower, "sz") ||
		strings.HasPrefix(lower, "hk") || strings.HasPrefix(lower, "us") ||
		strings.HasPrefix(lower, "gb_") {
		return false
	}
	// 纯数字即为裸代码
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(code) > 0
}

func (h userDataHTTPHandler) handleUnfollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		WriteAuthError(w, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证"))
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
	result, err := h.service.Unfollow(r.Context(), principal.UserID, req.StockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("取消关注股票失败: %v", err)
	}
	WriteJSON(w, map[string]string{"result": result})
}

func (h userDataHTTPHandler) handleGetFollowList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		WriteAuthError(w, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证"))
		return
	}
	groupID, _ := strconv.Atoi(r.URL.Query().Get("groupId"))
	result, err := h.service.ListFollowedStocks(r.Context(), principal.UserID, uint(groupID))
	if err != nil {
		logger.SugaredLogger.Errorf("获取关注股票列表失败: %v", err)
	}
	type FollowItem struct {
		StockCode          string  `json:"stockCode"`
		Name               string  `json:"name"`
		Price              float64 `json:"price"`
		ChangePercent      float64 `json:"changePercent"`
		AlarmChangePercent float64 `json:"alarmChangePercent"`
		Time               string  `json:"time"`
	}
	var items []FollowItem
	for _, s := range result {
		items = append(items, FollowItem{
			StockCode:          s.StockCode,
			Name:               s.Name,
			Price:              s.Price,
			ChangePercent:      s.ChangePercent,
			AlarmChangePercent: s.AlarmChangePercent,
			Time:               s.Time.Format("2006-01-02 15:04:05"),
		})
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
		Code               string  `json:"code"`
		Name               string  `json:"name"`
		Price              float64 `json:"price"`
		ChangePercent      float64 `json:"changePercent"`
		Volume             int64   `json:"volume"`
		Amount             float64 `json:"amount"`
		Open               float64 `json:"open"`
		PreClose           float64 `json:"preClose"`
		High               float64 `json:"high"`
		Low                float64 `json:"low"`
		MainForceNetInflow float64 `json:"mainForceNetInflow"`
		MainForceNetRatio  float64 `json:"mainForceNetRatio"`
		DayNetInflow       float64 `json:"dayNetInflow"`   // 当日净流入
		AccumNetInflow     float64 `json:"accumNetInflow"` // 累计净流入
		ServerTime         int64   `json:"serverTime"`     // 服务端毫秒时间戳
		Date               string  `json:"date"`           // 行情日期 yyyy-MM-dd
	}
	var items []RealtimeItem
	for _, s := range *result {
		price, _ := strconv.ParseFloat(s.Price, 64)
		volume, _ := strconv.ParseFloat(s.Volume, 64)
		amount, _ := strconv.ParseFloat(s.Amount, 64)
		// 腾讯接口amount单位为万元，转为元
		amount *= 10000
		open, _ := strconv.ParseFloat(s.Open, 64)
		preClose, _ := strconv.ParseFloat(s.PreClose, 64)
		high, _ := strconv.ParseFloat(s.High, 64)
		low, _ := strconv.ParseFloat(s.Low, 64)
		changePct := 0.0
		if preClose > 0 {
			changePct = (price - preClose) / preClose * 100
		}

		// 从新浪财经获取多日资金趋势数据（三个值全部用新浪数据，单位保持千元不转换）
		dayNetInflow := 0.0
		mainForceNetInflow := 0.0
		mainForceNetRatio := 0.0
		accumNetInflow := 0.0
		if trend := data.NewStockDataApi().GetStockMoneyTrendByDay(s.Code, 20); len(trend) > 0 {
			// 当日净流入 = 最近一天的 netamount（千元）
			if net, ok := trend[0]["netamount"]; ok {
				if n, err := strconv.ParseFloat(fmt.Sprintf("%v", net), 64); err == nil {
					dayNetInflow = n
				}
			}
			// 主力净流入 = 最近一天的 r0_net（千元）
			if r0, ok := trend[0]["r0_net"]; ok {
				if n, err := strconv.ParseFloat(fmt.Sprintf("%v", r0), 64); err == nil {
					mainForceNetInflow = n
				}
			}
			// 主力净占比 = 最近一天的 r0_ratio（%）
			if ratio, ok := trend[0]["r0_ratio"]; ok {
				if n, err := strconv.ParseFloat(fmt.Sprintf("%v", ratio), 64); err == nil {
					mainForceNetRatio = n * 100
				}
			}
			// 累计净流入 = 所有天的 netamount 累加（千元）
			for _, day := range trend {
				if net, ok := day["netamount"]; ok {
					if n, err := strconv.ParseFloat(fmt.Sprintf("%v", net), 64); err == nil {
						accumNetInflow += n
					}
				}
			}
		}

		items = append(items, RealtimeItem{
			Code:               s.Code,
			Name:               s.Name,
			Price:              price,
			ChangePercent:      changePct,
			Volume:             int64(volume),
			Amount:             amount,
			Open:               open,
			PreClose:           preClose,
			High:               high,
			Low:                low,
			MainForceNetInflow: mainForceNetInflow,
			MainForceNetRatio:  mainForceNetRatio,
			DayNetInflow:       dayNetInflow,
			AccumNetInflow:     accumNetInflow,
			ServerTime:         time.Now().UnixMilli(),
			Date:               s.Date,
		})
	}
	WriteJSON(w, items)
}

func getMoneyDataWithCache() map[string]models.StockMoneyDataDiff {
	moneyDataMutex.RLock()
	if time.Since(moneyDataCacheTime) < moneyDataCacheTTL && len(moneyDataCache) > 0 {
		cached := make(map[string]models.StockMoneyDataDiff, len(moneyDataCache))
		for k, v := range moneyDataCache {
			cached[k] = v
		}
		moneyDataMutex.RUnlock()
		return cached
	}
	moneyDataMutex.RUnlock()

	moneyDataMutex.Lock()
	defer moneyDataMutex.Unlock()

	if time.Since(moneyDataCacheTime) < moneyDataCacheTTL && len(moneyDataCache) > 0 {
		cached := make(map[string]models.StockMoneyDataDiff, len(moneyDataCache))
		for k, v := range moneyDataCache {
			cached[k] = v
		}
		return cached
	}

	resp := data.NewStockDataApi().GetStockMoneyData()
	if len(resp.Data.Diff) > 0 {
		newCache := make(map[string]models.StockMoneyDataDiff)
		for _, item := range resp.Data.Diff {
			newCache[item.F12] = item
		}
		moneyDataCache = newCache
		moneyDataCacheTime = time.Now()
		logger.SugaredLogger.Infof("[资金流向] 缓存更新，共 %d 条数据", len(moneyDataCache))
	}

	return moneyDataCache
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
	var items []searchStockItem
	for _, s := range result {
		items = append(items, searchStockItem{
			StockCode: data.ConvertTushareCodeToStockCode(s.TsCode),
			Name:      s.Name,
			Market:    s.Market,
		})
	}

	// 如果本地数据库搜索不到，使用在线搜索兜底
	if len(items) == 0 {
		items = searchStockOnline(keyword)
	}

	WriteJSON(w, items)
}

// searchStockOnline 使用东方财富在线搜索股票
type searchStockItem struct {
	StockCode string `json:"stockCode"`
	Name      string `json:"name"`
	Market    string `json:"market"`
}

func searchStockOnline(keyword string) []searchStockItem {
	type eastMoneySuggest struct {
		Code string `json:"Code"`
		Name string `json:"Name"`
	}
	var result struct {
		QuotationCodeTable struct {
			Data []eastMoneySuggest `json:"Data"`
		} `json:"QuotationCodeTable"`
	}
	url := fmt.Sprintf("https://searchadapter.eastmoney.com/api/suggest/get?input=%s&count=20&type=14", url.QueryEscape(keyword))
	resp, err := data.SharedHTTPClient.SetTimeout(10*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("Referer", "https://www.eastmoney.com/").
		Get(url)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil || len(result.QuotationCodeTable.Data) == 0 {
		return nil
	}
	var items []searchStockItem
	for _, s := range result.QuotationCodeTable.Data {
		market := "SZ"
		if len(s.Code) >= 3 {
			prefix := s.Code[:3]
			if prefix == "600" || prefix == "601" || prefix == "603" || prefix == "605" || prefix == "688" || prefix == "510" {
				market = "SH"
			}
		}
		items = append(items, searchStockItem{
			StockCode: s.Code,
			Name:      s.Name,
			Market:    market,
		})
	}
	return items
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, map[string]string{"status": "ok"})
}

func handleDebugMoney(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		code = "sz000066"
	}
	type moneyDetail struct {
		F62  float64 `json:"f62"`  // 主力净额
		F184 float64 `json:"f184"` // 主力净占比
		F66  float64 `json:"f66"`  // 超大单净额
		F69  float64 `json:"f69"`  // 超大单净占比
		F72  float64 `json:"f72"`  // 大单净额
		F75  float64 `json:"f75"`  // 大单净占比
		F78  float64 `json:"f78"`  // 中单净额
		F81  float64 `json:"f81"`  // 中单净占比
		F84  float64 `json:"f84"`  // 小单净额
		F87  float64 `json:"f87"`  // 小单净占比
		F45  float64 `json:"f45"`  // 外盘(主动买入量)
		F46  float64 `json:"f46"`  // 内盘(主动卖出量)
	}
	codeLower := strings.ToLower(code)
	var secid string
	if strings.HasPrefix(codeLower, "sh") {
		secid = "1." + code[2:]
	} else if strings.HasPrefix(codeLower, "sz") {
		secid = "0." + code[2:]
	} else {
		WriteJSON(w, map[string]string{"error": "code must start with sh or sz"})
		return
	}
	url := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f62,f184,f66,f69,f72,f75,f78,f81,f84,f87,f45,f46", secid)
	req := data.SharedHTTPClient.SetTimeout(10*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0").
		SetHeader("Referer", "https://quote.eastmoney.com")
	resp, err := req.Get(url)
	if err != nil {
		WriteJSON(w, map[string]string{"error": err.Error()})
		return
	}
	var fullResp struct {
		Data moneyDetail `json:"data"`
	}
	json.Unmarshal(resp.Body(), &fullResp)
	WriteJSON(w, map[string]interface{}{
		"code":      code,
		"secid":     secid,
		"moneyData": fullResp.Data,
		"note":      "f45=外盘(主动买入量)  f46=内盘(主动卖出量), 主力净额=f66+f72, 也可用(外盘-内盘)*均价估算净流入",
	})
}

func newWebSocketHandler(authService *AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			WriteAuthError(w, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证"))
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.SugaredLogger.Errorf("WebSocket upgrade error: %v", err)
			return
		}
		client := &wsClient{
			conn:      conn,
			userID:    principal.UserID,
			sessionID: principal.SessionID,
		}
		wsClientsMu.Lock()
		wsClients[conn] = client
		wsClientsMu.Unlock()

		revalidated, err := authService.Authenticate(r.Context(), bearerTokenFromRequest(r))
		if err != nil || revalidated.UserID != principal.UserID || revalidated.SessionID != principal.SessionID {
			removeWebSocketClient(client)
			client.close(websocket.ClosePolicyViolation, "session invalid")
			return
		}

		logger.SugaredLogger.Infof("WebSocket client connected: %s", conn.RemoteAddr())
		go func() {
			defer func() {
				removeWebSocketClient(client)
				client.close(websocket.CloseNormalClosure, "")
				logger.SugaredLogger.Infof("WebSocket client disconnected: %s", conn.RemoteAddr())
			}()
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					break
				}
			}
		}()
	}
}

func removeWebSocketClient(client *wsClient) {
	wsClientsMu.Lock()
	if registered, ok := wsClients[client.conn]; ok && registered == client {
		delete(wsClients, client.conn)
	}
	wsClientsMu.Unlock()
}

func closeWebSocketsForSessions(userID string, sessionIDs []string) {
	if len(sessionIDs) == 0 {
		return
	}
	sessionSet := make(map[string]struct{}, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		sessionSet[sessionID] = struct{}{}
	}

	wsClientsMu.Lock()
	clients := make([]*wsClient, 0)
	for conn, client := range wsClients {
		if client.userID != userID {
			continue
		}
		if _, ok := sessionSet[client.sessionID]; !ok {
			continue
		}
		delete(wsClients, conn)
		clients = append(clients, client)
	}
	wsClientsMu.Unlock()

	for _, client := range clients {
		client.close(websocket.ClosePolicyViolation, "session replaced")
	}
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
	clients := make([]*wsClient, 0, len(wsClients))
	for _, client := range wsClients {
		clients = append(clients, client)
	}
	wsClientsMu.RUnlock()
	for _, client := range clients {
		if err := client.writeMessage(websocket.TextMessage, raw); err != nil {
			logger.SugaredLogger.Warnf("Broadcast write error: %v", err)
			removeWebSocketClient(client)
			client.close(websocket.CloseGoingAway, "write failed")
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
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		WriteAuthError(w, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证"))
		return
	}
	userID := principal.UserID
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
			u, err := api.GetUserPoints(userID)
			if err != nil {
				WriteJSON(w, map[string]interface{}{"points": 0, "totalIn": 0, "totalOut": 0})
				return
			}
			WriteJSON(w, u)
		case "checkin_status":
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
			liked := api.HasLiked(uint(postID), userID)
			viewed := api.HasViewed(uint(postID), userID)
			WriteJSON(w, map[string]bool{"liked": liked, "viewed": viewed})
		case "today_reply_points":
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
		nickname, err := api.GetAuthenticatedNickname(userID)
		if err != nil {
			WriteJSON(w, map[string]string{"error": err.Error()})
			return
		}
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
