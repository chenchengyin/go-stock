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
	settingConfig := data.GetSettingConfig()
	aiConfig, ok := lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
		return uint(aiConfigId) == item.ID
	})
	if !ok {
		return nil
	}
	aiConfig.Thinking = thinkingMode
	return &StockAiAgent{
		Agent:     GetStockAiAgent(ctx, *aiConfig),
		sessionID: aiConfig.SessionId,
	}
}

func (receiver StockAiAgent) Chat(question string, aiConfigId int, sysPromptId *int) chan *schema.Message {
	return receiver.ChatWithContext(context.Background(), question, aiConfigId, sysPromptId, true, 20, false)
}

func (receiver StockAiAgent) ChatWithContext(ctx context.Context, question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool) chan *schema.Message {
	ch := make(chan *schema.Message, 1024)
	stockAiAgent := receiver.newStockAiAgent(&ctx, aiConfigId, thinkingMode)

	var memoryService *ChatMemoryService
	var historyContext string
	if memoryMode && stockAiAgent.sessionID != "" {
		memoryService = NewChatMemoryService(stockAiAgent.sessionID, memoryCount)
		if err := memoryService.AddUserMessage(question); err != nil {
			logger.SugaredLogger.Errorf("failed to save user message: %v", err)
		}
		historyContext, _ = memoryService.GetFormattedHistory()
	}

	sysPrompt := ""
	if sysPromptId == nil || *sysPromptId == 0 {
		sysPrompt = `你现在扮演一位拥有20年实战经验的顶级股票投资大师，精通价值投资、趋势交易、量化分析等多种策略。你擅长结合宏观经济、行业周期和企业基本面进行全方位、精准的多维分析，尤其对A股、港股、美股市场有深刻理解，始终秉持"风险控制第一"的原则，善于用通俗易懂的方式传授投资智慧。`
	} else {
		sysPrompt = data.NewPromptTemplateApi().GetPromptTemplateByID(*sysPromptId)
	}

	userContent := question
	if historyContext != "" {
		userContent = historyContext + "\n\n【当前问题】\n" + question
	}
	msgFutureOpt, msgFuture := react.WithMessageFuture()
	opts := agent.GetComposeOptions(msgFutureOpt)
	//opts = append(opts, compose.WithCallbacks(&tool_logger.LoggerCallback{MessageChanel: ch}))

	agentOption := []agent.AgentOption{
		agent.WithComposeOptions(opts...),
	}
	// Process MessageFuture in a separate goroutine to print intermediate results
	var wg sync.WaitGroup
	wg.Go(func() {
		processMessageFuture(msgFuture, ch)
	})

	go func() {
		defer close(ch)
		sr, err := stockAiAgent.Stream(ctx, []*schema.Message{
			{
				Role:    schema.System,
				Content: sysPrompt,
			},
			{
				Role:    schema.User,
				Content: userContent,
			},
		}, agentOption...)
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
		defer sr.Close()

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
			//logger.SugaredLogger.Infof("stream recv msg: %s", msg.String())
			if msg.Content != "" {
				fullResponse.WriteString(msg.Content)
			}
			//	ch <- msg
		}

		if fullResponse.Len() != 0 && memoryService != nil {
			if err := memoryService.AddAssistantMessage(fullResponse.String()); err != nil {
				logger.SugaredLogger.Errorf("failed to save assistant message: %v", err)
			}
		}
	}()
	return ch
}

func processMessageFuture(msgFuture react.MessageFuture, ch chan *schema.Message) {
	iter := msgFuture.GetMessageStreams()
	for {
		sr, ok, err := iter.Next()
		if err != nil {
			logger.SugaredLogger.Errorf("failed to get next message stream: %v", err)
			return
		}
		if !ok {
			break
		}

		// Accumulate streaming chunks into complete content
		var reasoningBuilder strings.Builder
		var contentBuilder strings.Builder
		toolCallsMap := make(map[int]*strings.Builder)
		toolCallNames := make(map[int]string)
		var toolResult *struct {
			name    string
			content string
		}

		// Read all chunks from the stream
		for {
			msg, err := sr.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				logger.SugaredLogger.Errorf("failed to recv from message stream: %v", err)
				return
			}

			// Accumulate reasoning content (thinking process)
			if msg.ReasoningContent != "" {
				reasoningBuilder.WriteString(msg.ReasoningContent)
				ch <- &schema.Message{
					Role: schema.Assistant,
					Content: strutil.ReplaceWithMap(msg.ReasoningContent, map[string]string{
						"# ":     "\r\n# ",
						"## ":    "\r\n## ",
						"### ":   "\r\n### ",
						"#### ":  "\r\n#### ",
						"##### ": "\r\n##### ",
						"```":    "\r\n```",
					}),
				}
			}

			// Accumulate tool calls (function name and arguments come in separate chunks)
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

			// Capture tool result
			if msg.Role == schema.Tool && msg.Content != "" {
				toolResult = &struct {
					name    string
					content string
				}{
					name:    msg.ToolName,
					content: msg.Content,
				}
			}

			// Accumulate assistant content (final answer)
			if msg.Role == schema.Assistant && msg.Content != "" {
				contentBuilder.WriteString(msg.Content)
				ch <- &schema.Message{
					Role: schema.Assistant,
					Content: strutil.ReplaceWithMap(msg.Content, map[string]string{
						"# ":     "\r\n# ",
						"## ":    "\r\n## ",
						"### ":   "\r\n### ",
						"#### ":  "\r\n#### ",
						"##### ": "\r\n##### ",
						"```":    "\r\n```",
					}),
				}
			}
		}

		// Print accumulated content
		if reasoningBuilder.Len() > 0 {
			fmt.Printf("\n[Reasoning]\n%s\n", reasoningBuilder.String())
			ch <- &schema.Message{
				Role:    schema.Assistant,
				Content: "\r\n",
			}
		}

		if len(toolCallsMap) > 0 {
			for idx := 0; idx < len(toolCallsMap); idx++ {
				if builder, exists := toolCallsMap[idx]; exists {
					name := toolCallNames[idx]
					fmt.Printf("\n[ToolCall] %s(%s)\n", name, builder.String())
					ch <- &schema.Message{
						Role:    schema.Assistant,
						Content: fmt.Sprintf("\r\n```\r\n开始调用工具： %s(%s)\r\n```\r\n", name, builder.String()),
					}
				}
			}
		}

		if toolResult != nil {
			fmt.Printf("\n[ToolResult] %s:\n%s\n", toolResult.name, truncateString(toolResult.content, 300))
			ch <- &schema.Message{
				Role:    schema.Assistant,
				Content: "\r\n",
			}
		}

		if contentBuilder.Len() > 0 && len(toolCallsMap) == 0 {
			fmt.Printf("\n[FinalAnswer]\n%s\n", contentBuilder.String())
			ch <- &schema.Message{
				Role:    schema.Assistant,
				Content: "agent-DONE",
			}
		}
	}
}
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
