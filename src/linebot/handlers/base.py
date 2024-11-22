"""
Linebot Base Handler
"""
from abc import ABC, abstractmethod
from sqlalchemy.orm import Session
from linebot.v3.webhooks import Event
from linebot.v3.messaging import MessagingApi

class BaseHandler(ABC):
    """
    Base Handler
    """
    @abstractmethod
    async def handle(self, event: Event, line_bot_api: MessagingApi, db: Session = None) -> None:
        pass