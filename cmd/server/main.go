// Command server 为 Flutter 前端提供 REST API 的独立 HTTP 服务（不依赖 Wails）。
package main

import (
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/httpserver"
)

func main() {
	db.Init("")
	data.InitAnalyzeSentiment()
	httpserver.Start()
}
