/// 异动类型条目
class ChangeTypeItem {
  const ChangeTypeItem(this.id, this.name);

  final int id;
  final String name;
}

/// 异动类型配置管理
/// 
/// 所有异动类型定义在此集中管理，前端通过 [allTypes] 获取所有类型。
/// 新增/修改类型只需在此添加，无需四处改代码。
class ChangeTypeConfig {
  ChangeTypeConfig._();

  /// 所有异动类型定义（常用类型排前面）
  static const List<ChangeTypeItem> allTypes = [
    // ── 常用类型 ──
    ChangeTypeItem(8201, '火箭发射'),
    ChangeTypeItem(8193, '大笔买入'),
    ChangeTypeItem(64, '有大买盘'),
    ChangeTypeItem(8204, '加速下跌'),
    ChangeTypeItem(8194, '大笔卖出'),
    ChangeTypeItem(128, '有大卖盘'),
    ChangeTypeItem(8203, '高台跳水'),

    // ── 上涨方向 ──
    ChangeTypeItem(8202, '快速反弹'),
    ChangeTypeItem(9002, '30秒急速波动'),
    ChangeTypeItem(9003, '60秒急速波动'),
    ChangeTypeItem(4, '封涨停板'),
    ChangeTypeItem(32, '打开跌停板'),
    ChangeTypeItem(8207, '竞价上涨'),
    ChangeTypeItem(8209, '高开5日线'),
    ChangeTypeItem(8211, '向上缺口'),
    ChangeTypeItem(8213, '60日新高'),
    ChangeTypeItem(8215, '60日大幅上涨'),
    ChangeTypeItem(16, '打开涨停板'),

    // ── 下跌方向 ──
    ChangeTypeItem(8, '封跌停板'),
    ChangeTypeItem(8208, '竞价下跌'),
    ChangeTypeItem(8210, '低开5日线'),
    ChangeTypeItem(8212, '向下缺口'),
    ChangeTypeItem(8214, '60日新低'),
    ChangeTypeItem(8216, '60日大幅下跌'),

    // ── 本地监控（价格/量能急速波动） ──
    // ChangeTypeItem(9001, '10秒急速波动'),
  
    // ChangeTypeItem(9004, '10秒量能异动'),
    ChangeTypeItem(9005, '30秒量能异动'),
    ChangeTypeItem(9006, '60秒量能异动'),
  ];

  /// 根据 ID 查找类型名称
  static String getName(int id) {
    return allTypes.firstWhere(
      (t) => t.id == id,
      orElse: () => const ChangeTypeItem(0, '未知'),
    ).name;
  }

  /// 默认选中的异动类型 ID（常用监控类型）
  ///
  /// 注意：本地量能异动（9005/9006）默认不勾选，需要用户手动在设置页开启。
  static Set<int> get defaultMonitorIds =>
      {8201, 8193, 64, 8204, 8194, 128, 8203, 8202, 9002, 9003};

  /// 所有有效类型 ID（用于过滤旧缓存中已下架/无效的 ID）
  static Set<int> get validTypeIds =>
      allTypes.map((t) => t.id).toSet();

  /// 全量类型 ID（用于判断是否全选）
  static Set<int> get defaultSelectedIds =>
      allTypes.map((t) => t.id).toSet();

  /// shared_preferences 的存储 key
  static const String storageKey = 'monitored_change_types';
}
