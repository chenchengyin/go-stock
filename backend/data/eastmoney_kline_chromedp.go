package data

import (
	"context"
	"fmt"
	"go-stock/backend/logger"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// 仅拉 Cookie 时仍需冷启动浏览器，但不必等 K 线 JSON 整页渲染，超时可略短于原「整页抓取」
const eastMoneyCookieChromedpMinTimeout = 2 * time.Minute

// EastMoneyCookieCacheTTL Cookie 缓存有效期；过期后下次 K 线请求才会再次 chromedp（K 线 HTTP 仍每次都发真实请求）
const EastMoneyCookieCacheTTL = 12 * time.Minute

const quoteEastMoneyPage = "https://quote.eastmoney.com/"

var (
	eastMoneyCookieMu     sync.Mutex
	eastMoneyCookiePath   string
	eastMoneyCookieHeader string
	eastMoneyCookieExpiry time.Time
)

// InvalidateEastMoneyCookieCache 清空 Cookie 缓存（例如切换浏览器路径或调试时可调用）
func InvalidateEastMoneyCookieCache() {
	eastMoneyCookieMu.Lock()
	defer eastMoneyCookieMu.Unlock()
	eastMoneyCookiePath = ""
	eastMoneyCookieHeader = ""
	eastMoneyCookieExpiry = time.Time{}
}

// EastMoneyCookieHeaderForPush2his 供所有访问 push2his.eastmoney.com 的 HTTP 请求复用，与 K 线共用 chromedp Cookie 缓存。
// browserPath 为空时自动检测系统浏览器（Edge/Chrome/Firefox），检测失败时返回空串。
func EastMoneyCookieHeaderForPush2his(config *SettingConfig) string {
	if config == nil {
		return ""
	}
	browserPath := strings.TrimSpace(config.BrowserPath)
	crawl := time.Duration(config.CrawlTimeOut) * time.Second
	if crawl < 15*time.Second {
		crawl = 30 * time.Second
	}
	cdTimeout := crawl + 90*time.Second
	h, err := FetchEastMoneyCookiesViaChromedp(browserPath, cdTimeout)
	if err != nil {
		logger.SugaredLogger.Warnf("东财 chromedp 获取 cookie 失败，push2his 请求将不带 Cookie: %v", err)
		return ""
	}
	return h
}

// FetchEastMoneyCookiesViaChromedp 带缓存：命中则直接返回已缓存的 Cookie 头，不启动浏览器；
// K 线数据不在此函数内请求，调用方须每次对 push2his 发真实 HTTP（见 fetchKLineJSONBytesByHTTP）。
// 该函数为导出版本，供外部包调用。
func FetchEastMoneyCookiesViaChromedp(browserPath string, timeout time.Duration) (cookieHeader string, err error) {
	return fetchEastMoneyCookiesViaChromedp(browserPath, timeout)
}

// fetchEastMoneyCookiesViaChromedp 带缓存：命中则直接返回已缓存的 Cookie 头，不启动浏览器；
// K 线数据不在此函数内请求，调用方须每次对 push2his 发真实 HTTP（见 fetchKLineJSONBytesByHTTP）。
// browserPath 为空时自动检测系统浏览器（Edge/Chrome/Firefox）
func fetchEastMoneyCookiesViaChromedp(browserPath string, timeout time.Duration) (cookieHeader string, err error) {
	browserPath = strings.TrimSpace(browserPath)
	if browserPath == "" {
		// 自动检测系统浏览器
		browserPath, _ = CheckBrowser()
		if browserPath == "" {
			return "", fmt.Errorf("chromedp: 未配置浏览器路径且未检测到系统浏览器 (Edge/Chrome/Firefox)")
		}
		logger.SugaredLogger.Infof("chromedp: 自动检测到浏览器路径：%s", browserPath)
	}

	now := time.Now()
	eastMoneyCookieMu.Lock()
	if eastMoneyCookiePath == browserPath && now.Before(eastMoneyCookieExpiry) {
		h := eastMoneyCookieHeader
		eastMoneyCookieMu.Unlock()
		logger.SugaredLogger.Debugf("东财 Cookie 使用缓存，至 %s 失效", eastMoneyCookieExpiry.Format(time.RFC3339))
		return h, nil
	}
	eastMoneyCookieMu.Unlock()

	h, err := eastMoneyCookiesViaChromedpOnce(browserPath, timeout)
	if err != nil {
		return "", err
	}

	eastMoneyCookieMu.Lock()
	eastMoneyCookiePath = browserPath
	eastMoneyCookieHeader = h
	eastMoneyCookieExpiry = now.Add(EastMoneyCookieCacheTTL)
	eastMoneyCookieMu.Unlock()

	return h, nil
}

// eastMoneyCookiesViaChromedpOnce 单次 chromedp 拉 Cookie（无缓存）
func eastMoneyCookiesViaChromedpOnce(browserPath string, timeout time.Duration) (cookieHeader string, err error) {
	if timeout < eastMoneyCookieChromedpMinTimeout {
		timeout = eastMoneyCookieChromedpMinTimeout
	}

	parent, cancelParent := context.WithTimeout(context.Background(), timeout)
	defer cancelParent()

	opts := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath(browserPath),
		chromedp.Flag("headless", true),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.Flag("disable-javascript", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-features", "site-per-process,Translate,BlinkGenPropertyTrees"),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("enable-automation", true),
		chromedp.Flag("password-store", "basic"),
		chromedp.Flag("use-mock-keychain", true),
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parent, opts...)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(logger.SugaredLogger.Infof),
		chromedp.WithErrorf(logger.SugaredLogger.Errorf),
	)
	defer cancelCtx()

	var cookies []*network.Cookie
	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(actx context.Context) error {
			return network.Enable().Do(actx)
		}),
		chromedp.Navigate(quoteEastMoneyPage),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(400*time.Millisecond),
		chromedp.ActionFunc(func(actx context.Context) error {
			var inner error
			cookies, inner = network.GetCookies().WithURLs([]string{
				quoteEastMoneyPage,
			}).Do(actx)
			return inner
		}),
	)
	if err != nil {
		return "", err
	}
	if len(cookies) == 0 {
		return "", nil
	}
	var b strings.Builder
	first := true
	for _, c := range cookies {
		if c == nil || c.Name == "" {
			continue
		}
		if !first {
			b.WriteString("; ")
		}
		first = false
		b.WriteString(c.Name)
		b.WriteByte('=')
		b.WriteString(c.Value)
	}
	return b.String(), nil
}
