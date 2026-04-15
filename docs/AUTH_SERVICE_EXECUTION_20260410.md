# Auth Service 落地记录

## 当前完成度

已完成一套可用于业务讲解和面试表达的账号认证主链路：

- 独立 `auth-service` 承接注册、密码登录、验证码登录、refresh、logout、logout-all、me、忘记密码、重置密码
- `order-api` 作为统一 BFF 代理 `/api/auth/*`
- 商城前端继续只访问主站入口，不直接感知服务拆分
- JWT claim 已从简单 `user_id` 演进为 `sub + sid + session_version`
- `order-api` 下单前除验签外，还会校验会话状态
- `logout-all` 和重置密码后，旧 token 会因为会话状态或版本失效而被拦截
- 会话模型已支持“同端互斥、跨端共存”
- 会话状态已写入 Redis，`order-api` 可做本地强校验，不必每次回调认证服务

## 当前链路

### 1. 登录注册链路

- 用户通过商城页调用 `/api/auth/register` 或 `/api/auth/login`
- `order-api` 代理请求到 `auth-service`
- `auth-service` 校验验证码或密码
- 登录成功后签发：
  - 短期 `access token`
  - 长期 `refresh token`
- `refresh token` 通过 `HttpOnly Cookie` 返回

### 2. 业务访问链路

- 浏览器携带 `Authorization: Bearer <access_token>` 访问业务接口
- `order-api` 先做 JWT 验签
- 再提取 `sub / sid / session_version`
- 再校验会话是否仍然有效
- 最终用服务端可信身份覆盖请求体中的 `user_id`

### 3. 失效链路

- `logout` 会失效当前 refresh token
- `logout-all` 会让当前用户所有会话失效，并推进 `session_version`
- 重置密码会推进 `session_version` 并清理旧会话
- 即使 access token 还没过期，只要会话状态失效或版本不匹配，也会被拒绝

### 4. 设备维度会话策略

- 默认支持 `device_type`
- 同一 `device_type` 重复登录时，旧会话失效
- 不同 `device_type` 可以共存，例如 `web` 与 `ios`
- 这让“多端登录策略”从产品规则变成了代码里的真实约束

## 设计亮点

### 1. BFF 统一入口

前端永远只调用主站域名下的 `/api/auth/*`，不直接访问 `auth-service`。

好处：

- Cookie 域和跨域问题更简单
- 前端不感知服务拓扑
- 后续可以在 BFF 层统一做熔断、埋点、限流和灰度

### 2. JWT 只负责“短期访问凭证”

JWT 解决的是：

- 无状态验签
- 快速识别请求身份
- 减少每次请求都回源认证中心

JWT 不单独负责：

- 强制下线
- 同端互斥
- 密码修改后旧 token 立即失效

这些靠 `sid + session_version + session state` 补齐。

### 3. 会话模型更贴近业务

当前已经支持：

- 同一设备类型重复登录，旧会话失效
- 不同设备类型可同时在线

这比“一个用户只保留一个 session”更符合真实产品。

### 4. 强一致失效语义

当前不是只等 access token 自然过期，而是把会话失效能力补进来了：

- `logout-all`
- 重置密码
- 会话版本变更

这类能力是面试里很容易和“只会用 JWT 登录”的候选人拉开差距的点。

### 5. 业务服务本地强校验

当前不是简单把业务请求回调到认证中心，而是：

- `auth-service` 登录/刷新/登出时同步 Redis 会话状态
- `order-api` 读取本地 Redis 会话快照
- 结合 JWT 中的 `sub / sid / session_version` 做本地校验

这比“业务服务每次都调用认证服务问一句 token 是否有效”更适合高并发链路。

## 现阶段技术边界

当前仍是渐进式落地，不是最终态：

- `auth-service` 的业务逻辑已经独立，但底层还是以内存 store 为默认实现
- `order-api` 当前的会话强校验仍通过兼容型远程验证入口完成
- Redis / MySQL 持久化边界和 schema 已经定下，下一步是替换默认实现

## 适合面试表达的关键词

- 认证中心拆分
- BFF 统一入口
- JWT + Refresh Token 双令牌模型
- `HttpOnly Cookie`
- `sub / sid / session_version`
- 同端互斥、跨端共存
- 强制下线
- 密码重置触发会话失效
- 服务端可信身份覆盖前端 `user_id`
