package main

import (
	"go-stock/backend/db"
	"go-stock/backend/flutter_api"
)

// StartHTTPServer 启动 REST API + WebSocket 服务（非阻塞）
// 在 Wails 主程序中以 goroutine 方式调用
func StartHTTPServer() {
	db.Init("")
	go flutter_api.Start()
}

// Broadcast 向 WebSocket 客户端广播消息（供 Wails app.go 调用）
func Broadcast(event string, data interface{}) {
	flutter_api.Broadcast(event, data)
}
