package template

import "encoding/json"

// ReportCommand is the text that shows how to add, correct or take down a doctor's
// data; the rich menu's 「補充與更正」 sends it (docs/open-decisions.md OD-15, OD-16).
const ReportCommand = "回報"

// ContactEmail receives additions, corrections and takedown requests (OD-9, OD-15).
const ContactEmail = "jerrychuang@jerrytech.me"

// reportText is a plain text message, not Flex: LINE turns the address into a link
// only in text messages, and the text can be copied.
const reportText = "補充、更正與下架\n\n" +
	"想補充醫師學歷，請寄信至 " + ContactEmail + "：\n\n" +
	"1. 院所官網有醫師介紹，而且寫了學歷：附上網址就好，任何人都可以提供。我們照官網內容收錄，卡片附上官網連結。\n\n" +
	"2. 官網沒有寫學歷：請醫師本人來信，附上醫師證書字號與學歷證明（例如畢業證書，身分證字號等與學歷無關的資料請先遮住），並以院所官方信箱來信，或留下院所公開的電話讓我們確認。證明只用來核對，核對完就刪除；卡片會標示是醫師本人提供。\n\n" +
	"更正或下架：醫師本人或院所請寄到同一個信箱，並以院所官方信箱來信，或附上可證明身分的資料。\n\n" +
	"每封信都由維護者人工處理，需要一些時間。"

// ReportTextMessage returns the raw JSON for the reply to ReportCommand.
func ReportTextMessage() (json.RawMessage, error) {
	return json.Marshal(map[string]any{"type": "text", "text": reportText})
}
