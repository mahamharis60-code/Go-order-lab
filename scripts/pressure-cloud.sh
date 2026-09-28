#!/usr/bin/env bash
set -euo pipefail

# Run from the deployed project directory. Never print the credential values.
root=$(pwd)
stamp=$(date -u +%Y%m%dT%H%M%SZ)
out="$root/reports/pressure-$stamp"
mkdir -p "$out"
export ORDER_ADMIN_USERNAME=$(sed -n 's/^ORDER_ADMIN_USERNAME=//p' .env | tr -d '\r')
export ORDER_ADMIN_PASSWORD=$(sed -n 's/^ORDER_ADMIN_PASSWORD=//p' .env | tr -d '\r')

{
  date -u
  uname -a
  lscpu
  free -h
  docker ps --format '{{.Names}} {{.Image}}'
  for name in ORDER_RATE_LIMIT_ENABLED ORDER_RATE_LIMIT_RPS ORDER_RATE_LIMIT_BURST ORDER_WORKER_COUNT ORDER_DB_MAX_OPEN_CONNS ORDER_DB_MAX_IDLE_CONNS ORDER_COMPENSATION_ENABLED ORDER_REDIS_ENABLED ORDER_RABBITMQ_ENABLED; do
    docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' go-order-lab-app-cloud | grep -F "$name=" || true
  done
} > "$out/environment.txt"

sample_resources() {
  while true; do
    date -u
    docker stats --no-stream --format '{{.Name}} cpu={{.CPUPerc}} memory={{.MemUsage}} net={{.NetIO}}'
    docker exec go-order-lab-rabbitmq-cloud rabbitmqctl list_queues name messages_ready messages_unacknowledged consumers
    sleep 2
  done
}
sample_resources > "$out/resources.txt" 2>&1 &
sampler=$!
trap 'kill "$sampler" 2>/dev/null || true; wait "$sampler" 2>/dev/null || true' EXIT

status=0
for concurrency in ${PRESSURE_CONCURRENCIES:-10 20 50 100}; do
  if ! ./dist/pressure -concurrency "$concurrency" -users "${PRESSURE_USERS:-200}" \
    -stock 30 -duplicate-requests 100 -write-requests "${PRESSURE_WRITES:-1000}" \
    -duration "${PRESSURE_DURATION:-30s}" -out "$out/result-$concurrency.json" \
    > "$out/run-$concurrency.txt" 2>&1; then
    status=1
  fi
done
docker exec go-order-lab-rabbitmq-cloud rabbitmqctl list_queues name messages_ready messages_unacknowledged consumers > "$out/queues-after.txt"
echo "Reports: $out"
exit "$status"
