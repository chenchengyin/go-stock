import 'package:trading_app/features/radar/data/red_t1_intraday_repository.dart';
import 'strategy_chart_exposure.dart';

typedef RedT1IntradayExposureBuilder =
    StrategyChartExposureBuilder<RedT1IntradaySeries>;

/// Compatibility adapter with the original intraday retry and error semantics.
class RedT1IntradayExposure extends StrategyChartExposure<RedT1IntradaySeries> {
  const RedT1IntradayExposure({
    super.key,
    required super.detectorId,
    required super.load,
    required super.builder,
  }) : super(shouldRetry: _needsRetry, errorMessage: redT1IntradayErrorMessage);
}

bool _needsRetry(RedT1IntradaySeries series) =>
    series.bars.length < 2 || series.prevClose <= 0;
