package data

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go-stock/backend/logger"

	"github.com/chromedp/chromedp"
)

type WebSearchResult struct {
	Title       string `json:"title"`
	Url         string `json:"url"`
	Snippet     string `json:"snippet"`
	Source      string `json:"source"`
	PublishTime string `json:"publish_time"`
}

type WebSearchApi struct {
	timeout int
}

func NewWebSearchApi(timeout int) *WebSearchApi {
	if timeout <= 0 {
		timeout = 30
	}
	return &WebSearchApi{timeout: timeout}
}

func (s *WebSearchApi) Search(query string, maxResults int) []WebSearchResult {
	if maxResults <= 0 {
		maxResults = 10
	}
	results := s.searchBing(query, maxResults)
	if len(results) < maxResults {
		baiduResults := s.searchBaidu(query, maxResults-len(results))
		results = append(results, baiduResults...)
	}
	return results
}

func (s *WebSearchApi) searchBing(query string, maxResults int) []WebSearchResult {
	var results []WebSearchResult
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.timeout)*time.Second)
	defer cancel()

	path := GetSettingConfig().BrowserPath
	if path == "" {
		logger.SugaredLogger.Warnf("BrowserPath not configured, skipping Bing search")
		return results
	}

	searchUrl := fmt.Sprintf("https://www.bing.com/search?q=%s", strings.ReplaceAll(query, " ", "+"))

	pctx, _ := chromedp.NewExecAllocator(ctx,
		chromedp.ExecPath(path),
		chromedp.Flag("headless", true),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.Flag("disable-javascript", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
	)

	cctx, ccancel := chromedp.NewContext(pctx, chromedp.WithLogf(logger.SugaredLogger.Infof))
	defer ccancel()

	var htmlContent string
	err := chromedp.Run(cctx,
		chromedp.Navigate(searchUrl),
		chromedp.WaitVisible("#b_results", chromedp.ByID),
		chromedp.Sleep(2*time.Second),
		chromedp.InnerHTML("#b_results", &htmlContent),
	)
	if err != nil {
		logger.SugaredLogger.Errorf("Bing search failed: %v", err)
		return results
	}

	liPattern := regexp.MustCompile(`<li[^>]*class="b_algo"[^>]*>([\s\S]*?)</li>`)
	titlePattern := regexp.MustCompile(`<h2[^>]*><a[^>]*href="([^"]*)"[^>]*>([^<]*)</a></h2>`)
	snippetPattern := regexp.MustCompile(`<p[^>]*class="b_paractl"[^>]*>([\s\S]*?)</p>`)
	timePattern := regexp.MustCompile(`<span[^>]*class="b_attribution"[^>]*>([\s\S]*?)</span>`)

	liMatches := liPattern.FindAllStringSubmatch(htmlContent, -1)
	for i, liMatch := range liMatches {
		if i >= maxResults {
			break
		}
		content := liMatch[1]

		titleMatches := titlePattern.FindStringSubmatch(content)
		snippetMatches := snippetPattern.FindStringSubmatch(content)
		timeMatches := timePattern.FindStringSubmatch(content)

		if len(titleMatches) >= 3 {
			result := WebSearchResult{
				Title:  cleanHtml(titleMatches[2]),
				Url:    titleMatches[1],
				Source: "Bing",
			}
			if len(snippetMatches) > 1 {
				result.Snippet = cleanHtml(snippetMatches[1])
			}
			if len(timeMatches) > 1 {
				result.PublishTime = cleanHtml(timeMatches[1])
			}
			results = append(results, result)
		}
	}

	return results
}

func (s *WebSearchApi) searchBaidu(query string, maxResults int) []WebSearchResult {
	var results []WebSearchResult
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.timeout)*time.Second)
	defer cancel()

	path := GetSettingConfig().BrowserPath
	if path == "" {
		logger.SugaredLogger.Warnf("BrowserPath not configured, skipping Baidu search")
		return results
	}

	searchUrl := fmt.Sprintf("https://www.baidu.com/s?wd=%s", strings.ReplaceAll(query, " ", "+"))

	pctx, _ := chromedp.NewExecAllocator(ctx,
		chromedp.ExecPath(path),
		chromedp.Flag("headless", true),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.Flag("disable-javascript", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
	)

	cctx, ccancel := chromedp.NewContext(pctx, chromedp.WithLogf(logger.SugaredLogger.Infof))
	defer ccancel()

	var htmlContent string
	err := chromedp.Run(cctx,
		chromedp.Navigate(searchUrl),
		chromedp.WaitVisible("#content_left", chromedp.ByID),
		chromedp.Sleep(2*time.Second),
		chromedp.InnerHTML("#content_left", &htmlContent),
	)
	if err != nil {
		logger.SugaredLogger.Errorf("Baidu search failed: %v", err)
		return results
	}

	resultPattern := regexp.MustCompile(`<div[^>]*class="result[^"]*"[^>]*>[\s\S]*?<h3[^>]*class="t"[^>]*>[\s\S]*?<a[^>]*href="([^"]*)"[^>]*>([\s\S]*?)</a>[\s\S]*?</h3>[\s\S]*?<span[^>]*class="c-span-last"[^>]*>([\s\S]*?)</span>`)
	titleClean := regexp.MustCompile(`<[^>]+>`)
	snippetPattern := regexp.MustCompile(`<span[^>]*class="c-span-last"[^>]*>([\s\S]*?)</span>`)

	matches := resultPattern.FindAllStringSubmatch(htmlContent, -1)
	for i, match := range matches {
		if i >= maxResults {
			break
		}
		result := WebSearchResult{
			Url:    match[1],
			Title:  titleClean.ReplaceAllString(match[2], ""),
			Source: "Baidu",
		}

		snippetMatches := snippetPattern.FindStringSubmatch(match[3])
		if len(snippetMatches) > 1 {
			result.Snippet = titleClean.ReplaceAllString(snippetMatches[1], "")
		}
		results = append(results, result)
	}

	return results
}

func (s *WebSearchApi) SearchToJson(query string, maxResults int) string {
	results := s.Search(query, maxResults)
	if len(results) == 0 {
		return "未找到相关搜索结果"
	}
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Sprintf("搜索结果序列化失败: %v", err)
	}
	return string(jsonData)
}

func (s *WebSearchApi) SearchToMarkdown(query string, maxResults int) string {
	results := s.Search(query, maxResults)
	if len(results) == 0 {
		return "未找到相关搜索结果"
	}

	var md strings.Builder
	md.WriteString(fmt.Sprintf("## 搜索结果: %s\n\n", query))
	md.WriteString(fmt.Sprintf("共找到 %d 条结果\n\n", len(results)))

	for i, r := range results {
		md.WriteString(fmt.Sprintf("### %d. %s\n", i+1, r.Title))
		md.WriteString(fmt.Sprintf("- 来源: %s\n", r.Source))
		if r.PublishTime != "" {
			md.WriteString(fmt.Sprintf("- 时间: %s\n", r.PublishTime))
		}
		md.WriteString(fmt.Sprintf("- 链接: %s\n", r.Url))
		if r.Snippet != "" {
			md.WriteString(fmt.Sprintf("- 摘要: %s\n", r.Snippet))
		}
		md.WriteString("\n")
	}

	return md.String()
}

func cleanHtml(html string) string {
	clean := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(html, "")
	clean = strings.ReplaceAll(clean, "&nbsp;", " ")
	clean = strings.ReplaceAll(clean, "&amp;", "&")
	clean = strings.ReplaceAll(clean, "&lt;", "<")
	clean = strings.ReplaceAll(clean, "&gt;", ">")
	clean = strings.ReplaceAll(clean, "&quot;", `"`)
	clean = strings.ReplaceAll(clean, "&#39;", `'`)
	clean = strings.TrimSpace(clean)
	return clean
}
