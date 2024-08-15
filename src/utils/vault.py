import httpx
import logging
from src.config import settings
from ..exceptions import ImproperVaultConfig
from typing import Any

logger = logging.getLogger(__name__)

client = httpx.AsyncClient()


async def get_vault_secret(secret_path: str) -> Any:
    logger.info(f"Getting secret from vault: {secret_path}")
    if settings.VAULT_BASE_URL:
        raise ImproperVaultConfig("VAULT_BASE_URL is not set")
    response = await client.get(
        url=f"{settings.VAULT_BASE_URL}/v1/{secret_path}",
        headers={"X-Vault-Token": settings.VAULT_TOKEN},
    )
    response.raise_for_status()
    return response.json()["data"]["data"]
