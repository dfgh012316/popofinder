package reconcile

import (
	"html"
	"regexp"
	"strings"
)

// Record is one row parsed from the blog plaintext post's HTML table.
type Record struct {
	City       string
	Hospital   string
	Department string
	Name       string
	Education  string
}

var (
	trRe  = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	tdRe  = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	tagRe = regexp.MustCompile(`<[^>]*>`)
)

// cleanCell strips inner tags, unescapes HTML entities, and trims whitespace.
func cleanCell(raw string) string {
	return strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(raw, "")))
}

// ParsePost parses the blog post HTML and returns one Record per data row,
// skipping the header row and any malformed rows.
func ParsePost(postHTML string) []Record {
	records := []Record{}
	for _, tr := range trRe.FindAllStringSubmatch(postHTML, -1) {
		tds := tdRe.FindAllStringSubmatch(tr[1], -1)
		if len(tds) != 5 {
			continue
		}
		cells := make([]string, 5)
		for i, td := range tds {
			cells[i] = cleanCell(td[1])
		}
		city, hospital, department, name, education := cells[0], cells[1], cells[2], cells[3], cells[4]
		if name == "" {
			continue
		}
		if city == "縣市" || name == "姓名" {
			continue
		}
		records = append(records, Record{
			City:       city,
			Hospital:   hospital,
			Department: department,
			Name:       name,
			Education:  education,
		})
	}
	return records
}
