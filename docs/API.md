# API

## Base URL

本地默认：

```text
http://127.0.0.1:8090
```

VM 混合部署示例：

```text
http://192.168.220.138:8090
```

统一响应格式：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

所有接口都会返回 `X-Trace-ID` 响应头。客户端也可以主动传入 `X-Trace-ID`，服务端会沿用该值，便于压测或排查时关联一次请求。

健康检查：

| Method | Path | Auth | Body | Success |
| --- | --- | --- | --- | --- |
| GET | `/health` | no | none | `200` |
| GET | `/metrics` | no | none | `200` |

```bat
curl http://127.0.0.1:8090/health
```

Prometheus metrics：

```bat
curl http://127.0.0.1:8090/metrics
```

关键指标：

- `go_order_http_requests_total`：按 method、path、status 统计请求数。
- `go_order_http_request_duration_seconds_bucket/sum/count`：按 method、path、status 统计请求耗时。
- `go_order_business_events_total`：统计下单、支付回调、限流等业务事件。

## Auth

角色说明：

- 普通注册用户默认角色为 `USER`，用于地址、购物车、领券、下单、查询自己的订单。
- 服务启动时会根据 `ORDER_ADMIN_USERNAME` / `ORDER_ADMIN_PASSWORD` 初始化 `ADMIN` 账号。
- 商品、活动、优惠券创建、补偿、库存对账、后台统计和操作日志接口需要 `ADMIN` token。

### Register

| Method | Path | Auth | Success |
| --- | --- | --- | --- |
| POST | `/api/auth/register` | no | `201` |

```json
{
  "username": "demo1001",
  "password": "123456"
}
```

返回的 `data.token` 后续放到请求头：

```text
Authorization: Bearer <token>
```

### Login

| Method | Path | Auth | Success |
| --- | --- | --- | --- |
| POST | `/api/auth/login` | no | `200` |

```json
{
  "username": "demo1001",
  "password": "123456"
}
```

## Catalog and Activity

### Product

| Method | Path | Auth | Body | Success |
| --- | --- | --- | --- | --- |
| GET | `/api/products` | no | none | `200` |
| POST | `/api/products` | admin | product JSON | `201` |

创建商品：

```json
{
  "name": "demo-phone",
  "price": 299900,
  "stock": 20
}
```

### Activity

| Method | Path | Auth | Body | Success |
| --- | --- | --- | --- | --- |
| GET | `/api/activities` | no | none | `200` |
| POST | `/api/activities` | admin | activity JSON | `201` |
| PATCH | `/api/activities/:id/status` | admin | status JSON | `200` |

创建促销活动：

```json
{
  "product_id": 1,
  "name": "summer-sale",
  "price": 269900,
  "stock": 5,
  "duration_seconds": 3600
}
```

活动默认状态为 `PUBLISHED`。手动下线活动：

```json
{
  "status": "OFFLINE"
}
```

活动状态：

- `PUBLISHED`：发布中，且当前时间在活动窗口内才允许下单。
- `OFFLINE`：人工下线，活动下单返回 `409`。
- `ENDED`：活动结束，活动下单返回 `409`。

## Cart and Coupon

### Address

| Method | Path | Auth | Body | Success |
| --- | --- | --- | --- | --- |
| POST | `/api/addresses` | yes | address JSON | `201` |
| GET | `/api/addresses` | yes | none | `200` |
| POST | `/api/addresses/:id/default` | yes | none | `200` |

创建地址：

```json
{
  "receiver": "Tao",
  "phone": "18100000000",
  "province": "Hubei",
  "city": "Wuhan",
  "detail": "demo road",
  "is_default": true
}
```

### Cart

| Method | Path | Auth | Body | Success |
| --- | --- | --- | --- | --- |
| POST | `/api/cart/items` | yes | cart item JSON | `201` |
| GET | `/api/cart/items` | yes | none | `200` |
| PATCH | `/api/cart/items/:id` | yes | quantity JSON | `200` |
| DELETE | `/api/cart/items/:id` | yes | none | `200` |

添加购物车：

```json
{
  "product_id": 1,
  "quantity": 2
}
```

修改数量：

```json
{
  "quantity": 3
}
```

### Coupon

| Method | Path | Auth | Body | Success |
| --- | --- | --- | --- | --- |
| GET | `/api/coupons` | no | none | `200` |
| POST | `/api/coupons` | admin | coupon JSON | `201` |
| POST | `/api/coupons/:id/claim` | yes | none | `200` |
| GET | `/api/my-coupons` | yes | none | `200` |

创建优惠券：

```json
{
  "title": "demo-coupon-50",
  "threshold": 100000,
  "discount": 5000,
  "stock": 100,
  "duration_seconds": 86400
}
```

## Order

### Activity Order

| Method | Path | Auth | Body | Success | Common Conflict |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/orders` | yes | activity order JSON | `202` | `409` |

请求：

```json
{
  "activity_id": 1,
  "request_id": "req-001"
}
```

返回：

```json
{
  "code": 0,
  "message": "accepted",
  "data": {
    "order_no": "ORD...",
    "status": "QUEUED",
    "stock_left": 4
  }
}
```

说明：

- `202` 表示订单已进入异步处理链路。
- worker 随后将状态从 `QUEUED` 推进到 `WAIT_PAY`。
- 活动未发布、已下线、已结束、重复下单或库存不足会返回 `409`。
- 重复活动订单既会被 Redis/service 层拦截，也会被数据库 `activity_order_key` 唯一索引兜底，对外统一返回 `409`。
- 如果服务开启 `ORDER_RATE_LIMIT_ENABLED=true`，突发请求超过令牌桶容量会返回 `429 Too Many Requests`。

### Cart Checkout

| Method | Path | Auth | Body | Success |
| --- | --- | --- | --- | --- |
| POST | `/api/orders/checkout` | yes | checkout JSON | `201` |

```json
{
  "address_id": 1,
  "user_coupon_id": 1
}
```

### Query and State Change

| Method | Path | Auth | Body | Success |
| --- | --- | --- | --- | --- |
| GET | `/api/orders` | yes | none | `200` |
| GET | `/api/orders/:order_no` | yes | none | `200` |
| POST | `/api/orders/:order_no/cancel` | yes | none | `200` |
| POST | `/api/orders/expire` | admin | expire JSON | `200` |
| POST | `/api/ops/compensate` | admin | compensate JSON | `200` |
| POST | `/api/ops/stock/reconcile` | admin | stock reconcile JSON | `200` |

超时关单：

```json
{
  "timeout_seconds": 0
}
```

说明：

- 取消订单只允许取消 `QUEUED` 或 `WAIT_PAY`。
- 超时关单会关闭符合条件的待支付订单，并归还库存。

统一补偿：

```json
{
  "queued_timeout_seconds": 30,
  "pay_timeout_seconds": 900
}
```

返回：

```json
{
  "requeued_orders": 1,
  "closed_orders": 1,
  "ended_activities": 1,
  "failed_count": 0
}
```

说明：

- `QUEUED` 超时订单会重新投递异步任务。
- `WAIT_PAY` 超时订单会关闭并归还库存。
- `PUBLISHED` 且已过 `end_at` 的活动会标记为 `ENDED`。

库存对账：

```json
{
  "activity_id": 1,
  "repair": false
}
```

返回：

```json
{
  "checked": 1,
  "missing": 0,
  "mismatched": 1,
  "repaired": 0,
  "items": [
    {
      "activity_id": 1,
      "activity_name": "summer-sale",
      "status": "PUBLISHED",
      "mysql_stock": 8,
      "redis_stock": 2,
      "redis_exists": true,
      "repaired": false
    }
  ]
}
```

说明：

- `activity_id=0` 时扫描所有活动。
- `repair=false` 只检查不修复。
- `repair=true` 时以 MySQL `activities.stock` 为准修复 Redis `order:activity:<id>:stock`。
- Redis 未开启时返回 `500`，因为该能力依赖 Redis。

## Payment Callback

| Method | Path | Auth | Body | Success | Common Conflict |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/payments/callback` | no | callback JSON | `200` | `409` |

```json
{
  "order_no": "ORD...",
  "transaction_no": "pay-001",
  "status": "SUCCESS"
}
```

说明：

- 首次成功回调会把订单从 `WAIT_PAY` 更新为 `PAID`。
- 重复发送相同 `transaction_no` 时返回 `already_processed=true`。
- 如果订单不是 `WAIT_PAY`，成功支付回调会被状态机拦截。

## Admin

后台接口复用 JWT 鉴权，当前没有单独的管理员角色字段。个人项目阶段先用于运营排障和接口展示，后续可以扩展为 RBAC。

| Method | Path | Auth | Body | Success |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/overview` | admin | none | `200` |
| GET | `/api/admin/orders` | admin | query string | `200` |

后台概览返回订单、库存、GMV 和异常日志汇总：

```json
{
  "users": 10,
  "products": 5,
  "activities": 3,
  "total_orders": 20,
  "queued_orders": 1,
  "wait_pay_orders": 2,
  "paid_orders": 12,
  "cancelled_orders": 1,
  "closed_orders": 4,
  "paid_gmv": 269900,
  "product_stock": 100,
  "activity_stock": 8,
  "failed_logs": 1,
  "recent_failures": []
}
```

后台订单查询支持以下 query 参数：

| Query | Meaning |
| --- | --- |
| `status` | 订单状态，如 `WAIT_PAY`、`PAID`、`CLOSED` |
| `user_id` | 指定用户 ID |
| `activity_id` | 指定活动 ID |
| `limit` | 返回数量，默认 20，最大 100 |

示例：

```bat
curl "http://127.0.0.1:8090/api/admin/orders?status=WAIT_PAY&activity_id=1&limit=5" ^
  -H "Authorization: Bearer <token>"
```

## Operation Logs

| Method | Path | Auth | Body | Success |
| --- | --- | --- | --- | --- |
| GET | `/api/order-logs` | admin | none | `200` |

常见日志动作：

- `stock_reserved`
- `stock_reserved_redis`
- `order_task_published`
- `order_created_async`
- `payment_callback`
- `create_order_rejected`
- `cancel_order`
- `expire_order`
- `compensate_requeue_order`
- `compensate_close_order`
- `compensate_end_activity`
- `stock_reconcile_missing`
- `stock_reconcile_mismatch`
- `stock_reconcile_repair`

## Smoke Test Coverage

`scripts/smoke-test.js` 覆盖主链路：

1. 登录管理员并注册普通用户，验证普通用户创建商品返回 `403`。
2. 管理员创建商品、活动和优惠券。
3. 普通用户创建地址、领券、活动下单，验证异步订单返回 `202`。
4. 普通用户查询自己的订单列表。
5. 管理员查询后台概览和后台订单列表。
6. 支付回调，验证重复回调幂等。
7. 普通用户重复下单，验证 `409`。
8. 普通用户购物车结算。
9. 管理员下线活动，普通用户下单被拦截。
10. 管理员触发统一补偿关闭超时待支付订单。
11. 管理员查询操作日志。

`scripts/pressure-order.js` 覆盖并发链路：

1. 同一用户 20 个并发请求，只允许 1 个成功。
2. 多用户抢有限库存，成功数不超过库存数。

压测规模可通过环境变量调整：

```bat
set ORDER_PRESSURE_SAME_USER_CONCURRENCY=100
set ORDER_PRESSURE_SAME_USER_STOCK=20
set ORDER_PRESSURE_MULTI_USERS=200
set ORDER_PRESSURE_MULTI_STOCK=30
set ORDER_PRESSURE_REGISTER_CONCURRENCY=20
scripts\pressure.cmd
```

`scripts/rate-limit-test.js` 覆盖入口限流：

1. 需要服务启动时开启 `ORDER_RATE_LIMIT_ENABLED=true`。
2. 短时间并发请求 `POST /api/orders`。
3. 期望至少出现一个 `429`。

`scripts/stock-reconcile-test.js` 覆盖库存预热和对账：

1. 创建商品和活动。
2. 调用 `/api/ops/stock/reconcile` 检查新活动。
3. 期望 `checked=1`、`missing=0`、`mismatched=0`。

`scripts/metrics-test.js` 覆盖可观测性指标：

1. 调用 `/health` 产生基础 HTTP 指标。
2. 注册用户、创建商品和活动、发起活动下单。
3. 调用 `/metrics`，检查 HTTP 请求指标和 `activity_order=accepted` 业务指标。
