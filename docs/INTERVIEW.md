# Interview Notes

## 4-5 行简历版

Go 高并发订单与库存管理系统 | Go、Gin、GORM、MySQL、Redis、RabbitMQ、JWT、Docker  
面向促销活动和商品交易场景，设计用户登录、商品活动、购物车结算、库存预占、异步下单、支付回调和异常补偿等核心后端流程。  
基于 Gin + GORM 实现分层后端服务，下单链路通过 Redis Lua 原子完成库存预占和一人一单校验，再由 MySQL 事务和唯一约束落库 `QUEUED` 订单。  
引入 RabbitMQ 承接订单创建任务，后台 worker 异步推进订单为待支付状态；本地学习模式保留 channel fallback，降低环境依赖。  
实现支付回调幂等、活动/订单状态机、后台概览与跨用户订单查询、补偿任务、Redis 库存预热与对账修复、Context Timeout、Graceful Shutdown、Trace ID、Prometheus metrics、操作日志和下单接口令牌桶限流，并通过 smoke test 与并发压测验证重复下单、活动下线、限流和库存拦截效果。

## STAR 讲法

Situation：促销活动下单场景中，库存扣减、订单创建、支付回调和取消补偿都容易出现并发和状态一致性问题。

Task：我希望实现一个更接近真实后端项目的订单系统，覆盖登录鉴权、商品活动管理、购物车结算、库存预占、异步下单、支付幂等、订单取消、异常补偿、入口限流和请求追踪。

Action：我用 Gin 搭 HTTP 层，用 JWT 中间件保护用户态接口，并基于 `USER/ADMIN` 做 RBAC 权限隔离；用 GORM 建立用户、商品、活动、订单、支付流水和日志表；活动创建后主动预热 Redis 库存；高并发活动下单时先通过 Redis Lua 做库存和一人一单预占，再用 MySQL 事务创建 `QUEUED` 订单，并通过 `activity_order_key` 唯一索引作为数据库兜底；订单创建任务投递到 RabbitMQ，由 worker 异步推进到 `WAIT_PAY`；支付回调用 `transaction_no` 做幂等；补偿任务负责重投卡队列订单、关闭超时待支付订单并结束过期活动；同时提供后台概览/订单筛选、Redis/MySQL 库存对账修复、请求级 Context Timeout、Docker 停机时的 Graceful Shutdown、`X-Trace-ID`、Prometheus metrics 和令牌桶限流。

Result：项目已在 VM 中跑通 MySQL、Redis、RabbitMQ、Dockerized Go app 和 Docker Compose 链路；smoke test 验证普通用户创建商品返回 `403`、活动下线下单返回 `409`、补偿接口可关闭超时待支付订单；并发压测验证同一用户 100 个请求只成功 1 次，200 个用户抢 30 个库存只成功 30 次；库存对账脚本验证 Redis 被篡改后可发现 mismatch 并修复，metrics 脚本验证 `/metrics` 可输出下单请求和业务事件指标，限流开启时突发下单请求可返回 `429`。

## 核心链路

1. `cmd/server/main.go` 组装配置、数据库、Redis、RabbitMQ、service、handler 和中间件。
2. `internal/model` 定义用户、商品、活动、订单、支付流水和日志表。
3. `AuthService` 注册登录并签发 JWT。
4. `OrderService.CreateOrder` 根据配置选择 Redis 或 DB 路径。
5. Redis 路径：Lua 脚本原子判断库存、判断用户是否已买、扣减库存。
6. MySQL 事务：二次校验重复订单，扣减库存镜像，创建带 `activity_order_key` 的 `QUEUED` 订单。
7. RabbitMQ/channel：投递订单任务，worker 异步把 `QUEUED` 推进到 `WAIT_PAY`。
8. `PaymentCallback` 先查 `transaction_no`，重复回调直接返回已处理，首次成功回调把订单改为 `PAID`。
9. `Compensate` 重新投递长时间停在 `QUEUED` 的订单，关闭超时 `WAIT_PAY` 订单并归还库存。
10. 活动支持 `PUBLISHED/OFFLINE/ENDED`，下单入口统一校验活动状态和时间窗口。
11. `TraceID` 中间件为每个请求生成或透传 `X-Trace-ID`，访问日志带上 trace id。
12. `TokenBucketLimiter` 对 `POST /api/orders` 做单进程令牌桶限流，超限返回 `429`。
13. `ReconcileStock` 对比 MySQL 活动库存和 Redis 热点库存，支持 `repair=true` 修复 Redis。
14. `AdminService` 聚合订单状态分布、已支付 GMV、库存汇总和失败日志，并支持后台按状态/用户/活动筛选订单。
15. `Metrics` 中间件记录请求量和耗时，`/metrics` 暴露 Prometheus 文本指标，下单、支付和限流会记录业务事件。
16. `RequestTimeout` 为请求设置统一 deadline，下单核心链路通过 `CreateOrderContext` 和 `db.WithContext` 感知取消。
17. `main.go` 使用 `http.Server.Shutdown` 处理 `SIGINT/SIGTERM`，再通知 channel/RabbitMQ worker 和补偿任务退出。

## 面试官可能追问

Q：为什么 Redis 预占后还要写 MySQL？  
A：Redis 负责高并发入口的快速原子判断，MySQL 负责订单事实记录和最终状态。Redis 扣减成功只能说明“抢到资格”，订单仍然要落到 MySQL，后续支付、取消、日志都基于数据库记录。

Q：为什么 Redis Lua 可以防超卖？  
A：Lua 脚本在 Redis 单线程执行模型下是原子的。我把“判断库存是否大于 0、判断用户是否已买、扣减库存、记录用户”放到一个脚本里，中间不会被其他请求插入，所以不会出现多个请求同时看到同一份库存。

Q：如果 Redis 预占成功，但 MySQL 落单失败怎么办？  
A：代码里在 MySQL 事务失败后会调用 Redis Release，把用户购买标记移除并归还库存。这是补偿逻辑。真实生产还会把失败订单和补偿结果写入告警或定时对账任务。

Q：既然 Redis Lua 已经做了一人一单，为什么还要数据库唯一约束？  
A：Redis 是入口层的高并发拦截，数据库是最终事实源。Redis 挂掉、缓存被清理、代码绕过 Redis，或者极端并发下 service 层查询出现竞态时，数据库唯一约束还能兜住重复活动订单。我没有直接对 `(user_id, activity_id)` 建唯一索引，因为购物车订单没有活动 ID，容易误伤；所以用了 nullable 的 `activity_order_key=<user_id>:<activity_id>`，只约束活动订单。

Q：活动库存为什么要预热到 Redis？  
A：如果等第一个用户下单时再初始化 Redis，热点活动第一波请求会同时打到初始化逻辑，虽然 Lua 能兜底，但会增加入口复杂度。活动创建成功后主动把库存写到 Redis，可以让下单入口直接走 Redis 原子扣减，也更接近真实促销活动上线前预热热点数据的做法。

Q：Redis 库存和 MySQL 库存不一致怎么办？  
A：我把 MySQL 作为事实源，Redis 作为高并发入口缓存。项目里提供了库存对账接口，读取 MySQL `activities.stock` 和 Redis `order:activity:<id>:stock` 比较；发现 key 缺失或数值不一致时记录操作日志。如果传 `repair=true`，会用 MySQL 库存覆盖 Redis，并重新设置 TTL。

Q：为什么还保留 channel fallback？  
A：这是为了让项目在没有 Docker/RabbitMQ 的本地环境也能跑通核心业务。面试时我会说明线上链路用 RabbitMQ，本地学习链路用 channel 模拟异步队列，两者共用同一个 `processOrderCreated` 处理函数。

Q：RabbitMQ 消费失败怎么办？  
A：消息体里有 `retry` 字段。worker 处理失败时会重新发布 `retry+1` 的消息并 `Ack` 原消息，超过 `ORDER_RABBITMQ_MAX_RETRIES` 后写入 `<queue>.dlq` 死信队列，同时记录操作日志，后续可以人工排查或由补偿任务兜底。

Q：为什么要做活动状态，而不是只看开始和结束时间？  
A：时间窗口只能表达自然开始和结束，不能表达运营主动下线、异常止损、风控拦截等场景。我把活动状态抽成 `PUBLISHED/OFFLINE/ENDED`，下单入口统一校验状态和时间，这样业务含义更清楚，也便于补偿任务把过期活动收敛到 `ENDED`。

Q：补偿任务具体补什么？  
A：它处理三类异常：第一，订单长时间停在 `QUEUED`，说明异步任务可能丢了或 worker 失败，会重新投递；第二，订单长时间停在 `WAIT_PAY`，会关闭订单并归还库存；第三，活动已经过了 `end_at` 但状态仍是 `PUBLISHED`，会标记为 `ENDED`。每类动作都会写操作日志，方便追踪。

Q：为什么要做后台概览和后台订单查询？  
A：用户侧订单列表只能看当前用户自己的订单，排障时不够。比如活动下单异常时，我需要知道是否大量订单卡在 `QUEUED/WAIT_PAY`、已支付 GMV 是否正常、失败日志集中在哪类动作、某个活动下有哪些订单。所以我做了轻量后台接口，把运营排障常用指标聚合出来，并支持按状态、用户和活动筛选订单。

Q：为什么要区分普通用户和管理员？
A：因为商品、活动、优惠券、补偿和库存对账都属于运营或后台动作，如果普通用户 token 也能访问，就会出现越权问题。我在 JWT claims 里携带 `role`，注册用户默认是 `USER`，服务启动时初始化 `ADMIN` 账号；后台路由在 `Auth` 之后再加 `RequireRole(ADMIN)`，这样用户侧和管理侧权限边界更清楚。

Q：JWT 里带 role 会不会不安全？
A：JWT 本身由 `ORDER_JWT_SECRET` 签名，客户端不能直接篡改 role，否则签名校验会失败。但生产环境还需要注意 token 过期时间、密钥轮换、管理员账号初始化方式和权限变更后的 token 失效策略。当前项目为了演示，把 role 放进 claims，可以让无状态服务快速做权限判断。

Q：为什么要加 Trace ID？  
A：接口出问题时只看时间和 URL 很难定位一次具体请求，尤其是压测时请求很多。我在入口生成或透传 `X-Trace-ID`，响应头返回同一个值，访问日志也记录它。这样用户拿到异常响应后，可以按 trace id 去日志里找对应请求。

Q：为什么要做 `/metrics`，它解决什么问题？  
A：Trace ID 更适合定位单次请求，metrics 更适合看整体趋势。比如压测时我可以看 `/api/orders` 的请求数、状态码分布和耗时，也能看下单成功、重复下单、限流拦截这类业务事件数量。它不是为了替代日志，而是补充“系统现在整体是否健康”的视角。

Q：metrics 标签为什么用路由模板而不是原始 URL？  
A：如果直接用原始 URL，像 `/api/orders/:order_no` 会变成大量不同标签，Prometheus 里叫高基数问题，会导致内存和查询压力变大。我在中间件里用 Gin 的 `FullPath()` 记录路由模板，把动态参数收敛掉。

Q：你这个算完整监控系统吗？  
A：目前不是完整监控平台，只是服务侧已经暴露 Prometheus 文本指标，并通过脚本验证可抓取。完整方案还需要 Prometheus Server 定时 scrape、Grafana 面板、告警规则，以及结构化日志或链路追踪。

Q：令牌桶限流怎么工作的？  
A：桶里最多存 `Burst` 个令牌，按 `RPS` 速率补充。每个请求进来消耗 1 个令牌；如果没有令牌就返回 `429`，不让请求继续进入库存、事务和消息队列链路。它允许一定突发，但会限制持续高流量。

Q：你这个限流能支持多实例部署吗？  
A：当前是单进程内存令牌桶，适合单实例项目和面试演示。如果部署多个实例，每个实例都有自己的桶，总限流会失准。生产里我会把限流放到网关层，或者用 Redis/Lua 做共享计数，再结合用户维度、IP 维度和接口维度设置不同阈值。

Q：为什么要在 Go 后端里用 context？
A：context 主要负责请求生命周期控制。比如用户请求超时或客户端断开后，后端不应该继续无限占用 goroutine、数据库连接和 Redis 连接。我在入口给请求设置 deadline，handler 把 `c.Request.Context()` 传到订单 service，核心下单链路用 `db.WithContext(ctx)` 执行事务，Redis 预扣也使用这个 context 的子超时。

Q：请求超时后已经创建了一半订单怎么办？
A：数据库事务会保证要么整体提交、要么整体回滚。如果超时发生在事务提交前，GORM 会通过 context 感知取消并返回错误。如果事务已经提交成功，订单事实记录已经存在，后续异步任务和补偿任务会继续推进状态，所以不会单纯依赖客户端请求是否还在。

Q：服务优雅停机怎么做的？
A：入口不是直接 `router.Run`，而是使用标准 `http.Server`。收到 `SIGINT/SIGTERM` 后先调用 `server.Shutdown`，停止接收新请求并等待已有请求在超时时间内结束；然后取消 worker context，让 channel/RabbitMQ worker 和补偿任务退出；最后等待后台任务结束并关闭数据库连接。这样 Docker stop 或服务重启时不会直接硬杀进程。

Q：支付回调为什么不走 JWT？  
A：支付回调通常来自支付平台，不是用户浏览器请求，所以不会使用用户 JWT。真实项目应校验平台签名、时间戳和回调来源；当前项目重点实现业务幂等和订单状态流转。

Q：这个项目离生产还差什么？  
A：还需要更完整的 HTTP 集成测试、分布式限流、延迟重试、Prometheus Server/Grafana 告警、结构化日志和灰度部署。现在版本重点覆盖后端核心链路和可面试的并发一致性设计。
