package main

import (
	"go-stock/httpserver"
)

// StartHTTPServer 启动 REST API + WebSocket 服务（非阻塞）
// 在 Wails 主程序中以 goroutine 方式调用
func StartHTTPServer() {
	httpserver.Start()
}

// Broadcast 向 WebSocket 客户端广播消息（供 Wails app.go 调用）
func Broadcast(event string, data interface{}) {
	httpserver.Broadcast(event, data)
}
