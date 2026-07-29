# Run Report

验证时间：2026-07-28

## Code Verification

```bat
set GOPATH=D:\Codex\sit-work\cache\go
set GOMODCACHE=D:\Codex\sit-work\cache\go\pkg\mod
set GOCACHE=D:\Codex\sit-work\cache\go-build
set GOPROXY=https://goproxy.cn,direct
D:\Go\bin\go.exe test ./...
```

结果：

```text
?   	go-order-lab/cmd/server	[no test files]
?   	go-order-lab/internal/config	[no test files]
?   	go-order-lab/internal/database	[no test files]
?   	go-order-lab/internal/handler	[no test files]
?   	go-order-lab/internal/middleware	[no test files]
?   	go-order-lab/internal/model	[no test files]
?   	go-order-lab/internal/response	[no test files]
?   	go-order-lab/internal/service	[no test files]
```

## Local Smoke Test

运行模式：默认 SQLite + channel worker。

```bat
node scripts\smoke-test.js
```

关键结果：

```json
{
  "register": 201,
  "product": 201,
  "activity": 201,
  "order": 202,
  "payment": 200,
  "duplicatePayment": 200,
  "duplicateOrder": 409,
  "checkout": 201,
  "checkoutPayment": 200,
  "expireOrders": 200,
  "expiredCount": 1
}
```

说明：

- `order=202`：活动下单创建 `QUEUED` 订单，交给异步 worker。
- `duplicateOrder=409`：同一用户重复下单被拦截。
- `duplicatePayment=200`：重复支付回调被幂等处理。
- `expireOrders=200`：超时关单流程可用。

## Concurrent Test

```bat
node scripts\pressure-order.js
```

结果：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 19
  },
  "multiUserSummary": {
    "202": 3,
    "409": 7
  }
}
```

说明：

- 同一用户 20 个并发请求，只有 1 个成功，其余 19 个被重复下单拦截。
- 10 个用户并发抢 3 个活动库存，只有 3 个成功，其余 7 个被库存上限拦截。

## VM Deployment

运行环境：VMware Ubuntu 22.04，IP `192.168.220.138`。

由于 Docker Hub 镜像拉取在 VM 网络下长期卡住，本轮采用可控的混合部署：

- MySQL 8.0：Ubuntu apt 服务，库名 `order_lab`。
- Redis 6：Ubuntu apt 服务，监听 `127.0.0.1:6379`。
- RabbitMQ 3.9：Ubuntu apt 服务，队列 `order.created`。
- Go 服务：使用 `Dockerfile.scratch` 构建 `go-order-lab:vm`，通过 Docker 容器运行。

容器启动结果：

```text
CONTAINER ID   IMAGE             COMMAND               STATUS       NAMES
b88c5cf93231   go-order-lab:vm   "/app/go-order-lab"   Up 5 sec     go-order-lab-app-native
```

服务日志关键行：

```text
redis stock guard enabled at 127.0.0.1:6379
rabbitmq order queue enabled: order.created
go order lab listening on http://127.0.0.1:8090
```

健康检查：

```bat
curl http://192.168.220.138:8090/health
```

结果：

```json
{"status":"ok"}
```

## VM Seed Data Verification

在 VM 中确认 GORM `AutoMigrate` 已创建表后，执行固定演示数据导入：

```bash
mysql -uorder_user -p****** order_lab < deploy/sql/002_seed_data.sql
```

验证查询结果：

```text
products
id    name           stock
1001  demo-phone     100
1002  demo-headset   200

activities
id    name             stock
1001  demo-flash-sale  20

coupons
id    title           stock
1001  demo-coupon-50  100
```

说明：`deploy/sql/001_schema_notes.sql` 记录 MySQL database/user 初始化方式；`deploy/sql/002_seed_data.sql` 只负责插入可重复执行的演示商品、活动和优惠券数据。

## VM Docker Smoke Test

从 Windows 主机访问 VM 容器服务：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\vm-docker-run
node scripts\smoke-test.js
```

关键结果：

```json
{
  "register": 201,
  "product": 201,
  "activity": 201,
  "order": 202,
  "list": 200,
  "payment": 200,
  "duplicatePayment": 200,
  "duplicateOrder": 409,
  "logs": 200,
  "checkout": 201,
  "checkoutPayment": 200,
  "expireOrders": 200
}
```

## VM Docker Concurrent Test

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_PRESSURE_OUT=D:\Codex\sit-work\reports\vm-docker-run
node scripts\pressure-order.js
```

结果：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 19
  },
  "multiUserSummary": {
    "202": 3,
    "409": 7
  }
}
```

说明：

- 同一用户 20 个并发下单请求，1 个进入异步订单队列，19 个被重复下单限制拦截。
- 10 个用户并发抢 3 个活动库存，3 个成功，7 个被 Redis 预扣库存逻辑拦截。

## Stage 1 Foundation Verification

验证时间：2026-07-29

本轮验证内容：

- 环境配置样例：`.env.example`、`.env.vm.example`。
- Windows 命令脚本：`scripts/run-local.cmd`、`scripts/test.cmd`、`scripts/smoke.cmd`、`scripts/pressure.cmd`。
- MySQL 初始化和 seed SQL：`deploy/sql/001_schema_notes.sql`、`deploy/sql/002_seed_data.sql`。
- 架构/API/README 文档：`docs/ARCHITECTURE.md`、`docs/API.md`、`README.md`。

### Go Test

```bat
go test ./...
```

结果：

```text
?   	go-order-lab/cmd/server	[no test files]
?   	go-order-lab/internal/config	[no test files]
?   	go-order-lab/internal/database	[no test files]
?   	go-order-lab/internal/handler	[no test files]
?   	go-order-lab/internal/middleware	[no test files]
?   	go-order-lab/internal/model	[no test files]
?   	go-order-lab/internal/response	[no test files]
?   	go-order-lab/internal/service	[no test files]
```

### Local SQLite Service

本地启动方式：使用 `go-order-lab.exe`，配置 SQLite、关闭 Redis/RabbitMQ，监听 `:8091`。

健康检查：

```bat
curl --max-time 3 -s -i http://127.0.0.1:8091/health
```

结果：

```text
HTTP/1.1 200 OK
{"status":"ok"}
```

本地 smoke test：

```bat
set ORDER_BASE_URL=http://127.0.0.1:8091
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\stage1-local
node scripts\smoke-test.js
```

关键结果：

```json
{
  "register": 201,
  "product": 201,
  "activity": 201,
  "order": 202,
  "payment": 200,
  "duplicatePayment": 200,
  "duplicateOrder": 409,
  "checkout": 201,
  "checkoutPayment": 200,
  "expireOrders": 200,
  "expiredCount": 5
}
```

本地 pressure test：

```bat
set ORDER_BASE_URL=http://127.0.0.1:8091
set ORDER_PRESSURE_OUT=D:\Codex\sit-work\reports\stage1-local
node scripts\pressure-order.js
```

结果：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 19
  },
  "multiUserSummary": {
    "202": 3,
    "409": 7
  }
}
```

验证后已停止本地 `go-order-lab.exe` 进程。

### VM Middleware Service

VM 目标服务：`http://192.168.220.138:8090`。

VM smoke test：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\stage1-vm
node scripts\smoke-test.js
```

关键结果：

```json
{
  "register": 201,
  "product": 201,
  "activity": 201,
  "order": 202,
  "payment": 200,
  "duplicatePayment": 200,
  "duplicateOrder": 409,
  "checkout": 201,
  "checkoutPayment": 200,
  "expireOrders": 200,
  "expiredCount": 5
}
```

VM pressure test：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_PRESSURE_OUT=D:\Codex\sit-work\reports\stage1-vm
node scripts\pressure-order.js
```

结果：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 19
  },
  "multiUserSummary": {
    "202": 3,
    "409": 7
  }
}
```

补充验证：脚本封装命令也已通过。

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\stage1-vm-script
scripts\smoke.cmd

set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_PRESSURE_OUT=D:\Codex\sit-work\reports\stage1-vm-script
scripts\pressure.cmd
```

脚本版结果与上方 VM smoke/pressure test 一致。

结论：

- Stage 1 工程底座已完成并通过验证。
- 本地 SQLite 学习模式可跑通完整接口链路。
- VM 中间件模式可跑通 MySQL/Redis/RabbitMQ/Dockerized Go app 链路。
- 全 Docker Compose 容器化链路仍未完成，原因是 VM 拉取外部镜像不稳定。

## RabbitMQ Reliability Verification

验证时间：2026-07-29

本轮新增：

- RabbitMQ 消息 envelope：`{"order_no":"...","retry":0}`。
- 兼容旧 plain text `order_no` 消息。
- `ORDER_RABBITMQ_MAX_RETRIES` 配置项，默认 `3`。
- worker 失败后重新投递 `retry+1` 消息；超过阈值后写入 `<queue>.dlq`。
- `scripts/test.cmd` 和 `scripts/run-local.cmd` 增加 `GOTMPDIR/TMP/TEMP=D:\Codex\sit-work\cache\go-tmp`，避免 Go 测试二进制落到 C 盘临时目录。

全量测试：

```bat
scripts\test.cmd
```

结果：

```text
?   	go-order-lab/cmd/server	[no test files]
?   	go-order-lab/internal/config	[no test files]
?   	go-order-lab/internal/database	[no test files]
?   	go-order-lab/internal/handler	[no test files]
?   	go-order-lab/internal/middleware	[no test files]
?   	go-order-lab/internal/model	[no test files]
?   	go-order-lab/internal/response	[no test files]
ok  	go-order-lab/internal/service	65.486s
```

新增 service 单元测试：

```text
=== RUN   TestRabbitOrderMessageRoundTrip
--- PASS: TestRabbitOrderMessageRoundTrip (0.00s)
=== RUN   TestRabbitOrderMessagePlainTextCompatibility
--- PASS: TestRabbitOrderMessagePlainTextCompatibility (0.00s)
=== RUN   TestRabbitOrderMessageRejectsEmpty
--- PASS: TestRabbitOrderMessageRejectsEmpty (0.00s)
PASS
```

本地回归验证：

```bat
go build -o D:\Codex\sit-work\reports\go-order-lab-rabbit-check.exe ./cmd/server
curl --max-time 3 -s -i http://127.0.0.1:8092/health
set ORDER_BASE_URL=http://127.0.0.1:8092
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\rabbit-check-local
scripts\smoke.cmd
set ORDER_PRESSURE_OUT=D:\Codex\sit-work\reports\rabbit-check-local
scripts\pressure.cmd
```

关键结果：

```json
{
  "health": 200,
  "smoke_order": 202,
  "duplicate_order": 409,
  "same_user_summary": {
    "202": 1,
    "409": 19
  },
  "multi_user_summary": {
    "202": 3,
    "409": 7
  }
}
```

说明：本地回归验证使用 SQLite + channel fallback，证明主业务链路没有被 RabbitMQ 改造破坏。RabbitMQ retry/dead-letter 逻辑已通过代码编译和消息解析单元测试；后续如需验证真实死信队列，可在 VM 上重新构建镜像并制造一个不可处理的订单消息。

## Latest VM Deployment and DLQ Verification

验证时间：2026-07-29

本轮目标：把最新代码重新部署到 VM，并验证 MySQL/Redis/RabbitMQ/Dockerized Go app 主链路和 RabbitMQ 死信队列。

### Deployment

本地重新构建 Linux 二进制：

```bat
set GOOS=linux
set GOARCH=amd64
go build -o dist\go-order-lab-linux-amd64 ./cmd/server
```

VM 部署方式：

- 上传 `dist/go-order-lab-linux-amd64` 到 `/home/trrr/go-order-lab/dist/go-order-lab-linux-amd64`。
- 上传 `Dockerfile.scratch`。
- 在 VM 上执行 `docker build -f Dockerfile.scratch -t go-order-lab:vm .`。
- 删除旧容器并以 `--network host` 启动新容器 `go-order-lab-app-native`。

最新容器：

```text
CONTAINER ID   IMAGE             COMMAND               CREATED          STATUS          NAMES
8a029cd04d00   go-order-lab:vm   "/app/go-order-lab"   36 seconds ago   Up 36 seconds   go-order-lab-app-native
```

服务和中间件检查：

```text
curl http://127.0.0.1:8090/health -> {"status":"ok"}
mysqladmin ping -> mysqld is alive
redis-cli ping -> PONG
rabbitmq-diagnostics -q ping -> Ping succeeded
```

### Latest VM Smoke Test

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\latest-vm-deploy
scripts\smoke.cmd
```

关键结果：

```json
{
  "register": 201,
  "product": 201,
  "activity": 201,
  "order": 202,
  "payment": 200,
  "duplicatePayment": 200,
  "duplicateOrder": 409,
  "checkout": 201,
  "checkoutPayment": 200,
  "expireOrders": 200,
  "expiredCount": 9
}
```

### Latest VM Pressure Test

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_PRESSURE_OUT=D:\Codex\sit-work\reports\latest-vm-deploy
scripts\pressure.cmd
```

结果：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 19
  },
  "multiUserSummary": {
    "202": 3,
    "409": 7
  }
}
```

### RabbitMQ DLQ Verification

验证前队列：

```text
order.created      0  0
order.created.dlq  0  0
```

使用临时 AMQP publisher 投递不可处理的订单消息到 `order.created`，worker 查询订单失败后触发 retry 和 dead-letter。

第一次验证使用 plain text 兼容路径，验证重试递增后进入死信；第二次验证使用 JSON envelope：

```json
{"order_no":"ORD_MISSING_JSON_DLQ_TEST","retry":3}
```

第二次验证后队列：

```text
order.created      0  0
order.created.dlq  2  0
```

数据库操作日志：

```text
action                  result   message
order_task_retry         success  retry=1 reason=record not found
order_task_retry         success  retry=2 reason=record not found
order_task_retry         success  retry=3 reason=record not found
order_task_dead_letter   failed   retry=3 reason=record not found
order_task_dead_letter   failed   retry=3 reason=record not found  # JSON envelope path
```

结论：

- 最新版本已重新部署到 VM。
- 主链路 smoke test 通过。
- 并发压测通过。
- RabbitMQ 最大重试和死信队列在真实 VM 环境中验证通过。
- 本轮使用的临时部署/验证脚本位于 `D:\Codex\sit-work\scripts` 和 `D:\Codex\sit-work\tools`，不属于 GitHub 项目源码。

## Compensation and Activity Status Verification

日期：2026-07-29

本轮目标：补全活动状态管理和补偿任务，形成“活动下线拦截、卡队列订单重投、超时待支付关单、过期活动结束”的异常闭环。

### Code Build

Windows 本机完成：

```bat
D:\Go\bin\gofmt.exe -w ...
D:\Go\bin\go.exe build -o dist\go-order-lab.exe ./cmd/server
D:\Go\bin\go.exe test -c -o D:\Codex\sit-work\cache\test-bin\service.test.exe ./internal/service
```

结果：应用编译通过，service 测试二进制编译通过。

说明：Windows 本机执行 Go 自动生成的测试二进制时出现 `Access is denied`，因此本轮以 VM 真实服务接口验证为主。

### VM Deployment

部署方式：交叉编译 Linux 二进制，上传到 Ubuntu VM，通过 `Dockerfile.scratch` 构建镜像并启动容器。

最新容器：

```text
CONTAINER ID   IMAGE             COMMAND               STATUS
2ea7f3abb59a   go-order-lab:vm   "/app/go-order-lab"   Up
```

健康检查：

```json
{"status":"ok"}
```

容器路由日志确认新增接口已注册：

```text
PATCH /api/activities/:id/status
POST  /api/ops/compensate
```

### VM Smoke Test

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\compensation-vm-final
scripts\smoke.cmd
```

结果：

```json
{
  "register": 201,
  "product": 201,
  "activity": 201,
  "order": 202,
  "payment": 200,
  "duplicatePayment": 200,
  "duplicateOrder": 409,
  "offlineStatus": 200,
  "offlineOrder": 409,
  "checkout": 201,
  "checkoutPayment": 200,
  "timeoutCheckout": 201,
  "compensateOrders": 200,
  "compensateClosedOrders": 5,
  "compensateRequeuedOrders": 0,
  "compensateEndedActivities": 3,
  "expireOrders": 200,
  "expiredCount": 0
}
```

结论：

- 活动下线接口可用，`OFFLINE` 活动下单被 `409` 拦截。
- 补偿接口可用，成功关闭超时待支付订单并归还库存。
- 过期活动被标记为 `ENDED`。

### VM Pressure Test

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_PRESSURE_OUT=D:\Codex\sit-work\reports\compensation-vm-final
scripts\pressure.cmd
```

结果：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 19
  },
  "multiUserSummary": {
    "202": 3,
    "409": 7
  }
}
```

结论：

- 同一用户重复下单拦截仍有效。
- 多用户抢有限库存时成功数不超过库存数。

### MySQL Verification

查询结果：

```text
order_no                    status
ORD20260729013042a7a0ecac   CLOSED

action                    order_no                  result    message
compensate_end_activity                             success   activity=1016 marked ENDED
compensate_end_activity                             success   activity=1015 marked ENDED
compensate_close_order    ORD20260729013042a7a0ecac success   timeout wait-pay order closed and stock returned

status      count
ENDED       21
OFFLINE     2
PUBLISHED   7
```

结论：补偿动作写入数据库，订单状态、活动状态和操作日志均可追踪。

## Trace ID and Rate Limit Verification

日期：2026-07-29

本轮目标：补全 HTTP 入口保护和请求追踪能力，为下单接口增加可配置令牌桶限流，为所有请求增加 `X-Trace-ID` 和访问日志。

### Code Build

Windows 本机完成：

```bat
D:\Go\bin\gofmt.exe -w internal\middleware\*.go ...
D:\Go\bin\go.exe build -o dist\go-order-lab.exe ./cmd/server
D:\Go\bin\go.exe test -c -o D:\Codex\sit-work\cache\test-bin\middleware.test.exe ./internal/middleware
```

结果：应用编译通过，middleware 测试二进制编译通过。

说明：Windows 本机执行 Go 自动生成的测试二进制时仍出现 `Access is denied`，因此本轮继续以 VM 真实服务接口验证为主。

### Default VM Deployment

默认部署容器：

```text
CONTAINER ID   IMAGE             COMMAND               STATUS
72b6da053fe5   go-order-lab:vm   "/app/go-order-lab"   Up
```

健康检查：

```json
{"status":"ok"}
```

容器日志确认访问日志带 trace id：

```text
trace_id=trace_1785290138715305522_7036b78309e5 method=GET path=/health status=200 latency=115.052µs client_ip=127.0.0.1
```

### Default VM Smoke Test

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\trace-rate-limit-vm
scripts\smoke.cmd
```

结果：

```json
{
  "register": 201,
  "order": 202,
  "orderTraceID": "trace_1785290149881112143_7f2da4a850b0",
  "registerTraceID": "trace_1785290149818706939_eab75393236d",
  "duplicateOrder": 409,
  "offlineOrder": 409,
  "compensateOrders": 200
}
```

结论：主链路通过，响应头返回 `X-Trace-ID`。

### Default VM Pressure Test

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_PRESSURE_OUT=D:\Codex\sit-work\reports\trace-rate-limit-vm
scripts\pressure.cmd
```

结果：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 19
  },
  "multiUserSummary": {
    "202": 3,
    "409": 7
  }
}
```

结论：默认限流关闭时，重复下单和库存保护逻辑不受影响。

### Rate-Limited VM Test

临时重启容器开启限流：

```text
ORDER_RATE_LIMIT_ENABLED=true
ORDER_RATE_LIMIT_RPS=1
ORDER_RATE_LIMIT_BURST=2
```

限流容器：

```text
CONTAINER ID   IMAGE             COMMAND
eef865bb66b2   go-order-lab:vm   "/app/go-order-lab"
```

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_RATE_LIMIT_OUT=D:\Codex\sit-work\reports\trace-rate-limit-vm
set ORDER_RATE_LIMIT_CONCURRENCY=12
scripts\rate-limit.cmd
```

结果：

```json
{
  "summary": {
    "202": 1,
    "409": 1,
    "429": 10
  }
}
```

结论：

- 令牌桶限流生效，突发请求会返回 `429`。
- 允许少量突发请求进入业务链路，因此仍能看到 `202` 和业务级 `409`。
- 每个响应都带 `X-Trace-ID`，便于按请求追踪日志。

### Restored Default VM Container

验证限流后已恢复默认不限流容器：

```text
CONTAINER ID   IMAGE             COMMAND               STATUS
1f80e61020d1   go-order-lab:vm   "/app/go-order-lab"   Up
```

结论：当前 VM 服务已恢复到默认学习/演示状态，不会因为限流影响后续普通 smoke test。

## Stock Prewarm and Reconcile Verification

日期：2026-07-29

本轮目标：补全 Redis 库存预热和 Redis/MySQL 库存对账修复能力，说明活动库存如何进入 Redis，以及 Redis 与 MySQL 不一致时如何发现和修复。

### Code Build

Windows 本机完成：

```bat
D:\Go\bin\gofmt.exe -w internal\service\redis_stock.go internal\service\catalog.go internal\service\order.go ...
D:\Go\bin\go.exe build -o dist\go-order-lab.exe ./cmd/server
D:\Go\bin\go.exe test -c -o D:\Codex\sit-work\cache\test-bin\service.test.exe ./internal/service
```

结果：应用编译通过，service 测试二进制编译通过。

说明：Windows 本机执行 Go 自动生成的测试二进制时仍出现 `Access is denied`，因此本轮继续以 VM 真实服务接口验证为主。

### VM Deployment

部署方式：交叉编译 Linux 二进制，上传到 Ubuntu VM，通过 `Dockerfile.scratch` 构建镜像并启动容器。

容器：

```text
CONTAINER ID   IMAGE             COMMAND               STATUS
f21deb6a1df0   go-order-lab:vm   "/app/go-order-lab"   Up
```

健康检查：

```json
{"status":"ok"}
```

容器路由日志确认新增接口已注册：

```text
POST /api/ops/stock/reconcile
```

### VM Smoke Test

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\stock-reconcile-vm-smoke
scripts\smoke.cmd
```

结果摘要：

```json
{
  "register": 201,
  "activity": 201,
  "order": 202,
  "duplicateOrder": 409,
  "offlineOrder": 409,
  "compensateOrders": 200
}
```

结论：新增库存预热/对账能力没有破坏订单主链路。

### VM Pressure Test

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_PRESSURE_OUT=D:\Codex\sit-work\reports\stock-reconcile-vm-smoke
scripts\pressure.cmd
```

结果：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 19
  },
  "multiUserSummary": {
    "202": 3,
    "409": 7
  }
}
```

结论：同一用户重复下单拦截和多用户抢库存约束仍有效。

### API Prewarm Check

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_STOCK_RECONCILE_OUT=D:\Codex\sit-work\reports\stock-reconcile-vm-api
scripts\stock-reconcile.cmd
```

结果：

```json
{
  "register": 201,
  "product": 201,
  "activity": 201,
  "reconcile": 200,
  "activityID": 1034,
  "checked": 1,
  "missing": 0,
  "mismatched": 0,
  "repaired": 0
}
```

结论：新活动创建后 Redis 库存已预热，对账无缺失、无错配。

### Mismatch and Repair Check

命令：

```bat
D:\Codex\sit-work\tools\vm-venv\Scripts\python.exe D:\Codex\sit-work\scripts\verify_stock_reconcile_vm.py
```

验证步骤：

1. 创建活动，库存为 `8`。
2. 调用 `/api/ops/stock/reconcile`，初始结果无 mismatch。
3. 在 VM 上执行 `redis-cli SET order:activity:<id>:stock 2` 故意制造错配。
4. 再次对账，发现 `redis_stock=2`、`mysql_stock=8`。
5. 使用 `repair=true` 调用对账接口。
6. 再读 Redis，库存恢复为 `8`。

结果：

```json
{
  "activityID": 1035,
  "initial": {
    "checked": 1,
    "missing": 0,
    "mismatched": 0,
    "repaired": 0
  },
  "mismatch": {
    "checked": 1,
    "missing": 0,
    "mismatched": 1,
    "repaired": 0,
    "items": [
      {
        "activity_id": 1035,
        "mysql_stock": 8,
        "redis_stock": 2,
        "redis_exists": true,
        "repaired": false
      }
    ]
  },
  "repaired": {
    "checked": 1,
    "missing": 0,
    "mismatched": 1,
    "repaired": 1
  },
  "finalRedisStock": "8"
}
```

### Operation Log Check

查询结果：

```text
action                    result    message
stock_reconcile_repair    success   activity=1035 redis_stock=2 mysql_stock=8
stock_reconcile_mismatch  failed    activity=1035 redis_stock=2 mysql_stock=8
```

结论：

- 活动创建后 Redis 库存预热生效。
- Redis/MySQL 库存不一致可以被对账接口发现。
- `repair=true` 可以用 MySQL 库存修复 Redis。
- mismatch 和 repair 都写入操作日志，便于面试讲异常排查。

## Prometheus Metrics Verification

日期：2026-07-29

本轮目标：补全服务侧可观测性指标，让项目能说明“如何看接口请求量、状态码、耗时和关键业务事件”。

### Code Build

Windows 本机完成：

```bat
D:\Go\bin\gofmt.exe -w internal\metrics\*.go internal\middleware\metrics.go internal\handler\order.go ...
D:\Go\bin\go.exe build -o dist\go-order-lab.exe ./cmd/server
D:\Go\bin\go.exe test -c -o D:\Codex\sit-work\cache\test-bin\metrics.test.exe ./internal/metrics
D:\Go\bin\go.exe test -c -o D:\Codex\sit-work\cache\test-bin\middleware.test.exe ./internal/middleware
node --check scripts\metrics-test.js
```

结果：应用编译通过，metrics/middleware 测试二进制编译通过，Node 验证脚本语法通过。

说明：Windows 本机直接执行 Go 自动生成的测试二进制仍会卡住或出现权限问题，因此本轮以编译通过和 VM 真实服务接口验证为主。

### Deployment Debugging

第一次部署后，`scripts/metrics-test.js` 访问 `/metrics` 返回 `404 page not found`。

根因：

- 本地只重新编译了 Windows 二进制 `dist/go-order-lab.exe`。
- VM Docker 镜像使用的是 Linux 二进制 `dist/go-order-lab-linux-amd64`。
- 部署日志中 `COPY dist/go-order-lab-linux-amd64` 走了 Docker cache，说明容器里仍是旧版本。

修复：

```bat
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
D:\Go\bin\go.exe build -o dist\go-order-lab-linux-amd64 ./cmd/server
D:\Codex\sit-work\tools\vm-venv\Scripts\python.exe D:\Codex\sit-work\scripts\deploy_latest_order_vm_safe.py
```

重新部署时 Docker build 重新执行了 `COPY dist/go-order-lab-linux-amd64 /app/go-order-lab`，最新容器为：

```text
CONTAINER ID   IMAGE             COMMAND               STATUS
9df8045b2a95   go-order-lab:vm   "/app/go-order-lab"   Up
```

健康检查：

```json
{"status":"ok"}
```

### VM Metrics Test

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_METRICS_OUT=D:\Codex\sit-work\reports\metrics-vm
scripts\metrics.cmd
```

结果：

```json
{
  "health": 200,
  "register": 201,
  "product": 201,
  "activity": 201,
  "order": 202,
  "metrics": 200,
  "orderTraceID": "trace_1785292917523724023_97d2e8df8958"
}
```

`metrics.txt` 关键内容：

```text
go_order_http_requests_total{method="POST",path="/api/orders",status="202"} 1
go_order_http_request_duration_seconds_count{method="POST",path="/api/orders",status="202"} 1
go_order_business_events_total{event="activity_order",result="accepted"} 1
```

结论：`/metrics` 不是静态页面；脚本先真实走了健康检查、注册、建商品、建活动和下单，然后确认 HTTP 指标和业务事件指标被写入。

### VM Regression

Smoke test：

```json
{
  "register": 201,
  "product": 201,
  "activity": 201,
  "order": 202,
  "duplicateOrder": 409,
  "offlineOrder": 409,
  "compensateOrders": 200,
  "compensateClosedOrders": 6
}
```

Pressure test：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 19
  },
  "multiUserSummary": {
    "202": 3,
    "409": 7
  }
}
```

Stock reconcile test：

```json
{
  "activityID": 1044,
  "checked": 1,
  "missing": 0,
  "mismatched": 0,
  "repaired": 0
}
```

结论：

- Prometheus 文本指标已在 VM 真服务中验证通过。
- metrics 中间件没有破坏主业务链路、并发下单保护和库存对账能力。
- 当前只完成服务侧 metrics endpoint，尚未接入 Prometheus Server、Grafana 面板和告警规则。

## Database Constraint Verification

日期：2026-07-29

本轮目标：补强数据库层兜底约束，避免同一用户同一活动在绕过 Redis 或出现并发竞态时生成重复活动订单。

### Design

约束方案：

- `orders.order_no` 已有唯一索引，保证订单号唯一。
- `payments.transaction_no` 已有唯一索引，保证支付回调幂等。
- 新增 `orders.activity_order_key`，活动订单写入 `<user_id>:<activity_id>`。
- `activity_order_key` 建唯一索引；普通购物车订单保持 `NULL`，不参与活动订单唯一约束。

选择 nullable key 的原因：

- 如果直接对 `(user_id, activity_id)` 建唯一索引，普通购物车订单的 `activity_id=0` 会导致同一用户只能有一个购物车订单。
- MySQL 和 SQLite 都允许唯一索引中存在多个 `NULL`，因此 nullable key 能同时支持活动订单唯一和购物车订单多次创建。

### Local Test

新增测试文件：

```text
internal/service/order_constraints_test.go
```

验证内容：

- 同一 `activity_order_key` 的第二个活动订单会被数据库唯一约束拒绝。
- 同一用户可以创建多个 `activity_order_key=NULL` 的购物车订单。
- service 层会把数据库唯一冲突映射为 `ErrDuplicateOrder`。

命令：

```bat
set GOPATH=D:\Codex\sit-work\cache\go
set GOMODCACHE=D:\Codex\sit-work\cache\go\pkg\mod
set GOCACHE=D:\Codex\sit-work\cache\go-build
set GOTMPDIR=D:\Codex\sit-work\cache\go-tmp
D:\Go\bin\go.exe test ./internal/service -count=1
D:\Go\bin\go.exe build -o dist\go-order-lab.exe ./cmd/server
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
D:\Go\bin\go.exe build -o dist\go-order-lab-linux-amd64 ./cmd/server
```

结果：

```text
ok      go-order-lab/internal/service   1.253s
```

窄测试中可见 SQLite 唯一约束拦截日志：

```text
UNIQUE constraint failed: orders.activity_order_key
```

### VM Migration

部署时健康检查第一次过早执行，返回连接拒绝；容器日志确认服务随后正常启动，并执行了迁移：

```text
ALTER TABLE `orders` ADD `activity_order_key` varchar(96)
go order lab listening on http://127.0.0.1:8090
```

后续健康检查：

```json
{"status":"ok"}
```

历史活动订单 key 回填数量：

```text
filled_keys
74
```

MySQL 索引查询：

```text
Key_name                         Non_unique  Column_name
idx_orders_order_no              0           order_no
idx_orders_activity_order_key    0           activity_order_key
idx_orders_request_id            1           request_id
idx_orders_user_id               1           user_id
idx_orders_activity_id           1           activity_id
```

### VM Regression

Smoke test：

```json
{
  "register": 201,
  "product": 201,
  "activity": 201,
  "order": 202,
  "duplicateOrder": 409,
  "checkout": 201,
  "compensateOrders": 200,
  "compensateClosedOrders": 5
}
```

Pressure test：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 19
  },
  "multiUserSummary": {
    "202": 3,
    "409": 7
  }
}
```

Stock reconcile test：

```json
{
  "activityID": 1054,
  "checked": 1,
  "missing": 0,
  "mismatched": 0,
  "repaired": 0
}
```

结论：

- 活动订单数据库唯一约束已在 SQLite 单元测试和 VM MySQL 中验证。
- 普通购物车订单未被误伤，smoke test 中 `checkout=201`。
- 主链路、并发抢购和库存对账在新增约束后仍通过。
## Admin Ops Verification

本轮新增轻量后台运营接口：

- `GET /api/admin/overview`：汇总用户、商品、活动、订单状态分布、已支付 GMV、库存和失败日志。
- `GET /api/admin/orders`：按 `status/user_id/activity_id/limit` 查询订单。

### Unit and Handler Tests

VM 上使用最新源码运行：

```bash
cd /home/trrr/go-order-lab
/usr/local/go/bin/go test ./internal/service -count=1 -v
/usr/local/go/bin/go test ./internal/handler -count=1 -v
```

关键结果：

```text
=== RUN   TestAdminOverviewCountsPaidGMVAndFailureLogs
--- PASS: TestAdminOverviewCountsPaidGMVAndFailureLogs
=== RUN   TestAdminListOrdersFiltersByStatusUserAndActivity
--- PASS: TestAdminListOrdersFiltersByStatusUserAndActivity
ok  	go-order-lab/internal/service	3.878s

=== RUN   TestAdminOverviewHandlerReturnsWrappedMetrics
--- PASS: TestAdminOverviewHandlerReturnsWrappedMetrics
=== RUN   TestAdminListOrdersHandlerBindsQueryFilters
--- PASS: TestAdminListOrdersHandlerBindsQueryFilters
ok  	go-order-lab/internal/handler	0.920s
```

### Build and Deployment

Windows 本机完成：

```bat
go build -o dist\go-order-lab.exe ./cmd/server
set GOPROXY=https://goproxy.cn,direct
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -o dist\go-order-lab-linux-amd64 ./cmd/server
```

部署到 VM 后，Docker 日志确认新路由已注册：

```text
GET    /api/admin/overview       --> go-order-lab/internal/handler.(*AdminHandler).Overview-fm
GET    /api/admin/orders         --> go-order-lab/internal/handler.(*AdminHandler).ListOrders-fm
```

健康检查：

```json
{"status":"ok"}
```

### VM Smoke Test

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\admin-vm-smoke
scripts\smoke.cmd
```

关键结果：

```json
{
  "adminOverview": 200,
  "adminOrders": 200,
  "adminOrderTotal": 1,
  "duplicateOrder": 409,
  "offlineOrder": 409,
  "compensateOrders": 200
}
```

并发压测：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 19
  },
  "multiUserSummary": {
    "202": 3,
    "409": 7
  }
}
```

库存对账：

```json
{
  "reconcile": 200,
  "checked": 1,
  "missing": 0,
  "mismatched": 0,
  "repaired": 0
}
```

## RBAC and Compose Mixed Verification

时间：2026-07-29

### Code Tests

命令：

```bash
cd /home/trrr/go-order-lab
/usr/local/go/bin/go test ./... -count=1
```

结果：

```text
ok  	go-order-lab/internal/handler
ok  	go-order-lab/internal/metrics
ok  	go-order-lab/internal/middleware
ok  	go-order-lab/internal/service
```

说明：Windows 本地 `go test` 仍受测试二进制执行权限影响，报 `fork/exec ... test.exe: Access is denied`；Linux VM 上全量测试通过。

### VM Mixed Deployment

命令：

```bat
D:\Codex\sit-work\scripts\deploy_latest_order_vm_safe.py
```

关键结果：

```json
{"status":"ok"}
```

说明：Go app 以 Docker 容器运行在 VM，MySQL/Redis/RabbitMQ 使用宿主机服务，接口监听 `8090`。

### RBAC Smoke Test

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\order-rbac-vm-smoke
node scripts\smoke-test.js
```

关键结果：

```json
{
  "adminLogin": 200,
  "register": 201,
  "userCreateProduct": 403,
  "product": 201,
  "activity": 201,
  "order": 202,
  "adminOverview": 200,
  "adminOrders": 200,
  "duplicateOrder": 409,
  "offlineOrder": 409,
  "compensateOrders": 200
}
```

### Regression Scripts

库存对账：

```json
{
  "adminLogin": 200,
  "reconcile": 200,
  "checked": 1,
  "missing": 0,
  "mismatched": 0,
  "repaired": 0
}
```

Metrics：

```json
{
  "health": 200,
  "adminLogin": 200,
  "order": 202,
  "metrics": 200
}
```

并发压测：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 19
  },
  "multiUserSummary": {
    "202": 3,
    "409": 7
  }
}
```

### Docker Compose

全容器 Compose：

```bash
ORDER_APP_PORT=18090 ORDER_MYSQL_PORT=13306 ORDER_REDIS_PORT=16379 ORDER_RABBITMQ_PORT=15673 ORDER_RABBITMQ_MANAGEMENT_PORT=15674 sudo docker-compose -p go-order-compose up -d --build
```

结果：配置文件可解析，但 VM 中卡在 `mysql:8.4` 镜像拉取阶段，未声称全容器链路跑通。

VM mixed Compose：

```bash
cd /home/trrr/go-order-lab
ORDER_APP_PORT=18090 sudo docker-compose -f docker-compose.mixed.yml up -d --build
curl -fsS http://127.0.0.1:18090/health
```

结果：

```json
{"status":"ok"}
```

Compose mixed smoke test：

```bat
set ORDER_BASE_URL=http://192.168.220.138:18090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\order-rbac-compose-mixed-smoke
node scripts\smoke-test.js
```

关键结果：

```json
{
  "adminLogin": 200,
  "userCreateProduct": 403,
  "order": 202,
  "adminOverview": 200,
  "duplicateOrder": 409,
  "offlineOrder": 409,
  "compensateOrders": 200
}
```

## 100/200 Concurrent Pressure Verification

时间：2026-07-29

### Script Parameterization

新增压测参数：

```text
ORDER_PRESSURE_SAME_USER_CONCURRENCY
ORDER_PRESSURE_SAME_USER_STOCK
ORDER_PRESSURE_MULTI_USERS
ORDER_PRESSURE_MULTI_STOCK
ORDER_PRESSURE_REGISTER_CONCURRENCY
```

验证命令：

```bat
node scripts\pressure-config-test.js
node --check scripts\pressure-order.js
```

结果：通过。

### First 200-User Run

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:18090
set ORDER_PRESSURE_OUT=D:\Codex\sit-work\reports\pressure-200-vm
set ORDER_PRESSURE_SAME_USER_CONCURRENCY=100
set ORDER_PRESSURE_SAME_USER_STOCK=20
set ORDER_PRESSURE_MULTI_USERS=200
set ORDER_PRESSURE_MULTI_STOCK=30
set ORDER_PRESSURE_REGISTER_CONCURRENCY=20
node scripts\pressure-order.js
```

结果：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 99
  },
  "multiUserSummary": {
    "202": 30,
    "400": 8,
    "404": 51,
    "409": 111
  }
}
```

分析：库存一致性没有被击穿，成功数仍然等于库存数 30；但高并发下出现 MySQL `Too many connections`，说明 Go 服务默认连接池没有限制，瞬时请求会把数据库连接打满。

### Database Pool Fix

新增配置：

```text
ORDER_DB_MAX_OPEN_CONNS=30
ORDER_DB_MAX_IDLE_CONNS=15
ORDER_DB_CONN_MAX_LIFETIME_SECONDS=300
```

验证命令：

```bash
cd /home/trrr/go-order-lab
/usr/local/go/bin/go test ./... -count=1
```

结果：全量 Go 测试通过。

### Final 100/200 Run

命令：

```bat
set ORDER_BASE_URL=http://192.168.220.138:18090
set ORDER_PRESSURE_OUT=D:\Codex\sit-work\reports\pressure-200-vm-pool
set ORDER_PRESSURE_SAME_USER_CONCURRENCY=100
set ORDER_PRESSURE_SAME_USER_STOCK=20
set ORDER_PRESSURE_MULTI_USERS=200
set ORDER_PRESSURE_MULTI_STOCK=30
set ORDER_PRESSURE_REGISTER_CONCURRENCY=20
node scripts\pressure-order.js
```

结果：

```json
{
  "config": {
    "sameUserConcurrency": 100,
    "sameUserStock": 20,
    "multiUsers": 200,
    "multiStock": 30,
    "registerConcurrency": 20
  },
  "sameUserSummary": {
    "202": 1,
    "409": 99
  },
  "multiUserSummary": {
    "202": 30,
    "409": 170
  }
}
```

结论：

- 同一用户 100 并发请求只成功 1 个，重复下单拦截有效。
- 200 个用户抢 30 个库存，成功数正好为 30，未出现超卖。
- 加入数据库连接池限制后，异常的 `Too many connections`、`400`、`404` 响应被收敛，剩余失败请求全部按业务预期返回 `409`。

## Context Timeout and Graceful Shutdown

本轮目标：补全 Go 后端请求生命周期控制和服务停机闭环，使项目在 Docker/VM 部署时能讲清楚请求超时、context 传递和优雅退出。

### Code Changes

新增能力：

```text
ORDER_REQUEST_TIMEOUT_SECONDS=5
ORDER_SHUTDOWN_TIMEOUT_SECONDS=10
```

实现点：

- `middleware.RequestTimeout` 为每个 HTTP 请求设置 context deadline。
- `OrderHandler.CreateOrder` 将 `c.Request.Context()` 传入 `OrderService.CreateOrderContext`。
- 订单核心链路使用 `db.WithContext(ctx)` 执行 GORM 查询和事务，Redis 库存预扣使用请求 context 的子超时。
- `cmd/server/main.go` 从 `router.Run` 改为 `http.Server.ListenAndServe`，收到 `SIGINT/SIGTERM` 后执行 `server.Shutdown`。
- channel worker、RabbitMQ worker 和补偿任务新增 context 版本启动方法，停机时通过 context 退出，并由 `OrderService.Wait()` 等待。

### Local Verification

命令：

```bat
go test ./... -count=1 -timeout=2m
go build ./cmd/server
```

结果：

```text
ok  	go-order-lab/internal/config
ok  	go-order-lab/internal/handler
ok  	go-order-lab/internal/metrics
ok  	go-order-lab/internal/middleware
ok  	go-order-lab/internal/service
```

### VM Go Test

命令：

```bash
cd /home/trrr/go-order-lab
/usr/local/go/bin/go test ./... -count=1 -timeout=2m
```

结果：VM 上 Go 测试通过，覆盖 config、handler、middleware、service 等包。

### VM Native Docker Run

部署方式：交叉编译 Linux 二进制，上传到 VM，通过 `Dockerfile.scratch` 构建镜像并启动 `go-order-lab-app-native`。

健康检查：

```bash
curl -fsS http://127.0.0.1:8090/health
```

结果：

```json
{"status":"ok"}
```

完整 smoke test：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\context-shutdown-vm-smoke
scripts\smoke.cmd
```

关键结果：

```json
{
  "adminLogin": 200,
  "userCreateProduct": 403,
  "order": 202,
  "payment": 200,
  "duplicatePayment": 200,
  "duplicateOrder": 409,
  "offlineOrder": 409,
  "adminOverview": 200,
  "compensateOrders": 200
}
```

100/200 并发压测：

```bat
set ORDER_BASE_URL=http://192.168.220.138:8090
set ORDER_PRESSURE_OUT=D:\Codex\sit-work\reports\context-shutdown-vm-pressure
set ORDER_PRESSURE_SAME_USER_CONCURRENCY=100
set ORDER_PRESSURE_SAME_USER_STOCK=20
set ORDER_PRESSURE_MULTI_USERS=200
set ORDER_PRESSURE_MULTI_STOCK=30
set ORDER_PRESSURE_REGISTER_CONCURRENCY=20
scripts\pressure.cmd
```

结果：

```json
{
  "sameUserSummary": {
    "202": 1,
    "409": 99
  },
  "multiUserSummary": {
    "202": 30,
    "409": 170
  }
}
```

结论：Context 改造后，下单链路仍保持重复下单拦截和库存不超卖。

### Graceful Shutdown Verification

命令：

```bash
docker stop --time 15 go-order-lab-app-native
docker logs --tail 30 go-order-lab-app-native
```

日志关键行：

```text
shutdown signal received: context canceled
background workers stopped
server stopped
```

结论：Docker stop 触发 `SIGTERM` 后，服务没有直接硬退出，而是进入 HTTP shutdown、后台任务退出和数据库关闭流程。随后执行 `docker start go-order-lab-app-native` 并重新请求 `/health`，服务恢复正常。

### VM Mixed Compose Run

命令：

```bash
cd /home/trrr/go-order-lab
ORDER_APP_PORT=18090 sudo docker-compose -f docker-compose.mixed.yml up -d --build --force-recreate
curl -fsS http://127.0.0.1:18090/health
```

说明：第一次 recreate 时，VM 上旧版 `docker-compose 1.29.2` 对 scratch 镜像触发 `KeyError: 'ContainerConfig'`，清理旧 app 容器后重新 create 成功：

```bash
ORDER_APP_PORT=18090 sudo docker-compose -f docker-compose.mixed.yml rm -fsv app
ORDER_APP_PORT=18090 sudo docker-compose -f docker-compose.mixed.yml up -d --build --force-recreate
```

结果：

```json
{"status":"ok"}
```

Compose mixed smoke test：

```bat
set ORDER_BASE_URL=http://192.168.220.138:18090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\context-shutdown-compose-smoke
scripts\smoke.cmd
```

关键结果：

```json
{
  "adminLogin": 200,
  "userCreateProduct": 403,
  "order": 202,
  "payment": 200,
  "duplicateOrder": 409,
  "offlineOrder": 409,
  "adminOverview": 200,
  "compensateOrders": 200
}
```

## Docker Compose v2 Upgrade

本轮目标：解决 VM 上旧版 `docker-compose 1.29.2` recreate scratch 镜像时的 `KeyError: 'ContainerConfig'` 兼容问题，升级为 Docker Compose v2。

### Install

apt 源中未找到 `docker-compose-plugin`，因此采用 Docker CLI plugin 方式安装 Compose v2：

```bash
mkdir -p /tmp/docker-compose-install
curl -fL https://github.com/docker/compose/releases/download/v2.40.2/docker-compose-linux-x86_64 -o /tmp/docker-compose-install/docker-compose
chmod +x /tmp/docker-compose-install/docker-compose
sudo mkdir -p /usr/local/lib/docker/cli-plugins
sudo cp /tmp/docker-compose-install/docker-compose /usr/local/lib/docker/cli-plugins/docker-compose
sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
docker compose version
```

结果：

```text
Docker Compose version v2.40.2
```

### Mixed Compose Verification

命令：

```bash
cd /home/trrr/go-order-lab
ORDER_APP_PORT=18090 sudo docker compose -f docker-compose.mixed.yml up -d --build --force-recreate
curl -fsS http://127.0.0.1:18090/health
```

结果：

```json
{"status":"ok"}
```

完整 smoke test：

```bat
set ORDER_BASE_URL=http://192.168.220.138:18090
set ORDER_SMOKE_OUT=D:\Codex\sit-work\reports\compose-v2-smoke
scripts\smoke.cmd
```

关键结果：

```json
{
  "adminLogin": 200,
  "userCreateProduct": 403,
  "order": 202,
  "payment": 200,
  "duplicatePayment": 200,
  "duplicateOrder": 409,
  "offlineOrder": 409,
  "adminOverview": 200,
  "compensateOrders": 200
}
```

## CI, HTTP Integration Test and Full Compose Attempt

时间：2026-07-29

本轮目标：

- 确认仓库是否已有自动 `go test ./...`。
- 补强 GitHub Actions CI。
- 增加用户侧 HTTP 集成测试。
- 继续尝试 app + MySQL + Redis + RabbitMQ 全容器 Compose。

### CI

仓库已存在 `.github/workflows/ci.yml`。本轮增强为：

```yaml
- name: Format check
  run: test -z "$(gofmt -l .)"

- name: Test
  run: go test ./... -count=1 -timeout=2m
```

本地脚本 `scripts/test.cmd` 同步改为：

```bat
go test ./... -count=1 -timeout=2m
```

### HTTP Integration Test

新增：

```text
internal/handler/order_flow_integration_test.go
```

覆盖链路：

```text
POST /api/auth/register
  -> POST /api/orders
  -> 重复 POST /api/orders 返回 409
  -> worker 异步推进 QUEUED 到 WAIT_PAY
  -> POST /api/payments/callback 更新 PAID
  -> 重复支付回调返回 already_processed=true
```

VM Linux 全量测试：

```bash
cd ~/go-order-lab
/usr/local/go/bin/go test ./... -count=1 -timeout=2m
```

结果：

```text
?   	go-order-lab/cmd/server	[no test files]
ok  	go-order-lab/internal/config
ok  	go-order-lab/internal/handler
ok  	go-order-lab/internal/metrics
ok  	go-order-lab/internal/middleware
?   	go-order-lab/internal/model	[no test files]
?   	go-order-lab/internal/response	[no test files]
ok  	go-order-lab/internal/service
```

说明：Windows 本机仍出现测试二进制 `Access is denied` 的历史问题，因此本轮可信测试结果以 Linux VM 和 GitHub Actions CI 配置为准。

### Full Compose Attempt

默认 `docker-compose.yml` 源码构建路线：

```bash
ORDER_APP_PORT=28090 \
ORDER_MYSQL_PORT=23306 \
ORDER_REDIS_PORT=26379 \
ORDER_RABBITMQ_PORT=25672 \
ORDER_RABBITMQ_MANAGEMENT_PORT=25673 \
sudo -E docker compose up -d --build
```

结果：命令真实进入 `mysql` 和 `rabbitmq` 镜像拉取阶段，但下载长时间无进展，手动中断，退出码 `130`。没有声称该路线跑通。

新增 `docker-compose.full-runtime.yml`：

- app 使用 `Dockerfile.scratch` 和 `dist/go-order-lab-linux-amd64` 构建镜像。
- MySQL、Redis、RabbitMQ 仍然作为容器启动。
- 顶层项目名为 `go-order-lab-full`，避免和 mixed compose 容器混淆。

验证命令：

```bash
ORDER_APP_PORT=28090 \
ORDER_MYSQL_PORT=23306 \
ORDER_REDIS_PORT=26379 \
ORDER_RABBITMQ_PORT=25672 \
ORDER_RABBITMQ_MANAGEMENT_PORT=25673 \
sudo -E timeout 180s docker compose -f docker-compose.full-runtime.yml up -d --build
```

结果：`dist/go-order-lab-linux-amd64` 已存在，但 180 秒内仍停在 `mysql` / `rabbitmq` 镜像拉取阶段，退出码 `124`。

当前结论：

- 全容器 Compose 配置已补齐。
- 这台 VM 当前仍受 Docker Hub 镜像拉取影响，尚未完成 app/MySQL/Redis/RabbitMQ 全容器实跑。
- 已验证且推荐保留的部署路线仍是 Docker Compose v2 mixed：Go app 容器 + 宿主机 MySQL/Redis/RabbitMQ，监听 `18090`。
