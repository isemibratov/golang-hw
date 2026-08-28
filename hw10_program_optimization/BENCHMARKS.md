# Профилирование и бенчмарки

## Условия

Замеры выполнены 28.08.2026 на `darwin/arm64`, Go 1.19.13, `GOMAXPROCS=12`.
Версия benchstat: `golang.org/x/perf v0.0.0-20260825160852-19be9d8e6c70`
(бинарник собран Go 1.26.4).

Сравниваются исходный `stats.go` из коммита `b7e584d` и оптимизированная реализация.
Обе версии запускают один и тот же `BenchmarkGetDomainStat` на 100 000 записях
из `testdata/users.dat.zip` (17 375 349 байт после распаковки).
Каждый замер обрабатывает все записи; выполнено по 10 замеров без `-race`.

Бенчмарк распаковывает данные до `ResetTimer`, поэтому открытие ZIP, распаковка
и подготовка входного буфера не входят в `ns/op` и `B/op`. Сам `GetDomainStat`
получает новый `bytes.Reader` на каждой итерации. Готовый тест с тегом `bench`
отдельно проверяет полный путь, включая чтение и распаковку ZIP.

## Результат benchstat

| Метрика | До | После | Изменение |
| --- | ---: | ---: | ---: |
| Время на весь набор | 275,27 мс | 43,09 мс | −84,35% |
| Пропускная способность | 60,20 MiB/s | 384,55 MiB/s | +538,82% |
| Выделенная память на набор | 308,73 MiB | 10,80 MiB | −96,50% |
| Число аллокаций на набор | 3 144,4 тыс. | 711,1 тыс. | −77,39% |

Это медианы benchstat, `n=10` для каждой версии. `B/op` показывает суммарные
аллокации за обработку набора, а не пиковый RSS процесса. Время зависит от машины
и фоновой нагрузки.

Исходные измерения: [before.txt](benchmarks/before.txt),
[after.txt](benchmarks/after.txt). Полный вывод: [benchstat.txt](benchmarks/benchstat.txt).

## Причины улучшения

CPU- и allocation-профили исходной реализации показали затраты на `io.ReadAll`,
промежуточный массив пользователей, преобразования строк и повторную компиляцию
regexp. Эти операции заменены построчным декодированием, переиспользованием одной
структуры `User` и компиляцией regexp один раз на вызов. Домен извлекается через
`strings.Cut` и приводится к нижнему регистру один раз.

Для ускорения JSON-декодирования используется `github.com/segmentio/encoding/json`.
Строки с экранированием обрабатывает stdlib, а `userID` отдельно проверяет диапазон `int`:
это сохраняет поведение исходного декодера для ключей с NUL и переполнения чисел.
Проверка типов всех полей сохранена; ручного извлечения `Email` нет.
`Scanner.Err` сохраняет ошибки чтения, в том числе поступившие вместе с данными.
В новом CPU-профиле основная работа приходится на JSON-декодирование;
в allocation-профиле преобладают строки полей `User`.

## Воспроизведение сравнения

Из каталога этого ДЗ, с Go 1.19.13 и benchstat в `PATH`:

```bash
hw10_baseline=$(mktemp -d /tmp/hw10-baseline.XXXXXX)
git show b7e584d:hw10_program_optimization/stats.go > "$hw10_baseline/stats.go"
cp go.mod go.sum stats_benchmark_test.go "$hw10_baseline/"
mkdir "$hw10_baseline/testdata"
cp testdata/users.dat.zip "$hw10_baseline/testdata/"

(
    cd "$hw10_baseline" || exit 1
    go test -run '^$' -bench '^BenchmarkGetDomainStat$' \
        -benchmem -benchtime=1x -count=10 -timeout=2m . > /tmp/hw10-before.txt
)
go test -run '^$' -bench '^BenchmarkGetDomainStat$' \
    -benchmem -benchtime=1x -count=10 -timeout=2m . > /tmp/hw10-after.txt
benchstat /tmp/hw10-before.txt /tmp/hw10-after.txt
```

Исходный файл проверен по Git blob hash; переименование модуля и добавление
бенчмарка не меняют измеряемую реализацию. Замеры обеих версий выполняются
последовательно, без одновременного запуска линтера или других тестов.

Профили снимаются отдельно, чтобы накладные расходы профилировщика не влияли
на сравнение benchstat:

```bash
go test -run '^$' -bench '^BenchmarkGetDomainStat$' -benchtime=3x -count=1 \
    -cpuprofile=/tmp/hw10.cpu -memprofile=/tmp/hw10.mem -o /tmp/hw10.test .
go tool pprof -top -cum -focus='\.GetDomainStat$' /tmp/hw10.test /tmp/hw10.cpu
go tool pprof -top -alloc_space -focus='\.GetDomainStat$' /tmp/hw10.test /tmp/hw10.mem
```

Для исходной версии эти команды запускаются в `$hw10_baseline`. Фильтр `focus`
исключает подготовку ZIP из анализа профиля; в отличие от метрик бенчмарка,
профилировщик записывает и работу до `ResetTimer`.

## Проверки

- `golangci-lint run .` (1.50.1); новые обычные тесты также проверены отдельно
  по инструкции в README, с теми же правилами без build-тега `bench`.
- `go test -v -count=1 -race -timeout=1m .` — успешно на Go 1.19.13 и 1.26.4.
- `go test -count=1 -coverprofile=/tmp/hw10.cover .` — 100% покрытия statements.
- `go vet ./...`, `go mod tidy`, `go mod verify` — успешно.
- `go test -v -count=1 -timeout=30s -tags bench .` — успешно: 115,1 мс, 10 MiB
  по округлённому выводу теста при лимитах 300 мс / 30 MiB.
- Дополнительные пять запусков performance-теста — успешно: 116,6–118,3 мс,
  10–11 MiB по выводу теста; [полный лог](benchmarks/performance.txt).
- Обычные тесты также прошли в Linux/amd64 на Go 1.19.13 и 1.26.4.

Существующие `stats_test.go` и `stats_optimization_test.go` не изменены.

## Проверка после сбоя CI

Предыдущая реализация на `encoding/json.Decoder` проходила локальный performance-тест,
но в [GitHub Actions](https://github.com/isemibratov/golang-hw/actions/runs/33160020520)
заняла 360 мс при лимите 300 мс. Поэтому исправленная версия дополнительно проверена
в Linux/amd64 под эмуляцией Docker на том же Mac, `GOMAXPROCS=4`, квота 4 CPU.

| Версия Go | Полный performance-тест, три запуска | Память по выводу теста |
| --- | ---: | ---: |
| 1.19.13 | 241,1–248,3 мс | 10 MiB |
| 1.26.4 | 237,5–241,0 мс | 10 MiB |

Все шесть запусков прошли; [полный лог](benchmarks/performance_linux.txt).
Это другая среда, поэтому её время нельзя напрямую сравнивать с GitHub runner.
Окончательное подтверждение для PR — повторный успешный запуск CI.

Пример воспроизведения с Go 1.19.13 в `PATH` и заранее загруженным образом
`golang:1.19` для Linux/amd64:

```bash
GOOS=linux GOARCH=amd64 go test -c -tags bench -o /tmp/hw10-linux.test .
docker run --rm --pull=never --network none --platform linux/amd64 \
    --cpus=4 --read-only -e GOMAXPROCS=4 \
    -v /tmp/hw10-linux.test:/checks/hw10.test:ro \
    -v "$PWD":/workspace:ro -w /workspace golang:1.19 \
    /checks/hw10.test -test.v -test.count=3 -test.timeout=1m \
    -test.run='^TestGetDomainStat_Time_And_Memory$'
```
