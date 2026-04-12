package agent

import (
	"context"
	"fmt"
	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"runtime/debug"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ark"
	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type AgentMode string

const (
	AgentModeReact       AgentMode = "react"
	AgentModePlanExecute AgentMode = "plan_execute"
)

type AgentInstance struct {
	Mode       AgentMode
	ReactAgent *react.Agent
	AdkAgent   adk.ResumableAgent
}

func classifyComplexity(question string) AgentMode {
	lowerQ := strings.ToLower(question)

	simplePatterns := []string{
		"今天", "当前", "最新", "现在", "实时",
		"查询", "查一下", "帮我查", "告诉我",
		"什么是", "是什么", "多少", "几号",
		"是否", "有没有", "能不能",
		"开盘价", "收盘价", "最高价", "最低价",
		"停牌", "复牌", "上市",
		"代码", "名称", "简称",
	}

	for _, p := range simplePatterns {
		if strings.Contains(lowerQ, p) {
			wordCount := len([]rune(question))
			if wordCount < 30 {
				return AgentModeReact
			}
		}
	}

	complexPatterns := []string{
		"全面分析", "综合分析", "深度分析", "详细分析",
		"多维度", "全方位", "系统分析",
		"投资建议", "操作建议", "买卖建议",
		"对比分析", "比较分析", "横向对比",
		"行业分析", "赛道分析", "产业链",
		"投资组合", "资产配置", "仓位",
		"风险评估", "风险分析",
		"研究报告", "投资报告",
		"宏观", "政策", "周期",
	}

	for _, p := range complexPatterns {
		if strings.Contains(lowerQ, p) {
			return AgentModePlanExecute
		}
	}

	toolGroupCount := 0
	groups := tools.ClassifyQuestion(question)
	for range groups {
		toolGroupCount++
	}
	if toolGroupCount >= 4 {
		return AgentModePlanExecute
	}

	wordCount := len([]rune(question))
	if wordCount > 80 {
		return AgentModePlanExecute
	}

	return AgentModeReact
}

func GetStockAiAgent(ctx *context.Context, aiConfig data.AIConfig, question string, agentMode string) *AgentInstance {
	logger.SugaredLogger.Infof("GetStockAiAgent aiConfig: %v", aiConfig)
	toolableChatModel, err := createChatModel(*ctx, aiConfig)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return nil
	}

	allTools := getAllTools()
	//allTools := getToolsByQuestion(question)

	var mode AgentMode
	switch AgentMode(agentMode) {
	case AgentModeReact:
		mode = AgentModeReact
	case AgentModePlanExecute:
		mode = AgentModePlanExecute
	default:
		mode = classifyComplexity(question)
	}

	logger.SugaredLogger.Infof("Agent mode selected: %s (user=%q), question=%q, tools=%d", mode, agentMode, question, len(allTools))

	switch mode {
	case AgentModePlanExecute:
		return createPlanExecuteAgent(*ctx, toolableChatModel, allTools, aiConfig)
	default:
		return createReactAgent(*ctx, toolableChatModel, allTools, aiConfig)
	}
}

func createReactAgent(ctx context.Context, chatModel model.ToolCallingChatModel, allTools []tool.BaseTool, aiConfig data.AIConfig) *AgentInstance {
	aiTools := compose.ToolsNodeConfig{
		Tools:               allTools,
		ToolCallMiddlewares: []compose.ToolMiddleware{errorRecoveryMiddleware()},
		UnknownToolsHandler: func(ctx context.Context, name string, input string) (string, error) {
			return fmt.Sprintf("工具 '%s' 不存在，请使用可用的工具列表中的工具。", name), nil
		},
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      aiTools,
		MaxStep:          len(allTools) + 5,
		MessageRewriter: func(ctx context.Context, input []*schema.Message) []*schema.Message {
			maxTokens := getMaxInputTokens(aiConfig.MaxTokens)
			return compressMessages(input, maxTokens)
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
		logger.SugaredLogger.Errorf("创建React Agent失败: %v", err)
		return nil
	}

	return &AgentInstance{
		Mode:       AgentModeReact,
		ReactAgent: agent,
	}
}

func createPlanExecuteAgent(ctx context.Context, chatModel model.ToolCallingChatModel, allTools []tool.BaseTool, aiConfig data.AIConfig) *AgentInstance {
	planner, err := planexecute.NewPlanner(ctx, &planexecute.PlannerConfig{
		ToolCallingChatModel: chatModel,
		GenInputFn:           genPlannerInput,
	})
	if err != nil {
		logger.SugaredLogger.Errorf("创建Planner失败: %v", err)
		return nil
	}

	executor, err := planexecute.NewExecutor(ctx, &planexecute.ExecutorConfig{
		Model: chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools:               allTools,
				ToolCallMiddlewares: []compose.ToolMiddleware{errorRecoveryMiddleware()},
				UnknownToolsHandler: func(ctx context.Context, name string, input string) (string, error) {
					return fmt.Sprintf("工具 '%s' 不存在，请使用可用的工具列表中的工具。", name), nil
				},
			},
		},
		MaxIterations: 15,
		GenInputFn:    genExecutorInput,
	})
	if err != nil {
		logger.SugaredLogger.Errorf("创建Executor失败: %v", err)
		return nil
	}

	replanner, err := planexecute.NewReplanner(ctx, &planexecute.ReplannerConfig{
		ChatModel:  chatModel,
		GenInputFn: genReplannerInput,
	})
	if err != nil {
		logger.SugaredLogger.Errorf("创建Replanner失败: %v", err)
		return nil
	}

	peAgent, err := planexecute.New(ctx, &planexecute.Config{
		Planner:       planner,
		Executor:      executor,
		Replanner:     replanner,
		MaxIterations: 5,
	})
	if err != nil {
		logger.SugaredLogger.Errorf("创建PlanExecute Agent失败: %v", err)
		return nil
	}

	return &AgentInstance{
		Mode:     AgentModePlanExecute,
		AdkAgent: peAgent,
	}
}

func createChatModel(ctx context.Context, aiConfig data.AIConfig) (model.ToolCallingChatModel, error) {
	temperature := float32(aiConfig.Temperature)
	if aiConfig.BaseUrl == "https://ark.cn-beijing.volces.com/api/v3" {
		var thinking *ark.Thinking
		if aiConfig.Thinking {
			thinking = &ark.Thinking{
				Type: "enabled",
			}
		}
		return ark.NewChatModel(context.Background(), &ark.ChatModelConfig{
			BaseURL:     aiConfig.BaseUrl,
			Model:       aiConfig.ModelName,
			APIKey:      aiConfig.ApiKey,
			MaxTokens:   &aiConfig.MaxTokens,
			Temperature: &temperature,
			Thinking:    thinking,
		})
	}

	extraFields := make(map[string]any)
	if aiConfig.Thinking {
		extraFields["thinking"] = map[string]any{
			"type": "enabled",
		}
	}
	return einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		BaseURL:     aiConfig.BaseUrl,
		Model:       aiConfig.ModelName,
		APIKey:      aiConfig.ApiKey,
		Timeout:     time.Duration(aiConfig.TimeOut) * time.Second,
		MaxTokens:   &aiConfig.MaxTokens,
		Temperature: &temperature,
		ExtraFields: extraFields,
	})
}

func errorRecoveryMiddleware() compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (output *compose.ToolOutput, err error) {
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Errorf("工具调用 panic: %v\n%s", r, debug.Stack())
						output = &compose.ToolOutput{
							Result: fmt.Sprintf("工具调用异常: %v。请尝试其他方法或修正参数后重试。", r),
						}
						err = nil
					}
				}()
				output, err = next(ctx, input)
				if err != nil {
					logger.SugaredLogger.Warnf("工具调用出错: %v", err)
					return &compose.ToolOutput{
						Result: fmt.Sprintf("工具调用出错: %v。请尝试其他方法或修正参数后重试。", err),
					}, nil
				}
				if output != nil && len(output.Result) > 8000 {
					output.Result = trimToolResult(output.Result, 4000)
				}
				return output, nil
			}
		},
		Streamable: func(next compose.StreamableToolEndpoint) compose.StreamableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (output *compose.StreamToolOutput, err error) {
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Errorf("工具调用(stream) panic: %v\n%s", r, debug.Stack())
						output = &compose.StreamToolOutput{
							Result: schema.StreamReaderFromArray([]string{
								fmt.Sprintf("工具调用异常: %v。请尝试其他方法或修正参数后重试。", r),
							}),
						}
						err = nil
					}
				}()
				output, err = next(ctx, input)
				if err != nil {
					logger.SugaredLogger.Warnf("工具调用出错(stream): %v", err)
					return &compose.StreamToolOutput{
						Result: schema.StreamReaderFromArray([]string{
							fmt.Sprintf("工具调用出错: %v。请尝试其他方法或修正参数后重试。", err),
						}),
					}, nil
				}
				return output, nil
			}
		},
	}
}

func buildSkillPrompt(question string) string {
	skills := data.NewSkillApi().GetEnabledSkills()
	if len(skills) == 0 {
		return ""
	}

	var matched []models.Skill
	for _, skill := range skills {
		if skill.TriggerKeywords == "" {
			matched = append(matched, skill)
			continue
		}
		keywords := strings.Split(skill.TriggerKeywords, ",")
		for _, kw := range keywords {
			kw = strings.TrimSpace(kw)
			if kw != "" && strings.Contains(question, kw) {
				matched = append(matched, skill)
				break
			}
		}
	}

	if len(matched) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 你具备以下专业技能：\n")
	for _, skill := range matched {
		sb.WriteString(fmt.Sprintf("\n### %s\n", skill.Name))
		if skill.Description != "" {
			sb.WriteString(fmt.Sprintf("描述：%s\n", skill.Description))
		}
		if skill.SystemPrompt != "" {
			sb.WriteString(fmt.Sprintf("%s\n", skill.SystemPrompt))
		}
		if skill.TriggerKeywords != "" {
			sb.WriteString(fmt.Sprintf("触发关键词：%s\n", skill.TriggerKeywords))
		}
		if skill.Examples != "" {
			sb.WriteString(fmt.Sprintf("示例对话：\n%s\n", skill.Examples))
		}
	}
	return sb.String()
}

func getAllTools() []tool.BaseTool {
	var allTools []tool.BaseTool
	allTools = append(allTools, tools.GetQueryStockCodeInfoTool())
	allTools = append(allTools, tools.GetQueryStockNewsTool())
	allTools = append(allTools, tools.GetIndustryResearchReportTool())
	allTools = append(allTools, tools.GetQueryBKDictTool())

	allTools = append(allTools, tools.GetAllDataTools()...)

	allTools = append(allTools, tools.GetHolidayTools()...)

	allTools = append(allTools, tools.GetMCPServerTools()...)
	//allTools = append(allTools, tools.GetSkillTools()...)

	mcpTools := getMCPTools()
	if len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	return allTools
}

func getToolsByQuestion(question string) []tool.BaseTool {
	var allTools []tool.BaseTool

	allTools = append(allTools, tools.GetQueryStockCodeInfoTool())
	allTools = append(allTools, tools.GetQueryStockNewsTool())
	allTools = append(allTools, tools.GetIndustryResearchReportTool())
	allTools = append(allTools, tools.GetQueryBKDictTool())

	allTools = append(allTools, tools.GetAllDataTools()...)

	allTools = append(allTools, tools.GetHolidayTools()...)

	allTools = append(allTools, tools.GetMCPServerTools()...)
	//allTools = append(allTools, tools.GetSkillTools()...)

	mcpTools := getMCPTools()
	if len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	groups := tools.ClassifyQuestion(question)
	filtered := tools.FilterToolsByGroups(allTools, groups)

	logger.SugaredLogger.Infof("tool grouping: question=%q, matched_groups=%v, total=%d, filtered=%d",
		question, groupNames(groups), len(allTools), len(filtered))

	return filtered
}

func groupNames(groups map[tools.ToolGroup]bool) []string {
	var names []string
	for g := range groups {
		names = append(names, string(g))
	}
	return names
}

func getMCPTools() []tool.BaseTool {
	var mcpTools []tool.BaseTool

	var servers []models.MCPServer
	err := db.Dao.Where("enable = ? AND status = ?", true, "available").Find(&servers).Error
	if err != nil {
		logger.SugaredLogger.Errorf("获取MCP服务器列表失败: %v", err)
		return mcpTools
	}

	skillServerIDs := getSkillMCPServerIDs()
	for _, id := range skillServerIDs {
		found := false
		for _, s := range servers {
			if s.ID == id {
				found = true
				break
			}
		}
		if !found {
			var server models.MCPServer
			if err := db.Dao.Where("id = ? AND status = ?", id, "available").First(&server).Error; err == nil {
				servers = append(servers, server)
			}
		}
	}

	if len(servers) == 0 {
		return mcpTools
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, server := range servers {
		if server.URL == "" {
			continue
		}

		cli, err := mcpclient.NewStreamableHttpClient(server.URL)
		if err != nil {
			logger.SugaredLogger.Errorf("创建MCP客户端失败 [%s]: %v", server.Name, err)
			continue
		}

		initRequest := mcp.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcp.Implementation{
			Name:    "go-stock",
			Version: "1.0.0",
		}

		_, err = cli.Initialize(ctx, initRequest)
		if err != nil {
			logger.SugaredLogger.Errorf("初始化MCP连接失败 [%s]: %v", server.Name, err)
			continue
		}

		mcpToolList, err := einomcp.GetTools(ctx, &einomcp.Config{Cli: cli})
		if err != nil {
			logger.SugaredLogger.Errorf("获取MCP工具列表失败 [%s]: %v", server.Name, err)
			continue
		}

		if len(mcpToolList) > 0 {
			logger.SugaredLogger.Infof("从MCP服务器 [%s] 加载了 %d 个工具", server.Name, len(mcpToolList))
			mcpTools = append(mcpTools, mcpToolList...)
		}
	}

	return mcpTools
}

func getSkillMCPServerIDs() []uint {
	skills := data.NewSkillApi().GetEnabledSkills()
	var ids []uint
	seen := make(map[uint]bool)
	for _, skill := range skills {
		if skill.MCPServerIDs == "" {
			continue
		}
		for _, id := range data.NewSkillApi().GetMCPServerIDs(&skill) {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func genPlannerInput(ctx context.Context, userInput []adk.Message) ([]adk.Message, error) {
	var userContent strings.Builder
	for _, msg := range userInput {
		if msg.Content != "" {
			userContent.WriteString(msg.Content)
			userContent.WriteString("\n")
		}
	}

	systemMsg := schema.SystemMessage(`你是一个专业的股票分析规划师。根据用户目标，制定简洁的执行计划。

要求：
- 每步必须具体、可执行、独立
- 步骤按最优顺序排列
- 去除冗余步骤
- 最后一步必须产出完整答案
- 步骤数量控制在3-5步`)

	userMsg := schema.UserMessage(userContent.String())

	return []adk.Message{systemMsg, userMsg}, nil
}

func genExecutorInput(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
	planContent, err := in.Plan.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var stepsContent strings.Builder
	for _, s := range in.ExecutedSteps {
		result := s.Result
		if len(result) > 2000 {
			result = result[:2000] + "...(已截断)"
		}
		stepsContent.WriteString(fmt.Sprintf("步骤: %s\n结果: %s\n\n", s.Step, result))
	}

	var inputContent strings.Builder
	for _, msg := range in.UserInput {
		if msg.Content != "" {
			inputContent.WriteString(msg.Content)
			inputContent.WriteString("\n")
		}
	}

	systemMsg := schema.SystemMessage(`你是股票分析执行者。严格按照计划执行当前步骤，调用合适的工具获取数据，然后给出分析结果。结果要简洁精准。`)

	userMsg := schema.UserMessage(fmt.Sprintf("目标: %s\n\n当前计划: %s\n\n已完成步骤:\n%s\n\n请执行当前步骤: %s",
		inputContent.String(), string(planContent), stepsContent.String(), in.Plan.FirstStep()))

	return []adk.Message{systemMsg, userMsg}, nil
}

func genReplannerInput(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
	planContent, err := in.Plan.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var stepsContent strings.Builder
	for _, s := range in.ExecutedSteps {
		result := s.Result
		if len(result) > 2000 {
			result = result[:2000] + "...(已截断)"
		}
		stepsContent.WriteString(fmt.Sprintf("步骤: %s\n结果: %s\n\n", s.Step, result))
	}

	var inputContent strings.Builder
	for _, msg := range in.UserInput {
		if msg.Content != "" {
			inputContent.WriteString(msg.Content)
			inputContent.WriteString("\n")
		}
	}

	systemMsg := schema.SystemMessage(`你是进度审核员。根据已完成步骤判断下一步行动：

- 如果目标已达成，调用 respond 工具给出最终答案
- 如果还需继续，调用 plan 工具给出剩余步骤（不含已完成步骤）

判断标准：原始目标是否已完全满足？`)

	userMsg := schema.UserMessage(fmt.Sprintf("目标: %s\n\n原始计划: %s\n\n已完成步骤:\n%s",
		inputContent.String(), string(planContent), stepsContent.String()))

	return []adk.Message{systemMsg, userMsg}, nil
}
