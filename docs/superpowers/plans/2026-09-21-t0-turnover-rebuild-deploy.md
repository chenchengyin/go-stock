# T0 成交额缓存与形态数据库重建部署 Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 从 2026-07-01 起重新生成采用已验证成交额来源的 T0 GOB，基于这批可信数据重建形态统计数据库，并部署到阿里云生产服务。

**Architecture:** 复用现有 T0 重建入口，按交易日生成 schema=2 的 GOB 和选股归档；形态聚合只读取 2026-07-01 至当前交易日的可信缓存，再同步 `data/stock.db` 的 `t0_pattern_stats`。部署采用 Linux amd64 交叉编译、远端临时目录切换、systemd 重启和线上哈希/健康检查。

**Tech Stack:** Go 1.26、T0 GOB cache、SQLite/GORM、Flutter Web、Linux amd64、systemd。

**Spec:** 用户当前请求；成交额修复代码已在当前工作区完成。

## Global Constraints

- 使用 `/Users/vb/Projects/go-stock/data/stock.db` 作为形态数据库，不使用根目录零字节 `stock.db`。
- 不覆盖或清理当前工作区无关改动和无关缓存。
- GOB 成交额只接受数据源直接返回的元字段，统一为元并记录来源/已验证标记。
- 形态统计只使用 2026-07-01 至当前交易日的重建缓存，不混入 7 月以前的旧缓存。
- 部署目标为阿里云 `118.178.19.165:/root/go-stock`，服务为 `go-stock.service`；SSH 认证失败时停止远端操作，不猜账号或密钥。

## Review Focus

- 交易日缺失或节假日：重建应跳过无当日 K 线的日期并报告实际成功日期。
- 数据源成交额为空：该股票不能写入不可信的日线 bar，应在重建汇总中暴露缺失。
- 形态统计范围：数据库批次日期必须是 2026-07-01 至当前交易日，不能悄悄混入旧缓存。
- SQLite 发布：服务使用根目录 `data/stock.db`，必须同时备份和上传正确数据库。
- 线上发布：必须验证 systemd active、health 响应、远端二进制/Web/数据库文件存在。

### Task 1: Preflight and backup

**Files:**
- Read: `backend/data/cache/t0/daily/`
- Read: `backend/data/cache/t0/pattern/`
- Read: `data/stock.db`

- [ ] 记录当前 GOB/selection 覆盖范围、数据库表计数和 Git 工作区状态。
- [ ] 将当前形态数据库和目标日期缓存复制到带时间戳的 `/tmp` 备份目录，不删除原文件。
- [ ] 验证阿里云 SSH 身份、远端路径和 systemd 服务；认证失败则保留本地成果并报告阻塞。

### Task 2: Rebuild verified GOB and selection archives

**Files:**
- Modify: `backend/data/cache/t0/daily/t0_daily_cache_YYYY-MM-DD.gob`
- Modify: `backend/data/cache/t0/selection/t0_selection_YYYY-MM-DD.json`

- [ ] 使用现有重建辅助入口拉取截至当前交易日的日线，覆盖 2026-07-01 至当前交易日。
- [ ] 每个成功交易日写入 schema=2 GOB，并强制刷新对应选股归档。
- [ ] 对日期数量、当日 K 线数量、成交额来源/单位/verified 标记做汇总校验。

### Task 3: Rebuild pattern statistics database

**Files:**
- Modify: `backend/data/cache/t0/pattern/pattern_aggregate_2026-07-01-2026-09-21.json`
- Modify: `data/stock.db`

- [ ] 使用 2026-07-01 至当前交易日的 GOB 运行 3K 形态聚合。
- [ ] 使用 `cmd/t0-pattern-sync-db` 全量替换 `t0_pattern_stats` 并更新 `t0_pattern_config.batch_id`。
- [ ] 查询数据库确认批次范围、样本数、形态行数和关键股票成交额字段。

### Task 4: Build, deploy and verify

**Files:**
- Create: `/tmp/go-stock-deploy-*`
- Deploy: `/root/go-stock/go-stock-server`, `/root/go-stock/trading_app/build/web`, `/root/go-stock/data/stock.db`, refreshed T0 cache/pattern files

- [ ] 运行 Go 目标包测试和 Linux amd64 静态构建。
- [ ] 运行 Flutter Web release 构建。
- [ ] 通过远端临时目录上传二进制、Web、SQLite 和 GOB/形态文件，备份远端旧版本后原子切换。
- [ ] 重启 `go-stock.service`，验证 active、health、服务日志、远端文件哈希和数据库批次。
