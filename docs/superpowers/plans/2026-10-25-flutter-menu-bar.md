# Flutter 菜单栏入口实施计划

> **For agentic workers:** Use superpowers:executing-plans to implement this small change inline.

**Goal:** 顶部菜单栏图标一键展开/收起完整 Flutter App。
**Architecture:** MenuBarController 管理 NSStatusItem 与同一主窗口的显示；AppDelegate 持有控制器，主窗口关闭只隐藏。
**Tech Stack:** Swift/AppKit，既有 Flutter Mac/Xcode 工程。
**Spec:** docs/superpowers/specs/2026-10-25-flutter-menu-bar-design.md

## Constraints and review
保留已有页面/引擎/面板宽度、WidgetKit、用户数据；正常退出不被隐藏逻辑拦截；最小化后点击应恢复；收起不丢业务状态。

- [x] MenuBarController.swift 加入 Runner/RunnerTests，真实 NSWindow 测试覆盖 show/hide/toggle、内容保持及最小化恢复。
- [x] AppDelegate 接入菜单栏及 Dock 重开；MainFlutterWindow 关闭只隐藏，首启隐藏，显式展开使用右侧模式。
- [x] 跑8项原生测试（含真实按钮target/action）、编译Release、签名校验、更新安装版；独立预览副本运行与菜单栏右侧布局确认。
- [x] 原安装版PID82593实际通过系统桌面菜单栏点击验收：窗口数1→0→1，截图确认完整Flutter页面保留。
- [x] 更新 Mac README、验收结果及对话记录，明确系统确认与桌面鼠标验收仍待完成。
