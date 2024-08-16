import asyncio
import copy
from typing import Any

import yaml
import logging
from pathlib import Path
from .exceptions import NoTemplateFound

from .config import jinja_environment, settings, TEMPLATES_DIR

from .types import ManifestGenerationRequest
from .utils.encrypter import AesEncoder

logger = logging.getLogger(__name__)


class Generator:
    def __init__(
            self,
            payload: ManifestGenerationRequest,
            image: str,
            project_id: str,
            project_name: str,
            current_env: str,
            team: str,
            branch_name: str,
            commit: str,
    ):
        self.payload = payload
        self.image = image
        self.project_id = project_id
        self.project_name = project_name
        self.current_env = current_env
        self.team = team
        self.branch_name = branch_name
        self.commit = commit

        self.language = payload.engine.language.name.lower()

        self.default_common_dir = Path("_default")
        self._default_team_dir = Path(self.team) / "_default"
        self.target_lang_dir = Path(self.team) / self.language

        self._default_folders = [
            self.default_common_dir,
            self._default_team_dir,
            self.target_lang_dir,
        ]

        self.tolerations = self.get_tolerations()
        self.affinity = self.get_affinity()
        self.environment_variables = self.get_current_envs(payload.envs or {})

    def get_template_path(self, file_name: str) -> str:
        template_path = Path()
        for folder in self._default_folders:
            path = folder / file_name
            full_path = TEMPLATES_DIR / path
            if full_path.exists():
                template_path = path
        if template_path == Path():
            raise NoTemplateFound(
                status_code=400,
                detail=f"No {file_name} template found for team `{self.team}` and engine `{self.language}`", )
        return template_path.as_posix()

    def get_current_envs(self, envs: dict) -> dict[str, Any]:
        flattened_envs = {}
        for key, value in envs.items():
            flattened_envs[key] = self.get_value(value)
        return flattened_envs

    def get_tolerations(self) -> Any:
        default_tolerations = {}
        for folder in self._default_folders:
            tolerations_path = folder / "tolerations.yaml"
            if tolerations_path.exists():
                with tolerations_path.open() as f:
                    default_tolerations = yaml.safe_load(f)
        return self.get_value(default_tolerations)

    def get_affinity(self) -> Any:
        default_affinity = {}
        for folder in self._default_folders:
            affinity_path = folder / "affinity.yaml"
            if affinity_path.exists():
                with affinity_path.open() as f:
                    default_affinity = yaml.safe_load(f)
        return self.get_value(default_affinity)

    def get_value(self, d: dict | str | int | None) -> Any:
        if isinstance(d, dict):
            return d.get(self.current_env, d.get("_default"))
        return d

    @property
    def predefined_envs(self) -> dict[str, str]:
        return {
            "CURRENT_ENV": self.current_env,
            "COMMIT": self.commit,
        }

    def add_secret_values(self) -> dict[str, str]:
        envs = {k: self.get_value(v) for k, v in self.predefined_envs.items()}

        self.environment_variables.update(envs)
        if self.payload.secrets:
            encrypted_secrets = self.payload.secrets.envs or {}
            for key, encrypted_value in encrypted_secrets.items():
                encrypted_value_str = str(self.get_value(encrypted_value))
                decrypted_value = AesEncoder(
                    self.payload.secrets.key.encode(),
                ).decrypt(encrypted_value_str.encode())

                self.environment_variables[key] = decrypted_value.decode()
        return self.environment_variables

    def get_migration_manifest_task(self) -> list[Any]:
        migration_template_path = self.get_template_path("migration.yaml.jinja2")
        migration_template = jinja_environment.get_template(
            migration_template_path,
        )

        migration_tasks = []
        db_migrations = self.payload.db_migrations or []
        for migration in db_migrations:
            migration_envs = copy.deepcopy(self.environment_variables)
            migration_envs.update(self.get_current_envs(migration.envs or {}))
            migration_tasks.append(
                migration_template.render_async(
                    image=self.image,
                    project_name=self.project_name,
                    current_env=self.current_env,
                    tolerations=self.tolerations,
                    affinity=self.affinity,
                    command=self.get_value(migration.command),
                    envs=migration_envs,
                    manman_release=settings.RELEASE,
                    branch_name=self.branch_name,
                    commit=self.commit,
                    team=self.team,
                ),
            )
        return migration_tasks

    def get_server_manifest_task(self) -> list[Any]:
        deployment_template_path = self.get_template_path("server.yaml.jinja2")
        deployment_template = jinja_environment.get_template(
            deployment_template_path,
        )

        server = self.payload.server
        if not server:
            return []

        server_envs = copy.deepcopy(self.environment_variables)
        server_envs.update(self.get_current_envs(server.envs or {}))
        is_hpa_enabled = server.hpa is not None
        return [
            deployment_template.render_async(
                image=self.image,
                replicas=self.get_value(server.replicas),
                project_name=self.project_name,
                is_hpa_enabled=is_hpa_enabled,
                current_env=self.current_env,
                tolerations=self.tolerations,
                affinity=self.affinity,
                envs=server_envs,
                memory_limits=self.get_value(server.memory_limits),
                memory_requests=self.get_value(server.requests.memory),
                cpu_requests=self.get_value(server.requests.cpu),
                manman_release=settings.RELEASE,
                branch_name=self.branch_name,
                commit=self.commit,
                team=self.team,
            ),
        ]

    def get_server_hpa_manifest_task(self) -> list[Any]:
        server_hpa_template_path = self.get_template_path("server_hpa.yaml.jinja2")
        server_hpa_template = jinja_environment.get_template(
            server_hpa_template_path,
        )

        if not self.payload.server or not self.payload.server.hpa:
            return []

        server_hpa = self.payload.server.hpa
        return [
            server_hpa_template.render_async(
                project_name=self.project_name,
                min_replicas=self.get_value(server_hpa.min_replicas),
                max_replicas=self.get_value(server_hpa.max_replicas),
                target_cpu_utilization_percentage=self.get_value(
                    server_hpa.target_cpu_utilization_percent,
                ),
            ),
        ]

    def get_cronjob_manifest_task(self) -> list[Any]:
        cronjob_manifests_tasks = []

        cronjob_template_path = self.get_template_path("cronjob.yaml.jinja2")
        cronjob_template = jinja_environment.get_template(
            cronjob_template_path,
        )

        cronjobs = self.payload.cronjobs or []
        for cronjob in cronjobs:
            is_enabled = self.get_value(cronjob.enabled)
            if not is_enabled:
                continue

            cronjob_envs = copy.deepcopy(self.environment_variables)
            cronjob_envs.update(self.get_current_envs(cronjob.envs or {}))

            cronjob_manifests_tasks.append(
                cronjob_template.render_async(
                    image=self.image,
                    project_name=self.project_name,
                    current_env=self.current_env.value,
                    tolerations=self.tolerations,
                    affinity=self.affinity,
                    command=cronjob.command,
                    envs=cronjob_envs,
                    name=cronjob.name,
                    schedule=self.get_value(cronjob.schedule),
                    concurrency=cronjob.concurrency.value,
                    manman_release=settings.RELEASE,
                    branch_name=self.branch_name,
                    commit=self.commit,
                    team=self.team,
                ),
            )
        return cronjob_manifests_tasks

    def get_consumers_manifest_task(self) -> list[Any]:
        consumer_manifest_tasks = []
        consumers_template_path = self.get_template_path("consumer.yaml.jinja2")
        consumers_template = jinja_environment.get_template(
            consumers_template_path,
        )

        consumers = self.payload.consumers or []
        for consumer in consumers:
            is_enabled = self.get_value(consumer.enabled)
            if not is_enabled:
                continue

            worker_envs = copy.deepcopy(self.environment_variables)
            worker_envs.update(self.get_current_envs(consumer.envs or {}))

            consumer_manifest_tasks.append(
                consumers_template.render_async(
                    image=self.image,
                    project_name=self.project_name,
                    current_env=self.current_env.value,
                    tolerations=self.tolerations,
                    affinity=self.affinity,
                    envs=worker_envs,
                    name=consumer.name,
                    command=consumer.command,
                    replicas=self.get_value(consumer.replicas),
                    memory_limits=self.get_value(consumer.memory_limits),
                    memory_requests=self.get_value(consumer.requests.memory),
                    cpu_requests=self.get_value(consumer.requests.cpu),
                    manman_release=settings.RELEASE,
                    branch_name=self.branch_name,
                    commit=self.commit,
                    team=self.team,
                ),
            )

        return consumer_manifest_tasks

    async def generate_manifests(self) -> list[str]:
        self.add_secret_values()
        manifest_tasks = []

        manifest_tasks.extend(self.get_migration_manifest_task())
        manifest_tasks.extend(self.get_server_manifest_task())
        manifest_tasks.extend(self.get_server_hpa_manifest_task())
        manifest_tasks.extend(self.get_cronjob_manifest_task())
        manifest_tasks.extend(self.get_consumers_manifest_task())

        res = await asyncio.gather(*manifest_tasks)  # type: ignore[call-overload]
        return list(res)

    async def generate_dockerfile(self) -> str:
        dockerfile_path = self.get_template_path("dockerfile.jinja")
        dockerfile_template = jinja_environment.get_template(dockerfile_path)
        return await dockerfile_template.render_async(
            language=self.payload.engine.language.name,
            version=self.payload.engine.language.version,
            additional_system_packages=" ".join(
                self.payload.engine.additional_system_packages,
            ),
            package_manager=self.payload.engine.package_manager.name,
            package_manager_version=self.payload.engine.package_manager.version,
        )
