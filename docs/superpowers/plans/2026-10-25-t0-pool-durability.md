# T0 历史股票池持久化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline. Steps use checkbox syntax.

**Goal:** 留存/恢复真实 dated pool，输出全日期审计，阻止不可用历史日期进入下拉，安装轻量保活。
**Architecture:** 独立日期池 JSON 为第二份持久存储；共享 GOB 继续保存日线。原子写入、日期/代码校验、历史池冲突拒绝。审计 CLI 复用生产读取规则，不联网上拉当前股票池冒充历史。
**Tech Stack:** Go 标准库、现有 Go 测试、macOS launchd。
**Spec:** docs/superpowers/specs/2026-10-25-t0-pool-durability-design.md

## Global Constraints

不改已有已确认名单；不把当前池/近似归档标成历史完整池；保留本轮前端及用户原有 native 改动；不推送、不提交无关数据。仅本机运行部署，云端需有效认证。

## Review Focus

- 日线获取失败后仍存在当天池。
- 导出覆盖 GOB 后独立历史池仍可恢复。
- 损坏/日期不符/冲突池不会静默替代正式池。
- 下拉不将日线覆盖等同于可回放名单。
- 保活检查不会重复启动第二个服务。

### Task 1: 独立池归档与恢复
Files: backend/flutter_api/t0_pool_archive.go, t0_pool_archive_test.go, mainboard_shared_gob.go, server.go。
- [x] 写测试：真实获取后即归档、日线失败保留；重载缺池 GOB 后恢复；不可变历史池冲突；坏文件拒绝。
- [x] 运行测试确认 RED，最小实现；运行相关 Go 测试确认 GREEN。

### Task 2: 全日期审计和可用日期索引
Files: backend/flutter_api/t0_history_audit.go, t0_history_audit_test.go, cmd/t0-history-audit/main.go, t0_selection.go, mainboard_shared_gob.go。
- [x] 写测试：缺池日不进入下拉，已确认日保留，当天入口保留；审计列明所有缺失日；有效旧缓存恢复与无标记缓存拒绝。
- [x] RED → 实现 → GREEN；实际运行 CLI，保存恢复前后报告与9天池备份。

### Task 3: 保活和上线验证
Files: scripts/launchd/com.colin.go-stock.flutter-api.plist, scripts/launchd/README.md, docs/diagnostics/2026-10-25-history-audit/。
- [x] 设置登录加载和五分钟检查，plutil 验证，安装现有服务的 launchd 配置并确认不会重复启动。
- [x] 后端构建并重启验证健康接口、9天历史结果、缺失日明确状态、可用日期列表。
- [x] 全后端包测试、代码检查；记录外部网络既有失败和仍缺的原始历史数据。


## 执行记录

任务 1：完成。独立池和早落盘、坏文件单日隔离、GOB 覆盖恢复已 RED→GREEN。
任务 2：完成。生产/审计共用来源比对；38 项相关测试通过；9 天独立池留档，678 天覆盖报告完成。10 月 20 日原始池缺失仍明确保留，不能标记已准确恢复。
任务 3：完成本机部署和保活安装，实际接口 9 天全部 200；全量测试未绿色的原因与日志见排查与修复记录。
独立 reviewer 的 P1/P2 共三项问题已逐项补测试并修复。没有 deferred minor。
