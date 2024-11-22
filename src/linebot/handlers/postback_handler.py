"""
Postback Event Handler
"""
from urllib.parse import parse_qsl
from sqlalchemy.orm import Session
from linebot.v3.webhooks import PostbackEvent
from linebot.v3.messaging import ReplyMessageRequest, TextMessage
from src.popo.schemas import SearchCriteria, SearchType
from src.popo.services import search_doctor
from src.linebot.dependencies import get_search_state
from src.linebot.services import create_search_response
from src.infra.logger import get_logger
from .base import BaseHandler

logger = get_logger("linebot")

class PostbackEventHandler(BaseHandler):
    async def handle(self, event: PostbackEvent, line_bot_api, db: Session) -> None:
        user_id = event.source.user_id
        logger.info("[Postback] UserId: %s | Data: %s", user_id, event.postback.data)
        data = dict(parse_qsl(event.postback.data))

        if data.get('action') == 'next_page':
            await self._handle_next_page(event, line_bot_api, db, user_id, data)

    async def _handle_next_page(self, event, line_bot_api, db, user_id, data):
        state = get_search_state(user_id)
        if not state:
            await line_bot_api.reply_message(
                ReplyMessageRequest(
                    reply_token=event.reply_token,
                    messages=[TextMessage(text="搜尋已過期,請重新搜尋")]
                )
            )
            return

        try:
            offset = int(data.get('offset', 0))
            search_criteria = SearchCriteria(
                search_type=SearchType(state.get('search_type', 'name')),
                search_term=state.get('search_term', ''),
                city=state.get('city')
            )
            doctors, stats = search_doctor(search_criteria, db, offset=offset)
            messages = create_search_response(doctors, stats, search_criteria)

            await line_bot_api.reply_message(
                ReplyMessageRequest(
                    reply_token=event.reply_token,
                    messages=messages
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
