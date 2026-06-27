import 'package:flutter/foundation.dart';

import '../../../../shared/view_state.dart';
import '../../data/radar_repository.dart';
import '../../domain/radar_models.dart';

class RadarViewModel extends ChangeNotifier {
  RadarViewModel(this._repository);

  final RadarRepository _repository;

  ViewState state = const ViewState();
  List<WatchStock> stocks = [];

  Future<void> load() async {
    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();
    try {
      stocks = await _repository.loadWatchStocks();
      state = const ViewState(status: ViewStatus.ready);
    } catch (error) {
      state = ViewState(status: ViewStatus.error, message: error.toString());
    }
    notifyListeners();
  }
}

