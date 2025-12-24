# ManMan (Go CLI)

Локальный генератор Dockerfile и k8s‑манифестов из `app.yaml` и Go `text/template`.
Использует шаблоны из папки `templates/` и умеет подставлять значения по окружениям.

## Быстрый старт
Сборка не требуется, можно запускать через `go run`:
```bash
go run ./cmd/manman \
  -app ./app.yaml \
  -templates ./templates \
  -mode helm \
  -team _default \
  -env dev \
  -image myrepo/app:tag \
  -project-name myapp \
  -branch main \
  -commit abc123
```
Выходной файл будет в текущей директории (`manifests.yaml` или `Dockerfile` по умолчанию).

Сборка бинарника:
```bash
go build -o manman ./cmd/manman
./manman -h
```

## Флаги CLI
- `-app` — путь к `app.yaml` (обязательный).
- `-templates` — каталог шаблонов (обязательный). Внутри ожидаются `_default`, `<team>/_default`, `<team>/<language>`.
- `-mode` — `helm` или `dockerfile` (обязательный).
- `-output` — имя выходного файла (по умолчанию `manifests.yaml` или `Dockerfile`).
- `-team` — имя команды (по умолчанию `_default`).
- `-env` — окружение для env‑оверрайдов (по умолчанию `dev`).
- `-image` — образ контейнера для манифестов.
- `-project-name` — имя проекта, попадает в шаблоны.
- `-project-id` — передается в генератор (по умолчанию не используется в шаблонах).
- `-branch`, `-commit`, `-release` — значения, доступные в шаблонах.
- `-secret-key` — hex‑ключ AES для расшифровки `secrets.envs` и `secret-values.yaml`.

## Как выбираются шаблоны
Шаблоны ищутся в порядке:
1) `_default`
2) `<team>/_default`
3) `<team>/<language>`

Если файл найден позже, он перекрывает предыдущий.

Поддерживаемые файлы в шаблонах (пример из `templates/_default`):
- `dockerfile.tmpl`
- `api.yaml.tmpl`
- `api_hpa.yaml.tmpl`
- `cronjob.yaml.tmpl`
- `worker.yaml.tmpl`
- `migration.yaml.tmpl`
- `ingress.yaml.tmpl`
- `tolerations.yaml`, `affinity.yaml` (данные для подстановки в pod spec)

## Принцип env‑оверрайдов
Почти любое поле в `app.yaml` может быть обычным значением или map с ключами окружений.
Правило выбора:
- если есть ключ текущего окружения, берется он;
- иначе берется `_default`;
- если ничего нет, поле считается пустым.

Пример:
```yaml
port:
  _default: 8000
  production: 80
```

## Полная структура app.yaml
Ниже — «полный» пример, в котором есть все поддерживаемые секции и поля.
Можно копировать и удалять ненужное.

```yaml
engine:
  language:
    name: python        # влияет на выбор папки шаблонов <team>/<language>
    version: "3.11"     # используется в Dockerfile шаблоне
  additional_system_packages: [] # список пакетов для apt-get (Dockerfile)
  package_manager:
    name: pip           # Dockerfile (опционально)
    version: "23.2"

envs:
  LOG_LEVEL:
    _default: INFO
    production: WARN

apis:
  - name: api
    command: python app.py
    enabled: true
    replicas:
      _default: 2
      production: 4
    port: 8000
    memory_limits: 512Mi
    requests:
      memory: 256Mi
      cpu: 250m
    envs:
      SERVICE_MODE: api
    hpa:
      min_replicas: 2
      max_replicas: 10
      target_cpu_utilization_percent: 70
    ingress:
      domain:
        _default: api.dev.example.com
        production: api.example.com
      proxy-body-size: 70M

workers:
  - name: queue
    command: python worker.py
    enabled: true
    replicas: 1
    memory_limits: 256Mi
    requests:
      memory: 128Mi
      cpu: 100m
    envs:
      WORKER_QUEUE: main

cronjobs:
  - name: cleanup
    command: ./bin/cleanup
    enabled: true
    schedule: "*/5 * * * *"
    concurrency: forbid   # allow|forbid|replace (регистр не важен)
    envs:
      CLEANUP_MODE: fast

db_migrations:
  - command: ./bin/migrate up
    envs:
      MIGRATE_LOCK: "true"

secrets:
  envs:
    DATABASE_URL: "ENCRYPTED_HEX"
    API_KEY:
      _default: "ENCRYPTED_HEX"
      production: "ENCRYPTED_HEX"
```

## Что именно генерируется
Шаблоны `_default` (по умолчанию) формируют:
- Deployment для `apis` и `workers`
- CronJob для `cronjobs`
- Job для `db_migrations`
- HPA для `apis` при наличии `hpa`
- Ingress + Certificate для `apis` при наличии `ingress.domain`
- Dockerfile на основе `engine.*`

Если `ingress.domain` пустой, ingress не рендерится.

## Переменные, доступные в шаблонах
Эти значения доступны как `.xxx` в шаблонах:
- `.image`, `.project_name`, `.current_env`
- `.branch_name`, `.commit`, `.manman_release`
- `.team`
- `.envs` (глобальные `envs` + `envs` конкретной сущности)
- `.tolerations`, `.affinity` (из `tolerations.yaml` / `affinity.yaml`)

Дополнительно для каждого типа:
- API: `.name`, `.command`, `.replicas`, `.port`, `.memory_limits`,
  `.memory_requests`, `.cpu_requests`, `.is_hpa_enabled`
- Worker: `.name`, `.command`, `.replicas`, `.memory_limits`,
  `.memory_requests`, `.cpu_requests`
- CronJob: `.name`, `.command`, `.schedule`, `.concurrency`
- Migration: `.command`
- Ingress: `.name`, `.domain`, `.proxy_body_size`

## Секреты
Есть два источника секретов:
1) `app.yaml` → `secrets.envs`
2) файл рядом с `app.yaml`: `secret-values.yaml`

`secret-values.yaml` автоматически подмешивается. Вложенные ключи превращаются в имена
переменных окружения через `__`:
```yaml
videosdk:
  api_key: ENC
```
становится `videosdk__api_key: ENC`.

Если значение — это env‑оверрайд карта (`_default`, `production`), она сохраняется как есть:
```yaml
db:
  url:
    _default: ENC_DEV
    production: ENC_PROD
```
становится `db__url: { _default: ENC_DEV, production: ENC_PROD }`.

Если в итоговом манифесте есть `secrets.envs`, обязательно передайте `-secret-key`
(hex‑ключ AES). Иначе генерация завершится с ошибкой.

## Важные детали
- `enabled` может быть `true/false`, строкой (`"true"`) или числом (`1/0`).
- `cronjobs[].concurrency` нормализуется в `Allow/Forbid/Replace`.
  Пустое значение приведет к невалидному манифесту — укажите его явно.
- `envs` у сущностей перекрывают одноименные ключи из глобальных `envs`.
- Автоматически добавляются env: `CURRENT_ENV` и `COMMIT` (если передан флаг `-commit`).
- В дефолтном `api.yaml.tmpl` порт контейнера захардкожен в `8000`; поле `apis[].port`
  будет учитываться только если вы используете свой шаблон.
- В дефолтном `api_hpa.yaml.tmpl` `scaleTargetRef.name` равен `project_name`; если
  deployment назван иначе, обновите шаблон под себя.

## Минимальный app.yaml
```yaml
engine:
  language:
    name: python
    version: "3.11"
  additional_system_packages: []

apis:
  - name: api
    command: python app.py
    enabled: true
    replicas: 1
    port: 8000

cronjobs:
  - name: cleanup
    command: ./bin/cleanup
    enabled: true
    schedule: "0 * * * *"
    concurrency: allow
```
