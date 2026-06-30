import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/radar/domain/change_type_config.dart';

/// 监控异动设置页面
class MonitorSettingsPage extends StatefulWidget {
  const MonitorSettingsPage({super.key});

  @override
  State<MonitorSettingsPage> createState() => _MonitorSettingsPageState();
}

class _MonitorSettingsPageState extends State<MonitorSettingsPage> {
  Set<int> _selected = {};
  bool _loading = true;

  /// 常用异动类型 ID（名称显示红色）
  static const _commonTypeIds = {8201, 8193, 64, 8204, 8194, 128, 8203, 8202};

  @override
  void initState() {
    super.initState();
    _loadSettings();
  }

  Future<void> _loadSettings() async {
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString(ChangeTypeConfig.storageKey);
    if (raw != null && raw.isNotEmpty) {
      _selected = raw.split(',').map(int.parse).toSet();
    } else {
      _selected = Set.from(ChangeTypeConfig.defaultMonitorIds);
    }
    if (mounted) setState(() => _loading = false);
  }

  bool get _isAllSelected =>
      _selected.length == ChangeTypeConfig.allTypes.length;

  void _toggleAll() {
    setState(() {
      if (_isAllSelected) {
        _selected = {};
      } else {
        _selected = ChangeTypeConfig.allTypes.map((t) => t.id).toSet();
      }
    });
  }

  void _toggleItem(int id) {
    setState(() {
      if (_selected.contains(id)) {
        _selected.remove(id);
      } else {
        _selected.add(id);
      }
    });
  }

  Future<void> _save() async {
    if (_selected.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('请至少选择一个异动类型')),
      );
      return;
    }
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(
      ChangeTypeConfig.storageKey,
      _selected.join(','),
    );
    if (mounted) Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('监控异动设置'),
        backgroundColor: Colors.white,
        foregroundColor: Colors.black,
        elevation: 0.5,
      ),
      backgroundColor: AppColors.backgroundColor,
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : Column(
              children: [
                // 类型列表
                Expanded(
                  child: Container(
                    
                    margin: const EdgeInsets.symmetric(
                          horizontal: 16, vertical: 12),
                    child: ListView.separated(
                      itemCount: ChangeTypeConfig.allTypes.length,
                      separatorBuilder: (_, __) =>
                          const Divider(height: 0.5, indent: 0),
                      itemBuilder: (context, index) {
                        final item = ChangeTypeConfig.allTypes[index];
                        final checked = _selected.contains(item.id);
                        return Container(
                          color: AppColors.cardBg,
                          child: CheckboxListTile(
                                                
                            contentPadding: EdgeInsets.all(4),
                            title: Text(
                              item.name,
                                              
                              style: TextStyle(
                                fontSize: 16,
                                color: _commonTypeIds.contains(item.id)
                                    ? Colors.red
                                    : null,
                              ),
                            ),
                            // subtitle: Text(
                            //   '类型编码: ${item.id}',
                            //   style: TextStyle(
                            //       fontSize: 12, color: Colors.grey[500]),
                            // ),
                            value: checked,
                            activeColor: AppColors.brand,
                            onChanged: (_) => _toggleItem(item.id),
                            dense: true,
                            controlAffinity: ListTileControlAffinity.leading,
                          ),
                        );
                      },
                    ),
                  ),
                ),

                // 底部操作栏
                Container(
                  padding: const EdgeInsets.symmetric(
                      horizontal: 40, vertical: 16),
                  decoration: BoxDecoration(
                    color: Colors.white,
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withValues(alpha: 0.05),
                        blurRadius: 8,
                        offset: const Offset(0, -2),
                      ),
                    ],
                  ),
                  child: Row(
                    children: [
                      // 全选 / 取消全选
                      Expanded(
                        child: OutlinedButton(
                          onPressed: _toggleAll,
                          style: OutlinedButton.styleFrom(
                            padding:
                                const EdgeInsets.symmetric(vertical: 14),
                            side: BorderSide(
                                color: _isAllSelected
                                    ? Colors.grey
                                    : AppColors.brand),
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(8),
                            ),
                          ),
                          child: Text(
                            _isAllSelected ? '取消全选' : '全选',
                            style: TextStyle(
                              fontSize: 15,
                              color: _isAllSelected
                                  ? Colors.grey
                                  : AppColors.brand,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: 24),
                      // 保存
                      Expanded(
                        child: ElevatedButton(
                          onPressed: _save,
                          style: ElevatedButton.styleFrom(
                            backgroundColor: AppColors.brand,
                            foregroundColor: Colors.white,
                            padding:
                                const EdgeInsets.symmetric(vertical: 14),
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(8),
                            ),
                          ),
                          child: const Text(
                            '保存',
                            style: TextStyle(
                              fontSize: 15,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
    );
  }
}
