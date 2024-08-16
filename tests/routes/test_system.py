import pytest
from httpx import AsyncClient


@pytest.mark.parametrize("path", ["/liveness", "/ready"])
async def test__system_endpoints(client: AsyncClient, path: str) -> None:
    resp = await client.get(path)
    assert resp.status_code == 200

