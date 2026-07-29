# Docker Route

本项目默认可以不用 Docker：SQLite + channel 足够完成本地学习、smoke test 和接口压测。安装 Docker Desktop 后，可以切换到 MySQL + Redis + RabbitMQ 的完整中间件链路。

## Full Compose

```bat
cd /d path\to\go-order-lab
docker compose up --build
```

如果宿主机已经占用了 MySQL/Redis/RabbitMQ 或 8090 端口，可以覆盖端口后启动：

```bat
set ORDER_APP_PORT=18090
set ORDER_MYSQL_PORT=13306
set ORDER_REDIS_PORT=16379
set ORDER_RABBITMQ_PORT=15673
set ORDER_RABBITMQ_MANAGEMENT_PORT=15674
docker compose up --build
```

启动后访问：

- Go API: `http://127.0.0.1:8090`
- Health: `http://127.0.0.1:8090/health`
- MySQL: `127.0.0.1:3306`
- Redis: `127.0.0.1:6379`
- RabbitMQ Management: `http://127.0.0.1:15672`

RabbitMQ 管理页账号密码：

```text
order_user / order_pass
```

默认管理员账号用于后台接口和 smoke test：

```text
admin / admin123456
```

## Full Runtime Compose

如果目标机器已经有 `dist/go-order-lab-linux-amd64`，推荐先试这个版本。它仍然会一次性拉起 Go app、MySQL、Redis、RabbitMQ 四个容器，但 app 镜像用 `Dockerfile.scratch` 从二进制构建，避免再拉 `golang:<version>-alpine` 构建镜像。

```bash
cd /home/trrr/go-order-lab
ORDER_APP_PORT=28090 \
ORDER_MYSQL_PORT=23306 \
ORDER_REDIS_PORT=26379 \
ORDER_RABBITMQ_PORT=25672 \
ORDER_RABBITMQ_MANAGEMENT_PORT=25673 \
sudo -E docker compose -f docker-compose.full-runtime.yml up -d --build
curl http://127.0.0.1:28090/health
```

Windows 主机侧验证：

```bat
set ORDER_BASE_URL=http://192.168.220.138:28090
scripts\smoke.cmd
```

## VM Mixed Compose（推荐）

如果 VM 已经通过 apt/systemd 跑着 MySQL、Redis、RabbitMQ，可以用混合 Compose 只编排 Go app 容器，避免重复拉取数据库和消息队列镜像：

```bash
cd /home/trrr/go-order-lab
ORDER_APP_PORT=18090 sudo docker compose -f docker-compose.mixed.yml up -d --build
curl http://127.0.0.1:18090/health
```

VM 已升级到 Docker Compose v2。若仍在其他机器上使用旧版 `docker-compose 1.29.2`，重建 scratch 镜像容器时可能报 `KeyError: 'ContainerConfig'`，可先清理旧 app 容器再强制重建：

```bash
ORDER_APP_PORT=18090 sudo docker compose -f docker-compose.mixed.yml rm -fsv app
ORDER_APP_PORT=18090 sudo docker compose -f docker-compose.mixed.yml up -d --build --force-recreate
```

Windows 主机侧 smoke test：

```bat
set ORDER_BASE_URL=http://192.168.220.138:18090
scripts\smoke.cmd
```

说明：`18090` 是推荐展示端口，对应 Docker Compose v2 mixed 部署；`8090` 是手动 `docker run` 备用端口，对应 native Docker 部署。

## Environment Switches

Compose 中的 app 服务会自动开启：

```text
ORDER_DB_DRIVER=mysql
ORDER_ADMIN_USERNAME=admin
ORDER_ADMIN_PASSWORD=admin123456
ORDER_REDIS_ENABLED=true
ORDER_RABBITMQ_ENABLED=true
ORDER_RABBITMQ_QUEUE=order.created
ORDER_COMPENSATION_ENABLED=false
ORDER_REQUEST_TIMEOUT_SECONDS=5
ORDER_SHUTDOWN_TIMEOUT_SECONDS=10
```

本地 `go run` 默认不开 Redis/RabbitMQ。如果想连接本机中间件：

```bat
set ORDER_DB_DRIVER=mysql
set ORDER_DB_SOURCE=order_user:order_pass@tcp(127.0.0.1:3306)/order_lab?charset=utf8mb4&parseTime=True&loc=Local
set ORDER_REDIS_ENABLED=true
set ORDER_REDIS_ADDR=127.0.0.1:6379
set ORDER_RABBITMQ_ENABLED=true
set ORDER_RABBITMQ_URL=amqp://order_user:order_pass@127.0.0.1:5672/
set ORDER_RABBITMQ_QUEUE=order.created
set ORDER_COMPENSATION_ENABLED=false
set ORDER_REQUEST_TIMEOUT_SECONDS=5
set ORDER_SHUTDOWN_TIMEOUT_SECONDS=10
go run ./cmd/server
```

## Current Verification State

已完成：

- 本地 SQLite/channel 路径可编译、可跑 smoke test。
- Redis Lua 库存预占代码已接入 `CreateOrder`。
- RabbitMQ 发布/消费代码已接入 worker。
- 补偿接口已接入，可手动关闭超时待支付订单、重投卡队列订单、结束过期活动。
- Dockerfile 和 `docker-compose.yml` 已包含 app、MySQL、Redis、RabbitMQ。
- `docker-compose.full-runtime.yml` 已提供 app/MySQL/Redis/RabbitMQ 全容器运行路线，适合使用现成 Linux 二进制降低镜像拉取压力。
- `docker-compose.mixed.yml` 已支持在 VM 上用 Compose 编排 Go app 容器，并连接宿主机 MySQL/Redis/RabbitMQ。
- RBAC 已接入，普通用户 token 不能访问商品创建、活动管理、补偿、库存对账和后台接口。
- 请求级 timeout 和 Docker stop/SIGTERM 优雅停机已接入。
- VM 上已验证 Docker Compose v2 + `docker-compose.mixed.yml`：`curl http://127.0.0.1:18090/health` 返回 `{"status":"ok"}`，并通过完整 smoke test。
- 手动 Docker 备用路线仍可用：`go-order-lab-app-native` 监听 `8090`。

待你本机安装/启动 Docker Desktop 或网络可正常拉镜像后验证：

- `docker compose up --build`
- `docker compose -f docker-compose.full-runtime.yml up -d --build`
- Compose 环境下的 smoke test
- RabbitMQ 管理页确认 `order.created` 队列消息被消费

本轮 VM 中默认全容器 Compose 曾卡在镜像拉取阶段，所以没有声称 `docker-compose.yml` 源码构建链路已经在该 VM 跑通。

补充：`docker-compose.full-runtime.yml` 已规避 Go builder 镜像，但仍需要拉取 MySQL/RabbitMQ 镜像；当前 VM 网络下 180 秒内仍卡在镜像拉取阶段，所以 full-runtime 全容器链路也暂未声称跑通。
