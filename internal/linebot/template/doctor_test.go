package template

import (
	"database/sql"
	"strings"
	"testing"
	"time"

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

func TestDoctorBubbleShowsCheckedDate(t *testing.T) {
	raw, err := DoctorsFlexMessage([]database.MedicalPersonnel{{
		Name:            "測試醫師",
		City:            "台北市",
		Hospital:        "測試醫院",
		Source:          sql.NullString{String: "hospital_directory", Valid: true},
		SourceURL:       sql.NullString{String: "https://example.com/a", Valid: true},
		SourceCheckedAt: sql.NullTime{Time: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), Valid: true},
	}})
	if err != nil {
		t.Fatalf("DoctorsFlexMessage error: %v", err)
	}
	s := string(raw)
	for _, want := range []string{"醫療院所官網", "點此查看來源（2026-10-12 查到）"} {
		if !strings.Contains(s, want) {
			t.Fatalf("card JSON missing %q: %s", want, s)
		}
	}

	raw, err = DoctorsFlexMessage([]database.MedicalPersonnel{{
		Name:      "測試醫師",
		City:      "台北市",
		Hospital:  "測試醫院",
		Source:    sql.NullString{String: "hospital_directory", Valid: true},
		SourceURL: sql.NullString{String: "https://example.com/a", Valid: true},
	}})
	if err != nil {
		t.Fatalf("DoctorsFlexMessage error: %v", err)
	}
	s = string(raw)
	if !strings.Contains(s, `"text":"點此查看來源"`) {
		t.Fatalf("card JSON should show plain link text without a date: %s", s)
	}
	if strings.Contains(s, "查到") {
		t.Fatalf("card JSON should not show 查到 without SourceCheckedAt: %s", s)
	}
}

func TestDoctorBubbleCheckedDateNeedsURL(t *testing.T) {
	raw, err := DoctorsFlexMessage([]database.MedicalPersonnel{{
		Name:            "測試醫師",
		City:            "台北市",
		Hospital:        "測試醫院",
		Source:          sql.NullString{String: "hospital_directory", Valid: true},
		SourceCheckedAt: sql.NullTime{Time: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), Valid: true},
	}})
	if err != nil {
		t.Fatalf("DoctorsFlexMessage error: %v", err)
	}
	s := string(raw)
	if strings.Contains(s, "來源連結") || strings.Contains(s, "查到") {
		t.Fatalf("card JSON should not show 來源連結 or 查到 without SourceURL: %s", s)
	}
}

func TestDoctorBubbleHasNoTakedownButton(t *testing.T) {
	raw, err := DoctorsFlexMessage([]database.MedicalPersonnel{{
		Name:      "測試醫師",
		City:      "台北市",
		Hospital:  "測試醫院",
		Source:    sql.NullString{String: "blog", Valid: true},
		SourceURL: sql.NullString{String: "https://example.com/doc", Valid: true},
	}})
	if err != nil {
		t.Fatalf("DoctorsFlexMessage error: %v", err)
	}
	s := string(raw)
	// 下架改用寄信申請(OD-9),醫師卡片不得出現下架按鈕或任何 postback。
	if strings.Contains(s, "下架") || strings.Contains(s, `"type":"postback"`) {
		t.Fatalf("doctor card must not offer a takedown button or postback: %s", s)
	}
}
