"""
Config
"""

from functools import lru_cache
from pathlib import Path
from pydantic_settings import BaseSettings, SettingsConfigDict

base_dir = Path(__file__).resolve().parent.parent


class Settings(BaseSettings):
    """
    Settings
    """

    ENVIRONMENT: str

    @property
    def is_production(self):
        return self.ENVIRONMENT.lower() == "production"

    LINE_MESSAGE_CHANNEL_SECRET: str
    LINE_MESSAGE_CHANNEL_TOKEN: str

    DB_HOST: str
    DB_PORT: str
    DB_USER: str
    DB_PASSWORD: str
    DB_NAME: str

    model_config = SettingsConfigDict(env_file=f"{base_dir}/.env")


@lru_cache
def get_settings():
    """
    Get settings
    """
    return Settings()


settings = get_settings()
