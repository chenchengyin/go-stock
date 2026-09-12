import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_module_definitions.dart';

void main() {
  test(
    'radar catalog has seven independent stable modules in server order',
    () {
      expect(radarModuleDefinitions.map((item) => item.code), [
        'radar.monitored',
        'radar.red_strategy',
        'radar.purple_strategy',
        'radar.main_strategy',
        'radar.blue_strategy',
        'radar.watch_changes',
        'radar.all_changes',
      ]);
      expect(
        radarModuleDefinitions.where(
          (item) => item.accessMode == ModuleAccessMode.public,
        ),
        hasLength(3),
      );
      expect(
        radarModuleDefinitions.where(
          (item) => item.accessMode == ModuleAccessMode.userAllowlist,
        ),
        hasLength(4),
      );
      final red = radarModuleDefinitions[1];
      expect(red.name, '红策');
      expect(red.sort, 15);
      expect(red.contentKind, RadarContentKind.redStrategy);
    },
  );
}
