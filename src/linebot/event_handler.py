from linebot.v3.webhooks import MessageEvent, PostbackEvent, FollowEvent
from .handlers.message_handler import MessageEventHandler
from .handlers.postback_handler import PostbackEventHandler
from .handlers.follow_handler import FollowEventHandler

message_handler = MessageEventHandler()
postback_handler = PostbackEventHandler()
follow_handler = FollowEventHandler()

EVENT_HANDLERS = {
    MessageEvent: message_handler,
    PostbackEvent: postback_handler,
    FollowEvent: follow_handler
}

def get_handler(event):
    return EVENT_HANDLERS.get(type(event))
