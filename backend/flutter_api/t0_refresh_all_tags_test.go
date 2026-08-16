package flutter_api

import (
	"os"
	"testing"
)

// TestRefreshAllSelectionTags 批量重算所有归档的前日标记。
// 用法: RUN_REFRESH_TAGS=1 go test ./flutter_api -run TestRefreshAllSelectionTags -count=1 -v
func TestRefreshAllSelectionTags(t *testing.T) {
	if os.Getenv("RUN_REFRESH_TAGS") != "1" {
		t.Skip("set RUN_REFRESH_TAGS=1 to batch refresh selection tags")
	}
	dates := listSelectionArchiveDates()
	if len(dates) == 0 {
		t.Fatal("no selection archives found")
	}
	for _, d := range dates {
		out, err := refreshSelectionTags(d)
		if err != nil {
			t.Errorf("%s: %v", d, err)
			continue
		}
		t.Logf("%s: tagged=%v missing=%v count=%v", d, out["tagged"], out["missing"], out["count"])
	}
}
