package web

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	texttemplate "text/template"

	linetemplate "github.com/dfgh012316/popofinder/internal/linebot/template"
)

const (
	// siteURL is the web version's address, for canonical links, Open Graph and llms.txt.
	siteURL = "https://popo.jerrytech.me"
	// repoURL is where the code is public.
	repoURL = "https://github.com/dfgh012316/popofinder"
	// siteDescription is the default meta and Open Graph description.
	siteDescription = "查詢台灣醫師在哪裡受醫學教育。收錄醫療院所官網與網路公開彙整的醫師學歷，台灣與國外畢業的都有，照實顯示學歷與資料來源。"
)

// point is one titled line of text.
type point struct{ Title, Text string }

// differences says how POPO Finder differs from other doctor lists. The home page,
// its JSON-LD and llms.txt are all built from it, so they say the same thing. Only
// what this site does, never claims about any other site (OD-17).
var differences = []point{
	{"標明出處，可以自己核對", "每張卡片都寫明資料來源；來自醫院官網的附上原頁面連結與查到的日期，可以自己點回去看。"},
	{"只陳述事實，不貼標籤", "台灣與國外畢業的都收，照抄學歷原文，不替醫師分類，也不評價任何醫師。"},
	{"標示資料時效", "較舊的網路彙整資料標明「2025 年 3 月前」，不把可能過時的醫院資訊當成現況。"},
	{"醫師可以補充、更正、下架", "有公開的申請管道；院所官網寫了學歷的，任何人都可以提供網址。"},
	{"查詢更方便", "多個條件一起查，認得三總、林長、北榮等醫院簡稱，還能一鍵看同院同科的醫師；網頁和 LINE 都能用。"},
	{"免費，沒有廣告和追蹤碼", "網站本身不放廣告，也不載入任何追蹤程式或 script。"},
	{"程式碼公開", "收錄方式可以檢視，爬蟲遵守各醫院網站的 robots.txt，不繞過驗證碼。"},
}

// faqs is shown on the home page and repeated in its FAQPage JSON-LD and llms.txt.
var faqs = []struct{ Q, A string }{
	{"POPO Finder 是什麼？", "查詢台灣醫師在哪裡受醫學教育的免費工具。收錄醫療院所官網與網路公開彙整的醫師學歷，台灣與國外畢業的都有，照實顯示學歷原文與資料來源。"},
	{"POPO Finder 會告訴我某位醫師是不是「波波」嗎？", "不會。它只顯示學歷原文與資料來源，不替醫師分類，也不評價任何醫師。在台灣執業的醫師都通過了國家考試，學歷只是一項事實，怎麼看由你自己決定。"},
	{"資料從哪裡來？", "主要來自醫院官網的醫師介紹頁，卡片附上原頁面連結與查到的日期；另有 2025 年 3 月前的網路公開彙整，醫院可能已經變動，卡片會標示。"},
	{"為什麼查不到某位醫師？", "2026 年 11 月約收錄 6,100 位醫師，全國醫療院所的執業醫師有 8 萬多位，而且官網的部分只收有寫學歷的醫師。查不到不代表什麼。"},
	{"醫師想補上學歷、更正或下架，要怎麼做？", "寄信到 " + linetemplate.ContactEmail + "。院所官網寫了學歷的，附上網址就好；官網沒寫的，請醫師本人附上證書字號與學歷證明。更正或下架請以院所官方信箱來信，或附上可證明身分的資料。"},
	{"網站會記錄我查了誰嗎？", "網站程式不保存你的 IP。查詢紀錄只存查詢的文字與結果筆數，用來找出缺少的資料，不存是誰查的。"},
}

// homeJSONLD is the home page's structured data: the site as a free web application,
// and the FAQ shown on the page.
var homeJSONLD = func() template.JS {
	features := make([]string, len(differences))
	for i, d := range differences {
		features[i] = d.Title
	}
	type answer struct {
		Type string `json:"@type"`
		Text string `json:"text"`
	}
	type question struct {
		Type   string `json:"@type"`
		Name   string `json:"name"`
		Answer answer `json:"acceptedAnswer"`
	}
	questions := make([]question, len(faqs))
	for i, f := range faqs {
		questions[i] = question{"Question", f.Q, answer{"Answer", f.A}}
	}
	data := []any{
		map[string]any{
			"@context":            "https://schema.org",
			"@type":               "WebApplication",
			"name":                "POPO Finder 醫師學歷查詢",
			"url":                 siteURL + "/",
			"applicationCategory": "ReferenceApplication",
			"operatingSystem":     "Web",
			"inLanguage":          "zh-TW",
			"description":         siteDescription,
			"offers":              map[string]any{"@type": "Offer", "price": "0", "priceCurrency": "TWD"},
			"featureList":         features,
		},
		map[string]any{
			"@context":   "https://schema.org",
			"@type":      "FAQPage",
			"mainEntity": questions,
		},
	}
	// json.Marshal escapes <, > and &, so the result is safe inside <script>.
	b, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return template.JS(b)
}()

//go:embed static
var staticFS embed.FS

// llmsTxt is llms.txt (llmstxt.org) rendered from the same content as the home page.
var llmsTxt = func() []byte {
	t := texttemplate.Must(texttemplate.ParseFS(staticFS, "static/llms.txt.tmpl"))
	var buf bytes.Buffer
	err := t.Execute(&buf, struct {
		Description, SiteURL, LineURL, RepoURL string
		Differences                            []point
		FAQs                                   any
	}{siteDescription, siteURL, lineAddURL, repoURL, differences, faqs})
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}()

// serveBytes serves fixed content with a day of caching.
func serveBytes(b []byte, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(b)
	}
}

// staticFile serves one embedded file from static/.
func staticFile(name, contentType string) http.HandlerFunc {
	b, err := staticFS.ReadFile("static/" + name)
	if err != nil {
		panic(err)
	}
	return serveBytes(b, contentType)
}
