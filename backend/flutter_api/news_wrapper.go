// flutter_api/news_wrapper.go
// 新闻数据包装器 - 在 flutter_api 层处理去重逻辑，避免修改原始 data/market_news_api.go
package flutter_api

import (
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"sort"
)

// NewsWrapper 新闻数据包装器，在查询层做去重处理
type NewsWrapper struct {
	api *data.MarketNewsApi
}

// NewNewsWrapper 创建新闻包装器
func NewNewsWrapper() *NewsWrapper {
	return &NewsWrapper{api: data.NewMarketNewsApi()}
}

// GetNewsList 获取新闻列表（去重版）
func (w *NewsWrapper) GetNewsList(source string, limit int) *[]*models.Telegraph {
	news := w.api.GetNewsList(source, limit)
	return dedupeNews(news)
}

// GetNewsList2 获取新闻列表2（去重版）
func (w *NewsWrapper) GetNewsList2(source string, limit int) *[]*models.Telegraph {
	news := w.api.GetNewsList2(source, limit)
	return dedupeNews(news)
}

// GetDomesticNews 获取国内新闻（财联社+新浪，去重版）
func (w *NewsWrapper) GetDomesticNews(limit int) *[]*models.Telegraph {
	// 使用原项目的查询方式，不在数据库层去重
	// 先查财联社
	clsNews := w.api.GetNewsList("财联社电报", limit)
	// 再查新浪财经
	sinaNews := w.api.GetNewsList("新浪财经", limit)

	// 合并后去重
	allNews := make([]*models.Telegraph, 0, len(*clsNews)+len(*sinaNews))
	allNews = append(allNews, *clsNews...)
	allNews = append(allNews, *sinaNews...)

	// 按时间排序
	sort.Slice(allNews, func(i, j int) bool {
		if allNews[i].DataTime != nil && allNews[j].DataTime != nil {
			return allNews[i].DataTime.After(*allNews[j].DataTime)
		}
		return allNews[i].Time > allNews[j].Time
	})

	// 截取前 limit 条
	if len(allNews) > limit {
		allNews = allNews[:limit]
	}

	return dedupeNews(&allNews)
}

// GetTelegraphList 获取电报列表（去重版）
func (w *NewsWrapper) GetTelegraphList(source string) *[]*models.Telegraph {
	news := w.api.GetTelegraphList(source)
	return dedupeNews(news)
}

// GetTelegraphListWithPaging 分页获取电报列表（去重版）
func (w *NewsWrapper) GetTelegraphListWithPaging(source string, page, pageSize int) *[]*models.Telegraph {
	news := w.api.GetTelegraphListWithPaging(source, page, pageSize)
	return dedupeNews(news)
}

// dedupeNews 去重新闻数据（基于 title+source 去重）
func dedupeNews(news *[]*models.Telegraph) *[]*models.Telegraph {
	if news == nil || len(*news) == 0 {
		return news
	}

	seen := make(map[string]bool)
	result := make([]*models.Telegraph, 0, len(*news))

	for _, item := range *news {
		// 使用 title+source 作为去重 key
		key := dedupeKey(item)
		if key == "" || !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}

	return &result
}

// dedupeKey 生成去重 key
func dedupeKey(item *models.Telegraph) string {
	// 优先用 title，其次用 content
	text := item.Title
	if text == "" {
		text = item.Content
	}

	// 如果 title 和 content 都为空，使用 ID 作为 key，确保每条记录都保留
	if text == "" {
		return fmt.Sprintf("id:%d", item.ID)
	}

	// 去掉空格和标点符号
	key := cleanText(text)

	// 如果清理后仍为空，使用 ID
	if key == "" {
		return fmt.Sprintf("id:%d", item.ID)
	}

	return item.Source + ":" + key
}

// cleanText 清理文本用于去重比较
func cleanText(text string) string {
	// 去掉常见标点和空格
	puncts := " ，。！？、,.!?:：；\"\"\"''（）()[][]【】\t\n\r"
	result := make([]rune, 0, len(text))
	for _, c := range text {
		if !containsRune(puncts, c) {
			result = append(result, c)
		}
	}
	return string(result)
}

// containsRune 检查字符串是否包含指定 rune
func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
