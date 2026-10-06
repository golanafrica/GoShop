#!/bin/sh
set -e

HOST="${DB_HOST:-db}"
PORT="${DB_PORT:-5432}"
USER="${DB_USER:-postgres}"
PASS="${DB_PASSWORD:-root}"
NAME="${DB_NAME:-goshop_db}"
DIR="${MIGRATIONS_DIR:-/migrations}"

export PGPASSWORD="$PASS"

echo "==> Waiting for Postgres at ${HOST}:${PORT}..."
i=0
until psql -h "$HOST" -p "$PORT" -U "$USER" -d "$NAME" -c "SELECT 1" >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -gt 60 ]; then
    echo "ERROR: Postgres not ready after 60s"
    exit 1
  fi
  sleep 1
done
echo "==> Postgres is ready"

psql -h "$HOST" -p "$PORT" -U "$USER" -d "$NAME" -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  version    text PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);
SQL

count=0
for f in $(ls -1 "$DIR"/*.sql 2>/dev/null | sort); do
  base=$(basename "$f")
  version=${base%.sql}

  exists=$(psql -h "$HOST" -p "$PORT" -U "$USER" -d "$NAME" -tAc \
    "SELECT 1 FROM schema_migrations WHERE version = '${version}'")

  if [ "$exists" = "1" ]; then
    echo "SKIP  ${version}"
    continue
  fi

  echo "APPLY ${version}"
  psql -h "$HOST" -p "$PORT" -U "$USER" -d "$NAME" -v ON_ERROR_STOP=1 -f "$f"
  psql -h "$HOST" -p "$PORT" -U "$USER" -d "$NAME" -v ON_ERROR_STOP=1 \
    -c "INSERT INTO schema_migrations (version) VALUES ('${version}')"
  count=$((count + 1))
done

echo "==> Migrations done (${count} applied)"