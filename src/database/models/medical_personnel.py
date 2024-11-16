import enum
from sqlalchemy import Column, Integer, String, Enum
from .base import Base

class GraduationStatus(enum.Enum):
    STUDYING = "在學"
    GRADUATED = "畢業"

class MedicalPersonnel(Base):
    __tablename__ = 'medical_personnel'

    id = Column(Integer, primary_key=True, autoincrement=True)
    city = Column(String(50), nullable=False, index=True, comment='縣市')
    hospital = Column(String(200), nullable=False, comment='醫療院所')
    department = Column(String(100), comment='科別')
    name = Column(String(100), nullable=False, comment='姓名')
    education = Column(String(500), comment='學歷')
    university = Column(String(200), comment='大學名稱')
    graduation_status = Column(
        Enum(GraduationStatus),
        nullable=False,
        default=GraduationStatus.GRADUATED,
        comment='在學狀態'
    )

    def __repr__(self):
        return f"<MedicalPersonnel(id={self.id}, name={self.name}, hospital={self.hospital})>"

    def to_dict(self):
        return {
            'id': self.id,
            'city': self.city,
            'hospital': self.hospital,
            'department': self.department,
            'name': self.name,
            'education': self.education,
            'university': self.university,
            'graduation_status': self.graduation_status.value,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None
        }
