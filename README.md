# manman

ManMan, manifest manager, is a web server, designed for centralized management of dockerfiles and k8s manifests.

It allows you to create dockerfiles templates and kubernetes manifests for your teams and manage them in a centralized way

## Why it was built

When you have a lot of repositories, it becomes difficult to make the same changes to all docker files or deployment manifests: replace the base image with a patch release or replace the priority class for the production environment. 
Using manman, only configuration files related to the application remain in the repositories: env variables, number of replicas, launch commands, etc. and these configuration files are used to generate dockerfiles or deployment manifests.

At the same time, the internal settings, which language image or which mode selector will be used, are decided by the team responsible for manman's operation, usually the devops team. 
Thus, the infrastructure layer does not flow into the development layer, but developers still have the application configuration tools they develop. 


### Understading `_default`

Through the project, you will find the `_default` key. This key is used to define the default values for the values, unless a specific value is defined.
For instance, when you have `_default.python.dockerfile.jinja` defined and there is no `{{team name}}.python.dockerfile.jinja` template, the default template will be used.

Same logic will be applied to repository configuration file.

```yaml
cronjobs:
- command: python resend_messages.py
  concurrency: Forbid
  enabled: 
    _default: true
    sandbox: false
  name: resend_messages
  schedule: "*/30 * * * *"
```
This means, that for every environment, except for the `sandbox`, this cronjob will run

If a key in repo config file supports `_default` key, it will be marked as "supports _default", and means that you can define the default value for this key in the `_default` section.

## Repository config file

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

envs:
  POSTGRES_MAX_POOLSIZE: 5
  
  
servers:
- command: python run_server.py
  name: API
  enabled: true
  replicas: 
    _default: 1
    production: 2
  memory_limits: 256Mi
  requests:
    memory: 256Mi
    cpu: 256m
  hpa:
    min_replicas: 10
    max_replicas: 10
    target_cpu_utilization_percent: 50
  envs:
    LOG_LEVEL: INFO
    POSTGRES_MAX_POOLSIZE: 10
    POSTGRES_PASSWORD: my-secret


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

consumers:
- name: clickstream
  enabled: true
  command: python run_consumer.py clickstream
  replicas: 1
  envs:
    POSTGRES_MAX_POOLSIZE: 1
  memory_limits: 256Mi
  requests:
    memory: 256Mi
    cpu: 256m
 
db_migrations:
- command: alembic upgrade
  envs:
    POSTGRES_MAX_POOLSIZE: 1
```

Lets break it down:

### engine

`language.name`, *mandatory*, defines the language used in the project

`language.version`, *mandatory*, defines the version of the language used in the project

`package_manager.name`, *optional*, defines the package manager used in the project

`package_manager.version`, *optional*, defines the version of the package manager used in the project

`additional_system_packages`, *optional*, defines the additional system packages that should be installed in the dockerfile

### envs

Optional, supports `_default`. Environment variables that will be inherited by every k8s manifest. 

### servers

Optional, defines the servers deployment settings

`command`, *mandatory*, defines the command that should be executed to start the server

`enabled`, *mandatory*, supports `_default`. Defines if the server is enabled.

`name`, *mandatory*, defines the name of the server

`replicas`, *optional*, supports `_default`. Defines the number of replicas for the server deployment. Cant be used if `server.hpa` is specified


`memory_limits`, *optional*, defines the memory limit for the server deployment. Regex: `^[1-9][0-9]{0,3}[MG]i$`


`requests`, *optional*, defines the memory and cpu requests for the server deployment.

`requests.memory`, *optional*, supports `_default`. Defines the memory requests for the server deployment.

`requests.cpu`, *optional*, supports `_default`. Defines the cpu requests for the server deployment. Regex: `^[1-9][0-9]{0,4}m$`


`hpa`, *optional*, supports `_default`. Defines the hpa settings for the server deployment. Cant be used if `server.replicas` is specified

`hpa.min_replicas`, *mandatory*, supports `_default`. Defines the minimum number of replicas for the server deployment

`hpa.max_replicas`, *mandatory*, supports `_default`. Defines the maximum number of replicas for the server deployment

`hpa.target_cpu_utilization_percent`, *mandatory*, supports `_default`. Defines the target cpu utilization percent for the server deployment

`envs`, *optional*, supports `_default`. Defines the environment variables for the server deployment. Overwrites keys from the global `env` section if intersected.


### cronjobs

Optional, defines a list of cronjobs settings

`envs`, *optional*, supports `_default`. Defines the environment variables. Overwrites keys from the global `env` section if intersected.

`concurrency`, *mandatory*, defines the concurrency policy for the cronjob. Possible values: `allow`, `forbid`, `replace`

`command`, *mandatory*, defines the command that should be executed by the cronjob

`enabled`, *mandatory*, supports `_default`. Defines if the cronjob is enabled.

`name`, *mandatory*, defines the name of the cronjob. Regex: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`

`schedule`, *mandatory*, supports `_default`. Defines the schedule for the cronjob. Regular crontab syntax


### consumers
Optional, defines a list of consumers settings

`name`, *mandatory*, defines the name of the consumer

`enabled`, *mandatory*, supports `_default`. Defines if the cronjob is enabled.

`command`, *mandatory*, defines the command that should be executed by the consumer

`replicas`, *mandatory*, supports `_default`, defines the number of replicas for the consumer

`envs`, *optional*, supports `_default`. Defines the environment variables. Overwrites keys from the global `env` section if intersected.


`memory_limits`, *optional*, defines the memory limit for the server deployment. Regex: `^[1-9][0-9]{0,3}[MG]i$`


`requests`, *optional*, defines the memory and cpu requests for the server deployment.

`requests.memory`, *optional*, supports `_default`. Defines the memory requests.

`requests.cpu`, *optional*, supports `_default`. Defines the cpu requests. Regex: `^[1-9][0-9]{0,4}m$`


### db_migrations

Optional, defines a list of database migrations

`command`, *mandatory*, defines the command that should be executed by the migration

`envs`, *optional*, supports `_default`. Defines the environment variables. Overwrites keys from the global `env` section if intersected.


### Secret envs

manman supports encrypted evs. It might be handy, if you want to store some sensitive data in the repository config file, such as database passwords and more.
You can create a file with secret environment variables and encrypt it with the `manman` API.

For obtaining the encrypted value, you can use the following API endpoint:
```
curl -X 'POST' \
  'http://127.0.0.1:8000/secrets/encrypt' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
  "envs": {
    "POSTGRES_PASSWORD": "string"
  }
}'
```
In response body, you will get the encrypted value in body, that you can use in the repository config file.
```yaml
POSTGRES_PASSWORD:
  _default: 10006e7606e4fa3cbe0fe022243c1d76687655168d7d760acfe06e0cbef17c2f4c3c
```

In response headers, a secret key, that you should store in a safe place.
```
content-length: 100 
content-type: application/yaml 
date: Mon,19 Aug 2024 11:00:05 GMT 
server: uvicorn 
x-secret-key: 5978a12dc79c7946ac49d4b4a6ce12993914d0e4d26364cd46116bf09128e3ff
 ```
Values are AES256 encrypted and will be decrypted only with the secret key, during manifest generation.


> Recommendation: keep `secret-values.yaml` apart from the repo config file, so during code review it will be clear that this file contains sensitive data



## Templates

Once repository config file is created, you can create templates for dockerfiles and k8s manifests.
Each template - is a jinja2 template, you can read about it [here](https://jinja.palletsprojects.com/en/3.1.x/). 
It supports many handy statements, if-else, loops and more

`templates` directory should contain the following structure, but `_default` folder is optional:

```
templates
    - _default
       - {{language}}
            - dockerfile.jinja2
            - tolerations.yaml

    - {{team name}}
        - _default
            - dockerfile.jinja2
        - {{language}}
            - dockerfile.jinja2
            - server.yaml.jinja2
            - cronjobs.yaml.jinja2
            - consumers.yaml.jinja2
            - db_migrations.yaml.jinja2
```

Having the following structure, you can distinguish between the default templates and the templates for a specific team.
If a specific team template is not found, a template from `_default` folder will be used.

Discovery logic is the following:
1) Check if the template for the specific team language exists
2) Check if the template for the specific team exists
3) Check if the default template exists

For example, if you have the following structure:
```
templates
    - _default
       - python
            - dockerfile.jinja2
            - consumers.yaml.jinja2

    - mlsearch
        - python
            - consumers.yaml.jinja2
```

and you are requesting for the dockerfile, the default template will be used, because there is no dockerfile template for the `mlsearch` team.
But if you are requesting for the consumers, the template in the `mlsearch` team folder will be used.

### Dockerfile

mandatory filename: `dockerfile.jinja2`

set of available variables:

- {{language}}
- {{version}}
- {{package_manager}}
- {{package_manager_version}}
- {{additional_system_packages}}

### Server

mandatory filename: `server.yaml.jinja2`

set of available variables:

- {{image}}
- {{name}}
- {{command}}
- {{replicas}}
- {{project_name}}
- {{is_hpa_enabled}}
- {{current_env}}
- {{tolerations}}
- {{affinity}}
- {{envs}}
- {{memory_limits}}
- {{memory_requests}}
- {{cpu_requests}}
- {{manman_release}}
- {{branch_name}}
- {{commit}}
- {{team}}


### Server HPA

mandatory filename: `server_hpa.yaml.jinja2`

set of available variables:

- {{project_name}}
- {{min_replicas}} 
- {{max_replicas}}
- {{target_cpu_utilization_percentage}}


### Migration

mandatory filename: `migration.yaml.jinja2`

set of available variables:

- {{image}}
- {{project_name}}
- {{current_env}}
- {{tolerations}}
- {{affinity}}
- {{command}}
- {{envs}}
- {{manman_release}}
- {{branch_name}}
- {{commit}}
- {{team}}


### Cronjob

mandatory filename: `cronjob.yaml.jinja2`

set of available variables:

- {{image}}
- {{project_name}}
- {{current_env}}
- {{tolerations}}
- {{affinity}}
- {{command}}
- {{envs}}
- {{name}}
- {{schedule}}
- {{concurrency}}
- {{manman_release}}
- {{branch_name}}
- {{commit}}
- {{team}}


### Consumer

mandatory filename: `cronjob.yaml.jinja2`

set of available variables:

- {{image}}
- {{project_name}}
- {{current_env}}
- {{tolerations}}
- {{affinity}}
- {{envs}}
- {{name}}
- {{command}}
- {{replicas}}
- {{memory_limits}}
- {{memory_requests}}
- {{cpu_requests}}
- {{manman_release}}
- {{branch_name}}
- {{commit}}
- {{team}}

### Tolerations

mandatory filename: `tolerations.yaml`

Doesnt support any variables, just a list of tolerations, divied by environments

Example
```yaml
_default:
  tolerations: []
production:
  tolerations:
  - effect: NoExecute
    key: dedicated
    operator: Equal
    value: production
```

### Affinity

mandatory filename: `affinity.yaml`

Doesn`t support any variables, just a list of affinity, divied by environments

Example
```yaml
_default:
  affinity: {}
production:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node-role.kubernetes.io/nodes
            operator: Exists

```


## Putting all together

Once you have the repository config file and templates, you can generate the dockerfiles and k8s manifests using the `manman` API

API was built in a specific way, so CICD integration won't require much

`manman API` has the following endpoints:
- `/manifests/generate` - to generate k8s manifests
- `/dockerfiles/generate` - to generate dockerfiles
- `/secrets/encrypt` - to encrypt secret values

and provide the following environment variables:
- `ENVIRONMENTS`, a list of available envs in your infrastructure '["dev", "staging"]'


### Gitlab and gitlab CI example

As an example, here is a `.gitlab-ci.yml` file, that generates the k8s manifests and dockerfiles and stores them as artifacts


```yaml
stages:
  - build
  - deploy

build_commit:
  stage: build
  image:
    name: gcr.io/kaniko-project/executor:v1.14.0-debug
    entrypoint: [ "" ]
  script:
    - apk add --no-cache curl
    - |
       curl -X 'POST' \
      'http://127.0.0.1:8000/dockerfiles/generate' \
      -H 'accept: application/json' \
      -H 'x-project-id:$CI_PROJECT_ID' \
      -H 'x-project-name:$CI_PROJECT_NAME' \
      -H 'x-current-env:$ENVIRONMENT' \
      -H 'x-team:$TEAM_NAME' \
      -H 'x-branch-name:$CI_COMMIT_REF_NAME' \
      -H 'x-commit-hash:$CI_COMMIT_SHORT_SHA' \
      -H 'Content-Type: multipart/form-data' \
      -F 'file=@app.yaml;type=application/x-yaml' >> Dockerfile

    - /kaniko/executor
      --context "${CI_PROJECT_DIR}"
      --dockerfile "${CI_PROJECT_DIR}/Dockerfile"
      --destination "$CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA"
    
    - echo IMAGE=$CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA >> build.env;
    - cat build.env
  artifacts:
    reports:
      dotenv: build.env


deploy:
  stage: deploy
  image: alpine/helm
  script:
    - apk add --no-cache curl
    - mkdir -p .helm/templates
    - |
      curl -X 'POST' \
      'http://127.0.0.1:8000/manifests/generate' \
      -H 'x-project-id:$CI_PROJECT_ID' \
      -H 'x-project-name:$CI_PROJECT_NAME' \
      -H 'x-current-env:$ENVIRONMENT' \
      -H 'x-team:$TEAM_NAME' \
      -H 'x-branch-name:$CI_COMMIT_REF_NAME' \
      -H 'x-commit-hash:$CI_COMMIT_SHORT_SHA' \
      -H 'Content-Type: multipart/form-data' \
      -F 'file=@app.yaml;type=application/x-yaml' >> .helm/templates/manifests.yaml

    - helm upgrade
```

During `build_commit` stage, we request manman for dockerfiles generation, and then build the docker image using kaniko.
Then, requesting for deployment manifests generation and deploying them using helm

## Adding secret values

Assuming, you have `app.yaml` and `secret-values.yaml`, lets merge them

```bash
echo "\nsecrets:" >> app.yaml
cat secret-values.yaml | sed 's/^/  /' >> app.yaml
```

And on deploy stage, add `x-secret-key` header, so manman, can decrypt the values.


# QnA

Q: What if I want to include additional resources like RBAC or Ingress or SA into a release?

A: ManMan just generates the manifests, based on the repo config file and nothing stops you from adding additional resources to the release manually. 
Just keep all extra resources in the `.helm/templates` as usual and keep an eye not to overwrite a file with the `curl` response from manifests generation request to ManMan 
