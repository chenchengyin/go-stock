# 项目根目录缓存路径设计

## 目标

无论本机或 Ubuntu、无论从哪个工作目录启动，T0 日线 gob 与 selection JSON 都固定写入：

```text
<go-stock项目根>/backend/data/cache/t0/
```

继续使用现有目录和历史文件，不迁移数据。

## 路径解析

新增统一的数据缓存根目录解析器，优先级如下：

1. 环境变量 `GO_STOCK_CACHE_DIR`：若非空，转为绝对路径并使用。
2. 从当前工作目录开始逐级向上查找项目根。
3. 从当前可执行文件所在目录开始逐级向上查找项目根。

项目根的判定条件：目录同时包含 `go.mod` 和 `backend/`。找到后，缓存根为 `<项目根>/backend/data/cache`。

该顺序支持：

- 本地 `go run ./cmd/server`：通过当前工作目录找到项目根；
- Ubuntu 在仓库根运行预编译二进制：通过当前工作目录或二进制目录找到项目根；
- systemd/nohup 从其他工作目录启动仓库内二进制：通过二进制目录找到项目根；
- 特殊部署：用 `GO_STOCK_CACHE_DIR` 明确指定持久化目录。

## 失败行为

若没有设置 `GO_STOCK_CACHE_DIR`，且当前工作目录和二进制目录均无法定位项目根，服务启动直接返回明确错误，不再回退到相对路径，避免静默把数据写到错误目录。

## 代码边界

- 在 `backend/flutter_api` 新增项目根与缓存目录解析函数。
- `t0CacheRootPath` 初始化为解析后的绝对路径。
- 服务启动时校验路径并创建目录。
- 现有 `t0DailyCachePath`、`t0SelectionCachePath` 和归档逻辑不变，只消费新的绝对根路径。
- `scripts/ensure-flutter-api.sh` 与 `scripts/start-flutter-api.sh` 仍可继续使用，不再是路径正确性的必要条件。

## 测试

- 环境变量覆盖优先。
- 从嵌套工作目录向上找到项目根。
- 从二进制目录向上找到项目根。
- 找不到项目根时返回错误。
- 解析结果为绝对路径，最终 daily/selection 路径仍位于 `backend/data/cache/t0`。
