# 主板策略蓝色买入信号优先排序

## 目标

`买入信号=blue` 的股票在所有客户端列表中排在最前。

## 排序键（依次）

1. blue 优先
2. 标记非空优先（沿用 `2026-08-12-t0-tag-first-sort-design.md`）
3. T0开盘涨幅(%) 降序
4. 稳定排序

## 竞价预览附加键（Flutter，09:15–09:25）

在 blue 之后：有实时行情优先；组内按实时涨幅降序。

## 不变

- 磁盘 `t0_selection_*.json` 写入顺序
- 行内 chip UI 与元素顺序

## API 出口

凡调用 `sortT0ResultsForClient` 的 `results` / `candidates` 响应均适用。
