package flutter_api

import (
    "fmt"
    "testing"
    "time"
)

func TestDebugTodayRed(t *testing.T) {
    old := t0CacheRootPath
    t0CacheRootPath = "/Users/vb/Projects/go-stock/backend/data/cache"
    t.Cleanup(func(){ t0CacheRootPath = old })
    a, ok := loadT0SelectionArchive("2026-09-21")
    if !ok { t.Fatal("archive missing") }
    ctx, err := loadT0ModuleSelectionContextForResults(redT0StrategyModuleCode, "2026-09-21", a.Results)
    if err != nil { t.Fatal(err) }
    got := filterRedT0Results(a.Results, ctx)
    fmt.Printf("ARCHIVE count=%d\n", len(a.Results))
    for _, r := range a.Results { fmt.Printf("ARCHIVE %s %s open=%.2f close=%.2f\n",r.StockCode,r.StockName,r.OpenGap,r.CloseRet) }
    fmt.Printf("RED count=%d\n", len(got))
    for _, r := range got { fmt.Printf("RED %s %s open=%.2f close=%.2f hits=%v deep=%v\n",r.StockCode,r.StockName,r.OpenGap,r.CloseRet,r.DisplayRuleHits,r.StrongDisplayRuleHit) }
    cached, ok := loadT0DailyCache("2026-09-21")
    if !ok { t.Fatal("daily cache missing") }
    resp := buildPrewarmReadyResponseAtWithCache("2026-09-21", time.Date(2026,9,21,10,0,0,0,chinaLocation()), cached)
    cands, _ := resp["candidates"].([]T0SelectionResult)
    preview, err := selectT0ResultsForModule(redT0StrategyModuleCode, cands, &t0ModuleSelectionContext{TradeDate:"2026-09-21",Daily:cached.Daily})
    if err != nil { t.Fatal(err) }
    fmt.Printf("PREVIEW candidates=%d red=%d\n",len(cands),len(preview))
    for _, r := range preview { fmt.Printf("PREVIEW %s %s open=%.2f close=%.2f hits=%v deep=%v\n",r.StockCode,r.StockName,r.OpenGap,r.CloseRet,r.DisplayRuleHits,r.StrongDisplayRuleHit) }
}
