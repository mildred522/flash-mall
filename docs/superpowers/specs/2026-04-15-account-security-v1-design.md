# Flash Mall Account Security V1 Design

## Goal

将 `flash-mall` 从“带 JWT 的商城项目”升级为“具备认证中心、会话强校验、防滥用和安全审计闭环的业务级账号安全项目”。

本次设计以面试价值为中心，不追求平台级大而全，而是优先落地一套能稳定演示、能量化验证、能被深入追问的 V1 闭环。

## Positioning

本次 V1 的最终定位是：

- 不是简单补几个登录接口
- 不是把 demo JWT 登录迁移到另一个目录
- 而是将账号安全能力收敛到独立 `auth-service`
- 并通过 `order-api` 维持统一 BFF 入口
- 最终形成“认证中心拆分 + 会话强校验 + 防滥用 + 安全审计”的业务级账号安全闭环

最终希望面试官记住这件事：

> 我把商城项目里的 demo 鉴权升级成了业务级账号安全闭环，核心包括认证中心拆分、BFF 单入口、Redis 会话强校验、防刷限流、refresh token rotation 和安全审计。

## Scope

### In Scope

V1 必须覆盖以下能力：

- 手机号注册
- 手机号 + 密码登录
- 手机号 + 验证码登录
- 验证码发送与消费
- `access token` + `refresh token`
- `refresh token rotation`
- 当前会话登出
- 全端登出 `logout-all`
- 忘记密码 / 重置密码
- 改密后旧会话立即失效
- 登录失败限流
- 发码防刷
- 基础安全审计事件
- `order-api` 侧 JWT + Redis 会话强校验

### Out Of Scope

本期明确不做：

- OAuth 第三方登录
- MFA / TOTP
- 图片验证码
- 平台级风控后台
- 独立风险服务
- 独立审计服务
- RBAC / 权限中心
- 多业务线统一账号中心

## Architecture

### Recommended Approach

采用“强化现有 `auth-service`，`order-api` 继续做 BFF”的增量架构。

这是当前仓库最优解，原因如下：

- 延续当前已存在的认证拆分方向，避免推翻重做
- 改动集中在 `auth-service` 和 `order-api` 两处，实施成本最低
- 边界清晰，最符合“认证域拆分”的面试叙事
- 能在当前分布式订单项目基础上自然叠加，不割裂原有主线

### Service Boundary

#### `auth-service`

负责账号安全主域：

- 注册
- 密码登录
- 验证码登录
- 验证码发送
- refresh
- logout
- logout-all
- 忘记密码 / 重置密码
- 会话状态维护
- 防滥用策略
- 安全审计记录

#### `order-api`

继续作为统一主站入口 / BFF：

- 浏览器只访问 `order-api`
- `/api/auth/*` 由 `order-api` 转发到 `auth-service`
- `/api/order/*` 继续由 `order-api` 暴露
- 对业务请求执行本地 JWT 验签和 Redis 会话强校验

#### `order-rpc` / `product-rpc`

保持业务服务边界不变：

- 不参与登录流程
- 不持有认证逻辑
- 只消费已经认证过的用户身份

### Why Not Other Approaches

不选择“把安全逻辑继续塞回 `order-api`”，因为这会弱化“认证域拆分”的价值。

也不选择本期继续拆独立 `risk-service` / `audit-service`，因为这会把 V1 从“业务级闭环”推向“平台级系统”，复杂度和联调成本过高，不符合当前项目节奏。

## Core Security Flows

### 1. Login Flow

浏览器调用：

- `POST /api/auth/login`
- `POST /api/auth/login/code`

实际链路：

1. 浏览器请求 `order-api`
2. `order-api` 将请求转发给 `auth-service`
3. `auth-service` 完成账号校验
4. 登录成功后创建或替换当前设备类型会话
5. `auth-service` 签发短期 `access token`
6. `auth-service` 生成新的 `refresh token`
7. `refresh token` 通过 `HttpOnly Cookie` 返回给浏览器
8. 同步写入 Redis 会话快照，供业务侧本地强校验

`access token` 的核心字段固定为：

- `sub`
- `sid`
- `session_version`

### 2. Business Access Flow

浏览器访问：

- `POST /api/order/create`

实际链路：

1. 浏览器携带 `Authorization: Bearer <access_token>`
2. `order-api` 先做 JWT 验签
3. 再从 Redis 读取 `sid` 对应会话快照
4. 校验 `sid` 是否存在
5. 校验 `sid` 对应 `user_id` 是否与 JWT `sub` 一致
6. 校验 `session_version` 是否与用户当前版本一致
7. 校验通过后才允许进入订单业务链路

这样能同时满足：

- 业务链路无需每次远程调用 `auth-service`
- 登出 / 改密 / 全端下线后，旧 token 可立即失效

### 3. Refresh Flow

浏览器调用：

- `POST /api/auth/refresh`

实际链路：

1. 浏览器请求 `order-api`
2. `order-api` 转发到 `auth-service`
3. `auth-service` 校验 refresh token 是否有效且属于当前会话
4. 校验通过后执行 rotation
5. 签发新的 `access token`
6. 签发新的 `refresh token cookie`
7. 作废旧的 refresh token

### 4. Logout / Logout-All / Reset Password Flow

#### `logout`

- 仅使当前 `sid` 失效
- 删除当前 Redis 会话快照
- 当前 refresh token 作废

#### `logout-all`

- 提升用户 `session_version`
- 清理该用户所有会话
- 旧 access token 即使未过期，也会因版本不匹配被拒绝

#### `reset password`

- 先完成验证码校验
- 再更新密码哈希
- 更新完成后强制执行 `logout-all`

这保证“改密后旧端立刻失效”是强语义，而不是等待 token 自然过期。

## Internal Module Decomposition

V1 不拆新服务，但在 `auth-service` 内部固定四个模块边界。

### `authstore`

职责：

- 账号主数据读写
- 身份信息读写
- 密码哈希读写
- session 持久化
- 验证码持久化

要求：

- 对上提供统一接口
- 逻辑层不直接拼 SQL
- 逻辑层不直接管理 Redis 事实数据

### `sessionstate`

职责：

- 维护 Redis 会话快照
- 维护用户级 `session_version`
- 为 `order-api` 提供本地强校验语义

### `risk`

职责：

- 登录失败计数
- 验证码发送限频
- 基础账号/IP 级防刷键设计

约束：

- 仅负责判定“是否允许 / 是否阻断 / 当前窗口状态”
- 不承载具体登录业务流程

### `audit`

职责：

- 记录安全事件
- 提供最近事件查询

约束：

- 记录事实，不反向控制业务流程
- 第一版聚焦“可展示、可追踪、可答辩”，不是做成 SIEM 平台

## Data Model

### Storage Responsibilities

#### MySQL

负责长期事实数据：

- 用户主数据
- 用户身份数据
- 密码哈希
- refresh token hash
- 验证码记录
- 安全事件记录

#### Redis

负责高频状态与快速传播：

- 会话快照
- 用户当前 `session_version`
- 登录失败计数
- 发码限流计数

#### JWT

仅作为短期访问凭证，不作为唯一会话真相来源。

### Core Objects

#### `User`

- `id`
- `status`
- `session_version`
- `created_at`

#### `UserIdentity`

- `user_id`
- `identity_type`
- `identity_value`
- `verified_at`

#### `UserCredential`

- `user_id`
- `password_hash`
- `password_algo`
- `password_updated_at`

#### `UserSession`

- `id`
- `user_id`
- `device_type`
- `refresh_token_hash`
- `status`
- `expires_at`
- `last_seen_at`

#### `VerificationCode`

- `phone`
- `scene`
- `code`
- `expires_at`
- `consumed_at`
- `attempt_count`

#### `SecurityAuditEvent`

- `event_type`
- `user_id`
- `subject`
- `result`
- `ip`
- `device_type`
- `created_at`

### Redis Key Model

- `auth:session:sid:{sid}`
  - 值至少包含 `user_id`、`device_type`、`session_version`
- `auth:session:userver:{user_id}`
  - 当前用户的 `session_version`
- `auth:risk:login:phone:{phone}`
  - 手机号登录失败计数
- `auth:risk:login:ip:{ip}`
  - IP 登录失败计数
- `auth:risk:code:phone:{scene}:{phone}`
  - 某场景某手机号发码限流
- `auth:risk:code:ip:{scene}:{ip}`
  - 某场景某 IP 发码限流

## Security Policies

### 1. Password Login Failure Throttling

维度：

- `phone`
- `ip`

规则：

- 同一手机号在 `15 分钟` 内连续失败 `5 次`，冻结 `15 分钟`
- 同一 IP 在 `15 分钟` 内连续失败 `20 次`，冻结 `15 分钟`

行为要求：

- 密码错误和账号不存在都计入失败次数
- 对外错误文案统一，不泄露账号存在性
- 登录成功后清空手机号失败计数

### 2. Verification Code Send Anti-Abuse

维度：

- `phone + scene`
- `ip + scene`

场景：

- `register`
- `login`
- `reset_password`

规则：

- 同手机号同场景 `1 分钟 1 次`
- 同手机号同场景 `10 分钟 3 次`
- 同 IP 同场景 `10 分钟 10 次`

行为要求：

- 命中限流时直接拒绝发码
- 开发环境即使返回 mock code，也必须走同样限流流程
- 验证码成功消费后清理对应发送节流键

### 3. Verification Code Consumption

规则：

- 验证码有效期 `5 分钟`
- 成功消费后立即失效
- 单条验证码最多尝试 `5 次`
- 必须绑定场景，不允许跨场景复用

### 4. Refresh Token Rotation

规则：

- 每次 `/refresh` 成功都签发新的 refresh token
- 服务端只保存 refresh token 的 hash
- 旧 refresh token 成功使用一次后立即作废

异常处理：

- 如果检测到旧 refresh token 重放，直接使当前会话失效
- 记录高风险审计事件

生命周期：

- `access token`：`15 分钟`
- `refresh token`：`7 天`

### 5. Session Invalidation

#### Same-Device Replace

- 同一 `device_type` 重复登录时，旧会话失效，新会话生效
- 不同 `device_type` 可以并存

#### Global Invalidation

以下动作必须触发强失效：

- `logout-all`
- `reset password`

实现方式：

- 提升用户 `session_version`
- 清理用户活跃会话

### 6. Security Audit Events

V1 至少记录以下事件：

- `register_success`
- `register_fail`
- `login_password_success`
- `login_password_fail`
- `login_code_success`
- `login_code_fail`
- `send_code_success`
- `send_code_blocked`
- `refresh_success`
- `refresh_replay_blocked`
- `logout_success`
- `logout_all_success`
- `reset_password_success`
- `session_invalidated`

每条事件至少带：

- `event_type`
- `user_id`
- `subject`
- `result`
- `ip`
- `device_type`
- `created_at`

### 7. Error Semantics

对外统一收敛为：

- `账号或凭证无效`
- `操作过于频繁`
- `会话已失效`

对内审计保留细粒度原因，例如：

- `password_mismatch`
- `user_not_found`
- `phone_rate_limited`
- `ip_rate_limited`
- `refresh_replayed`

## API Strategy

浏览器只访问 `order-api`，对外保留统一接口：

- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/login/code`
- `POST /api/auth/code/send`
- `POST /api/auth/refresh`
- `POST /api/auth/logout`
- `POST /api/auth/logout-all`
- `GET /api/auth/me`
- `POST /api/auth/password/forgot`
- `POST /api/auth/password/reset`

责任划分：

- `order-api`
  - 请求转发
  - cookie 透传
  - 错误透传
  - 业务接口 JWT + Redis 强校验
- `auth-service`
  - 真正的认证业务语义
  - 防滥用策略
  - 会话状态变更
  - 审计事件记录

### V1 Debug Endpoint

为了形成“安全闭环可见化”，增加一个仅用于开发演示的接口：

- `GET /api/auth/security/events/recent`

用途：

- 返回最近 N 条安全事件
- 由 `order-api` 继续代理到 `auth-service`
- 供商城开发者面板直接展示

这不是产品功能，而是演示和面试证据出口。

## Frontend Strategy

商城页继续保持主业务视图，不做重型后台。账号安全能力放在轻量开发者控制台或调试区展示，不污染主购物流程。

前端建议展示四类信息：

### 1. Current Session State

- 当前用户 ID
- 当前设备类型
- 当前会话 ID
- 当前 `session_version`
- access token 状态
- refresh cookie 状态

### 2. Auth Operations

- 密码登录
- 验证码登录
- 注册
- 发送验证码
- logout
- logout-all
- 重置密码

### 3. Security Status

- 最近一次失败原因
- 是否命中限流
- 是否发生会话失效
- refresh 是否完成轮转

### 4. Recent Security Events

最近 10 条事件，至少展示：

- 时间
- 事件类型
- 结果
- 用户
- 设备
- IP

## Demo Scenarios

V1 最终必须支持现场演示以下链路：

1. 正常登录后下单成功
2. 连续输错密码触发登录限流
3. 连续发验证码触发防刷拦截
4. 登录后调用 refresh，观察 token 轮转成功
5. 执行 `logout-all` 或重置密码后，旧 token 立即失效，再次下单失败
6. 查看最近安全事件，证明系统不是黑盒

## Interview Framing

这版 V1 完成后，面试表达应稳定聚焦以下几点：

1. 原项目已有 JWT，但只是 demo 级鉴权，不具备完整会话控制能力
2. 关键升级不是“多写几个 auth 接口”，而是把认证域从业务 API 中拆出
3. JWT 只负责短期访问凭证，服务端状态模型才决定会话是否真正有效
4. 通过 `refresh token rotation + Redis session snapshot + session_version` 实现立即失效能力
5. 通过登录限流、发码防刷和安全审计，把项目从“会登录”升级为“业务级账号安全闭环”

## Accepted Decisions

本次设计确认的最终决策如下：

- 采用强化现有 `auth-service` 的增量架构
- `order-api` 继续作为统一 BFF，不让浏览器直连 `auth-service`
- V1 聚焦认证中心、会话强校验、防滥用和审计闭环
- 风控和审计先作为 `auth-service` 内部模块存在，不拆独立服务
- V1 增加最近安全事件查询接口，作为演示证据出口
