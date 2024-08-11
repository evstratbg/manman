# manman

manman is a web server, designed for centralized management of dockerfiles and k8s manifests.

It allows you to create dockerfiles templates and kubernetes manifests for your teams and manage them in a centralized way

## Before you start

`manman API` has the following endpoints:
- `/manifests/generate` - to generate k8s manifests
- `/dockerfiles/generate` - to generate dockerfiles
- `/secrets/encrypt` - to encrypt secret values

Swagger is available at `/docs`

----
`templates` dir should contain the following structure, but `_default` folder is optional:

```
templates
    - _default
       - {{language}}
            - dockerfile.jinja

    - {{team name}}
        - {{language}}
            - dockerfile.jinja
            - server.yaml.jinja
            - cronjobs.yaml.jinja
            - workers.yaml.jinja
            - db_migrations.yaml.jinja
```

Having the following structure, you can distinguish between the default templates and the templates for a specific team.

### Understading `_default`

Through the project, you will find the `_default` key. This key is used to define the default values for the values, unless a specific value is defined.
For instance, when you have `_default.python.dockerfile.jinja` defined and there is no `{{team name}}.python.dockerfile.jinja` template, the default template will be used.
Same logic will be applied to environment variables, replicas, memory limits, etc.


### Example usage

Each repository should have a `.yml` file with the following content:

```yaml
engine:
  language: 
    name: python
    version: 3.10.8
  package_manager:
    name: poetry
    version: 1.1.11
  additional_system_packages:
    - libmagic1

server:
  replicas: 
    _default: 1
    production: 2
  memory_limits: 256Mi
  requests:
    memory: 256Mi
    cpu: 256m
  envs:
    LOG_LEVEL: INFO
    POSTGRES_MAX_POOLSIZE: 10
    POSTGRES_PASSWORD: vault://my-secret
envs:
  POSTGRES_MAX_POOLSIZE: 5

cronjobs:
- command: python resend_messages.py
  concurrency: Forbid
  enabled: 
    _default: true
    sandbox: false
  name: resend_messages
  schedule: "*/30 * * * *"

- command: python cleanup_tasks.py
  concurrency: Forbid
  enabled: true
  name: cleanup-tasks
  schedule: 
    _default: "* * * * *"
    dev: "0 1 * * *"

workers:
- name: my-best-worker
  enabled: true
  command: python run_worker.py main-worker
  replicas: 1
  envs:
    POSTGRES_MAX_POOLSIZE: 1
  memory_limits: 256Mi
  requests:
    memory: 256Mi
    cpu: 256m
 
db_migrations:
- command: aerich upgrade
  envs:
    POSTGRES_MAX_POOLSIZE: 1
``` 

## Understanding the structure
TBD

# How to use

## Generating dockerfile
In order to generate a dockerfile, just send a POST request to `/dockerfiles/generate`, converting the `.yml` file to a JSON object:

```curl
curl -X 'POST' \
  'http://127.0.0.1:8000/dockerfiles/generate' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
  "metadata": {
    "project_id": 100,
    "project_name": "test",
    "current_env": "dev",
    "team": "ml",
    "branch_name": "feature/new-search",
    "commit": "34543rfwfqw1"
  },
  "engine": {
    "language": {"name": "python", "version": "3.12"},
    "version": "3.12",
    "additional_system_packages": [
      "imagemagic"
    ],
    "package_manager": {"name": "rye", "version": " 0.34.0"}
}}'
```

The response will be a dockerfile, based on the payload and the template, defined in `templates.ml.python.dockerfile.jinja`:

```dockerfile
FROM python:3.12

ENV PYTHONDONTWRITEBYTECODE 1
ENV PYTHONUNBUFFERED 1

RUN apt-get update && apt-get install -y --no-install-recommends curl imagemagic&& \
     && \
    apt-get purge -y --auto-remove curl && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY . /app
```

## Generating k8s manifests

In order to generate k8s manifests, just send a POST request to `/manifests/generate`, converting the `.yml` file to a JSON object:

```curl
curl -X 'POST' \
  'http://127.0.0.1:8000/manifests/generate' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
  "metadata": {
    "project_id": 0,
    "project_name": "string",
    "current_env": "dev",
    "team": "string",
    "branch_name": "string",
    "commit": "string"
  },
  "engine": {
    "language": {
      "name": "python",
      "version": "3.10.8"
    },
    "package_manager": {
      "name": "poetry",
      "version": "1.1.11"
    },
    "additional_system_packages": [
      "libmagic1"
    ]
  },
  "server": {
    "replicas": {
      "_default": 1,
      "production": 2
    },
    "memory_limits": "256Mi",
    "requests": {
      "memory": "256Mi",
      "cpu": "256m"
    },
    "envs": {
      "LOG_LEVEL": "INFO",
      "POSTGRES_MAX_POOLSIZE": 10,
      "POSTGRES_PASSWORD": "vault://my-secret"
    }
  },
  "envs": {
    "POSTGRES_MAX_POOLSIZE": 5
  },
  "cronjobs": [
    {
      "command": "python resend_messages.py",
      "concurrency": "Forbid",
      "enabled": {
        "_default": true,
        "sandbox": false
      },
      "name": "resend_messages",
      "schedule": "*/30 * * * *"
    },
    {
      "command": "python cleanup_tasks.py",
      "concurrency": "Forbid",
      "enabled": true,
      "name": "cleanup-tasks",
      "schedule": {
        "_default": "* * * * *",
        "dev": "0 1 * * *"
      }
    }
  ],
  "workers": [
    {
      "name": "my-best-worker",
      "enabled": true,
      "command": "python run_worker.py main-worker",
      "replicas": 1,
      "envs": {
        "POSTGRES_MAX_POOLSIZE": 1
      },
      "memory_limits": "256Mi",
      "requests": {
        "memory": "256Mi",
        "cpu": "256m"
      }
    }
  ],
  "db_migrations": [
    {
      "command": "aerich upgrade",
      "envs": {
        "POSTGRES_MAX_POOLSIZE": 1
      }
    }
  ]
}'
```
As a result, you will receive yaml documents, k8s manifests, ready to deploy
