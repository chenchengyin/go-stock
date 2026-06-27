package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// AIOpinionPromptDeepSeek 调用 DeepSeek 分析新闻原文的 prompt
const aiOpinionPrompt = `你是一位资深股市分析师。请分析以下新闻资讯，输出你的专业意见。

要求：
1. 用2-3句话总结核心要点
2. 分析该消息对相关板块/个股的潜在影响（利好/利空/中性）
3. 给出投资者参考建议
4. 控制在150字以内

请以JSON格式输出，字段为：
{
  "summary": "核心要点总结",
  "impact": "对板块/个股的影响分析",
  "suggestion": "投资者建议"
}

新闻内容：
`

// GetAIAnalysisForNews 调用 DeepSeek 对新闻内容进行分析，返回 AI 意见文本
func GetAIAnalysisForNews(content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	settingConfig := GetSettingConfig()
	if !settingConfig.OpenAiEnable {
		return ""
	}

	var aiConfig *AIConfig
	for _, cfg := range settingConfig.AiConfigs {
		if cfg.ApiKey != "" {
			aiConfig = cfg
			break
		}
	}
	if aiConfig == nil || aiConfig.ApiKey == "" {
		return ""
	}

	timeout := 60
	if aiConfig.TimeOut > 0 {
		timeout = aiConfig.TimeOut
	}
	if timeout > 120 {
		timeout = 120
	}

	var client *resty.Client
	if aiConfig.HttpProxyEnabled && aiConfig.HttpProxy != "" {
		client = createHTTPClientWithProxy(aiConfig.HttpProxy, timeout)
	} else {
		client = CreateHTTPClientWithTimeout(time.Duration(timeout) * time.Second)
	}

	baseURL, chatPath := openAIChatEndpoint(aiConfig.BaseUrl)
	client.SetBaseURL(baseURL)

	userMsg := aiOpinionPrompt + content
	bodyMap := map[string]interface{}{
		"model": aiConfig.ModelName,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一位资深股票市场分析师，擅长分析新闻对股市的影响。"},
			{"role": "user", "content": userMsg},
		},
		"temperature": 0.7,
		"max_tokens":  500,
		"stream":      false,
	}

	reqBody, _ := json.Marshal(bodyMap)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+aiConfig.ApiKey).
		SetBody(reqBody).
		Post(chatPath)

	if err != nil {
		logger.SugaredLogger.Errorf("AI分析请求失败: %v", err)
		return ""
	}

	if resp.StatusCode() != 200 {
		logger.SugaredLogger.Errorf("AI分析响应异常: status=%d, body=%s", resp.StatusCode(), string(resp.Body()[:min(len(resp.Body()), 500)]))
		return ""
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		logger.SugaredLogger.Errorf("AI分析解析响应失败: %v", err)
		return ""
	}

	if len(result.Choices) == 0 {
		return ""
	}

	opinion := strings.TrimSpace(result.Choices[0].Message.Content)
	if opinion == "" {
		return ""
	}

	// 尝试提取 JSON 并格式化为易读文本
	var parsed struct {
		Summary    string `json:"summary"`
		Impact     string `json:"impact"`
		Suggestion string `json:"suggestion"`
	}
	if err := json.Unmarshal([]byte(opinion), &parsed); err == nil && parsed.Summary != "" {
		opinion = fmt.Sprintf("【核心要点】%s\n\n【影响分析】%s\n\n【投资建议】%s",
			parsed.Summary, parsed.Impact, parsed.Suggestion)
	}

	return opinion
}
