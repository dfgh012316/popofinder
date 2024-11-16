"""
Medical Personnel Search Service
"""
from sqlalchemy import select, func
from sqlalchemy.orm import Session
from src.database.models.medical_personnel import MedicalPersonnel

def search_doctor(name: str, city: str | None, db: Session, offset: int = 0) -> tuple[list, dict]:
    """
    搜尋醫生資料
    Args:
        name: 醫生姓名
        city: 城市名稱，如果為 None 則不限制城市
        db: 資料庫連線
        offset: 分頁偏移量
    Returns:
        (搜尋結果列表, 搜尋統計資訊)
    """
    base_query = select(MedicalPersonnel)
    
    if name:
        base_query = base_query.where(MedicalPersonnel.name.ilike(f"%{name}%"))

    # 加入城市搜尋條件
    if city:
        base_query = base_query.where(MedicalPersonnel.city == city)
        
    count_query = select(func.count(1)).select_from(base_query.subquery())
    total_count = db.execute(count_query).scalar()

    # 限制回傳數量
    query = base_query.offset(offset).limit(10)
    result = db.execute(query)
    
    popo_doctors = result.scalars().all()
    
    stats = {
        "total_count": total_count,
        "current_page": offset // 10 + 1,
        "total_pages": (total_count + 9) // 10,
        "has_more": (offset + 10) < total_count
    }

    return popo_doctors, stats
