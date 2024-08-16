from pathlib import Path
from jinja2 import Environment, FileSystemLoader, select_autoescape
from pydantic_settings import BaseSettings, SettingsConfigDict
import orjson

BASE_DIR = Path(__file__).parent.parent.resolve()
TEMPLATES_DIR = BASE_DIR / "templates"


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=BASE_DIR / ".env")

    RELEASE: str

    ENVIRONMENTS: list[str]


settings = Settings()

jinja_environment = Environment(
    loader=FileSystemLoader(TEMPLATES_DIR),
    enable_async=True,
    autoescape=select_autoescape(),
)
jinja_environment.filters["jsonify"] = lambda x: orjson.dumps(x).decode()
