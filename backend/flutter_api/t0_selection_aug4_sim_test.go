package flutter_api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
)

func init() {
	if logger.SugaredLogger == nil {
		logger.InitLogger()
	}
}

func stockDBPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// .../backend/flutter_api/t0_selection_aug4_sim_test.go → .../backend/data/stock.db
	return filepath.Join(filepath.Dir(file), "..", "data", "stock.db")
}

// TestSimulateAug4Path 模拟 date=2026-08-04 的路由与预热→选股链路（真实拉数，可能较慢）
func TestSimulateAug4Path(t *testing.T) {
	const date = "2026-08-04"

	db.Init(stockDBPath(t))
	AutoMigrate()

	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	auth := newTestAuthService(t)
	user := authHTTPRegisterUser(t, auth, "13800000000", "Alice", "device-a")
	modules := NewModuleService(auth.dao)
	if err := modules.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{user.User.ID}, []string{"radar.main_strategy"}); err != nil {
		t.Fatalf("grant main: %v", err)
	}
	handler := newHTTPHandler(auth.AuthService)

	now := time.Now().In(chinaLocation())
	t.Logf("现在(上海)=%s 目标日=%s today=%s", now.Format("2006-01-02 15:04:05"), date, now.Format("2006-01-02"))

	// 1) 非「今天」不应进入 00:00~09:25 自动预热窗口
	if isBeforeT0AuctionCutoff(now, date) {
		t.Fatalf("date=%s 不是今天，不应命中自动预热窗口", date)
	}

	// 2) 显式 prewarm=1：应立刻返回 warming 或 ready，并后台拉日线
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, authHTTPRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.main_strategy&prewarm=1&date="+date,
		user.AccessToken, ""))
	if rr1.Code != 200 {
		t.Fatalf("prewarm status=%d body=%s", rr1.Code, rr1.Body.String())
	}
	var prewarmResp map[string]interface{}
	if err := json.Unmarshal(rr1.Body.Bytes(), &prewarmResp); err != nil {
		t.Fatal(err)
	}
	t.Logf("首次 prewarm 响应: %s", rr1.Body.String())
	if prewarmResp["prewarm"] != true {
		t.Fatalf("expected prewarm=true, got %v", prewarmResp["prewarm"])
	}
	status, _ := prewarmResp["status"].(string)
	if status != "warming" && status != "ready" {
		t.Fatalf("expected status warming|ready, got %q body=%s", status, rr1.Body.String())
	}

	// 3) 预热进行中再调一次：应立刻返回，不阻塞
	started := time.Now()
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, authHTTPRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.main_strategy&prewarm=1&date="+date,
		user.AccessToken, ""))
	elapsed := time.Since(started)
	t.Logf("二次 prewarm 耗时=%.3fs 响应=%s", elapsed.Seconds(), rr2.Body.String())
	if elapsed > 2*time.Second {
		t.Fatalf("二次 prewarm 不应阻塞，耗时 %.1fs", elapsed.Seconds())
	}

	// 4) 等待日线缓存就绪（最长 20 分钟）
	deadline := time.Now().Add(20 * time.Minute)
	for !isT0DailyCacheFilePresent(date) {
		if time.Now().After(deadline) {
			prog := getT0WarmProgress(date)
			t.Fatalf("等待日线缓存超时 status=%s err=%s fetched=%d/%d",
				prog.Status, prog.Err, prog.DailyFetched, prog.DailyTotal)
		}
		time.Sleep(3 * time.Second)
		prog := getT0WarmProgress(date)
		t.Logf("等待缓存... status=%s fetched=%d/%d stock=%d", prog.Status, prog.DailyFetched, prog.DailyTotal, prog.StockCount)
		if prog.Status == t0WarmStatusFailed {
			t.Fatalf("预热失败: %s", prog.Err)
		}
	}
	t.Logf("日线缓存已就绪: %s", t0DailyCachePath(date))

	// 5) prewarm 完成应返回 ready
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, authHTTPRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.main_strategy&prewarm=1&date="+date,
		user.AccessToken, ""))
	t.Logf("完成后 prewarm 响应: %s", rr3.Body.String())
	var ready map[string]interface{}
	_ = json.Unmarshal(rr3.Body.Bytes(), &ready)
	if s, _ := ready["status"].(string); s != "ready" {
		t.Fatalf("期望 ready, body=%s", rr3.Body.String())
	}

	// 6) 正式选股：应命中日线缓存
	rr4 := httptest.NewRecorder()
	started = time.Now()
	handler.ServeHTTP(rr4, authHTTPRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.main_strategy&date="+date+"&save=1",
		user.AccessToken, ""))
	selElapsed := time.Since(started)
	t.Logf("正式选股耗时=%.1fs body前500字=%s", selElapsed.Seconds(), truncateRunes(rr4.Body.String(), 500))
	if rr4.Code != 200 {
		t.Fatalf("selection status=%d", rr4.Code)
	}
	var sel map[string]interface{}
	if err := json.Unmarshal(rr4.Body.Bytes(), &sel); err != nil {
		t.Fatal(err)
	}
	if sel["date"] != date {
		t.Fatalf("date mismatch: %v", sel["date"])
	}
	if sel["status"] == "warming" {
		t.Fatal("缓存已就绪时正式选股不应返回 warming")
	}
	if errMsg, ok := sel["error"].(string); ok && errMsg != "" {
		t.Logf("选股返回 error（历史日+实时竞价可能为空，可接受）: %s", errMsg)
	} else {
		count, _ := sel["count"].(float64)
		t.Logf("选股成功 count=%.0f", count)
	}

	// 7) archived
	rr5 := httptest.NewRecorder()
	handler.ServeHTTP(rr5, authHTTPRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.main_strategy&archived=1&date="+date,
		user.AccessToken, ""))
	t.Logf("archived 响应: %s", truncateRunes(rr5.Body.String(), 400))
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
