package flutter_api

import "testing"

func TestRegisteredModulesContainsCurrentRadarTabs(t *testing.T) {
	got := RegisteredModules()
	want := []string{
		"radar.monitored",
		"radar.red_strategy",
		"radar.purple_strategy",
		"radar.main_strategy",
		"radar.blue_strategy",
		"radar.watch_changes",
		"radar.all_changes",
	}

	if len(got) != len(want) {
		t.Fatalf("module count = %d, want %d", len(got), len(want))
	}

	seen := map[string]bool{}
	for index, module := range got {
		if module.Code != want[index] || seen[module.Code] {
			t.Fatalf("module[%d] = %+v", index, module)
		}
		seen[module.Code] = true
	}
}

func TestRegisteredStrategyModulesHaveSelectionDispatch(t *testing.T) {
	if err := validateRegisteredModuleContracts(); err != nil {
		t.Fatalf("registered module contract: %v", err)
	}
}

func TestRegisteredModuleContractsRejectMissingStrategyDispatch(t *testing.T) {
	original := registeredModules
	registeredModules = append(append([]ModuleDefinition(nil), original...), ModuleDefinition{
		Code:       "radar.future_strategy",
		Name:       "未来策略",
		Client:     "flutter_web",
		Placement:  "radar_tab",
		Sort:       70,
		AccessMode: ModuleAccessAllowlist,
	})
	t.Cleanup(func() {
		registeredModules = original
	})

	if err := validateRegisteredModuleContracts(); err == nil {
		t.Fatal("expected missing strategy dispatch to be rejected")
	}
}

func TestMigrateAuthTablesCreatesPermissionTablesAndIndexes(t *testing.T) {
	dao := newAuthTestDB(t)

	if err := MigrateAuthTables(dao); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if err := MigrateAuthTables(dao); err != nil {
		t.Fatalf("second migration: %v", err)
	}

	if !dao.Migrator().HasTable(&ModuleUserGrant{}) ||
		!dao.Migrator().HasTable(&AdminSession{}) {
		t.Fatal("permission tables were not created")
	}

	if !dao.Migrator().HasIndex(&ModuleUserGrant{}, "idx_module_user_grants_unique") {
		t.Fatal("unique grant index was not created")
	}
}
