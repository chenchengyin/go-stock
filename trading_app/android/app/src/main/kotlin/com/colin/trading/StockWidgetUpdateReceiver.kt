package com.colin.trading

import android.app.AlarmManager
import android.app.PendingIntent
import android.appwidget.AppWidgetManager
import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.os.SystemClock

/**
 * 桌面股票组件定时刷新接收器
 *
 * 通过 AlarmManager 每 1 分钟触发一次，主动刷新桌面组件。
 * 注意：小米等国产 ROM 在息屏或应用被清理后可能无法准时触发。
 */
class StockWidgetUpdateReceiver : BroadcastReceiver() {

    override fun onReceive(context: Context, intent: Intent?) {
        val manager = AppWidgetManager.getInstance(context)
        val componentName = ComponentName(context, StockWidget::class.java)
        val appWidgetIds = manager.getAppWidgetIds(componentName)
        if (appWidgetIds.isNotEmpty()) {
            StockWidget().onUpdate(context, manager, appWidgetIds)
        }
    }

    companion object {
        private const val REQUEST_CODE = 1001
        private const val INTERVAL_MILLIS = 60_000L // 1 分钟

        /**
         * 启动定时刷新。如果已经存在相同 PendingIntent，会先取消再重新设置。
         */
        fun start(context: Context) {
            stop(context)

            val alarmManager = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
            val intent = Intent(context, StockWidgetUpdateReceiver::class.java)
            val pendingIntent = PendingIntent.getBroadcast(
                context,
                REQUEST_CODE,
                intent,
                PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
            )

            alarmManager.setRepeating(
                AlarmManager.ELAPSED_REALTIME_WAKEUP,
                SystemClock.elapsedRealtime() + INTERVAL_MILLIS,
                INTERVAL_MILLIS,
                pendingIntent,
            )
        }

        /**
         * 停止定时刷新
         */
        fun stop(context: Context) {
            val alarmManager = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
            val intent = Intent(context, StockWidgetUpdateReceiver::class.java)
            val pendingIntent = PendingIntent.getBroadcast(
                context,
                REQUEST_CODE,
                intent,
                PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
            )
            alarmManager.cancel(pendingIntent)
            pendingIntent.cancel()
        }
    }
}
