from linebot.v3.messaging import FlexMessage, FlexContainer

HELP_MESSAGE_TEMPLATE = {
  "type": "bubble",
  "body": {
    "type": "box",
    "layout": "vertical",
    "contents": [
      {
        "type": "text",
        "text": "使用說明",
        "weight": "bold",
        "size": "xl",
        "color": "#1DB446"
      },
      {
        "type": "separator",
        "margin": "xxl"
      },
      {
        "type": "box",
        "layout": "vertical",
        "margin": "lg",
        "spacing": "sm",
        "contents": [
          {
            "type": "text",
            "text": "支援格式",
            "weight": "bold",
            "size": "lg"
          },
          {
            "type": "box",
            "layout": "vertical",
            "spacing": "sm",
            "contents": [
              {
                "type": "box",
                "layout": "baseline",
                "contents": [
                  {
                    "type": "text",
                    "text": "🏥",
                    "size": "sm",
                    "color": "#666666",
                    "flex": 1
                  },
                  {
                    "type": "text",
                    "text": "[城市]@醫院 醫院名稱",
                    "size": "sm",
                    "color": "#666666",
                    "flex": 5
                  }
                ]
              },
              {
                "type": "box",
                "layout": "baseline",
                "contents": [
                  {
                    "type": "text",
                    "text": "💡",
                    "size": "sm",
                    "color": "#666666",
                    "flex": 1
                  },
                  {
                    "type": "text",
                    "text": "例如：台北@醫院 三總",
                    "size": "sm",
                    "color": "#666666",
                    "flex": 5
                  }
                ]
              }
            ]
          },
          {
            "type": "separator",
            "margin": "lg"
          },
          {
            "type": "box",
            "layout": "vertical",
            "spacing": "sm",
            "contents": [
              {
                "type": "box",
                "layout": "baseline",
                "contents": [
                  {
                    "type": "text",
                    "text": "🏥",
                    "size": "sm",
                    "color": "#666666",
                    "flex": 1
                  },
                  {
                    "type": "text",
                    "text": "[城市]@科別 科別名稱",
                    "size": "sm",
                    "color": "#666666",
                    "flex": 5
                  }
                ]
              },
              {
                "type": "box",
                "layout": "baseline",
                "contents": [
                  {
                    "type": "text",
                    "text": "💡",
                    "size": "sm",
                    "color": "#666666",
                    "flex": 1
                  },
                  {
                    "type": "text",
                    "text": "例如：台北@科別 牙科",
                    "size": "sm",
                    "color": "#666666",
                    "flex": 5
                  }
                ]
              }
            ]
          },
          {
            "type": "separator",
            "margin": "lg"
          },
          {
            "type": "box",
            "layout": "vertical",
            "spacing": "sm",
            "contents": [
              {
                "type": "box",
                "layout": "baseline",
                "contents": [
                  {
                    "type": "text",
                    "text": "👤",
                    "size": "sm",
                    "color": "#666666",
                    "flex": 1
                  },
                  {
                    "type": "text",
                    "text": "[城市] 醫師名稱",
                    "size": "sm",
                    "color": "#666666",
                    "flex": 5
                  }
                ]
              },
              {
                "type": "box",
                "layout": "baseline",
                "contents": [
                  {
                    "type": "text",
                    "text": "💡",
                    "size": "sm",
                    "color": "#666666",
                    "flex": 1
                  },
                  {
                    "type": "text",
                    "text": "例如：台北 陳",
                    "size": "sm",
                    "color": "#666666",
                    "flex": 5
                  }
                ]
              }
            ]
          },
          {
            "type": "separator",
            "margin": "lg"
          },
          {
            "type": "box",
            "layout": "vertical",
            "margin": "lg",
            "contents": [
              {
                "type": "text",
                "text": "支援城市",
                "weight": "bold",
                "size": "lg"
              },
              {
                "type": "box",
                "layout": "vertical",
                "margin": "sm",
                "contents": [
                  {
                    "type": "box",
                    "layout": "horizontal",
                    "contents": [
                      {
                        "type": "text",
                        "text": "台北、新北、基隆",
                        "size": "sm",
                        "color": "#666666",
                        "flex": 1
                      }
                    ]
                  },
                  {
                    "type": "box",
                    "layout": "horizontal",
                    "contents": [
                      {
                        "type": "text",
                        "text": "桃園、新竹、苗栗",
                        "size": "sm",
                        "color": "#666666",
                        "flex": 1
                      }
                    ]
                  },
                  {
                    "type": "box",
                    "layout": "horizontal",
                    "contents": [
                      {
                        "type": "text",
                        "text": "台中、彰化、南投",
                        "size": "sm",
                        "color": "#666666",
                        "flex": 1
                      }
                    ]
                  },
                  {
                    "type": "box",
                    "layout": "horizontal",
                    "contents": [
                      {
                        "type": "text",
                        "text": "雲林、嘉義、台南",
                        "size": "sm",
                        "color": "#666666",
                        "flex": 1
                      }
                    ]
                  },
                  {
                    "type": "box",
                    "layout": "horizontal",
                    "contents": [
                      {
                        "type": "text",
                        "text": "高雄、屏東、台東",
                        "size": "sm",
                        "color": "#666666",
                        "flex": 1
                      }
                    ]
                  },
                  {
                    "type": "box",
                    "layout": "horizontal",
                    "contents": [
                      {
                        "type": "text",
                        "text": "花蓮、宜蘭",
                        "size": "sm",
                        "color": "#666666",
                        "flex": 1
                      }
                    ]
                  }
                ]
              }
            ]
          }
        ]
      }
    ]
  }
}

def create_help_message():
    return FlexMessage(
        alt_text='使用說明',
        contents=FlexContainer.from_dict(HELP_MESSAGE_TEMPLATE)
    )