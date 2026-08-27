#### Результатом выполнения следующих домашних заданий является сервис «Календарь»:
- [Домашнее задание №12 «Заготовка сервиса Календарь»](./docs/12_README.md)
- [Домашнее задание №13 «Реализация Rest API Календаря»](./docs/13_README.md)
- [Домашнее задание №14 «Интеграция Apache Kafka в Календарь»](./docs/14_README.md)
- [Домашнее задание №15 «Докеризация и интеграционное тестирование Календаря»](./docs/15_README.md)
- [Домашнее задание №16 «Мониторинг Календаря»](./docs/16_README.md)

#### Ветки при выполнении
- `hw12_calendar` (от `master`) -> Merge Request в `master`
- `hw13_calendar` (от `hw12_calendar`) -> Merge Request в `hw12_calendar` (если уже вмержена, то в `master`)
- `hw14_calendar` (от `hw13_calendar`) -> Merge Request в `hw13_calendar` (если уже вмержена, то в `master`)
- `hw15_calendar` (от `hw14_calendar`) -> Merge Request в `hw14_calendar` (если уже вмержена, то в `master`)
- `hw16_calendar` (от `hw15_calendar`) -> Merge Request в `hw15_calendar` (если уже вмержена, то в `master`)

**Домашнее задание не принимается, если не принято ДЗ, предшедствующее ему.**

## ДЗ №12

Сервис собирается и проверяется командами:

```bash
make build
make test
make lint
./bin/calendar --config=./configs/config.toml
```

После запуска hello-world доступен по адресу `GET /hello`. По умолчанию используется
in-memory хранилище. Для PostgreSQL укажите `type = "sql"` и DSN в конфиге, затем
примените миграцию:

```bash
export DATABASE_DSN='postgres://calendar:calendar@localhost:5432/calendar?sslmode=disable'
make migrate-up
```

SQL integration test запускается отдельно против подготовленной базы:

```bash
CALENDAR_TEST_POSTGRES_DSN="$DATABASE_DSN" go test -race ./internal/storage/sql
```

## ДЗ №13

REST API описан спецификацией OpenAPI 3.0.3 в [`api/swagger.json`](./api/swagger.json).
Типы запросов, ответов и интерфейс HTTP-сервера генерируются `oapi-codegen v1.12.4`:

```bash
make generate
make generate-check
```

Версия генератора закреплена в Makefile и совместима с Go 1.19, используемым в CI.
Сгенерированный код находится в `internal/server/http/openapi`; реализация хендлеров
расположена в родительском транспортном пакете и зависит только от интерфейса приложения.

| Метод | Эндпоинт | Назначение |
| --- | --- | --- |
| `POST` | `/api/v1/events` | Создать событие |
| `PUT` | `/api/v1/events/{eventId}` | Обновить событие |
| `DELETE` | `/api/v1/events/{eventId}` | Удалить событие |
| `GET` | `/api/v1/events/day?user_id=...&date=YYYY-MM-DD` | Получить события на день |
| `GET` | `/api/v1/events/week?user_id=...&date=YYYY-MM-DD` | Получить события на семь дней, начиная с даты |
| `GET` | `/api/v1/events/month?user_id=...&date=YYYY-MM-DD` | Получить события на календарный месяц |

Пример создания события:

```bash
curl -i -X POST http://localhost:8080/api/v1/events \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "event-1",
    "title": "Team meeting",
    "start_at": "2026-08-21T10:00:00Z",
    "end_at": "2026-08-21T11:00:00Z",
    "description": "Discuss the release plan",
    "user_id": "user-1",
    "notify_before_seconds": 900
  }'
```

Полная проверка реализации:

```bash
make generate-check
make build
make test
make lint
```
