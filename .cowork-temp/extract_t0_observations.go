package main

import (
  "encoding/json"
  "fmt"
  "os"
  "path/filepath"
  "sort"
  "strings"
  "go-stock/backend/analysis/candlepattern"
)

type Row struct {
 Date string `json:"date"`
 ShortCode string `json:"short_code"`
 Pattern string `json:"pattern"`
 Gap float64 `json:"open_gap"`
 PnL float64 `json:"pnl"`
}
func main() {
 root,err:=candlepattern.ResolveCacheRoot(".");if err!=nil{panic(err)}
 entries,err:=os.ReadDir(filepath.Join(root,"t0","daily"));if err!=nil{panic(err)}
 var dates []string
 for _,e:=range entries { n:=e.Name(); if strings.HasPrefix(n,"t0_daily_cache_")&&strings.HasSuffix(n,".gob") { d:=strings.TrimSuffix(strings.TrimPrefix(n,"t0_daily_cache_"),".gob"); if d>="2025-01-02"&&d<="2026-08-22" {dates=append(dates,d)} } }
 sort.Strings(dates)
 rows:=make([]Row,0)
 for _,d:=range dates { c,err:=candlepattern.LoadDailyCache(root,d);if err!=nil{fmt.Fprintln(os.Stderr,"skip",d,err);continue}; for _,o:=range candlepattern.CollectObservations(c,d,3){if o.InT0Subset{rows=append(rows,Row{d,o.ShortCode,o.Pattern,o.T0Gap,o.T0PnL})}} }
 out:=struct{DateStart string `json:"date_start"`;DateEnd string `json:"date_end"`;TradingDays int `json:"trading_days"`;Rows []Row `json:"rows"`}{"2025-01-02","2026-08-22",len(dates),rows}
 enc:=json.NewEncoder(os.Stdout);if err:=enc.Encode(out);err!=nil{panic(err)}
}
