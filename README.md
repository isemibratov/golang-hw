## Домашние задания курса [«Разработчик Golang»]
1) [«Hello, DIASOFT!»](./hw01_hello_diasoft)
2) [«Распаковка строки»](./hw02_unpack_string)
3) [«Частотный анализ»](./hw03_frequency_analysis)
4) [«LRU-кэш»](./hw04_lru_cache)
5) [«Параллельное исполнение»](./hw05_parallel_execution)
6) [«Пайплайн»](./hw06_pipeline_execution)
7) [«Утилита для копирования файлов»](./hw07_file_copying)
8) [«Утилита envdir»](./hw08_envdir_tool)
9) [«Валидатор структур»](./hw09_struct_validator)
10) [«Оптимизация программы»](./hw10_program_optimization)
11) [«Клиент TELNET»](./hw11_telnet_client)
12) [«Заготовка сервиса Календарь»](hw12_13_14_15_16_calendar/docs/12_README.md)
13) [«API к Календарю»](hw12_13_14_15_16_calendar/docs/13_README.md)
14) [«Кроликизация Календаря»](hw12_13_14_15_16_calendar/docs/14_README.md)
15) [«Докеризация и интеграционное тестирование Календаря»](hw12_13_14_15_16_calendar/docs/15_README.md)

---
## Инструкция по сдаче ДЗ.
### Настройка репозитория и создание Pull Request.
Для сдачи ДЗ необходимо выполнить следующие действия (по порядку):\
Нажмите на зелёную кнопку «Use this template» на странице данного репозитория.
Подождите пока сгенерируется новый репозиторий на основе текущего.
Новый репозиторий должен быть публичным.
Склонируйте себе ваш репозиторий и создайте ветку с именем таким же, как директория, где лежит ДЗ. Это важно!

```
$ cd hw-test
$ git checkout -b hw01_hello_diasoft 
```
Реализуйте код домашнего задания в директории с ДЗ.
Проверьте, что следующие команды завершаются успешно:
```
$ cd hw01_hello_diasoft
$ golangci-lint run .
$ go test -v -count=1 -race -timeout=1m .
$ ./test.sh # При наличии
```
Это те команды, которые запускаются в CI (см. .github/workflows/tests.yml).

Зафиксируйте изменения и запушьте ветку в репозиторий:
```
$ git commit -am "HW1 is completed"
$ git push origin hw01_hello_diasoft
remote: Create a pull request for 'hw01_hello_diasoft' on GitHub by visiting:
remote:      https://github.com/Antonboom/hw-test/pull/new/hw01_hello_diasoft
```
Как видно выше, GitHub предложит вам URL для создания пулл реквеста, пройдите по нему.\
Допишите в конец URL параметр вида &template=<имя_ветки>.md и нажмите Enter - PR обновится в соответствии с одним из шаблонов. Нажмите кнопку «Create pull request».
### Добавление наставников в репозиторий
После того, как вы отправили первую домашнюю работу на проверку, к вам будет закреплен наставник - его в этом ДЗ и в последующих нужно будет добавлять во все PR в качества ревьювера (зайдите на страницу настроек доступа (Settings -> Collaborators -> Manage access), нажмите "Add people" и пригласите наставника, никнейм наставники будет доступен после того, как он первый раз проверит ваш PR).\
Это необходимо для того, чтобы наше ревью в вашем PR имело "вес", а также, чтобы мы могли выставлять галочки в критериях оценки :)

### Защита ветки master
Зайдите на страницу настроек веток репозитория (Settings -> Branches):
добавить новое правило (Branch protection rules -> Add classic branch protection rule):
Branch name pattern -> master (именно так);
- выставить галочку "Require a pull request before merging";
- выставить галочку "Require approvals";
- выставить галочку "Require review from Code Owners";
- выставить галочку "Require status checks to pass before merging";
- выставить галочку "Require branches to be up to date before merging";
- добавить в "Status checks that are required" следующие статусы:
  - lint
  - tests
  - tests_by_makefile
- выставить галочку "Do not allow bypass the above settings";\

Нажать кнопку «Save changes».
Скиньте ссылку на PR в чат с преподавателем.
Пройдите ревью и после одобрения пулл реквеста вмержите PR в master.


При выполнении последующего ДЗ не забудьте перед бранчеванием переключиться назад в master, иначе hw02 будет отбранчевана от hw01, что повлечет за собой лишние коммиты в PR.
```
$ git checkout master
$ git checkout -b hw02_unpack_string
```
Убедительная просьба не мержить реквесты без апрува от проверяющего!

---
Используемая версия [golangci-lint](https://golangci-lint.run/usage/install/#other-ci): <b>v1.50.1</b>
```
$ golangci-lint version
golangci-lint has version 1.50.1 built from 8926a95 on 2022-10-22T10:48:48Z
```

---
Авторы ДЗ:
- [Дмитрий Смаль](https://github.com/mialinx)
- [Антон Телышев](https://github.com/Antonboom)

