# Go load testing

The Go command uses fixed request-worker goroutines and HTTP connection reuse.
It runs two finite order-creation correctness scenarios and one sustained
order-detail query workload. It is not a claim of maximum production capacity.

## Run

Build on your workstation or server:

```bash
go build -o dist/pressure ./cmd/pressure
```

In the existing cloud project directory, export only the required credentials
from your trusted `.env` without printing their values:

```bash
export ORDER_ADMIN_USERNAME="$(sed -n 's/^ORDER_ADMIN_USERNAME=//p' .env | tr -d '\r')"
export ORDER_ADMIN_PASSWORD="$(sed -n 's/^ORDER_ADMIN_PASSWORD=//p' .env | tr -d '\r')"
./dist/pressure -base-url http://127.0.0.1:8090 \
  -concurrency 20 -duplicate-requests 100 -users 200 -stock 30 \
  -duration 30s -out reports/pressure-go-20.json
```

This creates new test users, products, activities and orders. Registration,
activity setup and warmup are excluded from the measured windows. Do not run
against a production business database. The command does not delete data or
change rate-limit/compensation settings.

For a concurrency sweep, run separately at 10, 20, 50 and 100 workers. Each
invocation uses fresh data. If rate limiting is enabled, 429 is reported
separately and the exact-stock correctness gate may fail. Record the actual
rate-limit, compensation, database pool and worker settings alongside results.

## Measurements and gates

- Completed requests/second includes business rejections; accepted orders/second
  counts only HTTP 202. Finite bursts are not steady-state capacity estimates.
- Latency is client-side elapsed time through reading the response body.
  P50/P95/P99 use nearest-rank over every completed attempt, including errors.
- Transport failures, invalid JSON and HTTP 5xx are system errors. HTTP 409 and
  429 remain distinct status/message counts, not silently counted as successes.
- The duplicate scenario must accept exactly one order. The stock scenario must
  accept exactly the configured stock. Either can legitimately fail due to
  limiting or infrastructure faults; inspect the report rather than hiding it.
- Verification reads persisted orders through the admin API, waits up to 20s for
  QUEUED to settle, verifies WAIT_PAY, detects duplicate user IDs and compares
  MySQL/Redis stock with initial stock minus accepted orders. It never repairs
  stock to make a test pass. Inventory is capped at 100 to fit the admin API's
  current result limit.
- The sustained workload queries an existing order, not order creation. Its QPS
  must be labeled as order-detail query QPS. It excludes warmup and includes the
  tail time needed to finish in-flight requests after scheduling stops.
- `valid_order_writes` pre-creates enough stocked activities, sends unique
  user/activity requests, and requires every write to be accepted and persisted.
  It measures a finite successful-write batch, separately from rejection-heavy
  competition tests. Change its size using `-write-requests` (default 1000).
  `async_verification_seconds` measures time spent after the batch waiting and
  checking all expected orders and stock. This includes polling/API overhead;
  it is an upper bound on observed state-settling time, not pure broker latency.

The cloud helper runs a concurrency sweep and samples container resources and
queue depth without printing credentials:

```bash
bash scripts/pressure-cloud.sh
```

Its default container names match the existing cloud deployment. The results
are saved in a timestamped `reports/pressure-*` directory. No app settings are
changed. The server must already run the current API with Redis enabled.

## Server evidence

Collect host and container evidence before/during each run:

```bash
date -u
uname -a
lscpu
free -h
docker compose ps
docker stats --no-stream
docker compose exec -T rabbitmq rabbitmqctl list_queues \
  name messages_ready messages_unacknowledged consumers
```

For repeated sampling while the command runs:

```bash
while true; do
  date -u
  docker stats --no-stream
  docker compose exec -T rabbitmq rabbitmqctl list_queues \
    name messages_ready messages_unacknowledged consumers
  sleep 2
done
```

Use the same Compose `-f` file that started your cloud deployment. Stop sampling
with Ctrl+C. Keep credentials out of published reports. Save machine CPU/RAM,
image version, test location and settings. Same-host loopback tests share CPU
between client and server and exclude public-network latency; public health
checks do not replace a separate remote-client load test.

The suite's async-state settling check is not a measurement of dead-letter
recovery or compensation success rate. Those require separate fault-injection
scenarios. Do not quote such rates unless measured. Coverage is separately
available with `go test ./... -coverprofile=coverage.out` and
`go tool cover -func=coverage.out`; coverage measures exercised statements, not
load capacity.
