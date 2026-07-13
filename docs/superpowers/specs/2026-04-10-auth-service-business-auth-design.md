# Flash Mall Auth Service Business Auth Design

## Goal

将当前 `entry-api` 内部的 demo JWT 登录改造成业务级认证体系，落地独立 `auth-service`，同时保持商城前端与下单链路仍通过统一主站入口访问。

## Current State

当前系统的认证能力仅覆盖最小演示闭环：

- `POST /api/auth/login` 由 `entry-api` 直接提供
- 登录只校验 `user_id + AuthDemoPassword`
- 登录成功后签发短期 JWT
- `POST /api/order/create` 由 `rest.WithJwt(...)` 保护
- 下单身份通过 JWT claim 中的 `user_id` 注入

现状问题：

- 无真实用户体系
- 无注册
- 无短信验证码
- 无 refresh token
- 无设备会话管理
- 无找回密码
- 无强一致踢下线/封禁能力
- 认证与订单业务耦合在同一个 API 服务中

## Business-Level Target

第一阶段认证闭环必须覆盖：

- 手机号注册
- 手机号 + 密码登录
- 手机号 + 短信验证码登录
- 短期 `access token`
- 长期 `refresh token`
- 当前用户信息查询
- 当前设备登出
- 同端互斥，不同端可共存
- 找回密码 / 重置密码
- 认证审计
- 强一致会话失效

明确不在第一阶段实现：

- 第三方 OAuth 登录
- 多因子认证
- 图形验证码
- 独立网关产品化
- 独立短信平台治理后台

## Final Architecture

### 1. Service Topology

系统调整为三层：

1. `auth-service`
   - 负责账号域与认证域
   - 负责注册、登录、验证码、会话、refresh、logout、密码重置
2. `entry-api`
   - 继续作为统一主站入口/BFF
   - 承载商城页、调试页、订单相关接口
   - 将 `/api/auth/*` 请求转发给 `auth-service`
3. `order-rpc/product-rpc`
   - 不感知登录流程
   - 仅消费已认证后的业务身份

### 2. Trust Boundary

认证信任边界调整为：

- 浏览器只信任主站入口
- `auth-service` 是唯一签发认证凭证的服务
- `entry-api` 与其他业务服务本地验 JWT，不对每个请求回调认证服务
- 业务侧绝不信任请求体中的用户身份字段

### 3. Token Model

采用双令牌模型：

- `access token`
  - JWT
  - 短有效期
  - 用于业务接口鉴权
- `refresh token`
  - 长有效期
  - 服务端可控会话凭证
  - 通过 `HttpOnly Cookie` 存储

推荐生命周期：

- `access token`: 15 分钟
- `refresh token`: 7 天

### 4. Session Model

会话模型选择：

- 同端互斥
- 不同端可共存

示例：

- 新的 Web 登录会踢掉旧的 Web 会话
- 新的 iOS 登录会踢掉旧的 iOS 会话
- Web 与 iOS 可以同时在线

### 5. Strong Consistency Session Invalidation

业务要求已确认使用强一致会话失效。

含义：

- 用户登出后，旧会话不应继续可用到 access token 自然过期
- 用户被封禁后，旧 access token 不能继续放行
- 同端新登录后，旧同端 token 应尽快失效

实现策略：

- JWT 中包含 `sid` 与 `session_version`
- 业务服务在本地验签后，还需校验会话状态缓存
- 会话状态由 `auth-service` 维护，并同步到 Redis
- Redis 作为认证会话的强一致校验层

## Auth Domain Decomposition

`auth-service` 内部按六个子域划分：

### 1. Account

负责：

- 用户主档
- 昵称、头像、状态
- 用户生命周期

### 2. Identity

负责：

- 手机号主身份
- 邮箱补充身份
- 身份唯一性
- 身份是否已验证

### 3. Credential

负责：

- 密码哈希
- 算法版本
- 密码更新时间

### 4. Session

负责：

- refresh token
- 设备维度会话
- 同端互斥
- 登出
- 踢下线
- token rotation

### 5. Verification Code

负责：

- 注册验证码
- 登录验证码
- 找回密码验证码
- 过期与重放控制

### 6. Audit And Risk

负责：

- 登录/注册/重置密码审计
- 登录失败计数
- 发码频率限制
- 账号/IP 基础限流

## Login And Registration Policy

### Registration

注册流程要求：

- 手机号必须先通过短信验证码校验
- 验证通过后才能创建账号
- 创建账号同时创建默认会话
- 注册成功后直接登录

### Login

登录方式要求同时支持：

- 密码登录
- 短信验证码登录

### Password Reset

找回密码要求：

- 先发验证码
- 验证成功后允许重置密码
- 重置密码后使旧会话全部失效

## Frontend Integration

### 1. User-Facing Access Pattern

用户始终只访问主站入口，不直接访问 `auth-service`。

接入模式：

- 商城前端请求 `/api/auth/*`
- `entry-api` 作为 BFF 转发到 `auth-service`
- 主域统一写入/携带 `HttpOnly Cookie`

### 2. Storefront UI Requirements

商城前端需升级为：

- 登录 / 注册双入口
- 密码登录
- 短信验证码登录
- 注册
- 找回密码

同时保持一条原则：

- 主内容区只表达消费者视角
- 开发者相关信息仅保留在右下角开发者控制台中

## Data Boundary

数据隔离策略：

- 先共用现有 MySQL 基础设施
- 认证数据使用独立 `auth` 数据库或等价的独立逻辑边界
- 不将认证表混入订单库语义

这样可以在不引入额外数据库运维成本的前提下保持服务边界清晰。

## Security Decisions

### Password

- 不允许明文密码落库
- 第一阶段使用 `bcrypt`
- 预留算法升级能力

### Refresh Token

- 仅服务端保存 hash
- 浏览器持有明文 `HttpOnly Cookie`
- 支持 rotation

### Verification Code

- 开发环境使用可切换的 mock provider
- 生产环境接真实短信 provider
- 验证码必须限时、限次、单场景消费

### Rate Limiting

需要覆盖：

- 注册
- 登录
- 发码
- 重置密码

### Audit

必须记录：

- 注册成功/失败
- 登录成功/失败
- refresh
- logout
- 重置密码
- 强制失效

## Migration Strategy

采用增量迁移，不推翻当前订单链路。

### Phase 1

- 新建独立 `auth-service`
- 保持 `entry-api` 作为统一入口
- 新增 `/api/auth/*` 转发能力
- 下单接口仍继续从 JWT 中获取用户身份

### Phase 2

- 统一 JWT claim 语义，从 `user_id` 演进到 `sub + sid + session_version`
- 业务服务接入强一致会话状态校验
- 移除 demo 登录逻辑

### Phase 3

- 补齐验证码登录、找回密码、全端登出、风控与审计闭环

## Operational Notes

### Dev Environment

- 短信 provider 必须可切换为 mock
- 主站和 `auth-service` 通过本地脚本一键拉起
- 需要本地 Redis 作为会话与验证码依赖

### Production Direction

- `auth-service` 可独立扩容
- JWT 由认证服务统一签发
- 业务服务只做本地验签和会话状态校验

## Interview Framing

面试表达重点：

1. 当前系统已有 JWT 鉴权雏形，但只是 demo 级
2. 业务级改造的关键不是“加一个注册接口”，而是把认证域拆开
3. JWT 只负责短期访问凭证，不负责完整会话治理
4. refresh token + session + Redis 才能实现登出、踢下线、封禁和同端互斥
5. 使用 `entry-api` 作为 BFF，可在不打碎前端接入面的情况下完成认证服务独立化

## Accepted Decisions

本次讨论已确认的最终决策：

- 独立 `auth-service`
- 手机号主体系，邮箱补充
- 注册必须先验证码
- 支持密码登录与短信验证码登录
- 使用 refresh token
- refresh token 走 `HttpOnly Cookie`
- 同端互斥，不同端可共存
- `entry-api` 本地验 JWT
- `entry-api` 继续作为统一入口/BFF
- 第一阶段包含找回密码/重置密码
- 使用强一致会话失效
- 认证数据先与现有项目共用同一套 MySQL 基础设施，但逻辑独立
