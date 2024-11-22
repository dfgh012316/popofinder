"""
LINEBOT Router
"""
from sqlalchemy.orm import Session
from fastapi import APIRouter, Request, HTTPException, Depends
from linebot.v3.exceptions import InvalidSignatureError
from src.database.connection import get_db
from src.infra.logger import get_logger
from .dependencies import get_line_bot_api, parser
from .event_handler import get_handler


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
        handler = get_handler(event)
        if handler:
            await handler.handle(event, line_bot_api, db)

    return "OK"
