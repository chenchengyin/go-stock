import SwiftUI
import WidgetKit

struct StockWidgetEntry: TimelineEntry {
  let date: Date
}

struct StockWidgetProvider: TimelineProvider {
  func placeholder(in context: Context) -> StockWidgetEntry {
    StockWidgetEntry(date: Date())
  }

  func getSnapshot(in context: Context, completion: @escaping (StockWidgetEntry) -> Void) {
    completion(placeholder(in: context))
  }

  func getTimeline(in context: Context, completion: @escaping (Timeline<StockWidgetEntry>) -> Void) {
    completion(Timeline(entries: [placeholder(in: context)], policy: .never))
  }
}

@main
struct StockWidget: Widget {
  let kind = "TradingRadarStockWidget"

  var body: some WidgetConfiguration {
    StaticConfiguration(kind: kind, provider: StockWidgetProvider()) { _ in
      // Keep the first version empty; market data will be added separately.
      if #available(macOS 14.0, *) {
        Color.clear
          .containerBackground(.fill.tertiary, for: .widget)
      } else {
        Color.clear
          .background()
      }
    }
    .configurationDisplayName("交易雷达")
    .description("看盘小组件，内容将在后续版本加入。")
    .supportedFamilies([.systemSmall, .systemMedium, .systemLarge])
  }
}
