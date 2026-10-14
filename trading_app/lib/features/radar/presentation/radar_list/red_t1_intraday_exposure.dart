import 'dart:async';

import 'package:flutter/rendering.dart';
import 'package:flutter/widgets.dart';
import 'package:visibility_detector/visibility_detector.dart';
import 'package:trading_app/features/radar/data/red_t1_intraday_repository.dart';

typedef RedT1IntradayExposureBuilder =
    Widget Function(
      BuildContext context,
      RedT1IntradaySeries? series,
      bool loading,
      String? errorMessage,
      bool waitingForExposure,
    );

class RedT1IntradayExposure extends StatefulWidget {
  const RedT1IntradayExposure({
    super.key,
    required this.detectorId,
    required this.load,
    required this.builder,
  });

  final String detectorId;
  final Future<RedT1IntradaySeries> Function() load;
  final RedT1IntradayExposureBuilder builder;

  @override
  State<RedT1IntradayExposure> createState() => _RedT1IntradayExposureState();
}

class _RedT1IntradayExposureState extends State<RedT1IntradayExposure> {
  static const double _minimumVisibleFraction = 0.25;
  static const Duration _minimumExposure = Duration(milliseconds: 300);
  static const Duration _automaticRetryDelay = Duration(seconds: 1);

  Timer? _exposureTimer;
  Timer? _retryTimer;
  final GlobalKey _geometryKey = GlobalKey();
  final Set<ScrollPosition> _scrollPositions = <ScrollPosition>{};
  bool _visibilityCheckScheduled = false;
  Size? _screenSize;
  double _visibleFraction = 0;
  Future<RedT1IntradaySeries>? _future;
  int _automaticRetryCount = 0;
  bool _retryPending = false;

  @override
  void initState() {
    super.initState();
    _scheduleVisibilityCheck();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _screenSize = MediaQuery.sizeOf(context);
    final nextPositions = <ScrollPosition>{
      if (Scrollable.maybeOf(context) case final scrollable?)
        scrollable.position,
      if (Scrollable.maybeOf(context, axis: Axis.horizontal)
          case final scrollable?)
        scrollable.position,
    };
    if (_scrollPositions.length != nextPositions.length ||
        !_scrollPositions.containsAll(nextPositions)) {
      for (final position in _scrollPositions) {
        position.removeListener(_onScrollPositionChanged);
      }
      _scrollPositions
        ..clear()
        ..addAll(nextPositions);
      for (final position in _scrollPositions) {
        position.addListener(_onScrollPositionChanged);
      }
    }
    _scheduleVisibilityCheck();
  }

  void _onScrollPositionChanged() => _scheduleVisibilityCheck();

  void _scheduleVisibilityCheck() {
    if (_visibilityCheckScheduled) return;
    _visibilityCheckScheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _visibilityCheckScheduled = false;
      if (mounted) _setVisibleFraction(_measureVisibleFraction());
    });
  }

  double _measureVisibleFraction() {
    final screenSize = _screenSize;
    final renderObject = _geometryKey.currentContext?.findRenderObject();
    if (screenSize == null ||
        renderObject is! RenderBox ||
        !renderObject.attached ||
        !renderObject.hasSize) {
      return 0;
    }
    final viewport = RenderAbstractViewport.maybeOf(renderObject);
    if (viewport is! RenderBox) {
      return 0;
    }
    final viewportBox = viewport as RenderBox;
    if (!viewportBox.attached || !viewportBox.hasSize) return 0;

    final widgetRect = MatrixUtils.transformRect(
      renderObject.getTransformTo(null),
      renderObject.paintBounds,
    );
    final viewportRect = MatrixUtils.transformRect(
      viewportBox.getTransformTo(null),
      viewportBox.paintBounds,
    );
    final screenRect = Offset.zero & screenSize;
    final visibleRect = widgetRect
        .intersect(viewportRect)
        .intersect(screenRect);
    if (widgetRect.isEmpty || visibleRect.isEmpty) return 0;
    return (visibleRect.width * visibleRect.height) /
        (widgetRect.width * widgetRect.height);
  }

  void _setVisibleFraction(double visibleFraction) {
    _visibleFraction = visibleFraction;
    if (_future != null && !_retryPending) return;
    if (_visibleFraction >= _minimumVisibleFraction) {
      _exposureTimer ??= Timer(_minimumExposure, () {
        _exposureTimer = null;
        if (!mounted ||
            (_future != null && !_retryPending) ||
            _visibleFraction < _minimumVisibleFraction) {
          return;
        }
        _startLoad(automaticRetry: _future != null);
      });
    } else {
      _exposureTimer?.cancel();
      _exposureTimer = null;
    }
  }

  void _onVisibilityChanged(VisibilityInfo info) {
    _setVisibleFraction(info.visibleFraction);
  }

  void _startLoad({bool automaticRetry = false}) {
    if (automaticRetry) {
      _automaticRetryCount++;
      _retryPending = false;
    }
    final next = Future<RedT1IntradaySeries>.sync(widget.load);
    setState(() {
      _future = next;
    });
    next.then<void>(
      (series) {
        if (!mounted) return;
        if (series.bars.length < 2 || series.prevClose <= 0) {
          _scheduleAutomaticRetry();
        } else {
          _retryTimer?.cancel();
          _retryTimer = null;
          _retryPending = false;
        }
      },
      onError: (Object _, StackTrace __) {
        if (mounted) _scheduleAutomaticRetry();
      },
    );
  }

  void _scheduleAutomaticRetry() {
    if (_automaticRetryCount > 0 || _retryTimer != null) return;
    _retryTimer = Timer(_automaticRetryDelay, () {
      _retryTimer = null;
      if (!mounted) return;
      if (_visibleFraction >= _minimumVisibleFraction) {
        _startLoad(automaticRetry: true);
      } else {
        _retryPending = true;
      }
    });
  }

  @override
  void didUpdateWidget(covariant RedT1IntradayExposure oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.detectorId != widget.detectorId) {
      _exposureTimer?.cancel();
      _exposureTimer = null;
      _retryTimer?.cancel();
      _retryTimer = null;
      _visibleFraction = 0;
      _future = null;
      _automaticRetryCount = 0;
      _retryPending = false;
      _scheduleVisibilityCheck();
    }
  }

  @override
  void dispose() {
    _exposureTimer?.cancel();
    _retryTimer?.cancel();
    for (final position in _scrollPositions) {
      position.removeListener(_onScrollPositionChanged);
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => VisibilityDetector(
    key: ValueKey(widget.detectorId),
    onVisibilityChanged: _onVisibilityChanged,
    child: KeyedSubtree(
      key: _geometryKey,
      child: FutureBuilder<RedT1IntradaySeries>(
        future: _future,
        builder: (context, snapshot) => widget.builder(
          context,
          snapshot.data,
          _future != null && snapshot.connectionState != ConnectionState.done,
          snapshot.hasError ? redT1IntradayErrorMessage(snapshot.error!) : null,
          _future == null,
        ),
      ),
    ),
  );
}
