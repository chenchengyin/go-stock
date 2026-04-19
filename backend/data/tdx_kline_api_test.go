package data

import (
	"fmt"
	"go-stock/backend/db"
	"testing"
)

func init() {
	db.Init("../../data/stock.db")
}

func TestTdxKLineApi_GetCallAuction(t *testing.T) {
	api := NewTdxKLineApi()
	data := api.GetCallAuction("600519.SH", 0, 50)
	if data == nil {
		t.Fatal("GetCallAuction returned nil")
	}
	t.Logf("集合竞价数据条数: %d", len(*data))
	if len(*data) > 0 {
		first := (*data)[0]
		t.Logf("首条: time=%s price=%s matched=%s unmatched=%s flag=%s",
			first.Time, first.Price, first.Matched, first.Unmatched, first.Flag)
		last := (*data)[len(*data)-1]
		t.Logf("末条: time=%s price=%s matched=%s unmatched=%s flag=%s",
			last.Time, last.Price, last.Matched, last.Unmatched, last.Flag)
	}
}

func TestTdxKLineApi_GetKLineData(t *testing.T) {
	api := NewTdxKLineApi()
	data := api.GetKLineData("600519.SH", "101", 10)
	if data == nil {
		t.Fatal("GetKLineData returned nil")
	}
	t.Logf("日K数据条数: %d", len(*data))
	if len(*data) > 0 {
		first := (*data)[0]
		t.Logf("首条: day=%s open=%s close=%s high=%s low=%s vol=%s",
			first.Day, first.Open, first.Close, first.High, first.Low, first.Volume)
	}
}

func TestTdxKLineApi_AllPeriods(t *testing.T) {
	api := NewTdxKLineApi()
	periods := []struct {
		klt   string
		label string
	}{
		{"1", "1分钟"}, {"5", "5分钟"}, {"10", "10分钟(聚合)"},
		{"15", "15分钟"}, {"30", "30分钟"}, {"60", "60分钟"},
		{"120", "120分钟(聚合)"}, {"101", "日线"}, {"102", "周线"},
		{"103", "月线"}, {"104", "季线"}, {"105", "半年线(聚合)"},
		{"106", "年线"},
	}
	for _, p := range periods {
		t.Run(p.label, func(t *testing.T) {
			data := api.GetKLineData("600519.SH", p.klt, 5)
			if data == nil {
				t.Errorf("[%s] klt=%s returned nil", p.label, p.klt)
				return
			}
			count := len(*data)
			t.Logf("[%s] klt=%s 条数=%d", p.label, p.klt, count)
			if count > 0 {
				last := (*data)[count-1]
				t.Logf("  末条: day=%s open=%s close=%s high=%s low=%s vol=%s",
					last.Day, last.Open, last.Close, last.High, last.Low, last.Volume)
			}
		})
	}
}

func TestTdxKLineApi_GetF10Data(t *testing.T) {
	api := NewTdxKLineApi()
	bundle := api.GetF10Data("600519.SH")
	if bundle == nil {
		t.Fatal("GetF10Data returned nil")
	}
	t.Logf("公司信息分类数: %d", len(bundle.Sections))
	for _, s := range bundle.Sections {
		t.Logf("  分类: %s (内容长度: %d)", s.Name, len(s.Content))
		//t.Logf("  内容: %s", s.Content)
	}
	if bundle.Finance != nil {
		f := bundle.Finance
		t.Logf("财务信息: code=%s eps=%.4f netAssetsPerShare=%.4f totalShares=%.2f ipoDate=%s",
			f.Code, f.EPS, f.NetAssetsPerShare, f.TotalShares, f.IPODate)
	}
	t.Logf("除权除息记录数: %d", len(bundle.XDXR))
	if len(bundle.XDXR) > 0 {
		last := bundle.XDXR[len(bundle.XDXR)-1]
		t.Logf("  最近除权除息: date=%s name=%s", last.Date, last.Name)
	}

}

func TestTdxKLineApi_GetFinanceInfo(t *testing.T) {
	api := NewTdxKLineApi()
	f := api.GetFinanceInfo("600519.SH")
	if f == nil {
		t.Fatal("GetFinanceInfo returned nil")
	}
	t.Logf("code=%s eps=%.4f netAssetsPerShare=%.4f totalShares=%.2f floatShares=%.2f ipoDate=%s updatedDate=%s",
		f.Code, f.EPS, f.NetAssetsPerShare, f.TotalShares, f.FloatShares, f.IPODate, f.UpdatedDate)
	t.Logf("totalAssets=%.2f totalEquity=%.2f operatingRevenue=%.2f netProfit=%.2f shareholderCount=%.0f",
		f.TotalAssets, f.TotalEquity, f.OperatingRevenue, f.NetProfit, f.ShareholderCount)
}

func TestTdxKLineApi_GetXDXRInfo(t *testing.T) {
	api := NewTdxKLineApi()
	items := api.GetXDXRInfo("600519.SH")
	if items == nil {
		t.Fatal("GetXDXRInfo returned nil")
	}
	t.Logf("除权除息记录数: %d", len(*items))
	if len(*items) > 0 {
		for i := len(*items) - 3; i < len(*items); i++ {
			if i < 0 {
				continue
			}
			x := (*items)[i]
			fh := "-"
			if x.Fenhong != nil {
				fh = fmt.Sprintf("%.4f", *x.Fenhong)
			}
			t.Logf("  [%d] date=%s name=%s fenhong=%s", i, x.Date, x.Name, fh)
		}
	}
}

func TestTdxKLineApi_GetF10CategoryList(t *testing.T) {
	api := NewTdxKLineApi()
	cats := api.GetF10CategoryList("600519.SH")
	if cats == nil {
		t.Fatal("GetF10CategoryList returned nil")
	}
	t.Logf("分类数量: %d", len(*cats))
	for i, c := range *cats {
		t.Logf("  [%d] %s (filename=%s)", i+1, c.Name, c.Filename)
	}
}

func TestTdxKLineApi_GetF10CategoryContent(t *testing.T) {
	api := NewTdxKLineApi()

	t.Run("公司概况", func(t *testing.T) {
		section := api.GetF10CategoryContent("600519.SH", "公司概况")
		if section == nil || section.Content == "" {
			t.Fatal("GetF10CategoryContent returned empty")
		}
		t.Logf("分类: %s, 内容长度: %d", section.Name, len(section.Content))
		t.Logf("内容前200字: %s", truncateStr(section.Content, 200))
	})

	t.Run("财务分析", func(t *testing.T) {
		section := api.GetF10CategoryContent("600519.SH", "财务分析")
		if section == nil || section.Content == "" {
			t.Fatal("GetF10CategoryContent returned empty")
		}
		t.Logf("分类: %s, 内容长度: %d", section.Name, len(section.Content))
	})

	t.Run("不存在的分类", func(t *testing.T) {
		section := api.GetF10CategoryContent("600519.SH", "不存在的分类")
		if section == nil {
			t.Fatal("GetF10CategoryContent should return non-nil even for missing category")
		}
		t.Logf("不存在的分类: name=%s, content长度=%d", section.Name, len(section.Content))
	})
}
