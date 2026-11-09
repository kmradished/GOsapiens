#!/usr/bin/env bash
set -eu

# Перед запуском перезапусти приложение, чтобы хранилище пар было пустым.
app_url=${APP_URL:-http://127.0.0.1:8080}
profile_url=${PROFILE_URL:-http://127.0.0.1:8081}

request() {
    printf '\n>>> %s\n' "$1"
    shift
    curl --silent --show-error --include --max-time 5 "$@"
    printf '\n'
}

request 'Profile API: режим ok, ожидается 204' -X PUT "$profile_url/_mode/ok"
request 'Проверка приложения: 200' "$app_url/healthz"
request 'Первая страница: 200, u6 и u2' -H 'X-Demo-User: u1' "$app_url/v1/me/discovery?interest=Dune&limit=2&offset=0"
request 'Вторая страница: 200, u5 и u4' -H 'X-Demo-User: u1' "$app_url/v1/me/discovery?interest=Dune&limit=2&offset=2"
request 'Исходное состояние: 200, matched=false' -H 'X-Demo-User: u1' "$app_url/v1/me/pairs/u6"

request 'Первый Like: 200, changed=true, matched=false' -X PUT -H 'X-Demo-User: u1' "$app_url/v1/me/likes/u6"
request 'Повтор: 200, changed=false, matched_now=false' -X PUT -H 'X-Demo-User: u1' "$app_url/v1/me/likes/u6"
request 'После Like: 200, u2 и u5' -H 'X-Demo-User: u1' "$app_url/v1/me/discovery?interest=Dune&limit=2&offset=0"
request 'Встречный Like: 200, matched=true, changed=true, matched_now=true' -X PUT -H 'X-Demo-User: u6' "$app_url/v1/me/likes/u1"
request 'Чтение результата от u1: 200, matched=true' -H 'X-Demo-User: u1' "$app_url/v1/me/pairs/u6"
request 'Чтение результата от u6: 200, matched=true' -H 'X-Demo-User: u6' "$app_url/v1/me/pairs/u1"
request 'Повтор после взаимности: 200, matched=true, changed=false, matched_now=false' -X PUT -H 'X-Demo-User: u6' "$app_url/v1/me/likes/u1"

request 'Пустая страница: 200, items=[]' -H 'X-Demo-User: u1' "$app_url/v1/me/discovery?limit=2&offset=100"
request 'Неверный limit: 400, invalid_query' -H 'X-Demo-User: u1' "$app_url/v1/me/discovery?limit=0"
request 'Нет пользователя: 400, invalid_demo_user' "$app_url/v1/me/discovery"
request 'Запрещённое тело: 400, invalid_body' -X PUT -H 'X-Demo-User: u1' -d '{}' "$app_url/v1/me/likes/u2"
request 'Like себе: 409, self_like' -X PUT -H 'X-Demo-User: u1' "$app_url/v1/me/likes/u1"
request 'Неактивный профиль: 409, inactive_profile' -X PUT -H 'X-Demo-User: u1' "$app_url/v1/me/likes/u3"
request 'Блокировка: 403, blocked' -X PUT -H 'X-Demo-User: u1' "$app_url/v1/me/likes/u7"

# Пара u1-u2 ещё не сохранена. После каждого отказа u2 остаётся в выдаче.
request 'Включаем missing: 204' -X PUT "$profile_url/_mode/missing"
request 'missing: 404, profile_not_found' -X PUT -H 'X-Demo-User: u1' "$app_url/v1/me/likes/u2"
request 'После missing: 200, u2 и u5' -H 'X-Demo-User: u1' "$app_url/v1/me/discovery?interest=Dune&limit=2&offset=0"

request 'Включаем error: 204' -X PUT "$profile_url/_mode/error"
request 'error: 502, dependency_unavailable' -X PUT -H 'X-Demo-User: u1' "$app_url/v1/me/likes/u2"
request 'После error: 200, u2 и u5' -H 'X-Demo-User: u1' "$app_url/v1/me/discovery?interest=Dune&limit=2&offset=0"

request 'Включаем slow: 204' -X PUT "$profile_url/_mode/slow"
request 'slow: 504, dependency_timeout' -X PUT -H 'X-Demo-User: u1' "$app_url/v1/me/likes/u2"
request 'После slow: 200, u2 и u5' -H 'X-Demo-User: u1' "$app_url/v1/me/discovery?interest=Dune&limit=2&offset=0"

request 'Включаем large: 204' -X PUT "$profile_url/_mode/large"
request 'large: 502, dependency_unavailable' -X PUT -H 'X-Demo-User: u1' "$app_url/v1/me/likes/u2"
request 'После large: 200, u2 и u5' -H 'X-Demo-User: u1' "$app_url/v1/me/discovery?interest=Dune&limit=2&offset=0"

request 'Возвращаем ok: 204' -X PUT "$profile_url/_mode/ok"
request 'Повтор после восстановления: 200, changed=true' -X PUT -H 'X-Demo-User: u1' "$app_url/v1/me/likes/u2"
request 'Итоговая первая страница: 200, u5 и u4' -H 'X-Demo-User: u1' "$app_url/v1/me/discovery?interest=Dune&limit=2&offset=0"
