"""
Message Event Handler
"""
from sqlalchemy.orm import Session
from linebot.v3.webhooks import MessageEvent, TextMessageContent
from linebot.v3.messaging import ReplyMessageRequest
from src.popo.services import search_doctor
from src.linebot.services import parse_search_criteria
from src.linebot.message_templates.help_template import create_help_message
from src.linebot.services import create_search_response
from src.linebot.dependencies import update_search_state
from src.infra.logger import get_logger
from .base import BaseHandler

logger = get_logger("linebot")

class MessageEventHandler(BaseHandler):
    """
    Message Event Handler
    """
    async def handle(self, event: MessageEvent, line_bot_api, db: Session) -> None:
        if not isinstance(event.message, TextMessageContent):
            return

        user_id = event.source.user_id
        message = event.message.text.strip()
        logger.info("[Request] UserId: %s | Message: %s", user_id, message)

        if message.lower() in ["report", "回報", "回報波波", "波波回報"]:
            return

        if message.lower() in ["幫助", "help"]:
            await line_bot_api.reply_message(
                ReplyMessageRequest(
                    reply_token=event.reply_token,
                    messages=[create_help_message()]
                )
            )
            return

        search_criteria = parse_search_criteria(message)
        update_search_state(user_id, {
            'search_term': search_criteria.search_term,
            'city': search_criteria.city,
            'search_type': search_criteria.search_type.value
        })

        doctors, stats = search_doctor(search_criteria, db)
        messages = create_search_response(doctors, stats, search_criteria)

        await line_bot_api.reply_message(
            ReplyMessageRequest(reply_token=event.reply_token, messages=messages)
        )
