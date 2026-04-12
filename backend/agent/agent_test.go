package agent

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/duke-git/lancet/v2/fileutil"
)

func TestGetStockAiAgent(t *testing.T) {
	ctx := context.Background()
	db.Init("../../data/stock.db")
	config := data.GetSettingConfig()
	agentInstance := GetStockAiAgent(&ctx, *config.AiConfigs[0], "分析当前市场情绪和热点", "")

	if agentInstance == nil {
		t.Fatal("agent instance is nil")
	}

	t.Logf("Agent mode: %s", agentInstance.Mode)

	switch agentInstance.Mode {
	case AgentModePlanExecute:
		runner := adk.NewRunner(ctx, adk.RunnerConfig{
			Agent: agentInstance.AdkAgent,
		})
		messages := []*schema.Message{
			{Role: schema.System, Content: config.Settings.Prompt + ""},
			{Role: schema.User, Content: "结合以上提供的宏观经济数据/市场指数行情/国内外市场资讯/电报/会议/事件/投资者关注的问题，\n结合宏观经济，事件驱动，政策支持，投资者关注的问题，分析当前市场情绪和热点 找出有潜力/优质的板块/行业/概念/标的/主题，\n多因子深度分析计算上涨或下跌的逻辑和概率，\n最后按风险和投资周期给出具体推荐标的操作建议"},
		}
		iter := runner.Run(ctx, messages)
		md := strings.Builder{}
		for {
			event, ok := iter.Next()
			if !ok {
				break
			}
			if event == nil || event.Err != nil {
				continue
			}
			if event.Output != nil && event.Output.MessageOutput != nil {
				mv := event.Output.MessageOutput
				if mv.Message != nil {
					if mv.Message.ReasoningContent != "" {
						md.WriteString(mv.Message.ReasoningContent)
					}
					if mv.Message.Content != "" {
						md.WriteString(mv.Message.Content)
					}
				}
			}
		}
		logger.SugaredLogger.Info(md.String())

	default:
		md := strings.Builder{}
		ch := NewStockAiAgentApi().Chat("分析一下立讯精密", 2, nil)
		for message := range ch {
			logger.SugaredLogger.Infof("res:%s", message.String())
			md.WriteString(message.String())
		}
		logger.SugaredLogger.Info(md.String())
	}
}

func TestClassifyComplexity(t *testing.T) {
	tests := []struct {
		question string
		expected AgentMode
	}{
		{"今天茅台股价多少", AgentModeReact},
		{"查询一下平安银行的代码", AgentModeReact},
		{"全面分析贵州茅台的投资价值", AgentModePlanExecute},
		{"综合分析当前市场热点和投资机会", AgentModePlanExecute},
		{"帮我查一下今天大盘行情", AgentModeReact},
		{"深度分析新能源汽车产业链投资机会，包括上游锂矿、中游电池、下游整车的竞争格局和投资建议", AgentModePlanExecute},
	}

	for _, tt := range tests {
		result := classifyComplexity(tt.question)
		status := "✓"
		if result != tt.expected {
			status = "✗"
		}
		fmt.Printf("%s question=%q expected=%s got=%s\n", status, tt.question, tt.expected, result)
	}
}

func TestAgent(t *testing.T) {
	db.Init("../../data/stock.db")

	md := strings.Builder{}
	ch := NewStockAiAgentApi().Chat("分析一下立讯精密", 2, nil)
	for message := range ch {
		logger.SugaredLogger.Infof("res:%s", message.String())
		md.WriteString(message.String())
	}
	logger.SugaredLogger.Info(md.String())
	fileutil.WriteStringToFile("../../data/result.md", md.String(), false)
}
