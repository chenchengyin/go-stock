# 主板策略可组合 UI 验证记录

日期：2026-10-25。实现范围见设计和计划同名文档。

## 已通过

- 38 项定向 Flutter 测试：mainboard_strategy_ui_test、strategy_card_test、strategy_chart_settings_test、strategy_chart_exposure_test、red_t1_intraday_repository_test、red_strategy_charts_test、red_kline_test、red_t1_intraday_exposure_test。
- 后端 11 个顶层图表测试通过（另有实盘探针按设计跳过）：主板日K、分时日期及昨收、非法模块/未来分时日期、原红策行情入口与窗口/均线/交易日回归。
- 新增组件、设置、行情Repository和相关测试共19个文件的定向Flutter静态检查：No issues found。
- 实际Flutter组件以模拟行情渲染390×844及1100×860，截图人工检查：信息头正常换行，卡片与两个图表均未溢出。截图使用测试字体；图标与日期显示中的测试字体痕迹不作为正式运行版本效果。
- Web编译成功；独立输出目录/tmp/go-stock-mainboard-ui-web，不替换运行中的服务。
- git diff --check通过。

## 完整测试的限制

完整Flutter测试：205项通过，9项失败，不能宣称全量通过。以下6项在修改前定向基线中已失败，修改后仍失败：

1. 命中显示条件的股票名称在四个策略页均显示为红色（旧红策tab夹具）。
2. 紫策隐藏开盘涨幅且主板策略仍显示。
3. 蓝策位于主板策略右侧并显示蓝灯数量（tab目录夹具）。
4. 紫策位于主板策略左侧并只显示严格阈值达标的股票（tab目录夹具）。
5. 切换进入蓝策也只初始化一次。
6. 自选异动下拉刷新仍调用自选异动接口。

另3项失败来自未改动的源文件和测试文件，对应源码/测试git diff为空：

7. radar catalog has nine independent stable modules in server order。
8. 切到无数据交易日不沿用旧分时价格且仍可返回（分时空数据提示文案预期）。
9. 切日请求失败时保留原图并允许重试（分时错误提示文案预期）。

完整backend/flutter_api测试：下列测试失败后，后台选股的行情回退路径发生nil Gorm DB崩溃，未完成整包验证：

- TestAdminModuleHTTPListsModulesAccessAndGrantedUsers（模块目录预期）。
- TestRegisteredModulesContainsCurrentRadarTabs（8个模块与旧预期9个不一致）。
- TestSharedGobRedDateListRevisionChangesWhenListIsConfirmed（revision为空）。
- 崩溃位置：backend/data/settings_api.go GetSettingConfig → gorm.DB.getInstance；调用来自现有EastMoney行情回退、fetchAllDailyKLine。

上述目录/行情路径不在此次UI改动范围，工作区中其他任务的变更保持原状。没有提交、发布或重启服务。

## 独立审查与修复

由gpt-6.1-sol进行只读审查，发现1个Important刷新竞态，无Critical/Minor：旧分时请求在清缓存后返回，可能重新写入成功缓存，导致刷新复用旧数据。用挂起响应的测试复现（期望2次请求，实际1次），修复为仅当前pending请求允许写缓存；新增按模块清缓存测试，确保刷新主板不清除红策缓存。修复后的38项定向测试通过，完整Flutter复跑仍为上述9项失败。

## 主要组件

- strategy_card.dart：StrategyCard的header/sections插槽，以及StrategyCardHeader的信息和操作插槽。
- strategy_stock_card.dart：标准股票卡片的组合方式；保留原有不同策略的数据展示。
- strategy_stock_chips.dart：可单独组合的信号、评级、参考形态、红策形态和卖出目标标签。
- strategy_charts.dart：日K/均线/长按与分时切日联动，保留RedStrategyCharts兼容别名。
- strategy_chart_exposure.dart：与具体图表无关的可见加载与有限重试组件。
- strategy_stock_charts.dart：按模块/日期/股票/刷新版本装配独立加载的日K与分时。
- strategy_chart_settings.dart：主板、红策独立保存图表开关，保留旧RedKlineSettings兼容别名。

主板图表默认开启，可在系统设置中分别关闭“主板K线图”和“主板T+1分时图”。组合示例见设计文档末尾。
