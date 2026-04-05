package agent

import (
	"context"
	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ark"
	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

func GetStockAiAgent(ctx *context.Context, aiConfig data.AIConfig) *react.Agent {
	logger.SugaredLogger.Infof("GetStockAiAgent aiConfig: %v", aiConfig)
	temperature := float32(aiConfig.Temperature)
	var toolableChatModel model.ToolCallingChatModel
	var err error
	if aiConfig.BaseUrl == "https://ark.cn-beijing.volces.com/api/v3" {
		var thinking *ark.Thinking
		if aiConfig.Thinking {
			thinking = &ark.Thinking{
				Type: "enabled",
			}
		}
		toolableChatModel, err = ark.NewChatModel(context.Background(), &ark.ChatModelConfig{
			BaseURL:     aiConfig.BaseUrl,
			Model:       aiConfig.ModelName,
			APIKey:      aiConfig.ApiKey,
			MaxTokens:   &aiConfig.MaxTokens,
			Temperature: &temperature,
			Thinking:    thinking,
		})

	} else {
		extraFields := make(map[string]any)
		if aiConfig.Thinking {
			extraFields["thinking"] = map[string]any{
				"type": "enabled",
			}
		}
		toolableChatModel, err = einoopenai.NewChatModel(*ctx, &einoopenai.ChatModelConfig{
			BaseURL:     aiConfig.BaseUrl,
			Model:       aiConfig.ModelName,
			APIKey:      aiConfig.ApiKey,
			Timeout:     time.Duration(aiConfig.TimeOut) * time.Second,
			MaxTokens:   &aiConfig.MaxTokens,
			Temperature: &temperature,
			ExtraFields: extraFields,
		})
	}

	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return nil
	}

	allTools := getAllTools()

	aiTools := compose.ToolsNodeConfig{
		Tools: allTools,
	}

	agent, err := react.NewAgent(*ctx, &react.AgentConfig{
		ToolCallingModel: toolableChatModel,
		ToolsConfig:      aiTools,
		MaxStep:          len(allTools) + 5,
		MessageModifier: func(ctx context.Context, input []*schema.Message) []*schema.Message {
			return input
		},
		StreamToolCallChecker: func(ctx context.Context, modelOutput *schema.StreamReader[*schema.Message]) (bool, error) {
			hasToolCall := false
			for {
				msg, err := modelOutput.Recv()
				if err != nil {
					break
				}
				if len(msg.ToolCalls) > 0 {
					hasToolCall = true
				}
			}
			return hasToolCall, nil
		},
	})
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return nil
	}
	return agent
}

func getAllTools() []tool.BaseTool {
	var allTools []tool.BaseTool

	//allTools = append(allTools, tools.GetQueryEconomicDataTool())
	//allTools = append(allTools, tools.GetQueryStockPriceInfoTool())
	allTools = append(allTools, tools.GetQueryStockCodeInfoTool())
	//allTools = append(allTools, tools.GetQueryMarketNewsTool())
	//allTools = append(allTools, tools.GetChoiceStockByIndicatorsTool())
	//allTools = append(allTools, tools.GetStockKLineTool())
	//allTools = append(allTools, tools.GetInteractiveAnswerDataTool())
	//allTools = append(allTools, tools.GetFinancialReportTool())
	allTools = append(allTools, tools.GetQueryStockNewsTool())
	allTools = append(allTools, tools.GetIndustryResearchReportTool())
	allTools = append(allTools, tools.GetQueryBKDictTool())

	allTools = append(allTools, tools.GetAllDataTools()...)

	allTools = append(allTools, tools.GetHolidayTools()...)

	return allTools
}
