#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

case "${1:-}" in
  migrate) sql_file=sql/001.sql ;;
  reset) sql_file=sql/reset.sql ;;
  *) printf 'Usage: bash scripts/db.sh migrate|reset\n' >&2; exit 1 ;;
esac

# Команды относятся только к локальной учебной БД из compose.yaml.
docker compose exec -T db psql -U gosapiens -d gosapiens \
  -v ON_ERROR_STOP=1 < "$sql_file"
