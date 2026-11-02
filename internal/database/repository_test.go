package database

import (
	"strings"
	"testing"
)

func TestSelectColumnsIncludeNewFields(t *testing.T) {
	if !strings.Contains(selectColumns, "source") {
		t.Fatalf("selectColumns missing %q: %s", "source", selectColumns)
	}
	if !strings.Contains(selectColumns, "verification_status") {
		t.Fatalf("selectColumns missing %q: %s", "verification_status", selectColumns)
	}
	if !strings.Contains(selectColumns, "source_url") {
		t.Fatalf("selectColumns missing %q: %s", "source_url", selectColumns)
	}
	for _, col := range []string{"specialty", "experience"} {
		if !strings.Contains(selectColumns, col) {
			t.Fatalf("selectColumns missing %q: %s", col, selectColumns)
		}
	}
	if !strings.Contains(selectColumns, "source_checked_at") {
		t.Fatalf("selectColumns missing %q: %s", "source_checked_at", selectColumns)
	}
}
