from typing import Annotated
from litestar import Controller, HttpMethod, Response, route
from litestar.enums import RequestEncodingType
from litestar.params import Body, Parameter
from litestar.datastructures import UploadFile
from src.types import ManifestGenerationRequest, EnvironmentsEnum
from src.core import Generator
from pydantic import ValidationError
import yaml


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
            secret_key: Annotated[str, Parameter(header="x-secret-key", default="")],
            data: Annotated[UploadFile, Body(media_type=RequestEncodingType.MULTI_PART)]
    ) -> Response:
        content = await data.read()
        try:
            data = yaml.safe_load(content)
        except yaml.YAMLError as e:
            return Response(status_code=400, content={"error": str(e)})

        try:
            data = ManifestGenerationRequest(**(data or {}))
            if data.secrets and not secret_key:
                return Response(status_code=400, content={"error": "Secret key header is required to decrypt secrets."})
        except ValidationError as e:
            return Response(status_code=400, content=e.errors())

        manifests = await Generator(
            payload=data,
            image=image,
            project_id=project_id,
            project_name=project_name,
            current_env=current_env.value,
            team=team,
            branch_name=branch_name,
            commit=commit,
            secret_key=secret_key,
        ).generate_manifests()
        content = "\n---\n".join(manifests)
        return Response(
            status_code=201,
            content=content,
            headers={"content-type": "application/x-yaml"},
        )
