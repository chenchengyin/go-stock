String permission() => 'unsupported';

bool pairingAvailable() => false;

bool localReminderEnabled() => false;

Future<String> pairLocalReminder(String accessToken) async => 'unsupported';

Future<String> requestPermission() async => 'unsupported';

bool show({
  required String date,
  required String userId,
  required int count,
  required String body,
  required void Function() onTap,
}) => false;
