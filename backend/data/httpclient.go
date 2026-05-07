package data

import (
	"net"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

// SharedHTTPClient 全局共享 HTTP 客户端，所有请求复用同一个连接池，避免路由器 NAT 表被撑爆。
var SharedHTTPClient *resty.Client

func init() {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   4,
		MaxConnsPerHost:       10,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	SharedHTTPClient = resty.NewWithClient(httpClient).
		SetRetryCount(0).
		SetTimeout(30 * time.Second)
}
