"""
Medical Personnel Search Service
"""
from sqlalchemy import select, func
from sqlalchemy.orm import Session
from src.database.models.medical_personnel import MedicalPersonnel
from .schemas import SearchType, SearchCriteria


def search_doctor(criteria: SearchCriteria, db: Session, offset: int = 0) -> tuple[list, dict]:
    """
    搜尋醫生資料
    Args:
        criteria: 搜尋條件
        db: 資料庫連線
        offset: 分頁偏移量
    Returns:
        (搜尋結果列表, 搜尋統計資訊)
    """
    base_query = select(MedicalPersonnel)

    # 根據搜尋類型加入不同的條件
    if criteria.search_type == SearchType.NAME:
        base_query = base_query.where(
            MedicalPersonnel.name.ilike(f"%{criteria.search_term}%")
        )
    elif criteria.search_type == SearchType.HOSPITAL:
        base_query = base_query.where(
            MedicalPersonnel.hospital.ilike(f"%{criteria.search_term}%")
        )
    elif criteria.search_type == SearchType.DEPARTMENT:
        base_query = base_query.where(
            MedicalPersonnel.department.ilike(f"%{criteria.search_term}%")
        )

    # 加入城市搜尋條件
    if criteria.city:
        base_query = base_query.where(MedicalPersonnel.city == criteria.city)

    # 計算總筆數
    count_query = select(func.count(1)).select_from(base_query.subquery())
    total_count = db.execute(count_query).scalar()

    # 限制回傳數量
    query = base_query.offset(offset).limit(10)
    result = db.execute(query)

    doctors = result.scalars().all()

    stats = {
        "total_count": total_count,
        "current_page": offset // 10 + 1,
        "total_pages": (total_count + 9) // 10,
        "has_more": (offset + 10) < total_count
    }

    return doctors, stats
