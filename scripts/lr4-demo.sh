#!/usr/bin/env bash
set -eu

app_url=${APP_URL:-http://127.0.0.1:8080}
profile_url=${PROFILE_URL:-http://127.0.0.1:8081}
body_file=$(mktemp)
trap 'rm -f "$body_file"' EXIT

request() {
    local expected=$1 method=$2 url=$3 status
    shift 3
    status=$(curl --silent --show-error --max-time 5 -o "$body_file" -w '%{http_code}' -X "$method" "$url" "$@")
    printf '\n>>> %s %s — %s\n' "$method" "$url" "$status"
    cat "$body_file"
    if [[ "$status" != "$expected" ]]; then
        printf 'Ожидался статус %s, получен %s\n' "$expected" "$status" >&2
        exit 1
    fi
}

expect() {
    if ! grep -Fq -- "$1" "$body_file"; then
        printf 'В ответе ожидалось %s\n' "$1" >&2
        exit 1
    fi
}

check_saved() {
    request 200 GET "$app_url/v1/profiles/u1" -H 'X-Demo-User: u1'
    expect '"name":"Марго"'
    expect '"version":2'
    request 403 PUT "$app_url/v1/me/likes/u6" -H 'X-Demo-User: u1'
    expect '"code":"blocked"'
}

case "${1:-}" in
    after-restart) check_saved; exit 0 ;;
    "") ;;
    *) printf 'Usage: bash scripts/lr4-demo.sh [after-restart]\n' >&2; exit 2 ;;
esac

request 204 PUT "$profile_url/_mode/ok"
request 200 GET "$app_url/healthz"
expect '"status":"ok"'
request 200 GET "$app_url/v1/me/discovery?limit=1" -H 'X-Demo-User: u1'
expect '"candidate_id":"u6"'

request 200 PUT "$app_url/v1/me/likes/u6" -H 'X-Demo-User: u1'
expect '"changed":true'
expect '"matched":false'
request 200 PUT "$app_url/v1/me/likes/u6" -H 'X-Demo-User: u1'
expect '"changed":false'
request 200 GET "$app_url/v1/me/discovery?limit=1" -H 'X-Demo-User: u1'
expect '"candidate_id":"u2"'
request 200 PUT "$app_url/v1/me/likes/u1" -H 'X-Demo-User: u6'
expect '"matched_now":true'
request 200 GET "$app_url/v1/me/pairs/u6" -H 'X-Demo-User: u1'
expect '"matched":true'

request 200 DELETE "$app_url/v1/me/likes/u6" -H 'X-Demo-User: u1'
expect '"changed":true'
expect '"matched":false'
request 200 DELETE "$app_url/v1/me/likes/u6" -H 'X-Demo-User: u1'
expect '"changed":false'
request 200 GET "$app_url/v1/me/pairs/u6" -H 'X-Demo-User: u1'
expect '"matched":false'
request 200 GET "$app_url/v1/me/discovery?limit=1" -H 'X-Demo-User: u1'
expect '"candidate_id":"u6"'

request 200 PUT "$app_url/v1/me/blocks/u1" -H 'X-Demo-User: u6'
expect '"changed":true'
request 200 PUT "$app_url/v1/me/blocks/u1" -H 'X-Demo-User: u6'
expect '"changed":false'
request 200 GET "$app_url/v1/me/discovery?limit=1" -H 'X-Demo-User: u1'
expect '"candidate_id":"u2"'

request 200 GET "$app_url/v1/profiles/u1" -H 'X-Demo-User: u1'
expect '"version":1'
edit='{"name":"Марго","bio":"Люблю кино","version":1}'
request 200 PUT "$app_url/v1/profiles/u1" -H 'X-Demo-User: u1' -H 'Content-Type: application/json' -d "$edit"
expect '"version":2'
request 409 PUT "$app_url/v1/profiles/u1" -H 'X-Demo-User: u1' -H 'Content-Type: application/json' -d "$edit"
expect '"code":"version_conflict"'
check_saved

for mode in missing error slow large; do
    case "$mode" in
        missing) status=404; code=profile_not_found ;;
        slow) status=504; code=dependency_timeout ;;
        *) status=502; code=dependency_unavailable ;;
    esac
    request 204 PUT "$profile_url/_mode/$mode"
    request "$status" PUT "$app_url/v1/me/likes/u2" -H 'X-Demo-User: u1'
    expect "\"code\":\"$code\""
    request 200 GET "$app_url/v1/me/discovery?limit=1" -H 'X-Demo-User: u1'
    expect '"candidate_id":"u2"'
done
request 204 PUT "$profile_url/_mode/ok"
printf '\nСценарий ЛР4 прошёл.\n'
