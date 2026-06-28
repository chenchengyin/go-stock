// Command server 为 Flutter 前端提供 REST API 的独立 HTTP 服务（不依赖 Wails）。
package main

import (
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/flutter_api"
)

func main() {
	db.Init("")
	data.InitAnalyzeSentiment()

	// 自动迁移（Flutter 端需要的表）
	flutter_api.AutoMigrate()

	flutter_api.Start()
}
