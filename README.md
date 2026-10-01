# GOsapiens

## Команда

- Полина (`@polisha067`)
- Даша (`@klarty-n`)
- Марго (`@kmradished`)
- Арина (`@arina280607-cell`)

## Структура проекта

- `internal/domain` — общие типы предметной области
- `internal/discovery/interests` — подбор по общим интересам
- `internal/discovery/available` — подбор кандидатов с ограничениями
- `internal/likes` — проверка допустимости лайка
- `internal/matching` — создание мэтча
- `cmd/demo` — демонстрационный запуск

## Запуск

```bash
go run ./cmd/demo
```

## Тесты

```bash
go test ./...
```
