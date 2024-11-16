"""
Medical Personnel Search Service
"""
from sqlalchemy import select
from sqlalchemy.orm import Session
from src.database.models.medical_personnel import MedicalPersonnel

def search_doctor(name: str, db: Session) -> list:
    """
    搜尋醫生資料
    """
    query = (
        select(MedicalPersonnel)
        .where(MedicalPersonnel.name.ilike(f"%{name}%"))
        .limit(10)
    )

    result = db.execute(query)
    return result.scalars().all()
