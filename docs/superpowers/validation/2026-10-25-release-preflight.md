# 2026-10-25 提交与部署前验证

用户要求：提交所有未提交的对话代码，推送远端 dev 与 master，并部署阿里云。远端默认分支为 dev；预检时 master 不存在，按本次明确要求创建同版本 master，不变更默认分支。

## 提交范围

- 主板策略组件、图表独立设置与行情刷新缓存。
- macOS 菜单栏入口、右侧面板、小组件工程与测试。
- 历史股票池独立归档、恢复、来源校验、审计 CLI、明确标记的重建回放。
- 本地后台保活、历史补采证据、研究脚本与报告、选股缓存和 Git LFS 共享日线更新。
- 不提交本机运行数据库 data/stock.db，不上传它覆盖服务器账户和设置。

## 本次重新验证

- 38 项相关 Flutter 测试通过。
- 历史归档、审计、重建和主板图表路由定向 Go 测试通过。
- 管理后台 7 个测试文件、23 项测试通过；生产构建通过。
- Linux amd64 静态后端构建通过，Flutter Web release 构建通过。
- macOS 控制器和小组件 Swift 语法、工程 plist 检查通过；未在本次重跑完整 macOS 构建。
- 待提交 JSON 均可解析，独立原始池与重建池的版本/来源/完整性标记校验通过。
- 独立只读审查未发现本次改动引入的 Critical/Important 阻断。
- 源代码 diff 检查通过；研究图 SVG 为工具生成，保留其既有路径空格格式。

## 全量测试边界

完整 Flutter 测试为 205 通过、9 失败，失败集合与同日主板 UI 验证记录一致：旧模块目录、旧页面夹具及分时空数据/错误提示预期。详情见 2026-10-25-composable-strategy-ui.md。

完整 Go 包测试仍有模块目录、历史显示规则测试失败，并在 TestPrewarmCandidatesPurpleUseTrueEarnRate 出现全局状态相关越界 panic。用 HEAD 原源码 overlay 重新核对，TestAdminModuleHTTPListsModulesAccessAndGrantedUsers、TestRegisteredModulesContainsCurrentRadarTabs、TestBuildPrewarmReadyResponseEnrichesHistoricalDisplayRules 在未提交修改前也失败；Purple 用例在基线单独运行未失败。不宣称全量测试通过，不将原有测试问题混入本次提交部署工作。

## 发布产物与服务器状态

发布包包括 Linux 后端、Flutter Web、管理后台及三个带不确定性标记的历史重建池，不打包本机 SQLite、凭据或服务器配置。共享日线和正式确认归档的远端更新应先读取服务器状态、备份并检查冲突，不能用本地数据库或共享快照直接覆盖。

截至本次预检，阿里云 118.178.19.165:8080 的 /api/health 返回 HTTP 200、status=ok；SSH root 登录被现有认证拒绝。尚未替换或重启远端服务，部署需要用户提供可用 SSH 连接方式。

## 推送传输修正

119 MB 共享日线 LFS 对象已上传成功。普通 Git 两次遭遇 HTTP 408；SSH 认证账号无仓库写权限。将 docs/research 下压缩数据集改为现有 Git LFS 存储，逐文件校验内容 SHA256 不变，减少普通 Git 传输体积；源代码和全部研究成果均保留。
