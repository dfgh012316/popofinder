package crawl

import (
	"regexp"
	"strings"
)

// Medical-degree rule from docs/open-decisions.md OD-10: a line counts only at
// bachelor-of-medicine level, so postgraduate lines are excluded first.
var (
	excludeRe = regexp.MustCompile(`碩士|博士|研究所|(?i:ph\.?\s?d|master|\bmph\b|\bmba\b|\bm\.?sc?\b)`)
	englishRe = regexp.MustCompile(`(?i:\bm\.?d\b|\bd\.?d\.?s\b|\bdmd\b|\bmbbs\b|doctor of medicine|bachelor of medicine|medicine|dental surgery)`)
)

// hasMedicalDegree reports whether any education line is a medical degree.
func hasMedicalDegree(lines []string) bool {
	for _, line := range lines {
		if isMedicalDegreeLine(line) {
			return true
		}
	}
	return false
}

func isMedicalDegreeLine(line string) bool {
	if excludeRe.MatchString(line) {
		return false
	}
	// Covers 牙醫學系, 中醫學系, 學士後醫學系 and 牙醫學士.
	if strings.Contains(line, "醫學系") || strings.Contains(line, "醫學士") {
		return true
	}
	// A bare 醫學院/醫學大學 counts, but not another 學系 at a medical university (e.g. 藥學系).
	if !strings.Contains(line, "學系") && (strings.Contains(line, "醫學院") || strings.Contains(line, "醫學大學")) {
		return true
	}
	return englishRe.MatchString(line)
}

// formatEducation joins the non-empty trimmed lines with "；", capped at 500 runes
// to fit medical_personnel.education.
func formatEducation(lines []string) string {
	var kept []string
	for _, line := range lines {
		if s := strings.TrimSpace(line); s != "" {
			kept = append(kept, s)
		}
	}
	return truncateRunes(strings.Join(kept, "；"), 500)
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
