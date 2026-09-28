# Go Order Lab

Go 并发测试与性能报告：见 [压测说明](docs/PRESSURE.md)。运行 `go run ./cmd/pressure`，使用固定请求 worker，统计 QPS、P50/P95/P99、错误分类，并核对订单与 Redis/MySQL 库存。云端可使用 `bash scripts/pressure-cloud.sh` 采集机器配置、容器资源和消息积压。

实测环境、结果和限制见 [2026-09-28 云端压测报告](docs/benchmarks/2026-09-28/REPORT.md)，包含原始 JSON 数据。

[![CI](https://github.com/mahamharis60-code/Go-order-lab/actions/workflows/ci.yml/badge.svg)](https://github.com/mahamharis60-code/Go-order-lab/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Docker Compose](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](https://docs.docker.com/compose/)

面向促销交易场景的 Go 后端工程实践项目。系统以订单和库存一致性为核心，覆盖活动库存预占、异步订单处理、支付回调幂等、超时关单、异常补偿、权限控制、可观测性和容器化部署。

## 核心能力

- **并发库存控制**：Redis Lua 原子完成库存判断、扣减和同一用户重复购买校验，MySQL 条件更新与唯一约束负责最终落库兜底。
- **异步订单处理**：订单以 `QUEUED` 状态落库并投递 RabbitMQ，由 worker 推进到 `WAIT_PAY`；支持手动 Ack/Nack、有限重试和死信队列。
- **状态与补偿**：订单状态机覆盖 `QUEUED / WAIT_PAY / PAID / CANCELLED / CLOSED`，补偿任务负责卡队列重投、超时关单、库存归还和过期活动结束。
- **支付幂等**：使用支付流水号唯一约束识别重复回调，避免同一回调重复推进订单状态。
- **权限与运营接口**：JWT 身份认证与 `USER / ADMIN` RBAC，提供商品、活动、优惠券、库存对账、订单查询和后台概览接口。
- **工程化运行**：Docker Compose 编排 Go 服务、MySQL、Redis 和 RabbitMQ；支持优雅停机、请求超时、Trace ID、令牌桶限流和 Prometheus 格式指标。

## 系统架构

```text
HTTP Client
    |
    v
Gin Router
    |-- Trace / Metrics / Timeout / JWT / RBAC / Rate Limit
    |
    v
Handler -> Service -> GORM -> MySQL
                      |
                      +-> Redis Lua stock reservation
                      |
                      +-> RabbitMQ -> workers -> order state transition
```

## 核心链路

```text
注册或登录
  -> 管理员创建商品与促销活动
  -> 活动库存预热到 Redis
  -> 用户提交活动订单
  -> Redis Lua 原子预占库存并记录用户
  -> MySQL 事务扣减库存并创建 QUEUED 订单
  -> RabbitMQ 投递订单任务
  -> worker 推进订单到 WAIT_PAY
  -> 支付回调幂等推进到 PAID
```

异常恢复链路：

```text
QUEUED 长时间未处理 -> 重新投递订单任务
WAIT_PAY 超时       -> 关闭订单并归还库存
活动超过结束时间     -> 更新为 ENDED
Redis/MySQL 不一致  -> 库存对账并按 MySQL 修复缓存
```

## 技术栈

| 分类 | 技术 |
| --- | --- |
| Web | Go、Gin、RESTful API |
| 数据访问 | GORM、MySQL |
| 缓存与并发控制 | Redis、Lua |
| 异步任务 | RabbitMQ、goroutine、channel |
| 安全 | JWT、RBAC、bcrypt |
| 工程化 | Docker、Docker Compose、GitHub Actions |
| 可观测性 | Trace ID、访问日志、Prometheus text metrics |

## 项目结构

```text
.
|-- cmd/server/              # 服务启动、依赖装配与优雅停机
|-- internal/
|   |-- handler/             # HTTP 参数解析与响应
|   |-- service/             # 订单、库存、支付和补偿逻辑
|   |-- middleware/          # JWT、RBAC、限流、超时、Trace
|   |-- model/               # GORM 数据模型与状态定义
|   |-- database/            # 数据库连接和迁移
|   `-- metrics/             # Prometheus 格式指标
|-- scripts/                 # 核心链路 smoke 与并发验证
|-- docs/                    # API 和架构文档
|-- Dockerfile
`-- docker-compose.yml
```

## Docker 快速启动

环境要求：Docker Engine 24+ 与 Docker Compose v2。

```bash
git clone https://github.com/mahamharis60-code/Go-order-lab.git
cd Go-order-lab
cp .env.example .env
docker compose up -d --build
```

查看容器状态并验证服务：

```bash
docker compose ps
curl http://127.0.0.1:8090/health
```

预期响应：

```json
{"status":"ok"}
```

默认管理员由应用启动时创建，账号来自 `.env` 中的 `ORDER_ADMIN_USERNAME` 和 `ORDER_ADMIN_PASSWORD`。公开部署前请修改示例密码与 JWT 密钥。

查看日志、停止服务或清理数据卷：

```bash
docker compose logs -f app
docker compose down
docker compose down -v
```

## API 概览

| Method | Path | 权限 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/auth/register` | Public | 用户注册 |
| POST | `/api/auth/login` | Public | 登录并签发 JWT |
| GET | `/api/products` | Public | 商品列表 |
| GET | `/api/activities` | Public | 活动列表 |
| POST | `/api/products` | ADMIN | 创建商品 |
| POST | `/api/activities` | ADMIN | 创建促销活动 |
| POST | `/api/orders` | USER | 活动下单 |
| GET | `/api/orders/:order_no` | USER | 查询订单 |
| POST | `/api/orders/:order_no/cancel` | USER | 取消订单 |
| POST | `/api/payments/callback` | Public | 支付状态回调 |
| POST | `/api/ops/compensate` | ADMIN | 执行异常补偿 |
| POST | `/api/ops/stock/reconcile` | ADMIN | 库存对账与修复 |
| GET | `/api/admin/overview` | ADMIN | 后台业务概览 |
| GET | `/metrics` | Public | Prometheus 格式指标 |

完整请求与响应参见 [API 文档](docs/API.md)，核心设计参见 [架构文档](docs/ARCHITECTURE.md)。

## 验证

运行 Go 测试：

```bash
go test ./... -count=1
```

服务启动后，可使用 Node.js 20+ 执行完整 HTTP 链路验证：

```bash
node scripts/smoke-test.js
```

Go 并发正确性与性能验证（先设置 `ORDER_ADMIN_PASSWORD`）：

```bash
go run ./cmd/pressure -concurrency 20 -duplicate-requests 100 \
  -users 200 -stock 30 -write-requests 1000 -duration 30s
```

验证内容包括普通用户与管理员权限隔离、活动下单、异步状态推进、重复下单拦截、支付回调幂等、库存边界和补偿流程。仓库的 GitHub Actions 会在 push 与 pull request 时执行格式检查、Go 测试和真实 MySQL/Redis/RabbitMQ smoke test。

## 并发验证记录

- 同一用户发起 100 个并发活动下单请求，成功创建 1 个订单，其余请求被重复购买校验拦截。
- 200 个用户并发争抢 30 份活动库存，成功创建 30 个订单，MySQL 与 Redis 库存均未出现负数。

这些结果用于验证并发场景下的业务正确性；Go 压测工具还输出 QPS、延迟分位数、错误分类和最终数据核验。旧 JavaScript 压测脚本保留用于兼容。指标定义与云端资源采集见 [压测说明](docs/PRESSURE.md)。
