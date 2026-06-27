// Command server 为 Flutter 前端提供 REST API 的独立 HTTP 服务（不依赖 Wails）。
package main

import (
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/httpserver"
)

func main() {
	db.Init("")
	data.InitAnalyzeSentiment()

	// 自动迁移（Flutter 端需要的表）
	db.Dao.AutoMigrate(&models.Telegraph{})
	db.Dao.AutoMigrate(&models.TelegraphTags{})
	db.Dao.AutoMigrate(&models.Tags{})
	db.Dao.AutoMigrate(&data.StrategyUser{})
	db.Dao.AutoMigrate(&data.StrategyPost{})
	db.Dao.AutoMigrate(&data.StrategyComment{})
	db.Dao.AutoMigrate(&data.StrategyLike{})
	db.Dao.AutoMigrate(&data.StrategyCheckIn{})
	db.Dao.AutoMigrate(&data.StrategyPointsLog{})

	httpserver.Start()
}
