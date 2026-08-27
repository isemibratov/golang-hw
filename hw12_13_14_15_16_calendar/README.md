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

После запуска hello-world доступен по адресу `GET /hello`. В актуальном
`configs/config.toml` используется PostgreSQL (`type = "sql"`), поэтому перед
запуском примените миграции:

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

## ДЗ №14

Сборка создаёт три независимых исполняемых файла:

```text
bin/calendar
bin/calendar_scheduler
bin/calendar_storer
```

Планировщик при старте ожидает доступности Kafka с экспоненциальной задержкой между
попытками, затем сразу выполняет первый цикл и продолжает работу с периодом из
конфигурации. Событие считается готовым к отправке, если значение
`notify_before_seconds` больше нуля, время напоминания наступило, само событие ещё
не началось и уведомление ранее не было отмечено как отправленное. Нулевое значение
означает, что напоминание отключено. События, начавшиеся строго раньше календарной
даты `now - retention_years`, удаляются.

После подтверждённой Kafka записи событие получает отметку `notification_sent_at`.
Если процесс завершится между публикацией и сохранением отметки, сообщение может
быть доставлено повторно. Хранитель безопасно обрабатывает такой сценарий: запись
в таблице `notifications` обновляется по `event_id`. Kafka offset подтверждается
только после успешной записи в PostgreSQL. Некорректные JSON-сообщения повторять
бессмысленно: они логируются, пропускаются и подтверждаются, чтобы не блокировать
чтение топика.

Для новой базы примените обе миграции:

```bash
export DATABASE_DSN='postgres://calendar:calendar@localhost:5432/calendar?sslmode=disable'
make migrate-up
```

Если миграция ДЗ13 уже была применена, достаточно выполнить новую:

```bash
psql "$DATABASE_DSN" -v ON_ERROR_STOP=1 --single-transaction \
  -f migrations/00002_notifications.up.sql
```

Для совместной работы API и планировщик должны использовать одну базу PostgreSQL:
API создаёт и обновляет события, а планировщик выбирает их из этой же базы.
Предоставленные `configs/config.toml`, `configs/scheduler.toml` и
`configs/storer.toml` используют общий DSN; хранитель сохраняет уведомления в эту
же базу.

После запуска PostgreSQL и Kafka три команды ниже запускают компоненты единой
системы только с путями к своим TOML-конфигурациям:

```bash
./bin/calendar           --config=./configs/config.toml
./bin/calendar_scheduler --config=./configs/scheduler.toml
./bin/calendar_storer    --config=./configs/storer.toml
```

Адреса брокеров, topic, consumer group, таймауты, retry/backoff, период и размер
пакета планировщика вынесены в конфиги. Kafka-клиент изолирован в `internal/kafka`;
планировщик и хранитель зависят только от собственных узких интерфейсов.

Полная проверка ДЗ14:

```bash
go mod tidy
make generate-check
make build
make test
make lint
```
