# ManMan Go CLI

Локальный генератор dockerfile или helm-манифестов на Go. Работает поверх `app.yaml` и Go `text/template`, повторяя логику Python-версии: берёт значения из `_default` и текущего окружения, читает шаблоны из `_default`, `<team>/_default`, `<team>/<language>` (последний победит).

## Быстрый старт
- Зависимости уже описаны в `go.mod`. Запуск без установки в систему:
  ```bash
  cd src_go
  GOCACHE=$(pwd)/.gocache go run ./cmd/manman \
    -app ../app.yaml \
    -templates ./templates \
    -mode helm \
    -team _default \
    -env dev \
    -image myrepo/app:tag \
    -project-name myapp \
    -branch main \
    -commit abc123
  ```
  Выходной файл окажется в текущей директории (`manifests.yaml` или `Dockerfile` по умолчанию).
- Собрать бинарник:
  ```bash
  cd src_go
  GOCACHE=$(pwd)/.gocache go build -o manman ./cmd/manman
  ./manman -h
  ```

## Флаги
- `-app` — путь к `app.yaml` (обязательно).
- `-templates` — каталог с шаблонами (обязательно). Внутри — папки `_default`, `<team>/_default`, `<team>/<language>`.
- `-mode` — `helm` или `dockerfile` (обязательно).
- `-output` — имя файла результата (по умолчанию `manifests.yaml` или `Dockerfile`).
- `-team` — имя команды для поиска шаблонов (по умолчанию `_default`).
- `-env` — текущее окружение для `_default`-перекрытий.
- `-image`, `-project-name`, `-project-id`, `-branch`, `-commit`, `-release` — значения, доступные в шаблонах.
- `-secret-key` — hex-ключ AES для расшифровки `secrets.envs` из `app.yaml` (формат совместим с Python `AesEncoder`).

## Структура шаблонов
Каталог `src_go/templates/_default` содержит рабочие примеры (Go `text/template`):
- `dockerfile.tmpl`
- `api.yaml.tmpl`
- `api_hpa.yaml.tmpl`
- `cronjob.yaml.tmpl`
- `worker.yaml.tmpl`
- `migration.yaml.tmpl`
- `tolerations.yaml`, `affinity.yaml`
- `ingress.yaml.tmpl` (опциональный ingress для API)

Можно создать `myteam/_default` или `myteam/python` и положить туда файлы с теми же именами — они перекроют `_default`.

## Пример app.yaml (минимум)
```yaml
engine:
  language:
    name: python
    version: "3.11"
  additional_system_packages: []
  package_manager:
    name: pip
    version: "23.2"

apis:
  - command: python app.py
    name: api
    enabled: true
    replicas: 2
    memory_limits: 512Mi
    requests:
      memory: 256Mi
      cpu: 250m
    envs:
      LOG_LEVEL: INFO

workers:
  - name: queue
    command: python worker.py
    enabled: true
    replicas: 1
    requests:
      memory: 128Mi
      cpu: 100m

cronjobs:
  - concurrency: forbid
    command: echo hello
    name: sample
    enabled: true
    schedule: "*/5 * * * *"

  - name: api
    command: ./server
    enabled: true
    ingress:
      domain:
        _default: api.dev.example.com
        production: api.example.com
      proxy-body-size: 70M
```

## Секреты
Если в `app.yaml` есть `secrets.envs`, передайте `-secret-key <hex>` — иначе генерация остановится, чтобы не оставить значения зашифрованными.
