package flutter_api

import (
	"fmt"
	"strings"
)

func validateRegisteredModuleContracts() error {
	seen := make(map[string]struct{})
	for _, module := range RegisteredModules() {
		code := strings.TrimSpace(module.Code)
		if code == "" {
			return fmt.Errorf("模块编码不能为空")
		}
		if _, exists := seen[code]; exists {
			return fmt.Errorf("模块编码重复: %s", code)
		}
		seen[code] = struct{}{}

		if !strings.HasSuffix(code, "_strategy") {
			continue
		}
		if err := validateT0ModuleCode(code); err != nil {
			return fmt.Errorf("策略模块 %s 未接入 T0 分发: %w", code, err)
		}

		ctx := (*t0ModuleSelectionContext)(nil)
		if isT0DailyContextModule(code) {
			ctx = &t0ModuleSelectionContext{
				TradeDate: "contract",
				Daily:     map[string][]dailyBar{},
			}
		}
		selected, err := selectT0ResultsForModule(
			code,
			[]T0SelectionResult{{StockCode: "contract"}},
			ctx,
		)
		if err != nil {
			return fmt.Errorf("策略模块 %s 分发失败: %w", code, err)
		}
		if selected == nil {
			return fmt.Errorf("策略模块 %s 缺少分发实现", code)
		}
	}
	return nil
}
