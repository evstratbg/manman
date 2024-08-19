from litestar import Controller, HttpMethod, Response, route, Request
from litestar.params import Parameter
from typing import Annotated
from src.types import ManifestGenerationRequest, EnvironmentsEnum
from src.core import Generator
from pydantic import ValidationError


class ManifestsController(Controller):
    @route(path="/manifests/generate", http_method=HttpMethod.POST, tags=("Manifests generator",))
    async def generate_manifests(
            self,
            image: Annotated[str, Parameter(header="x-image")],
            project_id: Annotated[str, Parameter(header="x-project-id")],
            project_name: Annotated[str, Parameter(header="x-project-name")],
            current_env: Annotated[EnvironmentsEnum, Parameter(header="x-current-env")],
            team: Annotated[str, Parameter(header="x-team")],
            branch_name: Annotated[str, Parameter(header="x-branch-name")],
            commit: Annotated[str, Parameter(header="x-commit-hash")],
            request: Request
    ) -> Response:
        data = await request.json()
        try:
            data = ManifestGenerationRequest(**(data or {}))
        except ValidationError as e:
            return Response(status_code=400, content=e.errors())

        manifests = await Generator(
            payload=data,
            image=image,
            project_id=project_id,
            project_name=project_name,
            current_env=current_env,
            team=team,
            branch_name=branch_name,
            commit=commit,
        ).generate_manifests()
        content = "\n---\n".join(manifests)
        return Response(
            status_code=201,
            content=content,
            headers={"content-type": "application/yaml"},
        )
