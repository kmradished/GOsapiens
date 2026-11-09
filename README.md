# GOsapiens

## Команда

- Полина (`@polisha067`)
- Даша (`@klarty-n`)
- Марго (`@kmradished`)
- Арина (`@arina280607-cell`)

## HTTP-приложение — ЛР3

Нужен Go 1.22.2 или новее. Команды выполняются из корня репозитория.
Приложение и Profile API запускаются в двух терминалах; публичные API не используются.

Первый терминал — локальная зависимость:

```bash
go run ./cmd/profile-api
```

Второй терминал — приложение:

```bash
go run ./cmd/server
```

Приложение слушает `127.0.0.1:8080`, Profile API — `127.0.0.1:8081`.
Проверка запуска: `curl -i http://127.0.0.1:8080/healthz`.
Настройки адресов, бюджетов и лимитов описаны в [API-контракте](api-contract.md#конфигурация-и-ресурсы).
Пример с другими адресами:

```bash
go run ./cmd/profile-api -addr 127.0.0.1:9091
# В другом терминале:
HTTP_ADDR=127.0.0.1:9090 PROFILE_URL=http://127.0.0.1:9091 go run ./cmd/server
```

Общие фикстуры находятся в `internal/demodata`:

| ID | Состояние |
| --- | --- |
| `u1` | Текущий пользователь в примерах |
| `u6`, `u2`, `u5`, `u4` | Активные кандидаты для `u1` в таком порядке |
| `u3` | Неактивный профиль |
| `u7` | Активный профиль, заблокировавший `u1` |

У всех профилей есть фильм `Dune`. Поэтому для `u1` с `interest=Dune&limit=2`
первая страница содержит `u6`, `u2`, вторая — `u5`, `u4`.
Начальных Like, просмотров и дизлайков нет.

| Маршрут | Use case и прежняя логика | Автор |
| --- | --- | --- |
| `PUT /v1/me/likes/{targetID}` | `LikeContext`: допуск из ЛР1, чистый `DecideLike` из ЛР2, атомарный `Update` | Арина; проверка допуска и HTTP-профили — Полина |
| `GET /v1/me/pairs/{targetID}` | `PairMatched`: чтение сохранённой пары без записи | Арина |
| `GET /v1/me/discovery` | `DiscoveryService.Discover`: правила `available`/`interests` из ЛР1, фильтр, страницы и актуальные Like | Даша |
| Общая сборка | Один store для всех маршрутов, конфигурация, middleware, HTTP-сервер и его остановка | Марго |

Сценарий запросов с ожидаемыми статусами и существенными полями:

```bash
bash scripts/lr3-demo.sh
```

Перед каждым повтором сценария перезапусти приложение, чтобы очистить пары.
Скрипт сам возвращает Profile API в режим `ok`; переключение режима не очищает пары.
Для других адресов: `APP_URL=http://127.0.0.1:9090 PROFILE_URL=http://127.0.0.1:9091 bash scripts/lr3-demo.sh`.

Пары хранятся только в памяти одного процесса и теряются при перезапуске.
Независимые процессы приложения не делят ни память, ни mutex.
HTTP-команда использует `NewServiceWithUpdate`; прежний `NewService` с отдельными
`Load`/`Save` остаётся для демонстраций и тестов ЛР2.
Допуск Like теперь принимает context: `Check(ctx, actorID, targetID)`.

`X-Demo-User` — учебная идентичность, которую клиент может подставить самостоятельно.
Отсутствующий или неверный заголовок даёт `400`; предметные проверки остаются в приложении.
Каждый ответ получает серверный `X-Request-ID`; журналы содержат этот ID, метод,
путь, статус, длительность, а при ошибке — внутреннюю причину с тем же ID.

Для остановки приложения нажми `Ctrl+C` в его терминале. Оно ждёт активные запросы
до `SHUTDOWN_TIMEOUT`, затем принудительно закрывает соединения и сообщает об ошибке,
если времени не хватило. Команда `go build -o /tmp/gosapiens-server ./cmd/server`
создаёт отдельный бинарник для проверки `SIGTERM`: запускай его вместо `go run`.

## Структура проекта

- `internal/domain` — общие типы предметной области
- `internal/discovery/interests` — подбор по общим интересам
- `internal/discovery/available` — подбор кандидатов с ограничениями
- `internal/likes` — проверка допустимости лайка
- `internal/matching` — создание мэтча
- `internal/application` — общий сервис обработки лайка
- `internal/memory` — хранилища профилей, взаимодействий и состояния пар в памяти
- `internal/jsonpairs` — хранение состояния пар в JSON-байтах
- `internal/storecontract` — общие контрактные тесты хранилищ пар
- `internal/integration` — сквозной тест обработки лайков
- `internal/demodata` — общие фикстуры приложения и Profile API
- `internal/webserver` — настройки, middleware и остановка HTTP-сервера
- `cmd/server` — сборка и запуск трёх бизнес-маршрутов
- `cmd/profile-api` — отдельная локальная HTTP-зависимость
- `cmd/demo` — демонстрационный запуск контрактов A и C
- `cmd/available-demo` — демонстрационный запуск контракта B
- `cmd/matching-demo` — демонстрационный запуск контракта D

## Проверка лайка

Проверяет возможность поставить лайк кандидату.
Возвращает разрешение или причину отказа: лайк себе, отсутствующий
или неактивный профиль, блокировка.
Ошибка хранилища возвращается отдельно.

Повторный Like допускается; отсутствие изменений и пропуск `Save`
определяет application-слой.

Реализация: `internal/likes`, хранилище: `internal/memory/like_state.go`.

## Создание мэтча — контракт D

Исполнитель: @arina280607-cell. Задача: #4.

Service.Resolve принимает два непустых различных ID пользователей
и определяет результат по сохранённым реакциям:

- при отсутствии взаимных лайков возвращается waiting_for_like;
- при взаимных лайках создаётся и сохраняется мэтч — match_created;
- существующий мэтч повторно не сохраняется — match_already_exists.

Реакции имеют направление. Мэтч является общей связью:
пары u1–u10 и u10–u1 обозначают один мэтч.
Существующий мэтч подтверждает ранее состоявшиеся взаимные лайки.

Service.FullProfile выдаёт полный профиль только при наличии
сохранённого мэтча. Доступ предоставляется обоим пользователям.

Реализация включает бизнес-функцию Decide, Service, интерфейс
DataSource и in-memory хранилище Interactions.
Ошибки источника возвращаются отдельно от бизнес-результата;
при оборачивании через %w сохраняется причина ошибки.

Запуск демонстрации: `go run ./cmd/matching-demo`.

Проверки: `go test ./...` и `go vet ./...`.

## Подбор по общим интересам

Пакет `internal/discovery/interests` реализует подбор активных пользователей,
у которых с текущим пользователем совпадает хотя бы один фильм, музыкальный
интерес или книга.

Правила контракта:

- текущий пользователь исключается из результата;
- неактивные анкеты исключаются;
- пользователи без общих интересов исключаются;
- повторяющиеся интересы не создают дубликаты;
- порядок кандидатов сохраняется;
- результат содержит только ID кандидата и общие интересы, без имени и фотографии;
- отсутствие подходящих кандидатов возвращается как корректный пустой список;
- ошибки источника данных возвращаются отдельно от пустого результата.

Реализация разделена на чистую бизнес-функцию, Service, интерфейс источника
профилей и in-memory источник данных.

## Подбор доступных кандидатов

Пакет `internal/discovery/available` формирует список кандидатов, которых ещё
можно показать текущему пользователю.

Из результата исключаются:

- сам пользователь;
- неактивные анкеты;
- заблокированные пользователи;
- уже просмотренные анкеты;
- пользователи, которым уже поставили лайк или дизлайк.

Если доступных кандидатов нет, возвращается пустой список без ошибки. Service
получает профили и ограничения через интерфейсы, а правила отбора находятся в
отдельной функции.

## Выдача кандидатов — ЛР3

`GET /v1/me/discovery` → `DiscoveryService.Discover` → правила подбора из ЛР1,
фильтр и пагинация. Автор @klarty-n. Запрос требует `X-Demo-User` и пустое тело

- `interest` необязателен. Точное совпадение среди общих фильмов, музыки или книг
  Пробелы по краям удаляются, регистр сохраняется, пустое значение даёт `400`
- `limit` от 1 до 50, по умолчанию 20. `offset` от 0, по умолчанию 0
  Оба параметра целые. Фильтр и исключения, включая исходящие лайки,
  применяются до пагинации. Порядок профилей сохраняется

Неизвестные, повторённые и некорректные параметры дают `400 invalid_query`
Ответ `200` содержит только анонимные карточки. Пустая страница даёт `items: []`

```json
{"items":[{"candidate_id":"u6","common_movies":["Dune"],"common_music":[],"common_books":[]}],"limit":1,"offset":0}
```

Ошибки имеют поля `code` и `message`. Коды `invalid_query`, `invalid_body` и
`invalid_demo_user` дают `400`, `profile_not_found` — `404`, `internal_error` — `500`

Команды для общего сервера с чистым состоянием и четырьмя кандидатами `u6`,
`u2`, `u5`, `u4` для `u1` с общим фильмом `Dune`. Укажи адрес сервера в `base`

```bash
base=http://localhost:8080
curl -i -H 'X-Demo-User: u1' "$base/v1/me/discovery?interest=Dune&limit=2&offset=0" # 200, u6 и u2
curl -i -H 'X-Demo-User: u1' "$base/v1/me/discovery?interest=Dune&limit=2&offset=2" # 200, u5 и u4
curl -i -H 'X-Demo-User: u1' "$base/v1/me/discovery?interest=Dune&limit=2&offset=4" # 200, items: []
curl -i -H 'X-Demo-User: u1' "$base/v1/me/discovery?limit=0" # 400, invalid_query
curl -i -X PUT -H 'X-Demo-User: u1' "$base/v1/me/likes/u6" # 200, changed: true
curl -i -H 'X-Demo-User: u1' "$base/v1/me/discovery?interest=Dune&limit=2&offset=0" # 200, u2 и u5
```

## Обработка лайка — ЛР2

Исполнитель: @klarty-n

`internal/application.Service` получает `LikeChecker`, `PairStore` и
`DecideLikeFunc` через конструктор. Метод `Like(actorID, targetID)` сначала
проверяет допуск, затем создаёт канонический `domain.PairKey` и загружает
`domain.PairState`. Если пара ещё не сохранена (`found=false`), используется
нулевое состояние

После вызова `DecideLikeFunc` сервис сохраняет новое состояние ровно один раз
при `Changed=true`. При `Changed=false` запись не выполняется. Успешный
результат изменения возвращается только после успешного `Save`

При отказе проверки сервис возвращает `LikeDeniedError` с причиной отказа
При технической ошибке `Check`, `Load`, `DecideLikeFunc` или `Save` результат
равен `domain.LikeDecision{}`, а исходная ошибка доступна через `errors.Is`

Тесты в `internal/application/service_test.go` используют fake-зависимости:
проверяют обычный Like и канонический ключ, отсутствие пары, повтор без `Save`,
отказ допуска, ошибки `Load` и `Save`, а также успешный повтор после устранения
ошибки сохранения. Запуск: `go test ./internal/application`

Чистая функция `matching.DecideLike` вычисляет новое состояние пары и признаки
`Changed`, `Matched`, `MatchedNow`. Ответный Like создаёт взаимность с
`MatchedNow=true`, а повторы возвращают `Changed=false` и `MatchedNow=false`.
Функция не обращается к хранилищам и не изменяет входное состояние.

## Хранение состояния пары — ЛР2

Исполнитель: @kmradished. Задача: #14.

Реализованы два адаптера `application.PairStore`:

- `memory.PairStore` хранит состояния в map по ключу `domain.PairKey`;
- `jsonpairs.PairStore` хранит JSON-байты: `Save` выполняет `json.Marshal`,
  `Load` — `json.Unmarshal`.

Отсутствующая пара возвращает `found=false` без ошибки. Повторный `Save`
полностью заменяет состояние. Разные пары независимы. Невалидный ключ
возвращает `domain.ErrInvalidPair` без изменения сохранённых данных.

Оба адаптера проходят общий набор тестов `internal/storecontract.Check`.

## Сквозной тест — ЛР2

`TestScenario` в `internal/integration/scenario_test.go` объединяет реальные
`application.Service`, `likes.Service`, `matching.DecideLike` и memory-хранилища.

Проверяется последовательность:

- первый Like сохраняет состояние;
- повторный Like не вызывает новый `Save`;
- ответный Like создаёт взаимность с `MatchedNow=true`;
- повтор после взаимности не вызывает новый `Save` и возвращает `MatchedNow=false`.

После каждого шага проверяются сохранённое состояние пары
и количество вызовов `Save`: 1, 1, 2, 2.

## Запуск

Контракты A и C:

```bash
go run ./cmd/demo
```

Контракт B:

```bash
go run ./cmd/available-demo
```

Контракт D:

```bash
go run ./cmd/matching-demo
```

## Тесты

```bash
test -z "$(gofmt -l .)"
go build ./...
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
```

Отдельный запуск сквозного теста:

```bash
go test -count=1 ./internal/integration -run TestScenario -v
```

Сквозной HTTP-сценарий, отказы зависимости и отмена клиентского запроса:

```bash
go test -count=1 ./cmd/server -run TestHTTP -v
```

Два application-сервиса на одном store: 64 одновременных Like с повторами и
встречными реакциями дают две записи и ровно один `MatchedNow`:

```bash
go test -race -count=1 ./internal/integration -run TestConcurrentLikesAcrossServices -v
```

Ожидание активного запроса при Shutdown, принудительное закрытие после grace
и задержанное тело запроса проверяются без Sleep, с каналами и watchdog:

```bash
go test -count=1 ./internal/webserver -run 'TestRunShutdown|TestServerLimitsDelayedRequestBody' -v
```

CI запускает форматирование, сборку, тесты, vet и race:
[Go CI](https://github.com/kmradished/GOsapiens/actions/workflows/go.yml).

## ЛР2: допуск Like - Полина

Вызывайте `likes.Service.Check(ctx, actorID, targetID)` для проверки допуска:
лайк себе, отсутствующий или неактивный профиль и блокировка в любую сторону
возвращают отказ. Повторный Like проходит без проверки `HasReaction`;
успешный no-op без новой записи определяет application-слой.
Отсутствующий профиль обозначается `domain.ErrProfileNotFound`,
причины технических ошибок доступны через `errors.Is`

Проверка: `go test ./internal/likes ./internal/memory`

## ЛР3 Полина: локальный Profile API

Запустите зависимость отдельным процессом:

```bash
go run ./cmd/profile-api
```

`GET /profiles/u1` возвращает `{"id":"u1","active":true}`. Оба поля обязательны.
Активные профили — `u1`, `u2`, `u4`, `u5`, `u6`, `u7`. Профиль `u3` неактивен
Неизвестный ID даёт 404

Режим можно задать флагом `-mode` при запуске или переключить на работающем сервере:

```bash
curl -i -X PUT http://127.0.0.1:8081/_mode/slow
curl -i http://127.0.0.1:8081/profiles/u1
curl -i -X PUT http://127.0.0.1:8081/_mode/ok
```

| Режим | Ответ Profile API |
| --- | --- |
| `ok` | Профиль из фикстуры |
| `missing` | 404 |
| `error` | 500 |
| `slow` | Задержка с учётом отмены запроса |
| `large` | JSON больше лимита клиента |

Флаги сервера по умолчанию — `-addr 127.0.0.1:8081`, `-mode ok`,
`-slow-delay 2s`, `-large-bytes 8192`

В `cmd/server` в `profileapi.NewClient` передаются настроенный base URL,
общий `*http.Client` с таймаутом `600ms` и лимит `4096` байт.
Клиент используется в `likes.NewService` как источник профилей.
`Check` получает контекст команды с общим бюджетом `700ms`.
Ошибка `domain.ErrProfileNotFound` превращается в отказ `profile_not_found`
Технические ошибки распознаются через `errors.Is(err, profileapi.ErrDependency)`
Причина таймаута или отмены доступна через `errors.Is` и `errors.As`

## Issue, PR и review — ЛР3

| Участница | Issue | PR | Review |
| --- | --- | --- | --- |
| Полина | [#22](https://github.com/kmradished/GOsapiens/issues/22) | [#26](https://github.com/kmradished/GOsapiens/pull/26) | [Даша](https://github.com/kmradished/GOsapiens/pull/26#pullrequestreview-5830854295) |
| Арина | [#23](https://github.com/kmradished/GOsapiens/issues/23) | [#21](https://github.com/kmradished/GOsapiens/pull/21) | [Полина](https://github.com/kmradished/GOsapiens/pull/21#pullrequestreview-5831806415) |
| Даша | [#24](https://github.com/kmradished/GOsapiens/issues/24) | [#28](https://github.com/kmradished/GOsapiens/pull/28) | [Марго](https://github.com/kmradished/GOsapiens/pull/28#pullrequestreview-5827013696) |
| Марго | [#25](https://github.com/kmradished/GOsapiens/issues/25) | [#27 — CI](https://github.com/kmradished/GOsapiens/pull/27), [#29 — store и HTTP-обвязка](https://github.com/kmradished/GOsapiens/pull/29) | [Арина](https://github.com/kmradished/GOsapiens/pull/29#pullrequestreview-5831473166) |

При сдаче команда указывает точный commit итоговой сборки и ссылку на его
успешный CI. Текущий commit можно узнать командой `git rev-parse HEAD`.
