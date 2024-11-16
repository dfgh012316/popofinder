"""
LINEBOT Dependencies
"""

import sys
from linebot.v3 import WebhookParser
from linebot.v3.messaging import (
    Configuration,
    AsyncApiClient,
    AsyncMessagingApi,
)
from src.config import settings


class LineBotApiWrapper:
    "Linebot Api Wrapper"

    def __init__(self):
        if settings.LINE_MESSAGE_CHANNEL_SECRET is None:
            print("Specify LINE_CHANNEL_SECRET as environment variable.")
            sys.exit(1)
        if settings.LINE_MESSAGE_CHANNEL_TOKEN is None:
            print("Specify LINE_CHANNEL_ACCESS_TOKEN as environment variable.")
            sys.exit(1)

        self.configuration = Configuration(
            access_token=settings.LINE_MESSAGE_CHANNEL_TOKEN
        )
        self.async_api_client = None
        self.async_messaging_api = None

    async def get_api(self):
        "Get line message api client"
        if self.async_api_client is None:
            self.async_api_client = AsyncApiClient(self.configuration)
            self.async_messaging_api = AsyncMessagingApi(self.async_api_client)
        return self.async_messaging_api

    async def close(self):
        "Close"
        if self.async_api_client:
            await self.async_api_client.close()
            self.async_api_client = None
            self.async_messaging_api = None


line_bot_api_wrapper = LineBotApiWrapper()
parser = WebhookParser(settings.LINE_MESSAGE_CHANNEL_SECRET)


async def get_line_bot_api():
    "Get linebot api"
    return await line_bot_api_wrapper.get_api()
