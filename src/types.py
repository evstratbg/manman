import re
from enum import Enum
from typing import Any
from src.config import settings
from croniter import croniter

from pydantic import Field, BaseModel, field_validator


EnvValue = str | dict[str, str]
EnvironmentDict = dict[str, EnvValue]

RAM_REGEX = "^[1-9][0-9]{0,3}[MG]i$"
CPU_REGEX = "^[1-9][0-9]{0,4}m$"

EnvironmentsEnum = Enum("EnvironmentsEnum", {env.lower(): env.lower() for env in settings.ENVIRONMENTS})  # type: ignore[misc]


def validate_autoscalers(values: dict[str, Any]) -> dict[str, Any]:
    hpa_specified = values.get("hpa") is not None
    replicas_specified = values.get("replicas") is not None

    if hpa_specified and replicas_specified:
        raise ValueError(
            "Incompatible configuration: When using HPA, "
            "you cannot manually set the number of replicas. Please remove the replicas setting or disable HPA.",
        )

    if not hpa_specified and not replicas_specified:
        raise ValueError(
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
    memory: str | dict = Field(
        description="Memory limit for container",
        pattern=RAM_REGEX,
    )
    cpu: str | dict = Field(
        description="CPU requests for container",
        pattern=CPU_REGEX,
    )


class BaseHorizintalPodAutoscaler(BaseModel):
    min_replicas: int | dict = Field(
        description="Minimum amount of deployment replicas",
        gt=0,
        le=5,
    )
    max_replicas: int | dict = Field(
        description="Maximum amount of deployment replicas",
        gt=0,
        le=5,
    )


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
    replicas: int | dict | None = Field(
        description="Number of replicas",
        gt=0,
    )
    memory_limits: str | dict = Field(
        description="Memory limit for container",
        pattern=RAM_REGEX,
    )
    requests: Requests
    envs: dict[str, str | dict] | None
    hpa: ServerHorizontalPodAutoscaler | None

    @field_validator("*", mode="before")
    def check_autoscalers(cls, values: dict[str, Any]) -> dict[str, Any]:  # noqa: N805
        return validate_autoscalers(values)


class Concurrency(str, Enum):
    replace_ = "Replace"
    forbid = "Forbid"
    allow = "Allow"


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
                raise ValueError("Invalid cron schedule")
        else:
            for env, v in value.items():
                if not croniter.is_valid(v):
                    raise ValueError(f"Invalid cron schedule for env `{env}`")
        return value

    @field_validator("name")
    def validate_cronjob_name(cls, value: str) -> str:  # noqa: N805
        pattern = r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
        if re.match(pattern, value) is None:
            raise ValueError("Invalid cron name")
        return value


class Consumer(BaseModel):
    name: str
    enabled: bool | dict
    command: str
    replicas: int | dict | None = Field(
        description="Number of replicas",
        gt=0,
    )
    envs: dict[str, str | dict] | None
    memory_limits: str | dict = Field(
        description="Memory limit for container",
        pattern=RAM_REGEX,
    )
    requests: Requests

    @field_validator("name")
    def validate_consumer_name(cls, value: str) -> str:  # noqa: N805
        pattern = r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
        if re.match(pattern, value) is None:
            raise ValueError("Invalid consumer name")
        return value

    @field_validator("*", mode="before")
    def check_autoscalers(cls, values: dict[str, Any]) -> dict[str, Any]:  # noqa: N805
        return validate_autoscalers(values)


class DatabaseMigration(BaseModel):
    command: str | dict
    envs: EnvironmentDict | None = None


class Secrets(BaseModel):
    key: str
    envs: EnvironmentDict | None = None


class DockerfileGenerationRequest(BaseModel):
    metadata: Metadata
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
