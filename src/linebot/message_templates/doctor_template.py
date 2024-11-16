"""
Doctor Message Template
"""
from linebot.v3.messaging import FlexMessage, FlexContainer
from src.database.models.medical_personnel import MedicalPersonnel

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
