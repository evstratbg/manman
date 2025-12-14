# ManMan Go CLI

Локальный генератор Dockerfile или Helm-манифестов на Go. Принимает `app.yaml`, шаблоны и выводит результат в рабочую директорию.

## Структура
- `src_go/` — код CLI (`cmd/manman`), генератор, шаблоны и README с подробной инструкцией.
- `.github/workflows/build-manman-go.yml` — CI: тесты, сборка для linux/amd64 и linux/arm64, релизы по тегам `v*`.

## Быстрый старт
```bash
cd src_go
GOCACHE=$(pwd)/.gocache go run ./cmd/manman \
  -app ../app.yaml \
  -templates ./templates \
  -mode helm \
  -team _default \
  -env dev \
  -image myrepo/app:tag \
  -project-name myapp
```

Детали по флагам, структуре `app.yaml` и шаблонов — в `src_go/README.md`.
