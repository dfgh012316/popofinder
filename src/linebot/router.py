"""
LINEBOT Router
"""

from fastapi import APIRouter, Request, HTTPException, Depends
from linebot.v3.exceptions import InvalidSignatureError
from linebot.v3.messaging import ReplyMessageRequest, TextMessage
from linebot.v3.webhooks import MessageEvent, TextMessageContent
from sqlalchemy import select
from sqlalchemy.orm import Session
from .dependencies import get_line_bot_api, parser
from src.database.connection import get_db
from src.database.models.popo import MedicalPersonnel

router = APIRouter()


def search_doctor(name: str, db: Session) -> list:
    """
    搜尋醫生資料
    """
    query = (
        select(MedicalPersonnel)
        .where(MedicalPersonnel.name.ilike(f"%{name}%"))
        .limit(5)
    )  # 限制返回數量避免訊息太長

    result = db.execute(query)
    return result.scalars().all()


def format_doctor_info(doctor: MedicalPersonnel) -> str:
    """
    格式化醫生資訊
    """
    return (
        f"姓名: {doctor.name}\n"
        f"縣市: {doctor.city}\n"
        f"醫院: {doctor.hospital}\n"
        f"科別: {doctor.department or '未提供'}\n"
        f"學歷: {doctor.education or '未提供'}\n"
        "-------------------"
    )


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
        raise HTTPException(status_code=400, detail="Invalid signature") from exc

    for event in events:
        if not isinstance(event, MessageEvent):
            continue
        if not isinstance(event.message, TextMessageContent):
            continue

        doctors = search_doctor(event.message.text, db)

        if not doctors:
            reply_text = f"找不到與 {event.message.text} 相關的資料"
        else:
            reply_text = f"找到 {len(doctors)} 筆相關的資料 \n"
            reply_text += "\n".join(format_doctor_info(doctor) for doctor in doctors)

        await line_bot_api.reply_message(
            ReplyMessageRequest(
                reply_token=event.reply_token, messages=[TextMessage(text=reply_text)]
            )
        )

    return "OK"
