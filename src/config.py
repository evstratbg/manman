from pathlib import Path

from jinja2 import Environment, FileSystemLoader, select_autoescape, StrictUndefined
from pydantic_settings import BaseSettings
import orjson

BASE_DIR = Path(__file__).parent.resolve()


class Settings(BaseSettings):
    RELEASE: str
    VAULT_TOKEN: str = ""
    VAULT_BASE_URL: str | None

    ENVIRONMENTS: list[str]


settings = Settings()


jinja_environment = Environment(
    loader=FileSystemLoader(BASE_DIR / "templates"),
    enable_async=True,
    undefined=StrictUndefined,
    autoescape=select_autoescape(),
)
jinja_environment.filters["jsonify"] = lambda x: orjson.dumps(x).decode()
