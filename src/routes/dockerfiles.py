from litestar import Controller, HttpMethod, Response, route
from litestar.params import Parameter
from src.types import DockerfileGenerationRequest, ManifestGenerationRequest, EnvironmentsEnum
from src.core import Generator
from typing import Annotated


class DockerfilesController(Controller):
    @route(path="/dockerfiles/generate", http_method=HttpMethod.POST, tags=("Dockerfiles generator",))
    async def generate_dockerfiles(
            self,
            image: Annotated[str, Parameter(header="x-image")],
            project_id: Annotated[str, Parameter(header="x-project-id")],
            project_name: Annotated[str, Parameter(header="x-project-name")],
            current_env: Annotated[EnvironmentsEnum, Parameter(header="x-current-env")],
            team: Annotated[str, Parameter(header="x-team")],
            branch_name: Annotated[str, Parameter(header="x-branch-name")],
            commit: Annotated[str, Parameter(header="x-commit-hash")],
            data: DockerfileGenerationRequest
    ) -> Response:
        data = ManifestGenerationRequest(
            metadata=data.metadata,
            engine=data.engine,
        )
        dockerfile = await Generator(
            payload=data,
            image=image,
            project_id=project_id,
            project_name=project_name,
            current_env=current_env,
            team=team,
            branch_name=branch_name,
            commit=commit,
        ).generate_dockerfile()
        return Response(
            status_code=201,
            content=dockerfile,
            headers={"content-type": "text/plain"},
        )
