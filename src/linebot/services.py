"""
Medical Personnel Search Service
"""
from linebot.v3.messaging import TextMessage, FlexContainer, FlexMessage
from src.linebot.message_templates.doctor_template import create_flex_message
from src.popo.schemas import SearchType, SearchCriteria


def parse_search_criteria(message: str) -> SearchCriteria:
    """
    解析搜尋條件
    支援格式:
    - [城市]@醫院 醫院名稱  (例如: 台北@醫院 台大醫院)
    - [城市]@科別 科別名稱  (例如: 台北@科別 小兒科)
    - [城市] 醫師名稱      (例如: 台北王大明)
    """
    cities = [
        "南投", "台中", "台北", "台南", "台東", "嘉義", "基隆",
        "宜蘭", "屏東", "彰化", "新北", "新竹", "桃園", "花蓮",
        "苗栗", "雲林", "高雄"
    ]

    # 預設值
    city = None
    search_type = SearchType.NAME
    search_term = message.strip()

    # 檢查是否包含城市
    for possible_city in cities:
        if message.startswith(possible_city):
            city = possible_city
            search_term = message[len(city):].strip()
            break

    # 檢查搜尋類型
    if search_term.startswith("@醫院 "):
        search_type = SearchType.HOSPITAL
        search_term = search_term[4:].strip()
    elif search_term.startswith("@科別 "):
        search_type = SearchType.DEPARTMENT
        search_term = search_term[4:].strip()

    return SearchCriteria(search_type, search_term, city)


def format_search_summary(criteria: SearchCriteria, stats: dict) -> str:
    """
    格式化搜尋結果摘要
    """
    location_text = f"在{criteria.city}" if criteria.city else "全台"
    search_type_text = {
        SearchType.NAME: "醫師",
        SearchType.HOSPITAL: "醫院",
        SearchType.DEPARTMENT: "科別"
    }[criteria.search_type]

    current_range = f"{stats['current_page']*10-9} - {min(stats['current_page']*10, stats['total_count'])}"

    return (
        f"查詢{location_text}{search_type_text}「{criteria.search_term}」\n"
        f"共有 {stats['total_count']} 筆符合的結果\n"
        f"目前顯示第 {current_range} 筆"
    )

def create_search_response(doctors: list, stats: dict, criteria: SearchCriteria) -> list:
    """
    創建搜尋回應訊息
    """
    messages = []

    # 使用 format_search_summary 來生成搜尋統計訊息
    summary_text = format_search_summary(criteria, stats)
    messages.append(TextMessage(text=summary_text))

    # 添加搜尋結果
    messages.append(create_flex_message(doctors))

    # 如果還有更多結果，添加"顯示更多"按鈕
    if stats['has_more']:
        next_page_button = {
            "type": "bubble",
            "body": {
                "type": "box",
                "layout": "vertical",
                "contents": [
                    {
                        "type": "text",
                        "text": f"目前在第 {stats['current_page']}/{stats['total_pages']} 頁",
                        "size": "sm",
                        "wrap": True,
                        "align": "center"
                    },
                    {
                        "type": "button",
                        "action": {
                            "type": "postback",
                            "label": "顯示下一頁",
                            "data": f"action=next_page&offset={stats['current_page']*10}"
                        },
                        "style": "primary",
                        "margin": "md"
                    }
                ]
            }
        }
        messages.append(FlexMessage(
            alt_text="顯示更多",
            contents=FlexContainer.from_dict(next_page_button)
        ))

    return messages
