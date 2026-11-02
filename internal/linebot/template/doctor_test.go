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
	for _, want := range []string{"民眾回報", "https://example.com/doc"} {
		if !strings.Contains(s, want) {
			t.Fatalf("card JSON missing %q: %s", want, s)
		}
	}
	// 驗證狀態尚未實作醫師認領,不應顯示,以免讓人誤會全部資料未查證。
	if strings.Contains(s, "驗證狀態") {
		t.Fatalf("card JSON should not show 驗證狀態: %s", s)
	}
	// SourceURL should render as a tappable uri action, not raw text.
	for _, want := range []string{`"type":"uri"`, `"uri":"https://example.com/doc"`, `"label":"查看來源"`} {
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
	if strings.Contains(s, "查看來源") || strings.Contains(s, `"type":"uri"`) {
		t.Fatalf("card JSON should omit the source button when SourceURL empty: %s", s)
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
	for _, n := range bubble["body"].(map[string]any)["contents"].([]any) {
		node := n.(map[string]any)
		if node["text"] != "網路公開彙整（2025 年 3 月前）" {
			continue
		}
		if node["wrap"] != true {
			t.Fatalf("source should wrap: %v", node)
		}
		return
	}
	t.Fatalf("card has no blog source line: %s", raw)
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
	for _, want := range []string{`"text":"醫療院所官網，2026-10-12 查到"`, `"label":"查看來源"`} {
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
	if !strings.Contains(s, `"text":"醫療院所官網"`) {
		t.Fatalf("card JSON should show the source without a date: %s", s)
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
	if strings.Contains(s, "查看來源") || strings.Contains(s, "查到") {
		t.Fatalf("card JSON should not show 查看來源 or 查到 without SourceURL: %s", s)
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
			var action map[string]string
			for _, b := range footer.Contents {
				if b.Action["type"] == "message" {
					action = b.Action
				}
			}
			require.Equal(t, map[string]string{"type": "message", "label": "同院同科醫師", "text": tt.wantText}, action)

			got := search.ParseCriteria(action["text"])
			require.Equal(t, search.TypeAll, got.SearchType)
			require.Equal(t, tt.wantCity, *got.City)
			require.Equal(t, tt.wantSearch, got.SearchTerm)
		})
	}
}

// Missing hospital, department or city are left out rather than shown as 未提供;
// the 學歷 block always shows, one text per stored line.
func TestDoctorBubbleLayout(t *testing.T) {
	raw, err := DoctorsFlexMessage([]database.MedicalPersonnel{{
		Name:      "王小明",
		Hospital:  "測試醫院",
		Education: sql.NullString{String: "甲大學醫學系；乙大學公共衛生碩士", Valid: true},
	}, {
		Name: "李小華",
	}})
	require.NoError(t, err)
	var msg struct {
		Contents struct {
			Contents []struct {
				Body struct {
					Contents []struct {
						Text     string `json:"text"`
						Contents []struct {
							Text string `json:"text"`
						} `json:"contents"`
					} `json:"contents"`
				} `json:"body"`
			} `json:"contents"`
		} `json:"contents"`
	}
	require.NoError(t, json.Unmarshal(raw, &msg))
	texts := func(i int) (top, education []string) {
		for _, n := range msg.Contents.Contents[i].Body.Contents {
			if n.Text != "" {
				top = append(top, n.Text)
			}
			for _, c := range n.Contents {
				education = append(education, c.Text)
			}
		}
		return top, education
	}
	top, education := texts(0)
	require.Equal(t, []string{"王小明", "測試醫院", "尚未分類"}, top)
	require.Equal(t, []string{"學歷", "甲大學醫學系", "乙大學公共衛生碩士"}, education)
	top, education = texts(1)
	require.Equal(t, []string{"李小華", "尚未分類"}, top)
	require.Equal(t, []string{"學歷", "未提供"}, education)
}
