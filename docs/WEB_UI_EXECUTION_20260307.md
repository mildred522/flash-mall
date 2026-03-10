# 前端控制台适配执行记录（2026-03-07）

## 1. 目标

让项目可以直接通过浏览器演示完整流程，并拆分为两套页面：
1. 商城界面（正常购物流程）。
2. 调试界面（链路验证与性能观测）。

## 2. 主要改动

1. 新增内嵌 Web UI（拆分）：
- 路径：`GET /`（入口页）
- 路径：`GET /shop`（商城界面）
- 路径：`GET /debug`（调试界面）
- 文件：
  - `app/order/api/internal/handler/web/home.html`
  - `app/order/api/internal/handler/web/shop.html`
  - `app/order/api/internal/handler/web/debug.html`
- 实现：`app/order/api/internal/handler/webuihandler.go`

2. 新增系统健康检查接口：
- 路径：`GET /api/system/health`
- Handler：`app/order/api/internal/handler/systemhealthhandler.go`
- Logic：`app/order/api/internal/logic/systemhealthlogic.go`

3. 路由整合：
- `routes.go` 增加 `/`、`/shop`、`/debug` 与 `/api/system/health`
- 保留 `POST /api/auth/login`
- `POST /api/order/create` 继续受 JWT 保护

## 3. 页面能力

1. 商城界面（`/shop`）：
- 登录
- 商品选择与下单
- 订单回执与操作日志
2. 调试界面（`/debug`）：
- 登录
- 链路健康检查
- 幂等顺序/并发测试
- 浏览器内并发快测（p50/p95/p99）

## 4. 运行方式

1. 启动依赖：MySQL、Redis、DTM、order-rpc、product-rpc、RabbitMQ。
2. 启动 API：
```bash
go run ./app/order/api/order.go -f ./app/order/api/etc/order-api.yaml
```
3. 浏览器打开：
- `http://127.0.0.1:8888/`（入口）
- `http://127.0.0.1:8888/shop`（商城）
- `http://127.0.0.1:8888/debug`（调试）

## 5. 验证

执行：
```bash
go test ./app/order/api/...
```

结果：通过。

## 6. 简历一句话

为订单服务构建内嵌可视化控制台（JWT 登录、下单、幂等实验、并发快测、依赖健康检查），将项目从“接口级验证”升级为“端到端可演示链路”。
