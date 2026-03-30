package tools

import (
	"testing"
)

func TestGetAllDataTools(t *testing.T) {
	tools := GetAllDataTools()
	t.Logf("Total tools count: %d", len(tools))

	toolNames := make(map[string]int)
	for i, tool := range tools {
		info, err := tool.Info(nil)
		if err != nil {
			t.Errorf("Tool %d: failed to get info: %v", i, err)
			continue
		}
		t.Logf("Tool %d: %s - %s", i+1, info.Name, info.Desc)

		if count, exists := toolNames[info.Name]; exists {
			t.Errorf("Duplicate tool name found: %s (count: %d)", info.Name, count+1)
		}
		toolNames[info.Name]++
	}

	t.Log("\n=== Tool List ===")
	for name, count := range toolNames {
		if count > 1 {
			t.Errorf("Duplicate tool: %s (count: %d)", name, count)
		}
	}
}
