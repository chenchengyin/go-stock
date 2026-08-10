package com.colin.trading

import android.os.Bundle
import io.flutter.embedding.android.FlutterActivity

class MainActivity : FlutterActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // 应用启动时开启桌面组件定时刷新（如果用户已添加组件）
        StockWidgetUpdateReceiver.start(this)
    }
}
