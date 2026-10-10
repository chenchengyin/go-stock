import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_module_definitions.dart';

void main() {
  test('radar catalog has nine independent stable modules in server order', () {
    expect(radarModuleDefinitions.map((item) => item.code), [
      'radar.monitored',
      'radar.red_shared_gob_strategy',
      'radar.red_strategy',
      'radar.gold_strategy',
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
      hasLength(6),
    );
    final sharedRed = radarModuleDefinitions[1];
    expect(sharedRed.name, '红策（共享GOB）');
    expect(sharedRed.sort, 14);
    expect(sharedRed.contentKind, RadarContentKind.redStrategy);
    final red = radarModuleDefinitions[2];
    expect(red.name, '红策');
    expect(red.sort, 15);
    expect(red.contentKind, RadarContentKind.redStrategy);
    final gold = radarModuleDefinitions[3];
    expect(gold.name, '金策');
    expect(gold.sort, 18);
    expect(gold.contentKind, RadarContentKind.goldStrategy);
  });
}
