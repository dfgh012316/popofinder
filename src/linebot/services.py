"""
Medical Personnel Search Service
"""
from sqlalchemy import select
from sqlalchemy.orm import Session
from src.database.models.medical_personnel import MedicalPersonnel

def search_doctor(name: str, city: str, db: Session) -> list:
    """
    搜尋醫生資料
    Args:
        name: 醫生姓名
        city: 城市名稱，如果為 None 則不限制城市
        db: 資料庫連線
    Returns:
        符合條件的醫生列表
    """
    query = select(MedicalPersonnel)
    
    if name:
        query = query.where(MedicalPersonnel.name.ilike(f"%{name}%"))

    # 加入城市搜尋條件
    if city:
        query = query.where(MedicalPersonnel.city == city)

    # 限制回傳數量
    query = query.limit(10)

    result = db.execute(query)
    return result.scalars().all()
