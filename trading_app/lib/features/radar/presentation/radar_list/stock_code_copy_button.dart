import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/utils/stock_launcher.dart';

class StockCodeCopyButton extends StatelessWidget {
  const StockCodeCopyButton({super.key, required this.code});

  final String code;

  Future<void> _copy(BuildContext context) async {
    final normalizedCode = StockLauncher.normalizeStockCode(code);
    if (normalizedCode.isEmpty) return;
    await Clipboard.setData(ClipboardData(text: normalizedCode));
    if (!context.mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('股票代码已复制')));
  }

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: '复制股票代码',
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: () => _copy(context),
          borderRadius: BorderRadius.circular(6),
          child: Padding(
            padding: EdgeInsets.all(2),
            child: Icon(Icons.copy, size: 18, color: AppColors.textTertiary),
          ),
        ),
      ),
    );
  }
}
