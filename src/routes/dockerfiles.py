from litestar import Controller, HttpMethod, Response, route
from src.types import GenerationRequest
from src.core import Generator


class DockerfilesController(Controller):
    @route(
        path="/dockerfiles/generate",
        http_method=HttpMethod.POST,
        tags=("Dockerfiles generator",)
    )
    async def generate_dockerfiles(self, data: GenerationRequest) -> Response:
        dockerfile = await Generator(data).generate_dockerfile()
        return Response(
            status_code=201,
            content=dockerfile,
            headers={"content-type": "text/plain"},
        )