package template

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/search"
	"github.com/stretchr/testify/require"
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

func TestDoctorBubbleBlogSourceShowsDate(t *testing.T) {
	raw, err := DoctorsFlexMessage([]database.MedicalPersonnel{{
		Name:     "測試醫師",
		City:     "台北市",
		Hospital: "測試醫院",
		Source:   sql.NullString{String: "blog", Valid: true},
	}})
	if err != nil {
		t.Fatalf("DoctorsFlexMessage error: %v", err)
	}
	var msg map[string]any
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	carousel := msg["contents"].(map[string]any)
	bubble := carousel["contents"].([]any)[0].(map[string]any)
	rows := bubble["body"].(map[string]any)["contents"].([]any)

	for _, r := range rows {
		children := r.(map[string]any)["contents"].([]any)
		if children[0].(map[string]any)["text"] != "資料來源" {
			continue
		}
		value := children[1].(map[string]any)
		if value["text"] != "網路公開彙整（2025 年 3 月前）" {
			t.Fatalf("blog source label = %v", value["text"])
		}
		if value["wrap"] != true {
			t.Fatalf("source value should wrap: %v", value)
		}
		return
	}
	t.Fatalf("card has no 資料來源 row: %s", raw)
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

// The 「同院同科醫師」 button (OD-14) sends a keyword search for the doctor's city,
// hospital and first department; cards without a department have no button.
func TestDoctorBubbleSameDepartmentButton(t *testing.T) {
	dept := func(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }
	tests := []struct {
		name       string
		p          database.MedicalPersonnel
		wantText   string
		wantCity   string
		wantSearch string
	}{
		{"first of several departments", database.MedicalPersonnel{City: "桃園", Hospital: "林口長庚紀念醫院", Department: dept("心臟內科、心臟衰竭中心")},
			"桃園 林口長庚紀念醫院 心臟內科", "桃園", "林口長庚紀念醫院 心臟內科"},
		{"slash in the department, space in the hospital", database.MedicalPersonnel{City: "台北", Hospital: "甲診所 乙診所", Department: dept("醫美/內科")},
			"台北 甲診所 乙診所 醫美", "台北", "甲診所 乙診所 醫美"},
		{"no department", database.MedicalPersonnel{City: "台北", Hospital: "甲診所"}, "", "", ""},
		{"blank department", database.MedicalPersonnel{City: "台北", Hospital: "甲診所", Department: dept(" ")}, "", "", ""},
		{"over LINE's limit", database.MedicalPersonnel{City: "台北", Hospital: strings.Repeat("甲", 300), Department: dept("內科")}, "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.p.Name = "王小明"
			raw, err := DoctorsFlexMessage([]database.MedicalPersonnel{tt.p})
			require.NoError(t, err)
			var msg struct {
				Contents struct {
					Contents []struct {
						Footer *struct {
							Contents []struct {
								Action map[string]string `json:"action"`
							} `json:"contents"`
						} `json:"footer"`
					} `json:"contents"`
				} `json:"contents"`
			}
			require.NoError(t, json.Unmarshal(raw, &msg))
			footer := msg.Contents.Contents[0].Footer
			if tt.wantText == "" {
				require.Nil(t, footer)
				return
			}
			require.NotNil(t, footer)
			action := footer.Contents[0].Action
			require.Equal(t, map[string]string{"type": "message", "label": "同院同科醫師", "text": tt.wantText}, action)

			got := search.ParseCriteria(action["text"])
			require.Equal(t, search.TypeAll, got.SearchType)
			require.Equal(t, tt.wantCity, *got.City)
			require.Equal(t, tt.wantSearch, got.SearchTerm)
		})
	}
}
