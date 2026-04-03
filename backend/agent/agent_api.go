package agent

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"io"
	"strings"
	"sync"

	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/samber/lo"
)

type StockAiAgent struct {
	*react.Agent
	sessionID string
}

func NewStockAiAgentApi() *StockAiAgent {
	return &StockAiAgent{}
}

func (receiver StockAiAgent) newStockAiAgent(ctx *context.Context, aiConfigId int, thinkingMode bool) *StockAiAgent {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in newStockAiAgent: %v", r)
		}
	}()

	settingConfig := data.GetSettingConfig()
	if settingConfig == nil {
		logger.SugaredLogger.Errorf("settingConfig is nil")
		return nil
	}

	aiConfig, ok := lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
		return uint(aiConfigId) == item.ID
	})
	if !ok {
		logger.SugaredLogger.Errorf("ai config not found for id: %d", aiConfigId)
		return nil
	}
	if aiConfig == nil {
		logger.SugaredLogger.Errorf("aiConfig is nil for id: %d", aiConfigId)
		return nil
	}

	aiConfig.Thinking = thinkingMode
	sessionID := aiConfig.SessionId
	if sessionID == "" {
		sessionID = fmt.Sprintf("ai-config-%d", aiConfig.ID)
	}

	agentInstance := GetStockAiAgent(ctx, *aiConfig)
	if agentInstance == nil {
		logger.SugaredLogger.Errorf("failed to create agent for config id: %d", aiConfigId)
		return nil
	}

	return &StockAiAgent{
		Agent:     agentInstance,
		sessionID: sessionID,
	}
}

func (receiver StockAiAgent) Chat(question string, aiConfigId int, sysPromptId *int) chan *schema.Message {
	return receiver.ChatWithContext(context.Background(), question, aiConfigId, sysPromptId, true, 20, false)
}

func (receiver StockAiAgent) ChatWithContext(ctx context.Context, question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool) chan *schema.Message {
	ch := make(chan *schema.Message, 1024)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("panic in ChatWithContext: %v", r)
				ch <- &schema.Message{
					Role:    schema.Assistant,
					Content: fmt.Sprintf("❌ 内部错误: %v", r),
				}
				close(ch)
			}
		}()

		stockAiAgent := receiver.newStockAiAgent(&ctx, aiConfigId, thinkingMode)
		if stockAiAgent == nil {
			logger.SugaredLogger.Errorf("stockAiAgent is nil")
			ch <- &schema.Message{
				Role:    schema.Assistant,
				Content: "❌ AI 配置不存在或无效，请检查 AI 配置",
			}
			close(ch)
			return
		}

		var memoryService *ChatMemoryService
		var historyMessages []*schema.Message
		if memoryMode && stockAiAgent.sessionID != "" {
			memoryService = NewChatMemoryService(stockAiAgent.sessionID, memoryCount)
			var err error
			historyMessages, err = memoryService.GetHistoryMessages()
			if err != nil {
				logger.SugaredLogger.Errorf("failed to get history messages: %v", err)
				historyMessages = nil
			}
		}

		sysPrompt := ""
		if sysPromptId == nil || *sysPromptId == 0 {
			sysPrompt = `你现在扮演一位拥有20年实战经验的顶级股票投资大师，精通价值投资、趋势交易、量化分析等多种策略。你擅长结合宏观经济、行业周期和企业基本面进行全方位、精准的多维分析，尤其对A股、港股、美股市场有深刻理解，始终秉持"风险控制第一"的原则，善于用通俗易懂的方式传授投资智慧。`
		} else {
			sysPrompt = data.NewPromptTemplateApi().GetPromptTemplateByID(*sysPromptId)
		}

		var messages []*schema.Message
		messages = append(messages, &schema.Message{
			Role:    schema.System,
			Content: sysPrompt,
		})
		messages = append(messages, historyMessages...)
		messages = append(messages, &schema.Message{
			Role:    schema.User,
			Content: question,
		})

		if memoryService != nil {
			if err := memoryService.AddUserMessage(question); err != nil {
				logger.SugaredLogger.Errorf("failed to save user message: %v", err)
			}
		}

		msgFutureOpt, msgFuture := react.WithMessageFuture()
		opts := agent.GetComposeOptions(msgFutureOpt)

		agentOption := []agent.AgentOption{
			agent.WithComposeOptions(opts...),
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Errorf("panic in processMessageFuture: %v", r)
				}
				wg.Done()
			}()
			processMessageFuture(msgFuture, ch)
		}()

		func() {
			defer close(ch)

			if stockAiAgent.Agent == nil {
				logger.SugaredLogger.Errorf("stockAiAgent.Agent is nil")
				ch <- &schema.Message{
					Role:    schema.Assistant,
					Content: "❌ Agent 实例无效",
				}
				return
			}

			sr, err := stockAiAgent.Stream(ctx, messages, agentOption...)
			if err != nil {
				logger.SugaredLogger.Errorf("stream error: %v", err)
				errMsg := fmt.Sprintf("❌ Agent 调用失败：%v", err)
				if strings.Contains(err.Error(), "reasoning_content") || strings.Contains(err.Error(), "thinking is enabled") {
					errMsg += "\n\n**可能原因**：当前模型开启了 thinking/reasoning 模式，但该模式与 Agent 工具调用不兼容。\n\n**解决方案**：请在 AI 配置中关闭 thinking 模式，或切换到支持工具调用的模型（如 deepseek-chat、gpt-4o 等）。"
				}
				ch <- &schema.Message{
					Role:    schema.Assistant,
					Content: errMsg,
				}
				return
			}
			if sr == nil {
				logger.SugaredLogger.Errorf("stream result is nil")
				ch <- &schema.Message{
					Role:    schema.Assistant,
					Content: "❌ 流式响应无效",
				}
				return
			}
			defer func() {
				sr.Close()
			}()

			var fullResponse strings.Builder
			for {
				msg, err := sr.Recv()
				if err != nil {
					if errors.Is(err, io.EOF) {
						logger.SugaredLogger.Infof("stream finished with EOF")
						break
					}
					logger.SugaredLogger.Errorf("failed to recv: %v", err)
					ch <- &schema.Message{
						Role:    schema.Assistant,
						Content: fmt.Sprintf("❌ 接收消息失败：%v", err),
					}
					break
				}
				if msg != nil && msg.Content != "" {
					fullResponse.WriteString(msg.Content)
				}
			}

			if fullResponse.Len() != 0 && memoryService != nil {
				if err := memoryService.AddAssistantMessage(fullResponse.String()); err != nil {
					logger.SugaredLogger.Errorf("failed to save assistant message: %v", err)
				}
			}
		}()

		wg.Wait()
	}()

	return ch
}

func safeSend(ch chan *schema.Message, msg *schema.Message) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic when sending to channel: %v", r)
		}
	}()
	select {
	case ch <- msg:
	default:
		logger.SugaredLogger.Warnf("channel full, message dropped")
	}
}

func processMessageFuture(msgFuture react.MessageFuture, ch chan *schema.Message) {
	if msgFuture == nil || ch == nil {
		logger.SugaredLogger.Errorf("msgFuture or ch is nil")
		return
	}

	iter := msgFuture.GetMessageStreams()
	if iter == nil {
		logger.SugaredLogger.Errorf("message stream iterator is nil")
		return
	}

	for {
		sr, ok, err := iter.Next()
		if err != nil {
			logger.SugaredLogger.Errorf("failed to get next message stream: %v", err)
			return
		}
		if !ok {
			break
		}
		if sr == nil {
			continue
		}

		var reasoningBuilder strings.Builder
		var contentBuilder strings.Builder
		toolCallsMap := make(map[int]*strings.Builder)
		toolCallNames := make(map[int]string)
		var toolResult *struct {
			name    string
			content string
		}

		for {
			msg, err := sr.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				logger.SugaredLogger.Errorf("failed to recv from message stream: %v", err)
				return
			}
			if msg == nil {
				continue
			}

			if msg.ReasoningContent != "" {
				reasoningBuilder.WriteString(msg.ReasoningContent)
				safeSend(ch, &schema.Message{
					Role: schema.Assistant,
					Content: strutil.ReplaceWithMap(msg.ReasoningContent, map[string]string{
						"# ":     "\r\n# ",
						"## ":    "\r\n## ",
						"### ":   "\r\n### ",
						"#### ":  "\r\n#### ",
						"##### ": "\r\n##### ",
						"```":    "\r\n```",
					}),
				})
			}

			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					idx := 0
					if tc.Index != nil {
						idx = *tc.Index
					}
					if _, exists := toolCallsMap[idx]; !exists {
						toolCallsMap[idx] = &strings.Builder{}
					}
					if tc.Function.Name != "" {
						toolCallNames[idx] = tc.Function.Name
					}
					toolCallsMap[idx].WriteString(tc.Function.Arguments)
				}
			}

			if msg.Role == schema.Tool && msg.Content != "" {
				toolResult = &struct {
					name    string
					content string
				}{
					name:    msg.ToolName,
					content: msg.Content,
				}
			}

			if msg.Role == schema.Assistant && msg.Content != "" {
				contentBuilder.WriteString(msg.Content)
				safeSend(ch, &schema.Message{
					Role: schema.Assistant,
					Content: strutil.ReplaceWithMap(msg.Content, map[string]string{
						"# ":     "\r\n# ",
						"## ":    "\r\n## ",
						"### ":   "\r\n### ",
						"#### ":  "\r\n#### ",
						"##### ": "\r\n##### ",
						"```":    "\r\n```",
					}),
				})
			}
		}

		if reasoningBuilder.Len() > 0 {
			fmt.Printf("\n[Reasoning]\n%s\n", reasoningBuilder.String())
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: "\r\n",
			})
		}

		if len(toolCallsMap) > 0 {
			for idx := 0; idx < len(toolCallsMap); idx++ {
				if builder, exists := toolCallsMap[idx]; exists {
					name := toolCallNames[idx]
					fmt.Printf("\n[ToolCall] %s(%s)\n", name, builder.String())
					safeSend(ch, &schema.Message{
						Role:    schema.Assistant,
						Content: fmt.Sprintf("\r\n```\r\n开始调用工具： %s(%s)\r\n```\r\n", name, builder.String()),
					})
				}
			}
		}

		if toolResult != nil {
			fmt.Printf("\n[ToolResult] %s:\n%s\n", toolResult.name, truncateString(toolResult.content, 300))
			safeSend(ch, &schema.Message{
				Role:    schema.Assistant,
				Content: "\r\n",
			})
		}

		if contentBuilder.Len() > 0 && len(toolCallsMap) == 0 {
			fmt.Printf("\n[FinalAnswer]\n%s\n", contentBuilder.String())
			//safeSend(ch, &schema.Message{
			//	Role:    schema.Assistant,
			//	Content: "agent-DONE",
			//})
		}
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
