import httpx
import logging
from src.config import settings

logger = logging.getLogger(__name__)

client = httpx.AsyncClient()


async def get_vault_secret(secret_path: str):
    response = await client.get(
        url=f"{settings.VAULT_BASE_URL}/v1/{secret_path}",
        headers={'X-Vault-Token': settings.VAULT_TOKEN},
    )
    response.raise_for_status()
    secret_data = response.json()['data']['data']
    return secret_data

