import 'package:flutter/foundation.dart';

import '../../../shared/view_state.dart';
import '../data/short_term_emotion_repository.dart';
import '../domain/short_term_emotion_models.dart';

class ShortTermEmotionViewModel extends ChangeNotifier {
  ShortTermEmotionViewModel(this._repository);

  final ShortTermEmotionRepository _repository;

  ViewState state = const ViewState();
  ShortTermEmotion? emotion;

  Future<void> load() async {
    if (state.isLoading) return;
    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();

    try {
      emotion = await _repository.fetch();
      state = const ViewState(status: ViewStatus.ready);
    } catch (error) {
      state = ViewState(status: ViewStatus.error, message: error.toString());
    }
    notifyListeners();
  }

  Future<void> refresh() async {
    await load();
  }
}
