import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';

/// 语音播报管理页面
class VoiceManagerPage extends StatelessWidget {
  const VoiceManagerPage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('语音播报', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600)),
        backgroundColor: Colors.white,
        foregroundColor: Colors.black,
        elevation: 0.5,
      ),
      backgroundColor: AppColors.backgroundColor,
      body: Consumer<VoiceAnnouncementViewModel>(
        builder: (_, vm, __) {
          return ListView(
            padding: const EdgeInsets.all(16),
            children: [
              // 开关卡片
              _buildSwitchCard(context, vm),
              const SizedBox(height: 12),
              // 语速调节
              _buildSpeechRateCard(context, vm),
              const SizedBox(height: 12),
              // 当前播报
              if (vm.isSpeaking && vm.currentSpeakingText != null)
                _buildSpeakingCard(vm.currentSpeakingText!),
              if (vm.isSpeaking) const SizedBox(height: 12),
              // 操作按钮
              _buildActionButtons(context, vm),
              const SizedBox(height: 12),
              // 待播报队列
              _buildQueueHeader(vm.queue.length),
              const SizedBox(height: 8),
              _buildQueueList(vm),
              const SizedBox(height: 16),
              // 已播报队列
              _buildSpokenQueueHeader(vm.spokenQueue.length, vm),
              const SizedBox(height: 8),
              _buildSpokenQueueList(vm),
            ],
          );
        },
      ),
    );
  }

  Widget _buildSwitchCard(BuildContext context, VoiceAnnouncementViewModel vm) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      decoration: BoxDecoration(
        color: AppColors.cardBg,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: Colors.grey.shade200),
      ),
      child: Row(
        children: [
          Icon(
            vm.enabled ? Icons.volume_up : Icons.volume_off,
            color: vm.enabled ? AppColors.buttonPrimary : Colors.grey,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  '异动自动播报',
                  style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: 2),
                Text(
                  vm.enabled ? '有新异动时自动语音播报' : '已暂停自动语音播报',
                  style: TextStyle(fontSize: 12, color: AppColors.textSecondary),
                ),
              ],
            ),
          ),
          Switch(
            value: vm.enabled,
            activeThumbColor: AppColors.buttonPrimary,
            onChanged: (value) => vm.setEnabled(value),
          ),
        ],
      ),
    );
  }

  Widget _buildSpeechRateCard(BuildContext context, VoiceAnnouncementViewModel vm) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      decoration: BoxDecoration(
        color: AppColors.cardBg,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: Colors.grey.shade200),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text(
                '播报语速',
                style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
              ),
              Text(
                vm.speechRate.toStringAsFixed(2),
                style: TextStyle(fontSize: 14, color: AppColors.buttonPrimary, fontWeight: FontWeight.w600),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Text('慢', style: TextStyle(fontSize: 12, color: Colors.grey[500])),
              Expanded(
                child: Slider(
                  value: vm.speechRate,
                  min: 0.3,
                  max: 1.5,
                  divisions: 12,
                  label: vm.speechRate.toStringAsFixed(2),
                  activeColor: AppColors.buttonPrimary,
                  onChanged: (value) => vm.setSpeechRate(value),
                ),
              ),
              Text('快', style: TextStyle(fontSize: 12, color: Colors.grey[500])),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildSpeakingCard(String text) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppColors.cardBg,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.buttonPrimary.withValues(alpha: 0.3)),
      ),
      child: Row(
        children: [
          SizedBox(
            width: 24,
            height: 24,
            child: CircularProgressIndicator(
              strokeWidth: 2,
              valueColor: AlwaysStoppedAnimation<Color>(AppColors.buttonPrimary),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '正在播报',
                  style: TextStyle(fontSize: 12, color: AppColors.buttonPrimary, fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: 4),
                Text(
                  text,
                  style: const TextStyle(fontSize: 14),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildActionButtons(BuildContext context, VoiceAnnouncementViewModel vm) {
    return Row(
      children: [
        Expanded(
          child: _ActionButton(
            icon: Icons.stop,
            label: '停止播报',
            color: Colors.orange,
            onPressed: vm.isSpeaking ? () => vm.stopSpeaking() : null,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: _ActionButton(
            icon: Icons.clear_all,
            label: '清空队列',
            color: AppColors.buttonPrimary,
            onPressed: vm.queue.isNotEmpty ? () => vm.clearQueue() : null,
          ),
        ),
        // const SizedBox(width: 12),
        // Expanded(
        //   child: _ActionButton(
        //     icon: Icons.replay,
        //     label: '重新测试',
        //     color: Colors.green,
        //     onPressed: () => vm.restartTest(),
        //   ),
        // ),
      ],
    );
  }

  Widget _buildQueueHeader(int count) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          '待播报队列',
          style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: Colors.grey[800]),
        ),
        Text(
          '共 $count 条',
          style: TextStyle(fontSize: 12, color: AppColors.textSecondary),
        ),
      ],
    );
  }

  Widget _buildQueueList(VoiceAnnouncementViewModel vm) {
    if (vm.queue.isEmpty) {
      return Container(
        padding: const EdgeInsets.all(32),
        decoration: BoxDecoration(
          color: AppColors.cardBg,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: Colors.grey.shade200),
        ),
        child: Center(
          child: Column(
            children: [
              Icon(Icons.queue_music, size: 40, color: Colors.grey[400]),
              const SizedBox(height: 8),
              Text(
                '暂无待播报内容',
                style: TextStyle(fontSize: 13, color: Colors.grey[600]),
              ),
            ],
          ),
        ),
      );
    }

    return Container(
      decoration: BoxDecoration(
        color: AppColors.cardBg,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: Colors.grey.shade200),
      ),
      child: ListView.separated(
        shrinkWrap: true,
        physics: const NeverScrollableScrollPhysics(),
        itemCount: vm.queue.length,
        separatorBuilder: (_, __) => Divider(height: 1, color: Colors.grey.shade200),
        itemBuilder: (_, index) {
          final item = vm.queue[index];
          return ListTile(
            dense: true,
            leading: Text(
              '${index + 1}',
              style: TextStyle(fontSize: 12, color: Colors.grey[500]),
            ),
            title: Text(
              item.text,
              style: const TextStyle(fontSize: 13),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            subtitle: Text(
              _formatTime(item.enqueuedAt),
              style: TextStyle(fontSize: 11, color: Colors.grey[500]),
            ),
          );
        },
      ),
    );
  }

  String _formatTime(DateTime time) {
    return '${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}:${time.second.toString().padLeft(2, '0')}';
  }

  Widget _buildSpokenQueueHeader(int count, VoiceAnnouncementViewModel vm) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          '已播报队列',
          style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: Colors.grey[800]),
        ),
        Row(
          children: [
            Text(
              '共 $count 条',
              style: TextStyle(fontSize: 12, color: AppColors.textSecondary),
            ),
            const SizedBox(width: 8),
            GestureDetector(
              onTap: count > 0 ? () => vm.clearSpokenQueue() : null,
              child: Text(
                '清空',
                style: TextStyle(
                  fontSize: 12,
                  color: count > 0 ? AppColors.buttonPrimary : Colors.grey[400],
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
          ],
        ),
      ],
    );
  }

  Widget _buildSpokenQueueList(VoiceAnnouncementViewModel vm) {
    if (vm.spokenQueue.isEmpty) {
      return Container(
        padding: const EdgeInsets.all(32),
        decoration: BoxDecoration(
          color: AppColors.cardBg,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: Colors.grey.shade200),
        ),
        child: Center(
          child: Column(
            children: [
              Icon(Icons.check_circle_outline, size: 40, color: Colors.grey[400]),
              const SizedBox(height: 8),
              Text(
                '暂无已播报内容',
                style: TextStyle(fontSize: 13, color: Colors.grey[600]),
              ),
            ],
          ),
        ),
      );
    }

    return Container(
      decoration: BoxDecoration(
        color: AppColors.cardBg,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: Colors.grey.shade200),
      ),
      child: ListView.separated(
        shrinkWrap: true,
        physics: const NeverScrollableScrollPhysics(),
        itemCount: vm.spokenQueue.length,
        separatorBuilder: (_, __) => Divider(height: 1, color: Colors.grey.shade200),
        itemBuilder: (_, index) {
          final item = vm.spokenQueue[index];
          return ListTile(
            dense: true,
            leading: Icon(Icons.done, size: 16, color: Colors.grey[400]),
            title: Text(
              item.text,
              style: TextStyle(fontSize: 13, color: Colors.grey[700]),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            subtitle: Text(
              _formatTime(item.enqueuedAt),
              style: TextStyle(fontSize: 11, color: Colors.grey[500]),
            ),
          );
        },
      ),
    );
  }
}

class _ActionButton extends StatelessWidget {
  const _ActionButton({
    required this.icon,
    required this.label,
    required this.color,
    required this.onPressed,
  });

  final IconData icon;
  final String label;
  final Color color;
  final VoidCallback? onPressed;

  @override
  Widget build(BuildContext context) {
    return ElevatedButton.icon(
      onPressed: onPressed,
      icon: Icon(icon, size: 18),
      label: Text(label),
      style: ElevatedButton.styleFrom(
        backgroundColor: color,
        foregroundColor: Colors.white,
        disabledBackgroundColor: Colors.grey[300],
        disabledForegroundColor: Colors.grey[500],
        elevation: 0,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
        padding: const EdgeInsets.symmetric(vertical: 12),
      ),
    );
  }
}
