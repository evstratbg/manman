import re
from enum import Enum
from typing import Any
from src.config import settings
from croniter import croniter
from .exceptions import ImproperConfig

from pydantic import Field, BaseModel, model_validator, field_validator


EnvValue = str | int
EnvironmentValue = dict[str, EnvValue] | EnvValue
EnvironmentDict = dict[str, EnvironmentValue]


RAM_REGEX = "^[1-9][0-9]{0,3}[MG]i$"
CPU_REGEX = "^[1-9][0-9]{0,4}m$"

RAM_REGEX_COMPILED = re.compile(RAM_REGEX)
CPU_REGEX_COMPILED = re.compile(CPU_REGEX)


EnvironmentsEnum = Enum("EnvironmentsEnum", {env.lower(): env.lower() for env in settings.ENVIRONMENTS})  # type: ignore[misc]


def validate_autoscalers(values: dict[str, Any]) -> dict[str, Any]:
    hpa_specified = values.get("hpa") is not None
    replicas_specified = values.get("replicas") is not None

    if hpa_specified and replicas_specified:
        raise ImproperConfig(
            "Incompatible configuration: When using HPA, "
            "you cannot manually set the number of replicas. Please remove the replicas setting or disable HPA.",
        )

    if not hpa_specified and not replicas_specified:
        raise ImproperConfig(
            "Insufficient scaling information: You need to either specify a fixed "
            "number of replicas or configure HPA. Please add one of these parameters to your configuration.",
        )

    return values


class PackageManager(BaseModel):
    name: str
    version: str


class Language(BaseModel):
    name: str
    version: str


class Engine(BaseModel):
    language: Language
    additional_system_packages: list[str]
    package_manager: PackageManager | None


class Requests(BaseModel):
    memory: str | dict
    cpu: str | dict

    @model_validator(mode="before")
    def check_cpu_and_memory(cls, values: str | dict) -> str | dict:  # noqa: N805
        memory = values["memory"]
        cpu = values["cpu"]

        if isinstance(memory, str):
            memory = {"_default": memory}
        for value in memory.values():
            if not RAM_REGEX_COMPILED.match(value):
                raise ImproperConfig("Invalid RAM limit")

        if isinstance(cpu, str):
            cpu = {"_default": cpu}
        for value in cpu.values():
            if not CPU_REGEX_COMPILED.match(value):
                raise ImproperConfig("Invalid RAM limit")

        return values


class BaseHorizintalPodAutoscaler(BaseModel):
    min_replicas: int | dict
    max_replicas: int | dict


class ServerHorizontalPodAutoscaler(BaseHorizintalPodAutoscaler):
    target_cpu_utilization_percent: int = Field(
        description="CPU target utilization percentage for automated horizontal scaling",
        gt=0,
        le=100,
    )


class Metadata(BaseModel):
    image: str
    project_id: int
    project_name: str
    current_env: EnvironmentsEnum
    team: str
    branch_name: str
    commit: str


class Server(BaseModel):
    replicas: int | dict | None = None
    memory_limits: str | dict | None = None
    requests: Requests = None
    envs: dict[str, str | int | dict] | None = None
    hpa: ServerHorizontalPodAutoscaler | None = None

    @model_validator(mode="before")
    def check_autoscalers(cls, values: dict[str, Any]) -> dict[str, Any]:  # noqa: N805
        return validate_autoscalers(values)


class Concurrency(str, Enum):
    replace_ = "replace"
    forbid = "forbid"
    allow = "allow"


class Cronjobs(BaseModel):
    concurrency: Concurrency
    command: str
    enabled: bool | dict
    name: str
    schedule: str | dict
    envs: EnvironmentDict | None = None

    @field_validator("schedule")
    def validate_schedule(cls, value: str | dict) -> str | dict:  # noqa: N805
        if isinstance(value, str):
            if not croniter.is_valid(value):
                raise ImproperConfig("Invalid cron schedule")
        else:
            for env, v in value.items():
                if not croniter.is_valid(v):
                    raise ImproperConfig(f"Invalid cron schedule for env `{env}`")
        return value

    @field_validator("name")
    def validate_cronjob_name(cls, value: str) -> str:  # noqa: N805
        if not re.match(r'^[a-z0-9]([-a-z0-9]*[a-z0-9])?$', value):
            raise ImproperConfig("Invalid cron name")
        return value


class Consumer(BaseModel):
    name: str
    enabled: bool | dict
    command: str
    replicas: int | dict
    envs: EnvironmentDict | None = None
    memory_limits: str | dict | None = None
    requests: Requests | None = None

    @field_validator("name")
    def validate_consumer_name(cls, value: str) -> str:  # noqa: N805
        pattern = r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
        if re.match(pattern, value) is None:
            raise ImproperConfig("Invalid consumer name")
        return value

    @model_validator(mode="before")
    def check_autoscalers(cls, values: dict[str, Any]) -> dict[str, Any]:  # noqa: N805
        return validate_autoscalers(values)


class DatabaseMigration(BaseModel):
    command: str | dict
    envs: EnvironmentDict | None = None


class Secrets(BaseModel):
    envs: EnvironmentDict | None = None


class DockerfileGenerationRequest(BaseModel):
    engine: Engine


class ManifestGenerationRequest(DockerfileGenerationRequest):
    server: Server | None = None
    db_migrations: list[DatabaseMigration] | None = []
    cronjobs: list[Cronjobs] | None = []
    consumers: list[Consumer] | None = []
    secrets: Secrets | None = None

    envs: EnvironmentDict | None = None


class SecretsEncryptRequest(BaseModel):
    envs: dict[str, str | dict]
    secret_key: bytes | None = None
