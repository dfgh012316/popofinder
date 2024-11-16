"""
LINEBOT Router
"""
from sqlalchemy import select
from sqlalchemy.orm import Session
from fastapi import APIRouter, Request, HTTPException, Depends
from linebot.v3.exceptions import InvalidSignatureError
from linebot.v3.messaging import ReplyMessageRequest, TextMessage, FlexMessage, FlexContainer
from linebot.v3.webhooks import MessageEvent, TextMessageContent
from src.database.connection import get_db
from src.database.models.medical_personnel import MedicalPersonnel
from .dependencies import get_line_bot_api, parser


router = APIRouter()


def search_doctor(name: str, db: Session) -> list:
    """
    搜尋醫生資料
    """
    query = (
        select(MedicalPersonnel)
        .where(MedicalPersonnel.name.ilike(f"%{name}%"))
        .limit(10)
    )

    result = db.execute(query)
    return result.scalars().all()


def create_doctor_bubble(doctor: MedicalPersonnel) -> dict:
    """
    為每個醫生創建一個 bubble 容器
    """
    return {
        "type": "bubble",
        "header": {
            "type": "box",
            "layout": "vertical",
            "contents": [
                {
                    "type": "text",
                    "text": doctor.name,
                    "size": "xl",
                    "weight": "bold",
                    "color": "#4A4A4A"
                }
            ],
            "backgroundColor": "#F0F8FF"
        },
        "body": {
            "type": "box",
            "layout": "vertical",
            "contents": [
                {
                    "type": "box",
                    "layout": "horizontal",
                    "contents": [
                        {
                            "type": "text",
                            "text": "縣市",
                            "size": "sm",
                            "color": "#666666",
                            "flex": 2
                        },
                        {
                            "type": "text",
                            "text": doctor.city or "未提供",
                            "size": "sm",
                            "color": "#4A4A4A",
                            "flex": 4
                        }
                    ],
                    "spacing": "md"
                },
                {
                    "type": "box",
                    "layout": "horizontal",
                    "contents": [
                        {
                            "type": "text",
                            "text": "醫院",
                            "size": "sm",
                            "color": "#666666",
                            "flex": 2
                        },
                        {
                            "type": "text",
                            "text": doctor.hospital or "未提供",
                            "size": "sm",
                            "color": "#4A4A4A",
                            "flex": 4,
                            "wrap": True
                        }
                    ],
                    "spacing": "md",
                    "margin": "md"
                },
                {
                    "type": "box",
                    "layout": "horizontal",
                    "contents": [
                        {
                            "type": "text",
                            "text": "科別",
                            "size": "sm",
                            "color": "#666666",
                            "flex": 2
                        },
                        {
                            "type": "text",
                            "text": doctor.department or "未提供",
                            "size": "sm",
                            "color": "#4A4A4A",
                            "flex": 4
                        }
                    ],
                    "spacing": "md",
                    "margin": "md"
                },
                {
                    "type": "box",
                    "layout": "horizontal",
                    "contents": [
                        {
                            "type": "text",
                            "text": "學歷",
                            "size": "sm",
                            "color": "#666666",
                            "flex": 2
                        },
                        {
                            "type": "text",
                            "text": doctor.education or "未提供",
                            "size": "sm",
                            "color": "#4A4A4A",
                            "flex": 4,
                            "wrap": True
                        }
                    ],
                    "spacing": "md",
                    "margin": "md"
                }
            ],
            "backgroundColor": "#FFFFFF"
        }
    }


def create_flex_message(doctors: list[MedicalPersonnel]) -> FlexMessage:
    """
    創建 Flex Message
    """
    if not doctors:
        # 當沒有搜尋結果時的訊息
        contents = {
            "type": "bubble",
            "body": {
                "type": "box",
                "layout": "vertical",
                "contents": [
                    {
                        "type": "text",
                        "text": "找不到相關資料",
                        "size": "lg",
                        "weight": "bold",
                        "align": "center",
                        "color": "#666666"
                    }
                ]
            }
        }
    else:
        # 當有搜尋結果時，創建 carousel 容器
        contents = {
            "type": "carousel",
            "contents": [create_doctor_bubble(doctor) for doctor in doctors]
        }

    return FlexMessage(
        alt_text=f"找到 {len(doctors)} 筆相關資料" if doctors else "找不到相關資料",
        contents=FlexContainer.from_dict(contents)
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

        if event.message.text.strip() in ["Report", "report", "回報", "回報波波", "波波回報"]:
            return "OK"

        doctors = search_doctor(event.message.text, db)
        flex_message = create_flex_message(doctors)

        await line_bot_api.reply_message(
            ReplyMessageRequest(
                reply_token=event.reply_token, messages=[flex_message]
            )
        )

    return "OK"
