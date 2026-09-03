package flutter_api

import "testing"

func TestRegisteredModulesContainsCurrentRadarTabs(t *testing.T) {
	got := RegisteredModules()
	want := []string{
		"radar.monitored",
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
