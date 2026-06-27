package main

import (
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"sync"
)

// 批量对已有新闻补充 AI 分析意见
// 使用方式: go run ./cmd/backfill_ai
func main() {
	db.Init("")
	data.InitAnalyzeSentiment()

	// 读取所有重要新闻（isRed = true）
	var news []models.Telegraph
	db.Dao.Model(&models.Telegraph{}).
		Where("is_red = 1 AND (ai_opinion IS NULL OR ai_opinion = '')").
		Order("data_time DESC").
		Limit(200).
		Find(&news)

	logger.SugaredLogger.Infof("待处理重要新闻: %d 条", len(news))

	if len(news) == 0 {
		logger.SugaredLogger.Info("没有需要处理的新闻")
		return
	}

	// 并发控制（一次 3 个并发以免限流）
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	fail := 0

	for _, n := range news {
		wg.Add(1)
		sem <- struct{}{}
		go func(item models.Telegraph) {
			defer wg.Done()
			defer func() { <-sem }()

			content := item.Content
			if item.Url != "" {
				content += "\n原文链接: " + item.Url
			}
			opinion := data.GetAIAnalysisForNews(content)
			if opinion != "" {
				db.Dao.Model(&models.Telegraph{}).Where("id = ?", item.ID).Update("ai_opinion", opinion)
				mu.Lock()
				success++
				mu.Unlock()
				logger.SugaredLogger.Infof("[OK] ID=%d 写入成功", item.ID)
			} else {
				mu.Lock()
				fail++
				mu.Unlock()
				logger.SugaredLogger.Warnf("[FAIL] ID=%d AI分析返回为空", item.ID)
			}
		}(n)
	}

	wg.Wait()
	logger.SugaredLogger.Infof("处理完成: 成功=%d, 失败=%d", success, fail)
}
