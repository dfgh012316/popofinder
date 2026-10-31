package crawl

import (
	"regexp"
	"strings"
)

// Medical-degree rule from docs/open-decisions.md OD-10: a line counts only at
// bachelor-of-medicine level, so postgraduate lines are excluded first.
var (
	excludeRe = regexp.MustCompile(`碩士|博士|研究所|(?i:ph\.?\s?d|master|\bmph\b|\bmba\b|\bm\.?sc?\b)`)
	englishRe = regexp.MustCompile(`(?i:\bm\.?d\b|\bd\.?d\.?s\b|\bdmd\b|\bmbbs\b|doctor of medicine|bachelor of medicine|medicine|dental surgery|medical (university|college)|university of medical sciences?)`)
)

// medicalDegreeTerms: 醫學系 covers 牙醫學系, 中醫學系 and 學士後醫學系; 中醫系,
// 牙醫系, 牙科系 and 後醫系 (學士後醫系, 學士後中醫系) are their short forms, and
// 中西醫雙主修 is 長庚's double major.
var medicalDegreeTerms = []string{"醫學系", "醫學士", "醫學學士", "中醫系", "牙醫系", "牙科系", "後醫系", "中西醫雙主修"}

// hasMedicalDegree reports whether any education line is a medical degree. A line
// listing degrees separated by 「、」 is checked one degree at a time, so a master's
// degree next to a medical one does not exclude it.
func hasMedicalDegree(lines []string) bool {
	for _, line := range lines {
		for _, part := range strings.Split(line, "、") {
			if isMedicalDegreeLine(part) {
				return true
			}
		}
	}
	return false
}

func isMedicalDegreeLine(line string) bool {
	if excludeRe.MatchString(line) {
		return false
	}
	for _, term := range medicalDegreeTerms {
		if strings.Contains(line, term) {
			return true
		}
	}
	// A bare 醫學院/醫學大學 (or 醫藥大學/醫藥學院, as in 中國醫藥大學) counts, but not
	// another 學系 at such a university (e.g. 藥學系).
	if !strings.Contains(line, "學系") && (strings.Contains(line, "醫學院") || strings.Contains(line, "醫學大學") ||
		strings.Contains(line, "醫藥大學") || strings.Contains(line, "醫藥學院")) {
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
