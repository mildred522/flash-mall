# JWT 接入执行记录（2026-03-07）

## 1. 目标

为 `order-api` 增加认证能力，形成可面试讲解的最小闭环：
1. 登录签发 JWT。
2. 受保护接口验签。
3. 从 JWT claim 注入业务身份（`user_id`）。

## 2. 代码改动

1. 新增配置：
- `JwtAuthSecret`
- `JwtExpireSeconds`
- `AuthDemoPassword`

2. 新增登录接口：
- `POST /api/auth/login`
- 请求：`user_id + password`
- 返回：`access_token/token_type/expires_at`

3. 下单接口加 JWT 保护：
- `POST /api/order/create` 使用 `rest.WithJwt(...)`
- 保留限流中间件

4. 下单身份来源改造：
- 从 JWT claim 的 `user_id` 读取身份
- 覆盖请求体中的 `user_id`，避免伪造

## 3. 示例请求

1. 登录拿 token：
```bash
curl -X POST http://127.0.0.1:8888/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"user_id":1001,"password":"flashmall123"}'
```

2. 带 token 下单：
```bash
curl -X POST http://127.0.0.1:8888/api/order/create \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <access_token>" \
  -d '{"request_id":"req-1001-1","user_id":9999,"product_id":1,"amount":1}'
```

说明：示例里 body 的 `user_id=9999` 会被服务端 JWT claim 覆盖，真实下单身份仍是 1001。

## 4. 验证结果

执行命令：
```bash
go test ./app/order/api/...
```

结果：通过。

## 5. 面试可讲点

1. 为什么不用前端传 `user_id`：可伪造，属于不可信输入。
2. 为什么 JWT claim 注入更安全：服务端验签后再下发身份上下文。
3. 目前边界：是 demo 登录模型，下一步可接用户服务/刷新令牌/黑名单。

## 6. 简历可用一句话

为订单服务引入 JWT 认证链路（登录签发 + 路由验签 + user_id claim 注入），将下单身份从前端传参升级为服务端可信声明，并通过测试验证鉴权改造无回归。
