from litestar import Controller, HttpMethod, Response, route
from src.types import DockerfileGenerationRequest, ManifestGenerationRequest
from src.core import Generator


class DockerfilesController(Controller):
    @route(path="/dockerfiles/generate", http_method=HttpMethod.POST, tags=("Dockerfiles generator",))
    async def generate_dockerfiles(self, data: DockerfileGenerationRequest) -> Response:
        data = ManifestGenerationRequest(
            metadata=data.metadata,
            engine=data.engine,
        )
        dockerfile = await Generator(data).generate_dockerfile()
        return Response(
            status_code=201,
            content=dockerfile,
            headers={"content-type": "text/plain"},
        )
