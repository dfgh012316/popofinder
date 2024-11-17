"""
Handle Linebot Event
"""
from urllib.parse import parse_qsl
from sqlalchemy.orm import Session
from linebot.v3.messaging import ReplyMessageRequest, TextMessage, FlexMessage, FlexContainer
from linebot.v3.webhooks import PostbackEvent, MessageEvent, TextMessageContent
from src.infra.logger import get_logger
from .services import search_doctor
from .message_templates.doctor_template import create_flex_message
from .dependencies import get_search_state, update_search_state

logger = get_logger("linebot")

async def handle_postback_event(
    event: PostbackEvent,
    line_bot_api,
    db: Session
) -> None:
    """
    處理 PostbackEvent
    """
    user_id = event.source.user_id
    logger.info("[Postback] UserId: %s | Data: %s", user_id, event.postback.data)
    data = dict(parse_qsl(event.postback.data))

    if data.get('action') == 'next_page':
        await handle_next_page(event, line_bot_api, db, user_id, data)

async def handle_next_page(event, line_bot_api, db, user_id, data):
    """
    處理下一頁請求
    """
    state = get_search_state(user_id)
    if not state:
        await line_bot_api.reply_message(
            ReplyMessageRequest(
                reply_token=event.reply_token,
                message=[TextMessage(text="搜尋已過期,請重新搜尋")]
            )
        )
        return

    try:
        offset = int(data.get('offset', 0))
        doctors, stats = search_doctor(
            state['name'],
            state['city'],
            db,
            offset=offset
        )

        popo_result = create_search_response(doctors, stats, state)

        await line_bot_api.reply_message(
            ReplyMessageRequest(
                reply_token=event.reply_token,
                messages=popo_result
            )
        )
    except Exception as e:
        logger.error("Error processing next page requests: %s", str(e), exc_info=True)
        await line_bot_api.reply_message(
            ReplyMessageRequest(
                reply_token=event.reply_token,
                messages=[TextMessage(text="抱歉，處理分頁請求錯誤")]
            )
        )

async def handle_message_event(
    event: MessageEvent,
    line_bot_api,
    db: Session
) -> None:
    """
    處理 MessageEvent
    """
    if not isinstance(event.message, TextMessageContent):
        return

    user_id = event.source.user_id
    message = event.message.text.strip()

    logger.info("[Request] UserId: %s | Message: %s", user_id, message)

    if message.lower() in ["report", "help", "回報", "回報波波", "波波回報", "幫助"]:
        return

    city, name = parse_location_and_name(message)

    update_search_state(user_id, {
        'name': name,
        'city': city
    })

    popo_doctors, stats = search_doctor(name, city, db)
    popo_result = create_search_response(popo_doctors, stats, {'name': name, 'city': city})

    await line_bot_api.reply_message(
        ReplyMessageRequest(reply_token=event.reply_token, messages=popo_result)
    )

def parse_location_and_name(message: str) -> tuple[str | None, str]:
    """
    解析位置和名稱
    """
    cities = [
        "南投", "台中", "台北", "台南", "台東", "嘉義", "基隆",
        "宜蘭", "屏東", "彰化", "新北", "新竹", "桃園", "花蓮",
        "苗栗", "雲林", "高雄"
    ]

    for city in cities:
        if message.startswith(city):
            return city, message[len(city):].strip()

    return None, message

def create_search_response(doctors: list, stats: dict, state: dict) -> list:
    """
    創建搜尋回應訊息
    """
    messages = []

    # 添加搜尋統計訊息
    location_text = f"在{state['city']}" if state['city'] else "全台"
    current_range = f"{stats['current_page']*10-9} - {min(stats['current_page']*10, stats['total_count'])}"

    summary_text = (
        f"查詢到{location_text}共有 {stats['total_count']} 筆符合的結果\n"
        f"目前顯示第 {current_range} 筆"
    )
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
