package template

import (
	"encoding/json"
	"fmt"

	"github.com/dfgh012316/popofinder/internal/database"
)

// HospitalListNote ends the hospital list: the counts cover only doctors whose
// education is on the site, and other hospitals are partial.
const HospitalListNote = "人數為官網名單收錄的醫師數，只收官網有寫學歷的醫師。其他醫院與診所來自網路公開彙整與民眾回報，只有部分醫師。"

// HospitalListCommand is the text that shows the hospital list; the help card's quick reply sends it too.
const HospitalListCommand = "醫院清單"

// HospitalSearchText is what tapping a hospital sends: a hospital search limited to its
// city, so 「國泰綜合醫院」 does not also pull in 汐止國泰綜合醫院 in 新北.
func HospitalSearchText(h database.DirectoryHospital) string {
	return h.City + "@醫院 " + h.Hospital
}

// HospitalListFlexMessage lists the hospitals whose official directories are crawled
// (OD-12). Each one is a button that runs a hospital search; the note says the list
// covers only doctors whose education is on the site, and other hospitals are partial.
func HospitalListFlexMessage(hospitals []database.DirectoryHospital) (json.RawMessage, error) {
	contents := []any{
		map[string]any{"type": "text", "text": "已收錄官網醫師名單的醫院", "weight": "bold", "size": "lg", "wrap": true},
		map[string]any{"type": "text", "text": "點醫院名稱即可查詢", "size": "sm", "color": "#666666", "margin": "sm"},
	}
	for _, h := range hospitals {
		contents = append(contents, map[string]any{
			"type":   "button",
			"style":  "link",
			"height": "sm",
			"action": map[string]any{
				"type":  "message",
				"label": fmt.Sprintf("%s %d 位", h.Hospital, h.Count),
				"text":  HospitalSearchText(h),
			},
		})
	}
	contents = append(contents,
		map[string]any{"type": "separator", "margin": "lg"},
		map[string]any{"type": "text", "text": HospitalListNote, "size": "sm", "color": "#666666", "wrap": true, "margin": "lg"},
	)
	return buildFlexMessage("已收錄官網醫師名單的醫院", map[string]any{
		"type": "bubble",
		"body": map[string]any{"type": "box", "layout": "vertical", "contents": contents},
	})
}
