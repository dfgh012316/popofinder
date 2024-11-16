from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy.orm import Session
from sqlalchemy import select
from typing import List, Optional
from pydantic import BaseModel
from ..database.connection import get_db
from ..database.models.medical_personnel import MedicalPersonnel

router = APIRouter()


# Pydantic models
class PersonnelResponse(BaseModel):
    id: int
    city: str
    hospital: str
    department: Optional[str]
    name: str
    education: Optional[str]
    university: Optional[str]
    graduation_status: str

    class Config:
        from_attributes = True


class StatsResponse(BaseModel):
    total_count: int
    city_distribution: dict
    university_distribution: dict
    department_distribution: dict


@router.get("/personnel", response_model=List[PersonnelResponse])
async def get_personnel(
    db: Session = Depends(get_db),
    skip: int = Query(0, description="Skip first N records"),
    limit: int = Query(100, description="Limit the number of records returned"),
    city: Optional[str] = Query(None, description="Filter by city"),
    hospital: Optional[str] = Query(None, description="Filter by hospital"),
    department: Optional[str] = Query(None, description="Filter by department"),
    university: Optional[str] = Query(None, description="Filter by university"),
    name: Optional[str] = Query(None, description="Search by name"),
):
    query = select(MedicalPersonnel)

    if city:
        query = query.where(MedicalPersonnel.city == city)
    if hospital:
        query = query.where(MedicalPersonnel.hospital.ilike(f"%{hospital}%"))
    if department:
        query = query.where(MedicalPersonnel.department.ilike(f"%{department}%"))
    if university:
        query = query.where(MedicalPersonnel.university.ilike(f"%{university}%"))
    if name:
        query = query.where(MedicalPersonnel.name.ilike(f"%{name}%"))

    query = query.offset(skip).limit(limit)
    result = db.execute(query)
    return result.scalars().all()


@router.get("/personnel/cities")
async def get_cities(db: Session = Depends(get_db)):
    query = select(MedicalPersonnel.city).distinct()
    result = db.execute(query)
    return [row[0] for row in result]


@router.get("/personnel/departments")
async def get_departments(db: Session = Depends(get_db)):
    query = (
        select(MedicalPersonnel.department)
        .where(MedicalPersonnel.department != "")
        .distinct()
    )
    result = db.execute(query)
    return [row[0] for row in result]


@router.get("/personnel/universities")
async def get_universities(db: Session = Depends(get_db)):
    query = (
        select(MedicalPersonnel.university)
        .where(MedicalPersonnel.university != "")
        .distinct()
    )
    result = db.execute(query)
    return [row[0] for row in result]


@router.get("/personnel/{personnel_id}", response_model=PersonnelResponse)
async def get_personnel_by_id(personnel_id: int, db: Session = Depends(get_db)):
    query = select(MedicalPersonnel).where(MedicalPersonnel.id == personnel_id)
    result = db.execute(query)
    personnel = result.scalar_one_or_none()

    if not personnel:
        raise HTTPException(status_code=404, detail="Personnel not found")
    return personnel
