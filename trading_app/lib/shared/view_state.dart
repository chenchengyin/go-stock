enum ViewStatus { idle, loading, ready, error }

class ViewState {
  const ViewState({this.status = ViewStatus.idle, this.message = ''});

  final ViewStatus status;
  final String message;

  bool get isLoading => status == ViewStatus.loading;

  ViewState copyWith({ViewStatus? status, String? message}) {
    return ViewState(
      status: status ?? this.status,
      message: message ?? this.message,
    );
  }
}

