package template

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/dfgh012316/popofinder/internal/database"
)

func TestDoctorBubbleShowsSourceVerificationAndURL(t *testing.T) {
	raw, err := DoctorsFlexMessage([]database.MedicalPersonnel{{
		Name:               "測試醫師",
		City:               "台北市",
		Hospital:           "測試醫院",
		Source:             sql.NullString{String: "public_report", Valid: true},
		VerificationStatus: "verified",
		SourceURL:          sql.NullString{String: "https://example.com/doc", Valid: true},
	}})
	if err != nil {
		t.Fatalf("DoctorsFlexMessage error: %v", err)
	}
	s := string(raw)
	for _, want := range []string{"資料來源", "民眾回報", "驗證狀態", "已查證", "學歷佐證", "https://example.com/doc"} {
		if !strings.Contains(s, want) {
			t.Fatalf("card JSON missing %q: %s", want, s)
		}
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
		Name:               "測試醫師",
		City:               "台北市",
		Hospital:           "測試醫院",
		Source:             sql.NullString{String: "blog", Valid: true},
		VerificationStatus: "unverified",
	}})
	if err != nil {
		t.Fatalf("DoctorsFlexMessage error: %v", err)
	}
	s := string(raw)
	if strings.Contains(s, "學歷佐證") {
		t.Fatalf("card JSON should omit 學歷佐證 when SourceURL empty: %s", s)
	}
	if !strings.Contains(s, "網路公開彙整") {
		t.Fatalf("card JSON missing 網路公開彙整: %s", s)
	}
	if !strings.Contains(s, "尚未查證") {
		t.Fatalf("card JSON missing 尚未查證: %s", s)
	}
}
