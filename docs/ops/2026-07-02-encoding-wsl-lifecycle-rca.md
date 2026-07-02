# 2026-07-02 商品名编码与 WSL 生命周期 RCA

## 背景

本次排查同时处理两个反复出现的问题：

- 前端商品名再次显示为 `????`。
- 项目短暂可访问后，所有容器又被停掉。

这两个问题都不是单个 Go 服务业务逻辑错误。第一个是 MySQL 初始化链路的字符集污染，第二个是 WSL/Docker 生命周期被 Windows 回收导致的整体停机。

## 问题一：商品名显示 `????`

### 现象

商城接口 `/api/shop/catalog` 返回的商品名为 `????`、`?????`、`??T????`，前端只是展示了接口返回值。

### 证据

- `scripts/k8s/init-db.sql` 源文件是 UTF-8，商品 seed 文本本身是中文，例如 `首发风衣`。
- MySQL 表字段是 `utf8mb4`，字段层没有退化成 latin1。
- 查询当前库内数据时，`HEX(name)` 为 `3F3F3F...`，说明中文已经在写入 MySQL 时被转换成问号，属于不可逆脏数据。
- 手动使用 `mysql --default-character-set=utf8mb4` 写入中文后，`HEX(name)` 正常变为 UTF-8 字节。
- 只截取商品 seed 片段导入可以写入中文；跑整份 `init-db.sql` 时会复现写成问号，说明问题在整份初始化 SQL 的会话字符集状态，而不是前端/API/字段定义。

### 根因

初始化 SQL 里原先使用了：

```sql
SET NAMES utf8mb4;
SET CHARACTER SET utf8mb4;
```

`SET CHARACTER SET` 会根据当前默认 database/collation 重新设置连接状态。在历史卷或无默认库连接场景下，`character_set_database` 可能仍是 `latin1`，导致后续整份 SQL 导入中文 seed 时发生字符集退化。中文在写入时被转换成 `?`，之后 API 和前端只能继续展示坏数据。

同时，多个 MySQL 导入入口原本没有统一追加 `--default-character-set=utf8mb4`，包括 compose init、CI smoke、Windows 启动脚本、K8s job、K8s migrate。这让不同启动路径存在重复污染数据的风险。

### 修复

- `scripts/k8s/init-db.sql` 去掉 `SET CHARACTER SET utf8mb4`。
- 改为显式设置：
  - `character_set_client=utf8mb4`
  - `character_set_connection=utf8mb4`
  - `character_set_results=utf8mb4`
  - `collation_connection=utf8mb4_general_ci`
- 对现有数据库执行 `ALTER DATABASE ... CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci`，让历史卷也能被迁移。
- 所有 MySQL 导入入口统一加 `--default-character-set=utf8mb4`。
- 将编码规则写入 `PROMPT.md` 和 `PROJECT_NOTES.md`：商品名/SQL/API/HTML 中文乱码按 P0 处理，必须先查 SQL 源文件、MySQL `HEX(name)`、API JSON，再查前端。

### 验证

- 先将商品 100-104 名称主动改成坏值。
- 重新运行整份 MySQL init。
- 再查 `SELECT id,name,HEX(name)`，商品名自动恢复为中文，`HEX(name)` 为正确 UTF-8 字节。
- `/api/shop/catalog` 返回中文商品名。

## 问题二：短暂可用后服务全停

### 现象

项目启动后短时间可访问，随后 `entry-api` 拒绝连接，所有服务不可用。

### 证据

- `docker ps -a` 显示 `entry-api`、`order-rpc`、`auth-api`、`product-rpc`、`inventory-kitex`、Redis、MySQL、RabbitMQ、Etcd 等容器全部是 `Exited (0)`。
- 退出码是 0，不符合单个服务崩溃、panic、端口冲突、健康检查失败的模式。
- Docker journal 里有 `Stopping docker.service`、`Processing signal 'terminated'`、`Daemon shutdown complete`。
- 多次看到 WSL boot id 变化，说明 WSL distro 自身被终止/重启。
- `docker.service` 为 disabled，主要靠 socket activation 拉起；没有持久 WSL 会话时，Windows 可以回收 WSL，Docker daemon 随 WSL 停止，compose 容器也随之全部正常退出。

### 根因

启动脚本从 Windows 调用 WSL 执行 compose，命令结束后没有任何持久前台会话保住 WSL distro。Windows 回收 WSL 后，WSL 内 Docker daemon 收到终止信号并正常退出，所有 compose 容器随之 `Exited (0)`。

这不是 Go 服务崩溃，也不是 entry-api 自己停掉，而是宿主运行环境生命周期没有被守住。

### 修复

- 新增 `scripts/local/wsl-keepalive.sh`。
- 新增 `scripts/local/start-wsl-keepalive.ps1` 和 `scripts/local/stop-wsl-keepalive.ps1`。
- `scripts/local/start-wsl-compose.ps1` 默认启动隐藏 WSL keepalive，避免 Windows 在启动脚本结束后回收 WSL。
- `scripts/local/stop-wsl-compose.ps1` 默认在 compose down 后清理 keepalive；如需要保留 WSL，可传 `-KeepAlive`。
- `PROJECT_NOTES.md` 记录：从 Windows 启动 WSL compose 时必须保留 keepalive，否则 Docker 会随 WSL 被回收而停机。

### 验证

- 使用 `start-wsl-compose.ps1` 启动项目后，健康检查通过。
- 延时约 90 秒后再次查询容器状态，核心服务仍保持 `Up`。
- `/api/shop/catalog` 仍可返回商品列表。

## 防复发规则

- 看到中文 `????`，不要优先改前端；先查数据库 `HEX(name)`。
- MySQL 初始化、迁移、CI smoke、K8s job 都必须使用 `--default-character-set=utf8mb4`。
- 初始化 SQL 不再使用容易受默认库状态影响的 `SET CHARACTER SET`。
- Windows 调 WSL 启动 compose 时，必须有 keepalive 或其他持久 WSL 会话。
- 如果所有容器都是 `Exited (0)`，优先查 Docker/WSL 生命周期，不要先怀疑 Go 服务业务逻辑。
