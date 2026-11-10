#!/bin/sh
set -eu
# Isolated, ephemeral PostgreSQL; never use the developer or production database.
password=$(openssl rand -hex 24)
container=$(docker run -d --rm --tmpfs /var/lib/postgresql/data -e POSTGRES_PASSWORD="$password" -e POSTGRES_DB=journey_test -p 127.0.0.1::5432 postgres:17-alpine)
cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
attempt=0
until docker exec "$container" pg_isready -U postgres -d journey_test >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then echo 'Test PostgreSQL did not become ready.' >&2; exit 1; fi
  sleep 1
done
port=$(docker port "$container" 5432/tcp | sed 's/.*://')
cd apps/api
if [ "$#" -eq 0 ]; then set -- ./internal/...; fi
# Each package gets a fresh database. Migration tests intentionally reset their
# schema, while workers claim globally; neither may affect another package.
packages=$(go list "$@")
index=0
for test_package in $packages; do
  index=$((index + 1))
  test_db="package_$index"
  docker exec "$container" createdb -U postgres "$test_db"
  TEST_DATABASE_URL="postgres://postgres:$password@127.0.0.1:$port/$test_db?sslmode=disable" go test -race -count=1 -tags integration "$test_package"
done
