# Go 高并发交易与库存管理系统

一个用 Go 实现的交易后端训练项目，围绕促销活动、库存预扣、异步下单、支付回调幂等、超时关单和异常补偿展开。

项目当前是单体 Go 后端服务，提供本地学习模式和中间件部署模式：本地可以用 SQLite + Go channel 快速跑通，中间件版已经验证 MySQL、Redis、RabbitMQ 和 Dockerized Go app 的完整链路。

## 技术栈

- 语言与框架：Go、Gin、GORM
- 数据库与缓存：MySQL、SQLite、Redis
- 消息队列：RabbitMQ、Go channel fallback
- 鉴权与接口：JWT、RBAC、RESTful API、统一响应、参数绑定
- 工程化：Docker、Context Timeout、Graceful Shutdown、Trace ID、访问日志、Prometheus metrics、限流中间件、Windows cmd scripts、Node.js smoke/pressure scripts
- 业务能力：注册登录、商品、活动状态、库存预热与对账、购物车、优惠券、订单、支付回调、后台概览、超时关单、异常补偿、操作日志

## 项目亮点

1. 活动库存预扣：Redis Lua 原子完成库存判断、重复购买判断和扣减，数据库唯一约束与事务做最终落库兜底。
2. 异步订单链路：下单接口快速返回 `202`，RabbitMQ 或 channel worker 异步推进订单状态。
3. 支付回调幂等：基于 `transaction_no` 识别重复回调，避免支付平台重试导致订单状态重复更新。
4. 状态机约束：活动支持 `PUBLISHED/OFFLINE/ENDED`，订单只允许在合法状态之间流转。
5. 异常补偿闭环：补偿接口可重投卡在 `QUEUED` 的订单、关闭超时待支付订单并归还库存、结束过期活动。
6. 库存一致性治理：活动创建后主动预热 Redis 库存，并提供 Redis/MySQL 库存对账与修复接口。
7. RBAC 与后台运营视图：普通用户只能访问下单、领券、个人订单等接口，管理员可管理商品、活动、优惠券、补偿、库存对账和后台统计。
8. 入口保护与可观测性：每个请求带 `X-Trace-ID`，请求入口支持 Context Timeout，高频下单接口支持令牌桶限流，`/metrics` 暴露 Prometheus 文本指标。
9. 优雅停机：服务监听 `SIGINT/SIGTERM`，停机时先关闭 HTTP 入口，再通知 channel/RabbitMQ worker 和补偿任务退出。
10. 可验证压测：脚本验证同一用户重复下单拦截、多用户抢有限库存、重复支付回调、库存对账和限流拦截；已完成 100/200 级并发验证。

## 业务流程

```text
注册/登录
  -> 创建商品和促销活动
  -> 活动状态为 PUBLISHED
  -> 活动下单
  -> Redis/DB 预扣库存
  -> 创建 QUEUED 订单
  -> RabbitMQ 或 channel 投递异步任务
  -> worker 推进订单到 WAIT_PAY
  -> 支付回调更新为 PAID
```

异常补偿链路：

```text
POST /api/ops/compensate
  -> 扫描长时间停留 QUEUED 的订单并重新投递任务
  -> 扫描超时 WAIT_PAY 订单并关闭、归还库存
  -> 扫描已到 end_at 的活动并标记 ENDED
  -> 写入 operation_logs
```

购物车链路：

```text
创建地址
  -> 添加购物车
  -> 创建/领取优惠券
  -> 购物车结算
  -> 生成 WAIT_PAY 订单
  -> 支付回调
```

## Highlight 1: Redis Lua 库存预扣

当前完成：

- Redis key `order:activity:<id>:stock` 存储活动剩余库存。
- Redis set `order:activity:<id>:users` 存储已成功预扣库存的用户。
- 活动创建成功后主动写入 Redis 热点库存，减少首次下单时的初始化开销。
- Lua 脚本一次性完成“判断一人一单、判断库存、扣减库存、写入用户标记”。
- 活动订单写入 `activity_order_key=<user_id>:<activity_id>`，并通过唯一索引做数据库兜底，避免绕过缓存或并发竞态导致重复活动订单。
- 数据库事务失败时调用 release 脚本回补 Redis 库存并移除用户标记。
- 运维接口可对比 Redis 库存与 MySQL 活动库存，并在 `repair=true` 时用 MySQL 值修复 Redis。

可继续加深：

- 把库存对账做成定时任务，并增加告警通知。
- 多实例部署时把库存修复和下单链路的并发冲突纳入更严格的分布式控制。

## Highlight 2: RabbitMQ 异步订单链路

当前完成：

- 下单成功后只投递 `order_no`，worker 再查询数据库处理订单状态。
- RabbitMQ 消息使用 JSON envelope 携带 `order_no` 和 `retry`，同时兼容旧的 plain text `order_no` 消息。
- RabbitMQ 消息使用持久化投递，消费端手动 `Ack/Nack`。
- 订单处理失败时按 `ORDER_RABBITMQ_MAX_RETRIES` 重新投递，超过阈值后写入 `<queue>.dlq` 死信队列。
- RabbitMQ 不可用时回退到本地 channel worker，保证本地学习模式可运行。
- 操作日志记录消息发布、异步创建、重试、死信等关键节点。

可继续加深：

- 增加延迟重试，避免失败消息立即重投造成瞬时压力。
- 增加消息去重表，避免极端场景下重复消费造成状态反复处理。

## Highlight 3: Trace ID 和 Prometheus Metrics

当前完成：

- 所有请求都会生成或透传 `X-Trace-ID`，响应头和访问日志使用同一个 trace id。
- Gin 中间件记录 HTTP 请求总数、状态码和耗时直方图，路由维度使用 `FullPath`，避免把订单号、活动 ID 等动态参数打进指标标签。
- `/metrics` 暴露 Prometheus 文本格式指标，包括 `go_order_http_requests_total`、`go_order_http_request_duration_seconds_*`。
- 下单、支付回调和限流拦截记录业务事件计数，例如 `activity_order=accepted`、`rate_limit=blocked`。
- `scripts/metrics.cmd` 会真实走注册、创建活动、下单链路，再检查 `/metrics` 是否出现 HTTP 和业务指标。

可继续加深：

- 接入 Prometheus Server 和 Grafana 面板，把接口 QPS、错误率、P95 耗时可视化。
- 将访问日志改为 JSON 结构化日志，和 trace id、metrics 一起做排障闭环。

## 快速启动

### 本地学习模式

适合先理解 Go、Gin、GORM、事务、channel 和接口流程，不需要先安装 MySQL/Redis/RabbitMQ。

```bat
cd /d path\to\go-order-lab
scripts\run-local.cmd
```

健康检查：

```bat
curl http://127.0.0.1:8090/health
```

### VM 中间件模式（推荐）

已验证方案：

- MySQL/Redis/RabbitMQ 作为 Ubuntu 服务运行。
- Go 服务优先使用 Docker Compose v2 mixed 模式编排，监听 `18090`，并通过 host network 访问本机中间件。
- 手动 `docker run` 方式监听 `8090`，作为备用验证路线保留。
- 配置参考 `.env.vm.example`。
- SQL 初始化参考 `deploy/sql/001_schema_notes.sql`。
- 演示数据参考 `deploy/sql/002_seed_data.sql`。

说明：仓库保留 `docker-compose.yml` 全容器配置；本轮在 VM 上拉取 `mysql:8.4` 镜像时网络长期卡住，因此额外提供并验证了 `docker-compose.mixed.yml`，用 Docker Compose v2 编排 Go app 容器并连接宿主机 MySQL/Redis/RabbitMQ。

### Full Runtime Compose

如果已经有 Linux 二进制 `dist/go-order-lab-linux-amd64`，可以使用 `docker-compose.full-runtime.yml` 一次性拉起 Go app、MySQL、Redis 和 RabbitMQ 四个容器。这个版本不再依赖 `golang:1.26-alpine` 构建镜像，更适合 VM 网络不稳定时验证全容器部署。

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

## 验证命令

Go 编译与测试：

```bat
scripts\test.cmd
```

GitHub Actions 配置了两层验证：

- 基础回归：push 和 pull request 时执行 `gofmt` 检查和 `go test ./... -count=1 -timeout=2m`。
- 真实中间件 smoke：在 GitHub runner 中启动 MySQL、Redis、RabbitMQ，启动 Go 服务后执行 `scripts/smoke-test.js`，覆盖注册登录、商品活动、下单、支付回调、RBAC、购物车、优惠券、补偿等接口链路。

完整接口 smoke test：

```bat
scripts\smoke.cmd
```

重复下单和抢库存并发压测：

```bat
scripts\pressure.cmd
```

限流验证，需要服务启动时设置 `ORDER_RATE_LIMIT_ENABLED=true`：

```bat
scripts\rate-limit.cmd
```

库存预热与对账验证，需要开启 Redis：

```bat
scripts\stock-reconcile.cmd
```

Prometheus metrics 验证：

```bat
scripts\metrics.cmd
```

如果服务跑在 VM：

```bat
set ORDER_BASE_URL=http://192.168.220.138:18090
scripts\smoke.cmd
scripts\pressure.cmd
```

## 当前完成度

已完成：

- Go + Gin + GORM 后端基础结构。
- 注册登录、JWT 鉴权、商品、活动、地址、购物车、优惠券、订单、支付回调、操作日志。
- 订单号、支付流水和活动订单数据库唯一约束，其中活动订单使用 nullable `activity_order_key`，不影响普通购物车订单。
- 后台概览和后台订单查询接口，支持查看 GMV、订单状态分布、库存汇总、失败日志和跨用户订单筛选。
- RBAC 权限隔离，普通用户创建商品返回 `403`，后台、补偿、库存对账和日志接口仅允许管理员访问。
- SQLite 本地学习模式。
- MySQL/Redis/RabbitMQ 中间件模式。
- Dockerized Go app 在 Ubuntu VM 中运行。
- Docker Compose v2 mixed 模式在 Ubuntu VM 中运行，服务监听 `18090` 并通过完整 smoke test。
- GitHub Actions CI 已配置格式检查、全量 Go 测试和真实 MySQL/Redis/RabbitMQ smoke job。
- 用户侧 HTTP 集成测试覆盖注册、活动下单、重复下单、异步订单状态推进、支付回调和重复回调幂等。
- smoke test 和 pressure test 脚本。
- RabbitMQ 消息 retry envelope、最大重试和死信队列投递逻辑。
- 活动状态管理和补偿接口，覆盖卡队列重投、超时待支付关单、过期活动结束。
- Trace ID、访问日志、Prometheus metrics 和下单接口令牌桶限流。
- 请求级 Context Timeout、订单核心链路 `db.WithContext` 和 Docker 停机时的 Graceful Shutdown。
- Redis 库存预热、库存读取、Redis/MySQL 对账和修复接口。
- API、架构、运行报告和面试学习文档。

未完成或待增强：

- 尚未做 K8S 部署。
- 尚未拆成微服务。
- 尚未接真实支付渠道。
- RabbitMQ 还没有延迟重试和可视化监控面板。
- 当前限流器是单进程内存实现，尚不是 Redis 或网关级分布式限流。
- 库存对账当前是手动接口，尚未做自动定时调度和告警。
- 测试以脚本验证、service 单元测试、RBAC/后台 HTTP 集成测试、用户侧订单 HTTP 流程测试和 CI 真实中间件 smoke 为主，购物车/优惠券等 Go 侧集成测试还可以继续补。
- 已暴露 Prometheus 文本指标，但尚未接入 Prometheus Server 和 Grafana 面板。

## 文档

- `docs/API.md`：接口说明和可复制请求流。
- `docs/ARCHITECTURE.md`：模块边界、请求流、库存流、消息流和部署模式。
- `docs/RUN_REPORT.md`：本地和 VM 的真实运行记录。
- `docs/INTERVIEW.md`：面试讲解和 Q&A。
- `docs/DOCKER.md`：Docker/VM 部署记录。

## 后续规划

优先级从高到低：

1. 补订单核心 service 单元测试和更多 HTTP 集成测试。
2. 把库存对账升级为可配置定时任务。
3. 增加 RabbitMQ 延迟重试和消息去重表。
4. 将访问日志升级为结构化日志。
5. 接入 Prometheus Server 和 Grafana 面板。
6. 视时间再补 Docker Compose 全容器跑通和 K8S manifest。
