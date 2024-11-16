"""
Common Logger Configuration
用於整個專案的通用日誌模組
"""
import logging

# 設定基本的 logging 配置
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    handlers=[
        logging.FileHandler('app.log', encoding='utf-8'),
        logging.StreamHandler()
    ]
)

def get_logger(name: str) -> logging.Logger:
    """
    獲取指定名稱的 logger 實例
    
    Args:
        name: logger 名稱，通常使用模組名稱，如 'linebot', 'database' 等
        
    Returns:
        logging.Logger: 配置好的 logger 實例
    """
    return logging.getLogger(name)
