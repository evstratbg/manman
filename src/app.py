import logging.config
from logging import getLogger

import uvicorn
from litestar import Litestar, MediaType, Request, Response
from litestar.exceptions import ValidationException
from litestar.contrib.pydantic import PydanticPlugin
from litestar.openapi.config import OpenAPIConfig
from litestar.openapi.plugins import SwaggerRenderPlugin, JsonRenderPlugin
from litestar.status_codes import HTTP_500_INTERNAL_SERVER_ERROR
from src.routes.manifests import ManifestsController
from src.routes.secrets import SecretsController
from src.routes.system import SystemController
from src.routes.dockerfiles import DockerfilesController

logger = getLogger(__name__)


def validation_exception_handler(request: Request, exc: ValidationException) -> Response:
    return Response(
        media_type=MediaType.JSON,
        content={"message": exc.detail, "details": exc.extra or []},
        status_code=422,
    )


def plain_text_exception_handler(_: Request, exc: Exception) -> Response:
    status_code = getattr(exc, "status_code", HTTP_500_INTERNAL_SERVER_ERROR)
    if status_code >= 500:
        logger.exception(exc)
    detail = getattr(exc, "detail", "")

    return Response(
        media_type=MediaType.JSON,
        content={"message": detail},
        status_code=status_code,
    )


app = Litestar(
    route_handlers=[SystemController, ManifestsController, SecretsController, DockerfilesController],
    openapi_config=OpenAPIConfig(
        title="ManMan API",
        version="0.1.0",
        path="/docs",
        render_plugins=[SwaggerRenderPlugin(), JsonRenderPlugin()],
    ),
    plugins=[PydanticPlugin(prefer_alias=True)],
    exception_handlers={
        Exception: plain_text_exception_handler,
        ValidationException: validation_exception_handler,
    },
)

if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=8000)
else:
    uvicorn_logger = logging.getLogger("uvicorn.access")
    uvicorn_logger.addFilter(lambda record: "/metrics" not in record.getMessage())
    uvicorn_logger.addFilter(lambda record: "/healthz" not in record.getMessage())
    uvicorn_logger.addFilter(lambda record: "/ready" not in record.getMessage())
