# 本地一键启动说明（2026-03-07）

## 1. 新增脚本

1. 启动：`scripts/local/start-all.ps1`
2. 停止：`scripts/local/stop-all.ps1`

## 2. 启动能力

`start-all.ps1` 默认会执行：
1. `docker compose up -d` 拉起依赖：etcd/mysql/redis/dtm/rabbitmq。
2. 执行 `scripts/k8s/init-db.sql` 初始化数据库结构。
   - 脚本使用 `mysql --force`，若本地已有旧表结构冲突会给出 warning 并继续启动。
3. 执行 `seed_stock.go` 预置 Redis 分片库存（product=100, stock=10000, shards=4）。
4. 后台启动：
- `product-rpc`（8080）
- `order-rpc`（8090）
- `order-api`（8888）
5. 自动打开前端控制台：`http://127.0.0.1:8888/`

## 3. 使用方法

### 3.1 一键启动

```powershell
powershell -ExecutionPolicy Bypass -File scripts/local/start-all.ps1
```

### 3.2 可选参数

```powershell
# 跳过 docker compose（依赖已就绪时）
powershell -ExecutionPolicy Bypass -File scripts/local/start-all.ps1 -SkipCompose

# 跳过 DB 初始化
powershell -ExecutionPolicy Bypass -File scripts/local/start-all.ps1 -SkipDbInit

# 跳过库存预置
powershell -ExecutionPolicy Bypass -File scripts/local/start-all.ps1 -SkipSeedStock

# 不自动打开浏览器
powershell -ExecutionPolicy Bypass -File scripts/local/start-all.ps1 -NoBrowser

# 调整端口等待超时（秒）
powershell -ExecutionPolicy Bypass -File scripts/local/start-all.ps1 -PortWaitSeconds 120
```

### 3.3 停止

```powershell
# 仅停止 Go 服务进程
powershell -ExecutionPolicy Bypass -File scripts/local/stop-all.ps1

# 停止 Go 服务 + Docker 依赖
powershell -ExecutionPolicy Bypass -File scripts/local/stop-all.ps1 -WithDeps
```

## 4. 访问地址

1. 控制台：`http://127.0.0.1:8888/`
2. 商城页：`http://127.0.0.1:8888/shop`
3. 调试页：`http://127.0.0.1:8888/debug`
4. 健康检查：`http://127.0.0.1:8888/api/system/health`
5. Metrics：`http://127.0.0.1:9090/metrics`
6. PProf：`http://127.0.0.1:6060/debug/pprof/`
7. RabbitMQ 管理台：`http://127.0.0.1:15672/`（账号 `flashmall` / `flashmall123`）

## 5. 相关改动

1. `deploy/docker-compose.yml` 增加 RabbitMQ 服务。
2. `app/order/api/etc/order-api.yaml` 的 RPC 目标切换到 `127.0.0.1`，确保本机直跑可连通。
3. 控制台默认商品 ID 调整为 `100`，与初始化 SQL 的演示数据一致。
4. 启动前会检查 Docker daemon 是否可用，不可用会直接提示先启动 Docker Desktop。
