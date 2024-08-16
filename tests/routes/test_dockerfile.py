from httpx import AsyncClient
import pytest

from tests.routes.conftest import DirLevel


@pytest.mark.parametrize("temp_directory", [
    DirLevel.ROOT,
    DirLevel.TEAM,
    DirLevel.TEAM_LANGUAGE
], indirect=True)
async def test__generate_dockerfile(
        client: AsyncClient,
) -> None:
    resp = await client.post(
        url="/dockerfiles/generate",
        json={
            "metadata": {
                "image": "string",
                "project_id": 0,
                "project_name": "string",
                "current_env": "dev",
                "team": "ml",
                "branch_name": "string",
                "commit": "string"
            },
            "engine": {
                "language": {
                    "name": "python",
                    "version": "string"
                },
                "additional_system_packages": [
                    "string"
                ],
                "package_manager": {"name": "rye", "version": "0.1.3"}
            }
        }
    )
    assert resp.status_code == 201
    assert resp.headers["content-type"] == "text/plain"
    assert "FROM python:string" in resp.text
