# Flash Mall 独立桌面控制中心设计

## 目标

为 Windows + Ubuntu WSL 环境提供一个原生桌面入口。用户不需要理解 Compose、容器依赖或现有脚本，只需在“Flash Mall 控制中心”中启动、重建、停止项目并查看状态。

成功标准是：从桌面快捷方式打开控制中心，一次点击后启动当前 Hertz + Go-zero RPC + Kitex 栈，健康检查成功后可直接打开商城和后台；整个过程不调用 Docker Desktop，不启动旧 Windows exe，也不默认启动 legacy `entry-api`。

## 技术选择

使用 .NET 8 WPF。桌面程序保持 Windows 原生体验，通过 `wsl.exe -d Ubuntu` 调用单一 WSL 控制脚本。WPF 不直接拼装 Docker Compose 命令，也不承担镜像构建细节。

选择 WPF 的原因：本机已有 .NET 8；它比现有 PowerShell WinForms 更易维护、异步执行和测试；相比 Tauri，不需要引入 Rust/WebView 前端工具链。

## 用户界面

主窗口只暴露日常操作：

- 项目总状态：未启动、启动中、可用、部分异常、停止中。
- `启动项目`：使用已有镜像快速启动。
- `重新构建并启动`：构建五个默认业务服务镜像后启动；legacy `entry-api` 不参与。
- `停止项目`：停止 Compose 栈但保留数据卷。
- 快捷入口：商城、管理后台、RabbitMQ、Jaeger。
- 当前阶段与进度日志。

“高级操作”默认折叠，包含服务状态、单服务重新构建、关键容器日志和设置。界面同一时间只允许一个有状态操作，避免重复启动、并发构建或启动与停止互相覆盖。

## 组件边界

### WPF 桌面程序

放在 `tools/FlashMall.Launcher`，负责：

- 渲染状态和操作按钮。
- 使用参数列表调用 `wsl.exe`，不拼接未经转义的 shell 输入。
- 异步读取标准输出和错误输出，保持窗口响应。
- 将控制脚本的结构化事件转换为阶段、进度和错误信息。
- 打开本地服务页面。
- 保存非敏感设置，例如 WSL 发行版和工作区路径。

### WSL 控制脚本

新增 `scripts/local/flash-mall-control.sh`，提供稳定命令：

- `start`：检查环境，使用现有镜像启动，等待 Hertz 健康。
- `rebuild`：构建五个默认业务服务镜像，再启动并检查。
- `rebuild-service SERVICE`：重建并重建指定服务容器。
- `stop`：执行 Compose down，保留卷。
- `status`：输出项目总状态和服务状态。
- `logs [SERVICE]`：输出有限行数的近期日志。

控制脚本复用现有 `start-compose-all.sh`、`build-compose-images.sh`、`rebuild-compose-service.sh`、`health-compose.sh` 和 `stop-compose-all.sh`。它是桌面程序与脚本集合之间唯一的稳定接口。

## 数据流与状态协议

控制脚本向标准输出发送逐行 JSON 事件，至少包含 `type`、`phase`、`level`、`message` 和可选的 `service`。人类可读日志写入 `message`，WPF 不通过模糊字符串匹配推断状态。

典型启动阶段为：检查 WSL Docker、检查镜像、启动依赖、初始化数据库、启动服务、Hertz 健康检查、完成。最终 `status` 同时参考 `docker compose ps` 与 `http://127.0.0.1:8889/api/system/health`，不能仅凭容器处于 running 就宣告可用。

## 默认行为与兼容性

- 默认发行版为 `Ubuntu`，默认工作区为 `/home/mildred/code/flash-mall`，可在设置中修改。
- 默认入口为 Hertz：商城 `/shop`，后台 `/admin`，端口 `8889`。
- `entry-api` 保持 Compose profile 隔离，不属于默认启动集合。
- 启动器不启动、停止或重启 Docker Desktop。
- WSL Docker 守护进程不可达时立即报告，不尝试需要管理员权限或 sudo 密码的隐式修复。
- 关闭桌面窗口不会停止已经运行的项目。

## 错误与安全

启动器区分环境缺失、Docker 不可达、端口冲突、镜像缺失、构建失败、容器退出和健康超时。失败事件包含当前阶段、受影响服务和日志尾部；用户可以复制诊断信息或打开日志目录。

第一版不提供删除数据卷、清空数据库、删除镜像或清理全部 Docker 数据的按钮。停止操作只执行无 `--volumes` 的 Compose down。外部链接仅允许设计中声明的本地 HTTP 地址。

## 安装与产物

新增 PowerShell 安装脚本，将 WPF 程序发布到项目 `.runtime/launcher`，并在 Windows 桌面创建“Flash Mall 控制中心”快捷方式。发布目录和用户设置不提交 Git；仓库只提交源代码、项目文件、安装脚本与测试。

安装脚本支持 `-DryRun`，用于验证发行版、工作区、发布目录和快捷方式目标。第一版发布为 Windows x64、依赖本机 .NET 8 Desktop Runtime 的单文件程序，以控制体积；安装脚本在运行前检查运行时并给出明确错误。

## 验证

- 单元测试：命令参数生成、JSON 事件解析、状态机转换、错误归类、URL 白名单。
- 脚本测试：命令参数校验、JSON 输出格式、无效服务名、dry-run。
- 构建验证：WPF Release 发布成功，产物能从桌面快捷方式启动。
- 实际链路：安装、快速启动、Hertz 健康、打开商城和后台、停止、再次启动。
- 隔离验证：全过程无 Docker Desktop 进程调用，legacy `entry-api` 未启动，停止后 MySQL 和 Redis 数据卷仍存在。

## 非目标

第一版不做跨平台桌面应用、不内置 Docker/WSL 安装器、不承担生产部署、不提供数据库管理界面，也不替代 Compose 和现有构建脚本。
