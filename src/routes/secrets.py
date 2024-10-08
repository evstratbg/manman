import yaml

from litestar import Controller, HttpMethod, Response, route
from litestar.exceptions import HTTPException
from src.types import SecretsEncryptRequest
from src.utils.encrypter import AesEncoder


class SecretsController(Controller):
    @route(path="/secrets/encrypt", http_method=HttpMethod.POST, tags=("Secrets",))
    async def encrypt_envs(self, data: SecretsEncryptRequest) -> Response:
        if not data.secret_key:
            secret_key = AesEncoder.generate_key()
        else:
            is_key_valid = AesEncoder.is_valid_key(data.secret_key)
            if not is_key_valid:
                raise HTTPException(status_code=400, detail="Invalid secret key.")
            secret_key = data.secret_key

        encoder = AesEncoder(secret_key)
        envs_with_encrypted_values: dict[str, dict[str, str]] = {}
        for key, value in data.envs.items():
            if isinstance(value, dict):
                for env_name, env_value in value.items():
                    envs_with_encrypted_values.setdefault(key, {})[env_name] = encoder.encrypt(env_value.encode())
            else:
                envs_with_encrypted_values.setdefault(key, {})["_default"] = encoder.encrypt(value.encode())

        content = yaml.dump(envs_with_encrypted_values)
        return Response(
            status_code=201,
            content=content,
            headers={"content-type": "application/yaml", "x-secret-key": secret_key.decode()},
        )
