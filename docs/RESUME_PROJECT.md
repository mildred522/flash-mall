# Flash Mall 简历项目文本

> 本文件只提供可替换旧简历中 Flash Mall 项目经历的新文本，不修改或重新制作整份简历。

## Flash Mall 分布式商城　　　　　　　　　2025 年 12 月 - 2026 年 8 月

**技术栈：** Golang、Hertz、Go-zero、Kitex、gRPC/Thrift、DTM、MySQL、Redis、RabbitMQ、Prometheus、Grafana、Jaeger、Docker

**项目介绍：** 面向用户、商家和平台管理员三端的分布式商城，覆盖商家入驻与数据隔离、店铺运营、商品与橱窗管理、下单支付、履约退款、对账审计等核心业务。

**技术架构：** Hertz统一承接HTTP流量并完成鉴权与业务编排，Go-zero RPC承载商品与订单领域能力，Kitex负责库存预占、确认与释放命令；MySQL保存交易事实，Redis承载热点库存与多级缓存，RabbitMQ解耦支付后的派生任务。

**技术亮点：**

- **交易一致性：** 设计库存预占、支付确认、取消释放三阶段模型，以DTM SAGA协调订单与库存；Redis Lua原子执行分桶库存预占，MySQL账本持久化预占状态，`order_id`约束重复请求，Release支持空补偿与重复执行，恢复任务回收过期及悬挂预占。
- **支付与消息可靠性：** 通过订单/支付单行锁、金额校验、渠道事件唯一约束和状态机处理并发支付、重复回调、取消与退款；支付事实与Outbox事件在同一MySQL事务提交，Outbox发布失败执行退避重试并将超限事件标记为dead，RabbitMQ消费者按`event_id`去重，库存确认失败转入持久化恢复。
- **缓存治理：** 构建进程内L1与Redis L2缓存，使用版本化Bitmap布隆过滤器和负缓存拦截穿透，使用singleflight与软TTL后台刷新抑制击穿，使用过期时间抖动和陈旧值兜底降低雪崩影响，并通过Redis Pub/Sub同步多实例失效事件。
- **可观测与故障验证：** 接入Prometheus、Grafana和Jaeger统一观察HTTP、RPC、缓存、库存及Outbox链路，并设计可恢复故障注入；在4 vCPU环境实测公开读链路约594.68 QPS且8,921次请求全部成功，40次同幂等键请求仅生成1笔订单，RabbitMQ恢复后90秒内排空Outbox，最终负库存、悬挂预占和重复回调副作用均为0。

## 使用边界

- “约594.68 QPS”是4 vCPU、约8 GiB WSL Docker固定环境的最高已测公开读档位，不代表生产容量上限。
- 支付接入的是可重复演示的本地沙箱和支付宝开放平台沙箱，不写成真实商户生产支付。
- Kubernetes清单属于部署能力储备，不写成已经承载真实流量的生产集群。
