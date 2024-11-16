"""
LINEBOT Router
"""

from sqlalchemy.orm import Session
from fastapi import APIRouter, Request, HTTPException, Depends
from linebot.v3.exceptions import InvalidSignatureError
from linebot.v3.messaging import ReplyMessageRequest
from linebot.v3.webhooks import MessageEvent, TextMessageContent
from src.database.connection import get_db
from src.infra.logger import get_logger
from .dependencies import get_line_bot_api, parser
from .services import search_doctor
from .message_templates.doctor_template import create_flex_message

logger = get_logger("linebot")
router = APIRouter()


@router.post("/callback")
async def handle_callback(
    request: Request,
    line_bot_api=Depends(get_line_bot_api),
    db: Session = Depends(get_db),
):
    """
    Handle linebot callback
    """
    signature = request.headers["X-Line-Signature"]

    body = await request.body()
    body = body.decode()

    try:
        events = parser.parse(body, signature)
    except InvalidSignatureError as exc:
        logger.error("Invalid signature in request")
        raise HTTPException(status_code=400, detail="Invalid signature") from exc

    for event in events:
        if not isinstance(event, MessageEvent):
            continue
        if not isinstance(event.message, TextMessageContent):
            continue

        user_id = event.source.user_id
        message = event.message.text.strip()

        logger.info("[Request] UserId: %s | Message: %s", user_id, event.message.text)

        if event.message.text.strip() in [
            "Report",
            "report",
            "回報",
            "回報波波",
            "波波回報",
        ]:
            return "OK"

        city = None
        name = message
        cities = [
            "南投",
            "台中",
            "台北",
            "台南",
            "台東",
            "嘉義",
            "基隆",
            "宜蘭",
            "屏東",
            "彰化",
            "新北",
            "新竹",
            "桃園",
            "花蓮",
            "苗栗",
            "雲林",
            "高雄",
        ]
        
        for c in cities:
            if message.startswith(c):
                city = c
                name = message[len(c):].strip()
                break

        popo_doctors = search_doctor(name, city, db)
        popo_result = create_flex_message(popo_doctors)

        await line_bot_api.reply_message(
            ReplyMessageRequest(reply_token=event.reply_token, messages=[popo_result])
        )

    return "OK"
