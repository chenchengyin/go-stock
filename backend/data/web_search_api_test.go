package data

import (
	"testing"
)

func TestWebSearchApi_Search(t *testing.T) {
	api := NewWebSearchApi(30)
	results := api.Search("贵州茅台", 3)
	t.Logf("Search results count: %d", len(results))
	for i, r := range results {
		t.Logf("Result %d: Title=%s, Source=%s, Url=%s", i+1, r.Title, r.Source, r.Url)
	}
	if len(results) == 0 {
		t.Error("Expected search results, got none")
	}
}

func TestWebSearchApi_SearchToMarkdown(t *testing.T) {
	api := NewWebSearchApi(30)
	md := api.SearchToMarkdown("MACD指标参数设置", 3)
	t.Logf("Markdown result:\n%s", md)
	if md == "" || md == "未找到相关搜索结果" {
		t.Error("Expected search results in markdown format")
	}
}

func TestWebSearchApi_Bing(t *testing.T) {
	path := GetSettingConfig().BrowserPath
	if path == "" {
		t.Skip("BrowserPath not configured")
	}

	api := NewWebSearchApi(30)
	results := api.searchBing("A股今日行情", 30)
	t.Logf("Bing results count: %d", len(results))
	for i, r := range results {
		t.Logf("Result %d: %s", i+1, r.Title)
	}
}
