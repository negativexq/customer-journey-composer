#!/bin/sh
set -eu
password=$(openssl rand -hex 24)
container=$(docker run -d --rm --cpus 2 --memory 2g --shm-size 256m --tmpfs /var/lib/postgresql/data -e POSTGRES_PASSWORD="$password" -e POSTGRES_DB=journey_benchmark -p 127.0.0.1::5432 postgres:17-alpine -c shared_buffers=256MB -c work_mem=8MB)
cleanup() { docker stop "$container" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
attempt=0
until docker exec "$container" pg_isready -U postgres -d journey_benchmark >/dev/null 2>&1; do
 attempt=$((attempt+1)); if [ "$attempt" -ge 60 ]; then echo 'Benchmark database unavailable.' >&2; exit 1; fi
 sleep 1
done
port=$(docker port "$container" 5432/tcp | sed 's/.*://')
export BENCH_DATABASE_URL="postgres://postgres:$password@127.0.0.1:$port/journey_benchmark?sslmode=disable"
export BENCH_COMMIT=$(git rev-parse HEAD)
BENCH_DIRTY=false
if [ -n "$(git status --porcelain)" ]; then BENCH_DIRTY=true; fi
export BENCH_DIRTY
cd apps/api
go run ./cmd/benchmark "$@"
