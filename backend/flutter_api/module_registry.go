package flutter_api

var registeredModules = []ModuleDefinition{
	{
		Code:       "radar.monitored",
		Name:       "监控股票（自选）",
		Client:     "flutter_web",
		Placement:  "radar_tab",
		ParentCode: nil,
		Sort:       10,
		AccessMode: ModuleAccessPublic,
	},
	{
		Code:       "radar.purple_strategy",
		Name:       "紫策",
		Client:     "flutter_web",
		Placement:  "radar_tab",
		ParentCode: nil,
		Sort:       20,
		AccessMode: ModuleAccessAllowlist,
	},
	{
		Code:       "radar.main_strategy",
		Name:       "主板策略",
		Client:     "flutter_web",
		Placement:  "radar_tab",
		ParentCode: nil,
		Sort:       30,
		AccessMode: ModuleAccessAllowlist,
	},
	{
		Code:       "radar.blue_strategy",
		Name:       "蓝策",
		Client:     "flutter_web",
		Placement:  "radar_tab",
		ParentCode: nil,
		Sort:       40,
		AccessMode: ModuleAccessAllowlist,
	},
	{
		Code:       "radar.watch_changes",
		Name:       "自选异动",
		Client:     "flutter_web",
		Placement:  "radar_tab",
		ParentCode: nil,
		Sort:       50,
		AccessMode: ModuleAccessPublic,
	},
	{
		Code:       "radar.all_changes",
		Name:       "全市场",
		Client:     "flutter_web",
		Placement:  "radar_tab",
		ParentCode: nil,
		Sort:       60,
		AccessMode: ModuleAccessPublic,
	},
}

func RegisteredModules() []ModuleDefinition {
	return append([]ModuleDefinition(nil), registeredModules...)
}

func FindModule(code string) (ModuleDefinition, bool) {
	for _, module := range registeredModules {
		if module.Code == code {
			return module, true
		}
	}
	return ModuleDefinition{}, false
}
