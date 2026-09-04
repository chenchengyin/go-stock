package main

import (
	"go-stock/backend/db"
	"go-stock/backend/flutter_api"
)

// StartHTTPServer 在同一进程中启动 Flutter 用户 REST/WebSocket 监听器和
// 独立的管理 REST/SPA 监听器（均非阻塞）。用户服务默认监听 :8080，管理服务
// 默认监听 :18080，可通过 GO_STOCK_ADMIN_ADDR 覆盖；admin-web 开发时的 Vite
// 来源可通过 GO_STOCK_ADMIN_DEV_ORIGINS（逗号分隔，默认文档源为
// http://localhost:5174）配置。
// 在 Wails 主程序中以 goroutine 方式调用。
func StartHTTPServer() {
	db.Init("")
	go flutter_api.Start()
}

// Broadcast 向 WebSocket 客户端广播消息（供 Wails app.go 调用）
func Broadcast(event string, data interface{}) {
	flutter_api.Broadcast(event, data)
}
