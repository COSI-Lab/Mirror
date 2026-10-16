from fastapi import APIRouter
from ..config import projects

router = APIRouter(prefix="/projects")

@router.get("/")
def get_project_info():
    return projects