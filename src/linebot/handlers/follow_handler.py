"""
Follow Event Handler
"""
from sqlalchemy.orm import Session
from linebot.v3.webhooks import FollowEvent
from linebot.v3.messaging import ReplyMessageRequest
from src.linebot.message_templates.help_template import create_help_message
from src.infra.logger import get_logger
from .base import BaseHandler

logger = get_logger("linebot")

class FollowEventHandler(BaseHandler):
    async def handle(self, event: FollowEvent, line_bot_api, db: Session = None) -> None:
        user_id = event.source.user_id
        logger.info("[Follow] UserId: %s", user_id)

        await line_bot_api.reply_message(
            ReplyMessageRequest(
                reply_token=event.reply_token,
                messages=[create_help_message()]
            )
        )
