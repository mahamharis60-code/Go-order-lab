#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.cloud.yml}"
APP_PORT="${ORDER_APP_PORT:-8090}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is not installed. Install Docker first." >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "docker compose v2 is not available. Install Docker Compose plugin first." >&2
  exit 1
fi

if [ ! -f .env ]; then
  if [ -f .env.cloud.example ]; then
    cp .env.cloud.example .env
    echo "created .env from .env.cloud.example; edit secrets before production use"
  else
    echo ".env is missing and .env.cloud.example was not found" >&2
    exit 1
  fi
fi

docker compose -f "$COMPOSE_FILE" up -d --build

echo "waiting for app health on 127.0.0.1:${APP_PORT}"
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${APP_PORT}/health" >/dev/null; then
    echo "health check passed: http://127.0.0.1:${APP_PORT}/health"
    docker compose -f "$COMPOSE_FILE" ps
    exit 0
  fi
  sleep 2
done

echo "health check failed, app logs:" >&2
docker compose -f "$COMPOSE_FILE" logs --tail=120 app >&2
exit 1
