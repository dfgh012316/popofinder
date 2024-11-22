"""
PoPo Doctor Schema
"""
from enum import Enum
from dataclasses import dataclass

class SearchType(Enum):
    """
    Search Type
    """
    NAME = "name"
    HOSPITAL = "hospital"
    DEPARTMENT = "department"


@dataclass
class SearchCriteria:
    """
    SearchCriteria
    """
    search_type: SearchType
    search_term: str
    city: str | None
