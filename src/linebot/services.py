"""
Medical Personnel Search Service
"""
from enum import Enum
from dataclasses import dataclass
from sqlalchemy import select, func
from sqlalchemy.orm import Session
from src.database.models.medical_personnel import MedicalPersonnel


class SearchType(Enum):
    NAME = "name"
    HOSPITAL = "hospital"
    DEPARTMENT = "department"
    

@dataclass
class SearchCriteria:
    search_type: SearchType
    search_term: str
    city: str | None
    
def parse_search_criteria(message: str) -> SearchCriteria:
    """
    解析搜尋條件
    支援格式:
    - [城市]@醫院 醫院名稱  (例如: 台北@醫院 台大醫院)
    - [城市]@科別 科別名稱  (例如: 台北@科別 小兒科)
    - [城市] 醫師名稱      (例如: 台北王大明)
    """
    cities = [
        "南投", "台中", "台北", "台南", "台東", "嘉義", "基隆",
        "宜蘭", "屏東", "彰化", "新北", "新竹", "桃園", "花蓮",
        "苗栗", "雲林", "高雄"
    ]
    
    # 預設值
    city = None
    search_type = SearchType.NAME
    search_term = message.strip()
    
    # 檢查是否包含城市
    for possible_city in cities:
        if message.startswith(possible_city):
            city = possible_city
            search_term = message[len(city):].strip()
            break
    
    # 檢查搜尋類型
    if search_term.startswith("@醫院 "):
        search_type = SearchType.HOSPITAL
        search_term = search_term[4:].strip()
    elif search_term.startswith("@科別 "):
        search_type = SearchType.DEPARTMENT
        search_term = search_term[4:].strip()
    
    return SearchCriteria(search_type, search_term, city)

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

def format_search_summary(criteria: SearchCriteria, stats: dict) -> str:
    """
    格式化搜尋結果摘要
    """
    location_text = f"在{criteria.city}" if criteria.city else "全台"
    search_type_text = {
        SearchType.NAME: "醫師",
        SearchType.HOSPITAL: "醫院",
        SearchType.DEPARTMENT: "科別"
    }[criteria.search_type]
    
    current_range = f"{stats['current_page']*10-9} - {min(stats['current_page']*10, stats['total_count'])}"

    return (
        f"查詢{location_text}{search_type_text}「{criteria.search_term}」\n"
        f"共有 {stats['total_count']} 筆符合的結果\n"
        f"目前顯示第 {current_range} 筆"
    )