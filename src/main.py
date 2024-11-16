"""
Main
"""
from contextlib import asynccontextmanager
from fastapi import FastAPI, APIRouter

from src.linebot.router import router as linebot_router
from src.popo.router import router as popo_router
from src.linebot.dependencies import line_bot_api_wrapper
from .config import settings

@asynccontextmanager
async def lifespan(_app: FastAPI):
    """
    Lifespan
    """
    await line_bot_api_wrapper.get_api()
    yield
    await line_bot_api_wrapper.close()

def create_app() -> FastAPI:
    "Create and configure the FastApi application"
    _app = FastAPI(
        title="Company Spotlight",
        description="Company Spotlight API",
        version="0.1.0",
        docs_url="/docs" if not settings.is_production else None,
        redoc_url="/redoc" if not settings.is_production else None,
        openapi_url="/openapi.json" if not settings.is_production else None,
        lifespan=lifespan)


    api_router = APIRouter(prefix="/api")
    api_router.include_router(popo_router, prefix="/popo", tags=["popo"])
    api_router.include_router(linebot_router, prefix="/linebot", tags=["linebot"])

    _app.include_router(api_router)

    return _app

app = create_app()

if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, port=8000)
