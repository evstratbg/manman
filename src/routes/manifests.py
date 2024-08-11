import yaml

from litestar import Controller, HttpMethod, Response, route
from src.types import GenerationRequest
from src.core import Generator


class ManifestsController(Controller):
    @route(
        path="/manifests/generate",
        http_method=HttpMethod.POST,
        tags=("Manifests generator",)
    )
    async def generate_manifests(self, data: GenerationRequest) -> Response:
        manifests = await Generator(data).generate_manifests()



        manifests = []
        content = yaml.safe_load_all("\n---\n".join(manifests))
        return Response(
            status_code=201,
            content=content,
            headers={"content-type": "application/yaml"},
        )