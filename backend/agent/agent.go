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
	"unicode/utf8"

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

	//allTools := getAllTools()
	allTools := getToolsByQuestion(question)

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
		MaxIterations: 25,
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
		MaxIterations: 7,
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
	//allTools = append(allTools, tools.GetIndustryResearchReportTool())
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
	//allTools = append(allTools, tools.GetIndustryResearchReportTool())
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

func extractUserQuestion(messages []adk.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == schema.User && messages[i].Content != "" {
			return cleanUserInput(messages[i].Content)
		}
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Content != "" {
			return cleanUserInput(messages[i].Content)
		}
	}
	return ""
}

func genPlannerInput(ctx context.Context, userInput []adk.Message) ([]adk.Message, error) {
	question := extractUserQuestion(userInput)
	if question == "" {
		return userInput, nil
	}

	systemMsg := schema.SystemMessage(`你是股票分析规划师。将用户目标拆解为3-4步执行计划。
规则：步骤具体独立、按最优顺序、无冗余，末步须产出完整答案。`)

	userMsg := schema.UserMessage(question)

	return []adk.Message{systemMsg, userMsg}, nil
}

// safeTruncateString 安全地截断字符串，确保不会破坏UTF-8编码的中文字符
func safeTruncateString(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	// 找到最接近maxBytes的UTF-8字符边界
	for i := maxBytes; i > maxBytes-4 && i >= 0; i-- {
		if utf8.ValidString(s[:i]) {
			if i < len(s) {
				return s[:i] + "...(已截断)"
			}
			return s[:i]
		}
	}

	// 如果找不到合适的边界，返回安全的前缀
	return s[:maxBytes-10] + "...(已截断)"
}

// smartContentCompress 智能内容压缩，保留关键信息同时减少tokens消耗
func smartContentCompress(content string, maxTokens int) string {
	if len(content) <= maxTokens {
		return content
	}

	// 按行分割内容
	lines := strings.Split(content, "\n")

	// 识别不同类型的内容并分类
	var headers []string
	var dataLines []string
	var summaryLines []string
	var otherLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// 分类逻辑
		if isHeaderLine(trimmed) {
			headers = append(headers, line)
		} else if isDataLine(trimmed) {
			dataLines = append(dataLines, line)
		} else if isSummaryLine(trimmed) {
			summaryLines = append(summaryLines, line)
		} else {
			otherLines = append(otherLines, line)
		}
	}

	// 按优先级重组内容，优先保留重要信息
	var result []string

	// 1. 保留所有标题（通常很重要）
	result = append(result, headers...)

	// 2. 保留摘要信息（关键结论）
	summaryBudget := int(float64(maxTokens) * 0.3)
	if len(strings.Join(summaryLines, "\n")) > summaryBudget {
		result = append(result, smartTruncateLines(summaryLines, summaryBudget)...)
	} else {
		result = append(result, summaryLines...)
	}

	// 3. 保留部分数据（按重要性）
	dataBudget := int(float64(maxTokens)*0.5) - len(strings.Join(result, "\n"))
	if dataBudget > 0 && len(dataLines) > 0 {
		result = append(result, smartTruncateLines(dataLines, dataBudget)...)
	}

	// 4. 如果还有空间，保留其他内容
	otherBudget := maxTokens - len(strings.Join(result, "\n"))
	if otherBudget > 0 && len(otherLines) > 0 {
		result = append(result, smartTruncateLines(otherLines, otherBudget)...)
	}

	finalContent := strings.Join(result, "\n")

	// 如果还是太长，进行安全截断
	if len(finalContent) > maxTokens {
		finalContent = safeTruncateString(finalContent, maxTokens)
	}

	return finalContent
}

// isHeaderLine 判断是否为标题行
func isHeaderLine(line string) bool {
	// 包含常见标题关键词
	headerKeywords := []string{
		"分析", "结论", "建议", "总结", "评估", "预测", "风险", "机会",
		"价格", "涨跌", "涨幅", "成交量", "市值", "市盈率", "市净率",
		"营收", "利润", "增长率", "ROE", "ROA", "毛利率", "净利率",
	}

	for _, keyword := range headerKeywords {
		if strings.Contains(line, keyword) && len(line) < 100 {
			return true
		}
	}

	// 包含数字+单位的行（通常是关键指标）
	if containsKeyMetrics(line) {
		return true
	}

	return false
}

// isDataLine 判断是否为数据行
func isDataLine(line string) bool {
	// 包含数字和百分比的行
	return strings.Contains(line, "%") ||
		strings.Contains(line, "亿元") ||
		strings.Contains(line, "万元") ||
		(strings.Count(line, " ") > 2 && containsNumbers(line))
}

// isSummaryLine 判断是否为摘要行
func isSummaryLine(line string) bool {
	summaryKeywords := []string{
		"总体", "整体", "综合", "综上所述", "总体来看", "整体而言",
		"建议", "推荐", "关注", "警惕", "规避",
	}

	for _, keyword := range summaryKeywords {
		if strings.Contains(line, keyword) {
			return true
		}
	}
	return false
}

// containsKeyMetrics 检查是否包含关键指标
func containsKeyMetrics(line string) bool {
	metrics := []string{
		"市盈率", "市净率", "ROE", "ROA", "毛利率", "净利率",
		"营收", "利润", "增长率", "股价", "市值", "成交量",
	}

	for _, metric := range metrics {
		if strings.Contains(line, metric) {
			return true
		}
	}
	return false
}

// containsNumbers 检查是否包含数字
func containsNumbers(line string) bool {
	for _, r := range line {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// smartTruncateLines 智能截断行列表
func smartTruncateLines(lines []string, maxBytes int) []string {
	var result []string
	var currentSize int

	for _, line := range lines {
		if currentSize+len(line) > maxBytes {
			break
		}
		result = append(result, line)
		currentSize += len(line) + 1 // +1 for newline
	}

	return result
}

// cleanUserInput 清理用户输入，确保UTF-8编码正确
func cleanUserInput(input string) string {
	// 1. 确保字符串是有效的UTF-8
	if !utf8.ValidString(input) {
		// 如果包含无效UTF-8字符，进行修复
		valid := make([]rune, 0, len(input))
		for i, r := range input {
			if r == utf8.RuneError {
				// 跳过无效字符，但记录位置用于调试
				logger.SugaredLogger.Warnf("发现无效UTF-8字符在位置 %d", i)
				continue
			}
			valid = append(valid, r)
		}
		input = string(valid)
	}

	// 2. 标准化空白字符
	input = strings.TrimSpace(input)

	// 3. 移除可能的控制字符（除了换行和制表符）
	var cleaned strings.Builder
	for _, r := range input {
		if r == '\n' || r == '\t' || r == '\r' {
			cleaned.WriteRune(r)
		} else if r >= 32 && r <= 126 { // ASCII可打印字符
			cleaned.WriteRune(r)
		} else if r > 126 { // 非ASCII字符（包括中文）
			cleaned.WriteRune(r)
		}
		// 跳过其他控制字符
	}

	return cleaned.String()
}

func genExecutorInput(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
	planContent, err := in.Plan.MarshalJSON()
	if err != nil {
		logger.SugaredLogger.Errorf("Plan MarshalJSON error: %v", err)
		return nil, err
	}

	var stepsContent strings.Builder
	for _, s := range in.ExecutedSteps {
		result := smartContentCompress(s.Result, 1000)
		stepsContent.WriteString(fmt.Sprintf("步骤: %s\n结果: %s\n\n", s.Step, result))
	}

	question := extractUserQuestion(in.UserInput)

	systemMsg := schema.SystemMessage(`按计划执行当前步骤，调用工具获取数据，给出简洁精准的分析结果。`)

	userMsg := schema.UserMessage(fmt.Sprintf("目标: %s\n\n当前计划: %s\n\n已完成步骤:\n%s\n\n请执行当前步骤: %s",
		question, string(planContent), stepsContent.String(), in.Plan.FirstStep()))

	return []adk.Message{systemMsg, userMsg}, nil
}

func genReplannerInput(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
	planContent, err := in.Plan.MarshalJSON()
	if err != nil {
		logger.SugaredLogger.Errorf("Plan MarshalJSON error: %v", err)
		return nil, err
	}

	var stepsContent strings.Builder
	for _, s := range in.ExecutedSteps {
		result := smartContentCompress(s.Result, 1000)
		stepsContent.WriteString(fmt.Sprintf("步骤: %s\n结果: %s\n\n", s.Step, result))
	}

	question := extractUserQuestion(in.UserInput)

	systemMsg := schema.SystemMessage(`审核进度：目标达成则调用respond给最终答案，否则调用plan给剩余步骤（不含已完成）。`)

	userMsg := schema.UserMessage(fmt.Sprintf("目标: %s\n\n原始计划: %s\n\n已完成步骤:\n%s",
		question, string(planContent), stepsContent.String()))

	return []adk.Message{systemMsg, userMsg}, nil
}
