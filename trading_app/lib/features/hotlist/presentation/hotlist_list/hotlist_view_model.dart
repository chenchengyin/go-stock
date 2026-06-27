import 'package:flutter/foundation.dart';

import '../../../../shared/view_state.dart';
import '../../data/hotlist_repository.dart';
import '../../domain/hotlist_models.dart';

class HotlistViewModel extends ChangeNotifier {
  HotlistViewModel(this._repository);

  final HotlistRepository _repository;

  ViewState state = const ViewState();
  List<HotStock> stocks = [];

  Future<void> load() async {
    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();
    try {
      stocks = await _repository.load();
      state = const ViewState(status: ViewStatus.ready);
    } catch (error) {
      state = ViewState(status: ViewStatus.error, message: error.toString());
    }
    notifyListeners();
  }
}

