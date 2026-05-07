package data

import (
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

var (
	sharedTransport     *http.Transport
	sharedHTTPClient    *http.Client
	SharedHTTPClient    *resty.Client
	httpConfigMutex     sync.RWMutex
	currentProxyEnabled bool
	currentProxyURL     string
)

func init() {
	sharedTransport = &http.Transport{
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
		Proxy:                 nil,
	}

	sharedHTTPClient = &http.Client{
		Transport: sharedTransport,
		Timeout:   30 * time.Second,
	}

	SharedHTTPClient = resty.NewWithClient(sharedHTTPClient).
		SetRetryCount(0).
		SetTimeout(30 * time.Second)
}

func UpdateHTTPClientProxy(proxyURL string) {
	httpConfigMutex.Lock()
	defer httpConfigMutex.Unlock()

	if proxyURL == "" || proxyURL == currentProxyURL {
		return
	}

	sharedTransport.Proxy = http.ProxyURL(parseProxyURL(proxyURL))
	currentProxyURL = proxyURL
	currentProxyEnabled = true
}

func DisableHTTPClientProxy() {
	httpConfigMutex.Lock()
	defer httpConfigMutex.Unlock()

	sharedTransport.Proxy = nil
	currentProxyEnabled = false
	currentProxyURL = ""
}

func UpdateHTTPClientTimeout(timeout time.Duration) {
	sharedHTTPClient.Timeout = timeout
	SharedHTTPClient.SetTimeout(timeout)
}

func parseProxyURL(proxyURL string) *url.URL {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil
	}
	return u
}

func ConfigureFromSettings(config *SettingConfig) {
	if config == nil {
		return
	}

	if config.HttpProxyEnabled && config.HttpProxy != "" {
		UpdateHTTPClientProxy(config.HttpProxy)
	} else {
		DisableHTTPClientProxy()
	}

	if config.CrawlTimeOut > 0 {
		UpdateHTTPClientTimeout(time.Duration(config.CrawlTimeOut) * time.Second)
	} else {
		UpdateHTTPClientTimeout(30 * time.Second)
	}
}
