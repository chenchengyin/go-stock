package com.colin.trading

import android.app.PendingIntent
import android.appwidget.AppWidgetManager
import android.appwidget.AppWidgetProvider
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.widget.RemoteViews

/**
 * 桌面股票组件 Provider
 *
 * 负责读取 home_widget 插件写入 SharedPreferences 的行情数据，
 * 并渲染到桌面组件上。点击组件会打开主应用。
 */
class StockWidget : AppWidgetProvider() {

    override fun onUpdate(
        context: Context,
        appWidgetManager: AppWidgetManager,
        appWidgetIds: IntArray,
    ) {
        // home_widget 默认使用 "home_widget_data" 作为 SharedPreferences 名称
        val prefs = context.getSharedPreferences("home_widget_data", Context.MODE_PRIVATE)

        val name = prefs.getString("stock_name", "--") ?: "--"
        val price = prefs.getString("stock_price", "--") ?: "--"
        val changeRate = prefs.getString("stock_change_rate", "0.00%") ?: "0.00%"

        // 根据涨跌设置颜色：涨红跌绿
        val isUp = changeRate.startsWith("+")
        val colorRes = if (isUp) R.color.stock_red else R.color.stock_green

        for (appWidgetId in appWidgetIds) {
            val views = RemoteViews(context.packageName, R.layout.stock_widget).apply {
                setTextViewText(R.id.tv_name, name)
                setTextViewText(R.id.tv_price, price)
                setTextViewText(R.id.tv_change_rate, changeRate)
                setTextColor(R.id.tv_change_rate, context.getColor(colorRes))
                setOnClickPendingIntent(R.id.widget_container, createOpenAppPendingIntent(context))
            }
            appWidgetManager.updateAppWidget(appWidgetId, views)
        }
    }

    override fun onEnabled(context: Context) {
        super.onEnabled(context)
        // 用户首次添加组件时启动定时刷新
        StockWidgetUpdateReceiver.start(context)
    }

    override fun onDisabled(context: Context) {
        super.onDisabled(context)
        // 最后一个组件被移除时停止定时刷新
        val manager = AppWidgetManager.getInstance(context)
        val ids = manager.getAppWidgetIds(ComponentName(context, StockWidget::class.java))
        if (ids.isEmpty()) {
            StockWidgetUpdateReceiver.stop(context)
        }
    }

    /**
     * 创建点击组件打开应用的 PendingIntent
     */
    private fun createOpenAppPendingIntent(context: Context): PendingIntent {
        val intent = Intent(context, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP
        }
        return PendingIntent.getActivity(
            context,
            0,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
    }
}
