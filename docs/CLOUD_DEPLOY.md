# Cloud Deployment

This route deploys the project on an Ubuntu cloud server with Docker Compose.

## Target

- Go API is exposed on `8090`.
- MySQL, Redis, and RabbitMQ stay inside the Docker network and are not exposed to the public internet.
- The app can be verified by `/health` and `scripts/smoke-test.js`.

## Server Requirements

- Ubuntu 22.04 or 24.04
- 2 CPU / 4 GB RAM recommended
- Docker Engine
- Docker Compose v2
- Security group inbound rules: `22` for SSH and `8090` for the API demo

## Deploy

```bash
git clone https://github.com/mahamharis60-code/Go-order-lab.git
cd Go-order-lab
cp .env.cloud.example .env
```

Edit `.env` and change the placeholder passwords:

```text
MYSQL_ROOT_PASSWORD
MYSQL_PASSWORD
ORDER_DB_SOURCE
ORDER_JWT_SECRET
ORDER_ADMIN_PASSWORD
RABBITMQ_DEFAULT_PASS
ORDER_RABBITMQ_URL
```

Start the stack with source build:

```bash
bash scripts/deploy-cloud.sh
```

If the server has trouble pulling the Go builder image, use the runtime route.
Build the Linux binary locally or in CI, upload `dist/go-order-lab-linux-amd64`
to the server, then start the stack with:

```bash
COMPOSE_FILE=docker-compose.cloud-runtime.yml bash scripts/deploy-cloud.sh
```

This still runs the app, MySQL, Redis, and RabbitMQ in Docker Compose. Only the
app API port is exposed.

Verify:

```bash
curl http://127.0.0.1:8090/health
curl http://<public-ip>:8090/health
```

Run smoke test from your local machine:

```bash
ORDER_BASE_URL=http://<public-ip>:8090 node scripts/smoke-test.js
```

## Operations

```bash
docker compose -f docker-compose.cloud.yml ps
docker compose -f docker-compose.cloud.yml logs -f app
docker compose -f docker-compose.cloud.yml restart app
docker compose -f docker-compose.cloud.yml down
```

For the runtime route, replace `docker-compose.cloud.yml` with
`docker-compose.cloud-runtime.yml`.

Do not expose MySQL, Redis, or RabbitMQ ports to the public internet.
