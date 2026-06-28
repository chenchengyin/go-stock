import 'dart:js' as js;

void showWebNotification(String title, String body) {
  try {
    js.context['triggerNotification'].apply([title, body]);
  } catch (e) {
    print('Web notification error: $e');
  }
}