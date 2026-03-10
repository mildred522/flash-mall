# request_id 幂等键拆分执行记录（2026-03-07）

## 1. 变更目标

将原有 `request_id` 单 key（同时承担锁和结果）改造为双 key：
1. `order:request:lock:{request_id}`：仅表示“处理中”。
2. `order:request:result:{request_id}`：仅表示“已完成，保存最终 order_id”。

## 2. 核心改动

文件：`app/order/api/internal/logic/createorderlogic.go`

1. 读取路径：
- 先查 DB（`FindOneByRequestId`）。
- 再查 Redis result key。
- 命中 DB 时回填 result key，并清理 lock key（best effort）。

2. 抢占路径：
- 使用 `SetnxEx(lockKey, "processing", ttl)` 抢占。
- 抢占失败时只查 result key：
  - 命中 result：返回历史 order_id。
  - 未命中 result：返回 `codes.Aborted`（处理中）。

3. 提交路径：
- `SAGA Submit` 成功后写入 result key。
- result key 写入成功后删除 lock key。
- `SAGA Submit` 失败时释放 lock key，允许重试。

4. 资源优化：
- `gid/orderID` 生成延后到 lock 抢占成功后，减少重复请求的无效资源消耗。

## 3. 预期收益

1. 幂等语义更清晰：processing 与 done 明确分离。
2. 避免并发重复请求拿到“处理中占位值”被误当最终结果。
3. 面试可解释性更强：可以明确回答状态机与并发分支行为。

## 4. 风险与兜底

1. 风险：result key 写入失败时，重复请求只能看到 lock。
2. 兜底：
- lock 有 TTL，不会永久阻塞。
- DB 是最终幂等依据，TTL 后仍可回源命中。

## 5. 验证记录

执行命令：
```bash
go test ./app/order/api/internal/logic -run TestCreateOrderLogic_CreateOrder_RedisLimit -count=1
go test ./app/order/api/...
```

结果：
- 以上测试均通过。

## 6. 简历描述（可直接使用）

将 request_id 幂等机制从“单键复用”重构为“锁键/结果键分离”，消除并发重试场景的结果歧义，并通过测试验证无回归。
