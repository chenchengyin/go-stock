// Research-only exporter. Reads the existing cache without importing the app
// (and therefore without app startup, network calls, or database writes).
package main

import (
 "compress/gzip"
 "encoding/csv"
 "encoding/gob"
 "encoding/json"
 "fmt"
 "os"
 "sort"
 "strconv"
)

type Bar struct {
 Date string
 Open, Close, High, Low, Volume, AmountYi float64
 VolumeUnit, AmountSource, AmountUnit string
 AmountVerified bool
}
type Stock struct {
 Code, Name, KLineSource, FirstDate, LastDate, Status, Error string
 FloatMarketCapYi float64
 BarCount int
}
type PoolStock struct { Code, Name string; MarketCapYi float64 }
type Snapshot struct {
 SchemaVersion int
 GeneratedAt, MarketCapSnapshotAt, MarketCapSource, StartDate, EndDate string
 StockPoolTotal, EligibleStockCount int
 Stocks []Stock
 Daily map[string][]Bar
 SelectionPools map[string][]PoolStock
 ClosedDailyDates, NonTradingDates map[string]string
}
func main() {
 if len(os.Args)!=4 { panic("usage: export_gob input.gob output.csv.gz metadata.json") }
 f,err:=os.Open(os.Args[1]); if err!=nil {panic(err)}; defer f.Close()
 var s Snapshot
 if err=gob.NewDecoder(f).Decode(&s);err!=nil {panic(err)}
 out,err:=os.Create(os.Args[2]);if err!=nil {panic(err)}
 gz:=gzip.NewWriter(out); w:=csv.NewWriter(gz)
 w.Write([]string{"code","date","open","close","high","low","volume","amount_yi","amount_verified","amount_source","volume_unit"})
 keys:=make([]string,0,len(s.Daily));for code:=range s.Daily {keys=append(keys,code)};sort.Strings(keys)
 count:=0
 for _,code:=range keys { for _,b:=range s.Daily[code] {
  row:=[]string{code,b.Date};for _,v:=range []float64{b.Open,b.Close,b.High,b.Low,b.Volume,b.AmountYi} {row=append(row,strconv.FormatFloat(v,'g',-1,64))}
  row=append(row,strconv.FormatBool(b.AmountVerified),b.AmountSource,b.VolumeUnit)
  if err=w.Write(row);err!=nil {panic(err)};count++
 } }
 w.Flush();if err=w.Error();err!=nil {panic(err)};if err=gz.Close();err!=nil {panic(err)};if err=out.Close();err!=nil {panic(err)}
 s.Daily=nil
 meta,err:=os.Create(os.Args[3]);if err!=nil {panic(err)};defer meta.Close();if err=json.NewEncoder(meta).Encode(s);err!=nil {panic(err)}
 fmt.Printf("Exported %d bars, %d symbols, %d stocks, %d dated pools; generated=%s; end=%s\n",count,len(keys),len(s.Stocks),len(s.SelectionPools),s.GeneratedAt,s.EndDate)
}
