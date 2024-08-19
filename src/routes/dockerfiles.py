import yaml
from litestar import Controller, HttpMethod, Response, route
from litestar.enums import RequestEncodingType
from litestar.params import Body, Parameter
from litestar.datastructures import UploadFile
from src.types import ManifestGenerationRequest, EnvironmentsEnum, DockerfileGenerationRequest
from src.core import Generator
from typing import Annotated
from pydantic import ValidationError


class DockerfilesController(Controller):
    @route(path="/dockerfiles/generate", http_method=HttpMethod.POST, tags=("Dockerfiles generator",))
    async def generate_dockerfiles(
            self,
            project_id: Annotated[str, Parameter(header="x-project-id")],
            project_name: Annotated[str, Parameter(header="x-project-name")],
            current_env: Annotated[EnvironmentsEnum, Parameter(header="x-current-env")],
            team: Annotated[str, Parameter(header="x-team")],
            branch_name: Annotated[str, Parameter(header="x-branch-name")],
            commit: Annotated[str, Parameter(header="x-commit-hash")],
            data: Annotated[UploadFile, Body(media_type=RequestEncodingType.MULTI_PART)],
    ) -> Response:
        content = await data.read()
        try:
            data = yaml.safe_load(content)
        except yaml.YAMLError as e:
            return Response(status_code=400, content={"error": str(e)})
        try:
            data = ManifestGenerationRequest(engine=DockerfileGenerationRequest(**(data or {})).engine)
        except ValidationError as e:
            return Response(status_code=400, content=e.errors())

        dockerfile = await Generator(
            image="",
            payload=data,
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
