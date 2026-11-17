package template

import "encoding/json"

// DonateCommand is the text that shows how to support the service; the rich menu's
// 「贊助」 sends it (docs/open-decisions.md OD-21).
const DonateCommand = "贊助"

// DonateURL is the ECPay donation page, shared with 國會在幹嘛 (OD-20). Linked from
// the web support card and /about#support, and from DonateText; always after the rules.
const DonateURL = "https://p.ecpay.com.tw/6D8D974"

// DonateText is a plain text message, not Flex: LINE turns the URL into a link only in
// text messages. The rules mirror /about#support and come before the link (OD-20).
const DonateText = "支持 POPO Finder（波波照妖鏡）\n\n" +
	"這個服務免費、不放廣告。如果對你有用，歡迎贊助開發與營運：\n\n" +
	"．贊助換不到任何東西：收錄、排序、更正與下架都和有沒有贊助無關。\n" +
	"．不公開贊助者的名字、金額和留言。\n" +
	"．不是公益捐款，不能抵稅，也沒有捐款收據。\n" +
	"．金額自訂，每筆最低新台幣 100 元，單次付款不會自動扣款。\n" +
	"．付款在綠界的頁面完成，我們拿不到卡號。重複付款或金額輸入錯誤，請在付款後 7 天內寄信至 " + ContactEmail + "，確認後退回該筆款項。\n\n" +
	"到綠界贊助：\n" + DonateURL

// DonateTextMessage returns the raw JSON for the reply to DonateCommand.
func DonateTextMessage() (json.RawMessage, error) {
	return json.Marshal(map[string]any{"type": "text", "text": DonateText})
}
