from typing import ClassVar

from litestar import Controller, HttpMethod, Response, route


class SystemController(Controller):
    """Controller for system endpoints."""

    include_in_schema = True

    tags: ClassVar = ["System"]  # type: ignore[misc]

    @route(path="/liveness", http_method=HttpMethod.GET)
    async def liveness(self) -> Response:
        return Response(status_code=200, content="")

    @route(path="/ready", http_method=HttpMethod.GET)
    async def readiness(self) -> Response:
        return Response(status_code=200, content="")
