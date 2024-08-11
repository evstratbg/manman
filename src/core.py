import asyncio
import copy

import yaml
import logging
from pathlib import Path
from .exceptions import NoTemplateFound

from .config import jinja_environment, settings, BASE_DIR

from .types import GenerationRequest, DatabaseMigration
from .utils.encrypter import AesEncoder

logger = logging.getLogger(__name__)


class Generator:
    TEMPLATES_DIR = BASE_DIR / "templates"

    def __init__(self, payload: GenerationRequest):
        self.payload = payload
        self.current_env = payload.metadata.current_env.value
        self.team = payload.metadata.team
        self.language = payload.engine.language.name.lower()

        self.default_common_dir = Path("_default")
        self._default_team_dir = Path(self.team) / "_default"
        self.target_lang_dir = Path(self.team) / self.language

        self.tolerations = self.get_tolerations()
        self.affinity = self.get_affinity()
        self.environment_variables = self.get_current_envs(payload.envs or {})

    def get_current_envs(self, envs: dict):
        flattened_envs = {}
        for key, value in envs.items():
            flattened_envs[key] = self.get_value(value)
        return flattened_envs

    def get_tolerations(self):
        default_tolerations = []
        for dir in [self.default_common_dir, self._default_team_dir, self.target_lang_dir]:
            tolerations_path = dir / "tolerations.yaml"
            if tolerations_path.exists():
                with open(tolerations_path) as f:
                    default_tolerations = yaml.safe_load(f)
        return self.get_value(default_tolerations)

    def get_affinity(self):
        default_affinity = {}
        for dir in [self.default_common_dir, self._default_team_dir, self.target_lang_dir]:
            affinity_path = dir / "affinity.yaml"
            if affinity_path.exists():
                with open(affinity_path) as f:
                    default_affinity = yaml.safe_load(f)
        return self.get_value(default_affinity)

    def get_value(self, d: dict | str | int | None):
        if isinstance(d, dict):
            return d.get(self.current_env, d.get("_default"))
        return d

    @property
    def predefined_envs(self):
        return {
            "CURRENT_ENV": self.payload.metadata.current_env.value,
            "COMMIT": self.payload.metadata.commit,
        }

    def add_secret_values(self):
        envs = {k: self.get_value(v) for k, v in self.predefined_envs.items()}

        self.environment_variables.update(envs)
        if self.payload.metadata.secrets:
            encrypted_secrets = self.payload.metadata.secrets.envs or {}
            for key, encrypted_value in encrypted_secrets.items():
                encrypted_value_str = str(self.get_value(encrypted_value))
                decrypted_value = AesEncoder(
                    self.payload.metadata.secrets.key.encode(),
                ).decrypt(encrypted_value_str.encode())

                self.environment_variables[key] = decrypted_value.decode()
        return self.environment_variables

    def get_migration_manifest_task(self, migration: DatabaseMigration):
        migration_template = jinja_environment.get_template(
            f"{self.jinja_language_prefix}/migration.yaml.jinja2",
        )
        migration_envs = copy.deepcopy(self.environment_variables)
        migration_envs.update(self.get_current_envs(migration.envs or {}))

        return [
            migration_template.render_async(
                image=self.payload.metadata.image,
                project_name=self.payload.metadata.project_name,
                current_env=self.payload.metadata.current_env.value,
                tolerations=self.tolerations,
                affinity=self.affinity,
                command=self.get_value(migration.command),
                envs=migration_envs,
                manman_release=settings.RELEASE,
                branch_name=self.payload.metadata.branch_name,
                commit=self.payload.metadata.commit,
                team=self.team,
            ),
        ]

    def get_server_manifest_task(self):
        deployment_template = jinja_environment.get_template(
            f"{self.jinja_language_prefix}/server.yaml.jinja2",
        )

        server = self.payload.server

        if not server:
            return []

        server_envs = copy.deepcopy(self.environment_variables)
        server_envs.update(self.get_current_envs(server.envs or {}))
        is_hpa_enabled = server.hpa is not None
        return [
            deployment_template.render_async(
                image=self.payload.metadata.image,
                replicas=self.get_value(server.replicas),
                project_name=self.payload.metadata.project_name,
                enable_prom_metrics=True,
                is_hpa_enabled=is_hpa_enabled,
                current_env=self.payload.metadata.current_env.value,
                tolerations=self.tolerations,
                affinity=self.affinity,
                envs=server_envs,
                memory_limits=self.get_value(server.memory_limits),
                memory_requests=self.get_value(server.requests.memory),
                cpu_requests=self.get_value(server.requests.cpu),
                helmgen_release=settings.RELEASE,
                branch_name=self.payload.metadata.branch_name,
                commit=self.payload.metadata.commit,
                team=self.team,
            ),
        ]

    def get_server_hpa_manifest_task(self):
        server_hpa_template = jinja_environment.get_template(
            f"{self.jinja_language_prefix}/server_hpa.yaml.jinja2",
        )

        if not self.payload.server or not self.payload.server.hpa:
            return []

        server_hpa = self.payload.server.hpa
        return [
            server_hpa_template.render_async(
                project_name=self.payload.metadata.project_name,
                min_replicas=self.get_value(server_hpa.min_replicas),
                max_replicas=self.get_value(server_hpa.max_replicas),
                target_cpu_utilization_percenage=self.get_value(
                    server_hpa.target_cpu_utilization_percent,
                ),
            ),
        ]

    def get_cronjob_manifest_task(self):
        cronjob_manifests_tasks = []

        cronjob_template = jinja_environment.get_template(
            f"{self.jinja_language_prefix}/cronjob.yaml.jinja2",
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
                    image=self.payload.metadata.image,
                    project_name=self.payload.metadata.project_name,
                    current_env=self.payload.metadata.current_env.value,
                    tolerations=self.tolerations,
                    affinity=self.affinity,
                    command=cronjob.command,
                    envs=cronjob_envs,
                    name=cronjob.name,
                    schedule=self.get_value(cronjob.schedule),
                    concurrency=cronjob.concurrency.value,
                    helmgen_release=settings.RELEASE,
                    branch_name=self.payload.metadata.branch_name,
                    commit=self.payload.metadata.commit,
                    team=self.team,
                ),
            )
        return cronjob_manifests_tasks

    def get_workers_manifest_task(self):
        workers_manifest_tasks = []
        worker_template = jinja_environment.get_template(
            f"{self.jinja_language_prefix}/worker.yaml.jinja2",
        )

        workers = self.payload.workers or []
        for worker in workers:
            is_enabled = self.get_value(worker.enabled)
            if not is_enabled:
                continue

            worker_envs = copy.deepcopy(self.environment_variables)
            worker_envs.update(self.get_current_envs(worker.envs or {}))

            is_hpa_enabled = worker.hpa is not None
            workers_manifest_tasks.append(
                worker_template.render_async(
                    image=self.payload.metadata.image,
                    project_name=self.payload.metadata.project_name,
                    current_env=self.payload.metadata.current_env.value,
                    tolerations=self.tolerations,
                    affinity=self.affinity,
                    is_hpa_enabled=is_hpa_enabled,
                    envs=worker_envs,
                    name=worker.name,
                    command=worker.command,
                    replicas=self.get_value(worker.replicas),
                    memory_limits=self.get_value(worker.memory_limits),
                    memory_requests=self.get_value(worker.requests.memory),
                    cpu_requests=self.get_value(worker.requests.cpu),
                    manman_release=settings.RELEASE,
                    branch_name=self.payload.metadata.branch_name,
                    commit=self.payload.metadata.commit,
                    team=self.team,
                ),
            )

        return workers_manifest_tasks

    async def generate_manifests(self):
        self.add_secret_values()
        manifest_tasks = []

        db_migrations = self.payload.db_migrations or []
        for migration in db_migrations:
            manifest_tasks.append(
                self.get_migration_manifest_task(migration=migration),
            )
        manifest_tasks.extend(self.get_server_manifest_task())
        manifest_tasks.extend(self.get_server_hpa_manifest_task())
        manifest_tasks.extend(self.get_cronjob_manifest_task())
        manifest_tasks.extend(self.get_workers_manifest_task())

        return await asyncio.gather(*manifest_tasks)

    async def generate_dockerfile(self):
        dockerfile_path = ""
        possible_dockerfile_paths = [
            self.default_common_dir / self.language / "dockerfile.jinja",
            self._default_team_dir / "dockerfile.jinja",
            self.target_lang_dir / "dockerfile.jinja",
        ]
        for path in possible_dockerfile_paths:
            full_path = self.TEMPLATES_DIR / path
            if full_path.exists():
                dockerfile_path = path

        if not dockerfile_path:
            raise NoTemplateFound(detail=f"No dockerfile found for team `{self.team}`")

        dockerfile_template = jinja_environment.get_template(dockerfile_path.as_posix())

        return await dockerfile_template.render_async(
            language=self.payload.engine.language.name,
            version=self.payload.engine.language.version,
            additional_system_packages=" ".join(self.payload.engine.additional_system_packages,),
            package_manager_version=self.payload.engine.package_manager,
        )
