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

## ДЗ №15

API, планировщик и хранитель собираются в отдельные минимальные Docker-образы.
Compose-окружение также запускает PostgreSQL, ZooKeeper, Kafka и одноразовый контейнер
с SQL-миграциями. После запуска HTTP API доступен на `http://localhost:8888`:

```bash
make up
curl http://localhost:8888/hello
make down
```

Настройки из TOML можно переопределять переменными окружения с префиксом
`CALENDAR_`. В compose используются, в частности, `CALENDAR_STORAGE_DSN`,
`CALENDAR_KAFKA_BROKERS`, `CALENDAR_HTTP_HOST` и `CALENDAR_HTTP_PORT`. Порт API на
хосте можно изменить отдельно, например `CALENDAR_HOST_PORT=9999 make up`.

Интеграционные тесты находятся в отдельном пакете `integration_tests` и защищены
build tag `integration`, поэтому обычный `make test` их не запускает. Команда ниже
собирает тестовый образ, поднимает изолированное compose-окружение, проверяет API и
цепочку доставки уведомлений, а затем удаляет контейнеры, сеть и тестовый том:

```bash
make integration-tests
```

Проверяемые сценарии:

- создание события, конфликт идентификатора, пересечение времени и невалидный период;
- получение событий на день, неделю и месяц;
- передача уведомления планировщиком через Kafka и сохранение хранителем в PostgreSQL.

Полная проверка ДЗ15:

```bash
go mod tidy
make generate-check
make build
make test
make lint
docker compose --file deployments/docker-compose.yaml config --quiet
make integration-tests
```

## ДЗ №16

Все три процесса публикуют метрики в формате Prometheus по пути `/metrics`:

| Процесс | Локальный запуск | Адрес внутри Compose |
| --- | --- | --- |
| Calendar API | `http://localhost:8080/metrics` | `http://calendar:8080/metrics` |
| Scheduler | `http://localhost:8081/metrics` | `http://scheduler:8081/metrics` |
| Storer | `http://localhost:8082/metrics` | `http://storer:8082/metrics` |

При запуске через Compose API также доступен на
`http://localhost:8888/metrics`. Порты scheduler и storer наружу не публикуются:
их опрашивает Prometheus по внутренней сети Compose.

Сервис экспортирует следующие прикладные метрики:

| Метрика | Тип | Labels | Назначение |
| --- | --- | --- | --- |
| `calendar_http_requests_total` | counter | `method`, `route`, `status_code` | Число HTTP-запросов, включая ошибочные ответы; позволяет оценивать нагрузку и долю ошибок. |
| `calendar_http_request_duration_seconds` | histogram | `method`, `route` | Распределение времени обработки HTTP-запросов; используется для расчёта latency percentile. |
| `calendar_event_operations_total` | counter | `operation`, `result` | Результаты создания, обновления, удаления и чтения событий; показывает состояние ключевых бизнес-сценариев. |
| `calendar_scheduler_running` | gauge | — | Равен `1`, пока цикл scheduler активен; при штатной остановке сбрасывается в `0`. |
| `calendar_scheduler_cycles_total` | counter | `result` | Число успешных и ошибочных циклов scheduler. |
| `calendar_scheduler_cycle_duration_seconds` | histogram | — | Распределение длительности циклов scheduler; помогает выявлять замедление фоновой обработки. |
| `calendar_scheduler_last_success_timestamp_seconds` | gauge | — | Unix-время последнего успешного цикла scheduler; позволяет обнаруживать зависание или длительные сбои. |
| `calendar_notifications_published_total` | counter | `result` | Результаты публикации уведомлений в Kafka. |
| `calendar_storer_running` | gauge | — | Равен `1`, пока consumer storer активен; при штатной остановке сбрасывается в `0`. |
| `calendar_notifications_processed_total` | counter | `result` | Результаты разбора и сохранения уведомлений storer. |
| `calendar_storer_last_success_timestamp_seconds` | gauge | — | Unix-время последнего успешно сохранённого уведомления. |

Значение label `route` — шаблон маршрута, например
`/api/v1/events/{eventId}`, а не фактический URL с идентификатором события.
Labels не содержат идентификаторы пользователей, событий или произвольные строки:
нестандартные HTTP-методы нормализуются в `OTHER`. Поэтому число временных рядов
остаётся ограниченным и предсказуемым. Запросы самого Prometheus к `/metrics`
не входят в метрики API и не искажают RPS и latency.

Для HTTP-бизнес-операций, циклов scheduler и публикации используются значения
`result="success|error"`; у storer результат равен `stored`, `error` или
`invalid`.

Compose запускает закреплённую версию Prometheus с конфигурацией
`deployments/prometheus/prometheus.yml`. После запуска окружения его интерфейс
доступен на `http://localhost:9090`; состояние целей сбора можно проверить на
`http://localhost:9090/targets`. Хостовый порт переопределяется, например,
командой `CALENDAR_PROMETHEUS_PORT=19090 make up`.

Примеры полезных PromQL-запросов:

```promql
# Интенсивность запросов к REST API по маршрутам.
sum by (method, route) (
  rate(calendar_http_requests_total{route!="/hello"}[5m])
)

# Доля ответов 5xx среди всех HTTP-запросов.
sum(rate(calendar_http_requests_total{route!="/hello",status_code=~"5.."}[5m]))
/
clamp_min(
  sum(rate(calendar_http_requests_total{route!="/hello"}[5m])),
  1e-9
)

# 95-й процентиль времени ответа по маршрутам.
histogram_quantile(
  0.95,
  sum by (le, route) (
    rate(calendar_http_request_duration_seconds_bucket{route!="/hello"}[5m])
  )
)

# Интенсивность бизнес-операций и их результаты.
sum by (operation, result) (rate(calendar_event_operations_total[5m]))

# Сколько секунд прошло с последнего успешного цикла scheduler.
time() - calendar_scheduler_last_success_timestamp_seconds

# Доступность endpoint'ов фоновых процессов с точки зрения Prometheus.
up{job=~"scheduler|storer"}

# Разница между скоростью публикации и сохранения уведомлений.
sum(rate(calendar_notifications_published_total{result="success"}[5m]))
-
sum(rate(calendar_notifications_processed_total{result="stored"}[5m]))
```

Последний запрос полезен как ранний признак отставания consumer, но не заменяет
измерение Kafka consumer lag: повторная доставка допустима, а окна публикации и
сохранения могут кратковременно не совпадать.

Для контроля доступности процесса следует использовать встроенную метрику
Prometheus `up`: после остановки endpoint недоступен, поэтому итоговое значение
`calendar_*_running = 0` обычно не успевает попасть в очередной scrape.

Запуск мониторинга:

```bash
make up
curl -fsS http://localhost:8888/metrics
curl -fsS http://localhost:9090/-/ready
curl -fsSG --data-urlencode 'query=up{job=~"calendar|scheduler|storer"}' \
  http://localhost:9090/api/v1/query
make down
```

Полная проверка ДЗ16:

```bash
go mod tidy
make generate-check
make build
make test
make lint
docker compose --file deployments/docker-compose.yaml config --quiet
make integration-tests
make up
curl -fsS http://localhost:8888/metrics
docker compose --project-name calendar --file deployments/docker-compose.yaml \
  exec -T scheduler wget -qO- http://127.0.0.1:8081/metrics
docker compose --project-name calendar --file deployments/docker-compose.yaml \
  exec -T storer wget -qO- http://127.0.0.1:8082/metrics
curl -fsS http://localhost:9090/-/ready
curl -fsSG --data-urlencode 'query=up{job=~"calendar|scheduler|storer"}' \
  http://localhost:9090/api/v1/query
make down
```
