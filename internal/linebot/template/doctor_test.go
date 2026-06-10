package template

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/dfgh012316/popofinder/internal/database"
)

func TestDoctorBubbleShowsSourceAndURL(t *testing.T) {
	raw, err := DoctorsFlexMessage([]database.MedicalPersonnel{{
		Name:      "測試醫師",
		City:      "台北市",
		Hospital:  "測試醫院",
		Source:    sql.NullString{String: "public_report", Valid: true},
		SourceURL: sql.NullString{String: "https://example.com/doc", Valid: true},
	}})
	if err != nil {
		t.Fatalf("DoctorsFlexMessage error: %v", err)
	}
	s := string(raw)
	for _, want := range []string{"資料來源", "民眾回報", "來源連結", "https://example.com/doc"} {
		if !strings.Contains(s, want) {
			t.Fatalf("card JSON missing %q: %s", want, s)
		}
	}
	// 驗證狀態尚未實作醫師認領,不應顯示,以免讓人誤會全部資料未查證。
	if strings.Contains(s, "驗證狀態") {
		t.Fatalf("card JSON should not show 驗證狀態: %s", s)
	}
	// SourceURL should render as a tappable uri action, not raw text.
	for _, want := range []string{`"type":"uri"`, `"uri":"https://example.com/doc"`, "點此查看來源"} {
		if !strings.Contains(s, want) {
			t.Fatalf("card JSON missing link affordance %q: %s", want, s)
		}
	}
}

func TestDoctorBubbleOmitsSourceURLWhenEmpty(t *testing.T) {
	raw, err := DoctorsFlexMessage([]database.MedicalPersonnel{{
		Name:     "測試醫師",
		City:     "台北市",
		Hospital: "測試醫院",
		Source:   sql.NullString{String: "blog", Valid: true},
	}})
	if err != nil {
		t.Fatalf("DoctorsFlexMessage error: %v", err)
	}
	s := string(raw)
	if strings.Contains(s, "來源連結") {
		t.Fatalf("card JSON should omit 來源連結 when SourceURL empty: %s", s)
	}
	if !strings.Contains(s, "網路公開彙整") {
		t.Fatalf("card JSON missing 網路公開彙整: %s", s)
	}
}
