# Architecture

本文档按“面试能讲清楚、学习能顺着读代码”的方式整理项目结构。项目当前是一个单体 Go 后端服务，不是微服务；中间件版已经接入 MySQL、Redis 和 RabbitMQ。

## Module Map

```text
cmd/server/main.go       组装配置、数据库、中间件、service、handler 和路由
internal/config          读取 ORDER_* 环境变量
internal/database        建立 GORM 连接并执行 AutoMigrate
internal/model           数据表模型：用户、商品、活动、订单、支付、日志等
internal/middleware      JWT 鉴权、Context Timeout、Trace ID、访问日志、metrics 和令牌桶限流
internal/metrics         Prometheus 文本指标：HTTP 请求、耗时和业务事件
internal/handler         Gin HTTP 层：参数绑定、状态码、统一响应
internal/service         业务层：鉴权、商品、购物车、优惠券、订单、后台概览、库存预热/对账、消息队列
scripts                  smoke test、pressure test 和 Windows 启动脚本
deploy/sql               MySQL 初始化说明和演示数据
```

核心依赖关系：

```text
HTTP request
  -> Gin route
  -> handler
  -> service
  -> GORM / Redis / RabbitMQ
  -> response.JSON
```

## Request Flow

输入：客户端发送 HTTP 请求，常见格式是 JSON body 加 `Authorization: Bearer <token>`。

关键模块：

1. `cmd/server/main.go` 注册公开路由和受保护路由。
2. `middleware.Auth` 校验 JWT，并把 `user_id` 和 `user_role` 写入 Gin context。
3. `middleware.TraceID` 生成或透传 `X-Trace-ID`，`AccessLog` 记录请求摘要，`Metrics` 记录请求量和耗时。
4. `middleware.RequestTimeout` 为请求 context 设置统一 deadline，订单核心链路使用 `db.WithContext` 感知取消。
5. 如果开启限流，`POST /api/orders` 会先经过令牌桶中间件。
6. `handler` 使用 `ShouldBindJSON` 解析参数，调用对应 service。
7. `service` 执行业务规则、事务、缓存和消息队列操作。
8. `response` 统一返回 `code/message/data`。

输出：成功时返回 `200/201/202`，业务冲突返回 `409`，鉴权失败返回 `401`，参数错误返回 `400`。

## Trace and Rate Limit Flow

输入：任意 HTTP 请求。

关键模块：

- `middleware.TraceID`
- `middleware.AccessLog`
- `middleware.TokenBucketLimiter`
- `cmd/server/main.go`

流程：

1. 请求进入 Gin 后，`TraceID` 优先读取请求头 `X-Trace-ID`。
2. 如果客户端没有传入 trace id，则服务端生成一个 `trace_<timestamp>_<random>`。
3. trace id 写入 Gin context，并通过响应头 `X-Trace-ID` 返回给客户端。
4. `AccessLog` 在请求结束后记录 trace id、方法、路径、状态码、耗时和客户端 IP。
5. 当 `ORDER_RATE_LIMIT_ENABLED=true` 时，`POST /api/orders` 会经过单进程令牌桶限流器。
6. 令牌桶按 `ORDER_RATE_LIMIT_RPS` 补充令牌，最多保留 `ORDER_RATE_LIMIT_BURST` 个令牌。
7. 没有令牌时直接返回 `429 Too Many Requests`，避免突发流量继续进入库存和数据库链路。

说明：当前限流器是单进程内存实现，适合个人项目和单实例部署演示；多实例部署时应升级为 Redis、网关或服务治理层限流。

## Timeout and Shutdown Flow

输入：

- 任意 HTTP 请求
- Docker stop、Ctrl+C 或宿主机发送的 `SIGTERM/SIGINT`

关键模块：

- `middleware.RequestTimeout`
- `cmd/server/main.go`
- `OrderService.StartWorkersContext`
- `OrderService.StartCompensationWorkerContext`

流程：

1. 服务启动时读取 `ORDER_REQUEST_TIMEOUT_SECONDS` 和 `ORDER_SHUTDOWN_TIMEOUT_SECONDS`。
2. HTTP 请求进入后，`RequestTimeout` 基于原请求创建带 deadline 的 context。
3. 下单 handler 将 `c.Request.Context()` 传入 `OrderService.CreateOrderContext`。
4. 订单核心链路使用 `db.WithContext(ctx)` 执行 GORM 查询和事务，Redis 预扣使用该请求 context 的子超时。
5. 如果请求超时且下游没有写响应，中间件返回 `504 Gateway Timeout`；如果业务层感知到超时，也会映射为 `504`。
6. 服务收到停机信号后，先调用 `http.Server.Shutdown` 停止接收新请求并等待存量请求。
7. HTTP 入口关闭后，取消 worker context，channel/RabbitMQ worker 和补偿任务收到信号后退出。
8. 后台任务退出或达到停机超时后，关闭数据库连接并打印停机日志。

说明：这个设计重点解决“慢请求不要无限占用资源”和“容器重启时不要直接硬杀进程”。当前项目还没有做完整 outbox 表，因此极端断电场景仍依赖补偿任务兜底。

## Observability Metrics Flow

输入：

- 任意 HTTP 请求
- `GET /metrics`
- 下单、支付回调和限流拦截等关键业务事件

关键模块：

- `middleware.Metrics`
- `internal/metrics`
- `OrderHandler.CreateOrder`
- `OrderHandler.PaymentCallback`
- `TokenBucketLimiter`

流程：

1. 请求进入 Gin 后，`Metrics` 中间件记录开始时间。
2. handler 执行结束后，中间件使用 `c.FullPath()` 记录路由模板、HTTP method、status 和耗时。
3. 下单成功时记录 `go_order_business_events_total{event="activity_order",result="accepted"}`。
4. 下单失败时按错误类型记录 `duplicate_order`、`sold_out`、`activity_not_available` 等稳定标签。
5. 限流拦截时记录 `go_order_business_events_total{event="rate_limit",result="blocked"}`。
6. `GET /metrics` 将内存计数器渲染为 Prometheus text exposition format，便于 Prometheus Server 抓取。

说明：指标标签使用路由模板而不是原始 URL，避免把 `/api/orders/:order_no` 这种动态值展开成大量标签。当前项目先暴露 metrics endpoint，后续再接 Prometheus Server 和 Grafana 面板。

## Login and JWT Flow

输入：

- `POST /api/auth/register`
- `POST /api/auth/login`

关键模块：

- `AuthHandler.Register/Login`
- `AuthService`
- `model.User`
- `middleware.Auth`

存储触达：

- MySQL/SQLite 的 `users` 表。

流程：

1. 注册时检查用户名是否存在，写入用户记录和密码哈希。
2. 登录时校验用户名和密码。
3. 普通注册用户默认角色为 `USER`；服务启动时通过 `ORDER_ADMIN_USERNAME` / `ORDER_ADMIN_PASSWORD` 初始化 `ADMIN` 账号。
4. 成功后用 `ORDER_JWT_SECRET` 签发 JWT，token 中携带 `user_id` 和 `role`。
5. 后续受保护接口通过 `Authorization: Bearer <token>` 传入 token。
6. 鉴权中间件解析 token，把用户 ID 和角色写入请求上下文；后台、运维类接口再经过 `RequireRole(ADMIN)`。

输出：

- 注册成功：`201 Created`，返回 token。
- 登录成功：`200 OK`，返回 token。
- 鉴权失败：`401 Unauthorized`。

## Activity Order Flow

输入：

- `POST /api/activities` 创建活动。
- `POST /api/orders` 对活动下单。

关键模块：

- `CatalogHandler.CreateActivity`
- `OrderHandler.CreateOrder`
- `OrderService.CreateOrder`
- `OrderService.createOrderWithDB`
- `OrderService.createOrderWithRedis`

存储触达：

- `activities`
- `orders`
- `operation_logs`
- Redis 活动库存 key 和已购买用户 set，开启 Redis 时使用。
- RabbitMQ `order.created` 队列，开启 RabbitMQ 时使用。

流程：

1. 创建活动时写入活动价格、库存、状态、开始时间和结束时间，默认状态为 `PUBLISHED`。
2. 如果 Redis 已开启，活动创建成功后主动预热 `order:activity:<id>:stock`。
3. 下单时先校验活动存在、状态为 `PUBLISHED` 且处于有效时间。
4. 本地学习模式用数据库事务检查库存和用户是否已买，再扣减库存并创建 `QUEUED` 订单。
5. 中间件模式先用 Redis Lua 原子预扣库存和记录用户购买标记。
6. Redis 预扣成功后，再用数据库事务创建订单，并对数据库库存做兜底扣减。
7. 活动订单会写入 `activity_order_key=<user_id>:<activity_id>`，数据库唯一索引作为一人一单的最终兜底。
8. 订单创建成功后投递异步任务：RabbitMQ 可用时发布消息，不可用时回退到 Go channel。
9. worker 消费任务，把订单状态从 `QUEUED` 推进到 `WAIT_PAY`。

输出：

- 接口立即返回 `202 Accepted`，包含 `order_no/status/stock_left`。
- 活动下线、活动结束、重复购买或库存不足返回 `409 Conflict`。

## Database Constraint Flow

关键约束：

- `orders.order_no` 唯一，防止订单号重复。
- `payments.transaction_no` 唯一，防止支付平台重复回调生成多条支付流水。
- `orders.activity_order_key` 唯一，活动订单使用 `<user_id>:<activity_id>`，普通购物车订单保持 `NULL`。

设计原因：

1. Redis Lua 和 service 层查询可以拦截绝大多数重复下单，但数据库仍然需要最终兜底。
2. 直接对 `(user_id, activity_id)` 建唯一索引会误伤购物车订单，因为普通购物车订单没有活动 ID，历史上会表现为 `activity_id=0`。
3. 使用 nullable `activity_order_key` 后，只有活动订单参与唯一约束；购物车订单为 `NULL`，可以多次创建。
4. 服务启动时会回填历史活动订单的 `activity_order_key`，保证旧数据也进入约束范围。

异常处理：

- 如果并发竞态导致数据库唯一索引报错，service 会把 `activity_order_key` 冲突映射为 `ErrDuplicateOrder`。
- HTTP 层继续返回 `409 Conflict`，不会把底层 SQL 错误暴露给调用方。

## Activity Status Flow

输入：

- `POST /api/activities`
- `PATCH /api/activities/:id/status`
- `POST /api/ops/compensate`

关键模块：

- `CatalogService.CreateActivity`
- `CatalogService.UpdateActivityStatus`
- `OrderService.Compensate`
- `OrderService.ensureActivityOrderable`

状态：

```text
PUBLISHED -> OFFLINE
PUBLISHED -> ENDED
OFFLINE   -> PUBLISHED
ENDED     -> PUBLISHED
```

说明：

1. 活动创建后默认 `PUBLISHED`，但仍需要当前时间落在 `start_at/end_at` 内才能下单。
2. 人工下线使用 `OFFLINE`，用于运营临时止损或关闭异常活动。
3. 补偿任务扫描 `end_at <= now` 的 `PUBLISHED` 活动并标记为 `ENDED`。
4. 下单入口统一调用活动可用性校验，避免绕过状态判断。

## Redis Stock Guard Flow

输入：活动 ID、活动当前库存、用户 ID、活动结束时间。

关键模块：

- `RedisStockStore.Reserve`
- `reserveStockScript`
- `RedisStockStore.Release`
- `releaseStockScript`

存储触达：

- `order:activity:<id>:stock`
- `order:activity:<id>:users`

流程：

1. 活动创建成功后主动调用 `RedisStockStore.Prewarm` 写入 Redis 库存。
2. 如果 Redis 中没有库存 key，Lua 脚本仍会用数据库活动库存做兜底初始化。
3. Lua 脚本检查用户是否已在购买 set 中，已存在则返回重复下单。
4. Lua 脚本检查库存是否大于 0，不足则返回售罄。
5. 库存充足时执行 `DECR`，并把用户 ID 写入 set。
6. 设置 TTL，避免活动结束后热点 key 长期存在。
7. 如果后续数据库事务失败，调用 `Release` 释放用户标记并回补 Redis 库存。

输出：

- 成功：返回扣减后的剩余库存。
- 失败：返回重复下单或售罄错误。

## Stock Reconcile Flow

输入：

- `POST /api/ops/stock/reconcile`

关键模块：

- `OrderHandler.ReconcileStock`
- `OrderService.ReconcileStock`
- `RedisStockStore.GetStock`
- `RedisStockStore.SetStock`

存储触达：

- MySQL `activities`
- Redis `order:activity:<id>:stock`
- `operation_logs`

流程：

1. 读取指定活动或全部活动的 MySQL 库存。
2. 对每个活动读取 Redis 库存 key。
3. Redis key 不存在时计入 `missing`。
4. Redis 库存值与 MySQL 库存不同则计入 `mismatched`。
5. 当 `repair=false` 时只返回差异并写入操作日志。
6. 当 `repair=true` 时，以 MySQL `activities.stock` 为准覆盖 Redis 库存 key，并设置活动 TTL。
7. 修复动作写入 `operation_logs`。

输出：

- `checked`：检查活动数。
- `missing`：Redis 库存 key 缺失数。
- `mismatched`：Redis 与 MySQL 库存不一致数。
- `repaired`：已修复数。

说明：MySQL 是事实源，Redis 是高并发入口缓存。当前对账是手动接口，后续可以升级为定时任务和告警。

## RabbitMQ Async Order Flow

输入：已创建的 `order_no`。

关键模块：

- `RabbitQueue.Publish`
- `RabbitQueue.Consume`
- `OrderService.publishOrderTask`
- `OrderService.startRabbitWorkers`
- `OrderService.processOrderCreated`

存储触达：

- RabbitMQ 队列 `order.created`。
- `orders` 表。
- `operation_logs` 表。

流程：

1. 下单事务提交后调用 `publishOrderTask`。
2. RabbitMQ 开启且发布成功时，消息体使用 JSON envelope，包含 `order_no` 和 `retry`。
3. worker 从队列消费消息，通过 `order_no` 重新查询订单。
4. 如果订单仍是 `QUEUED`，将状态更新为 `WAIT_PAY`。
5. 处理成功后 `Ack`。
6. 处理失败时，如果 `retry < ORDER_RABBITMQ_MAX_RETRIES`，重新发布一条 `retry+1` 的消息并 `Ack` 原消息。
7. 超过最大重试次数后，把失败消息写入 `<queue>.dlq` 死信队列并记录操作日志。
8. RabbitMQ 不可用时，项目回退到本地 channel worker，保证学习模式能跑。

输出：

- 正常：订单从 `QUEUED` 变为 `WAIT_PAY`。
- 异常：写入操作日志，RabbitMQ 消息重新入队。

## Payment Callback Idempotency Flow

输入：

- `POST /api/payments/callback`
- body 包含 `order_no`、`transaction_no`、`status`。

关键模块：

- `OrderHandler.PaymentCallback`
- `OrderService.PaymentCallback`
- `model.Payment`
- `model.Order`

存储触达：

- `payments`
- `orders`
- `operation_logs`

流程：

1. 先用 `transaction_no` 查询支付流水。
2. 如果流水已存在，说明支付平台重复回调，直接返回已处理结果。
3. 如果是新流水，查询订单。
4. `SUCCESS` 回调只允许把 `WAIT_PAY` 订单推进到 `PAID`。
5. 非成功回调只记录支付流水，不改变订单状态。
6. 全过程放在数据库事务中，避免订单状态和支付流水不一致。

输出：

- 首次成功回调：`200 OK`，订单状态变为 `PAID`。
- 重复回调：`200 OK`，`already_processed=true`。
- 非法状态流转：`409 Conflict`。

## Cancel and Expire Order Flow

输入：

- `POST /api/orders/:order_no/cancel`
- `POST /api/orders/expire`

关键模块：

- `OrderService.CancelOrder`
- `OrderService.ExpireOrders`
- `OrderService.restoreStock`

存储触达：

- `orders`
- `activities` 或 `products`
- `order_items`
- Redis 活动库存和用户购买 set，开启 Redis 时使用。
- `operation_logs`

流程：

1. 取消订单时只允许取消 `QUEUED` 或 `WAIT_PAY` 状态。
2. 超时关单会扫描创建时间早于 cutoff 的 `QUEUED/WAIT_PAY` 订单。
3. 活动订单归还活动库存；购物车订单根据订单明细归还商品库存。
4. 如果开启 Redis，活动订单还会释放用户购买标记并回补 Redis 库存。
5. 取消订单状态改为 `CANCELLED`，超时订单状态改为 `CLOSED`。

输出：

- 取消成功：`200 OK`，返回 `returned_stock=true`。
- 超时关单成功：`200 OK`，返回 `expired_count`。
- 已支付订单不能取消，返回 `409 Conflict`。

## Compensation Flow

输入：

- `POST /api/ops/compensate`

关键模块：

- `OrderHandler.Compensate`
- `OrderService.Compensate`
- `OrderService.publishOrderTask`
- `OrderService.restoreStock`

存储触达：

- `orders`
- `activities`
- `products`
- `order_items`
- Redis 活动库存和用户购买 set，开启 Redis 时使用。
- RabbitMQ 或本地 channel。
- `operation_logs`

流程：

1. 扫描 `updated_at` 早于队列超时时间的 `QUEUED` 订单。
2. 更新这些订单的 `updated_at`，然后重新投递异步任务，避免订单长时间停在排队状态。
3. 扫描 `created_at` 早于支付超时时间的 `WAIT_PAY` 订单。
4. 对超时待支付订单执行库存回补，并把状态改为 `CLOSED`。
5. 扫描 `end_at <= now` 且仍是 `PUBLISHED` 的活动，将状态改为 `ENDED`。
6. 每个补偿动作写入 `operation_logs`，便于排查异常。

输出：

- `requeued_orders`：重新投递的排队订单数量。
- `closed_orders`：关闭并归还库存的待支付订单数量。
- `ended_activities`：标记结束的活动数量。
- `failed_count`：补偿失败的记录数量。

## Admin Ops Flow

输入：

- `GET /api/admin/overview`
- `GET /api/admin/orders`

关键模块：

- `AdminHandler`
- `AdminService`
- `model.Order`
- `model.Product`
- `model.Activity`
- `model.OperationLog`

存储触达：

- `orders`
- `products`
- `activities`
- `operation_logs`

流程：

1. 后台接口先走 JWT 保护路由，再通过 `RequireRole(ADMIN)` 判断角色。
2. 普通 `USER` token 可以下单、领券和查询自己的订单，但不能创建商品、活动、优惠券，也不能访问后台概览、补偿、库存对账和操作日志。
3. `AdminHandler` 解析 query 参数，保持 HTTP 层只做参数处理和统一响应。
4. `AdminService.Overview` 聚合用户数、商品数、活动数、订单状态分布、已支付 GMV、库存汇总和失败日志数量。
5. `AdminService.ListOrders` 按状态、用户 ID、活动 ID 组合过滤订单，默认限制 20 条，最大 100 条。
6. 该能力用于运营排障和面试展示，例如快速查看活动是否大量卡在 `WAIT_PAY`、是否出现失败日志、库存汇总是否异常。

说明：当前后台能力是轻量运维视图，不是完整管理后台，但已经具备基础 RBAC 权限边界。

## Current Deployment Modes

当前支持四种运行方式：

1. 本地学习模式：SQLite + Go channel worker，不依赖 Redis/RabbitMQ，适合理解代码和 smoke test。
2. VM mixed Compose：推荐路线，`docker-compose.mixed.yml` 只编排 Go app 容器，监听 `18090`，连接宿主机 MySQL/Redis/RabbitMQ，已验证成功。
3. VM 手动 Docker：备用路线，MySQL/Redis/RabbitMQ 使用 Ubuntu apt 服务，Go 服务通过 `docker run` 运行并监听 `8090`，已验证成功。
4. Docker Compose 全容器模式：`docker-compose.yml` 编排 app、MySQL、Redis、RabbitMQ，端口支持通过环境变量覆盖；本轮 VM 验证卡在 `mysql:8.4` 镜像拉取，配置已通过 `docker compose config` 检查。

已验证的中间件能力：

- MySQL 存储业务表。
- Redis Lua 做活动库存预扣和重复购买拦截。
- MySQL 唯一索引对订单号、支付流水和活动订单做最终一致性兜底。
- RabbitMQ 承接订单异步任务。
- Docker 容器运行 Go 服务。
- 下单接口可通过令牌桶限流保护，访问日志可通过 trace id 定位请求，`/metrics` 可查看请求量、耗时和业务事件。
- 请求入口支持统一超时，服务支持 Docker/系统信号触发的优雅停机。

## Known Gaps

当前阶段是“可运行、可学习、可面试讲清”的单体后端项目，还不是生产级系统。

已知待完善点：

- RabbitMQ 已有最大重试、死信队列投递和补偿任务，但还没有延迟重试和监控面板。
- 请求超时和优雅停机已接入，后续可补 outbox 表和更细粒度的后台任务 drain 策略。
- Redis 库存已在活动创建时预热，后续可把库存对账升级为定时任务和告警。
- 已暴露 Prometheus 文本指标，但尚未接入 Prometheus Server、Grafana 面板和分布式链路追踪。
- 核心补偿、数据库约束、库存对账、RBAC 和后台接口已有 service/handler/HTTP 集成测试，其他模块单元测试和集成测试还需要补。
- 还没有 Kubernetes manifest，也没有拆成商品、订单、支付等微服务。
- 没有真实支付渠道，只实现了支付回调模拟接口。
