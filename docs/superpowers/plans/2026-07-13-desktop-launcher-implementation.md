# Flash Mall Desktop Launcher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建一个可从 Windows 桌面打开的 .NET 8 WPF 控制中心，用单一 WSL 协议启动、重建、停止并观测当前 Hertz + Go-zero RPC + Kitex 本地栈。

**Architecture:** WPF 只依赖 `IControlRunner`，通过参数化 `wsl.exe` 调用 `scripts/local/flash-mall-control.sh`。控制脚本复用已有构建、Compose 与健康检查脚本并输出逐行 JSON 事件；视图模型消费事件形成有限状态机，安装脚本负责发布和桌面快捷方式。

**Tech Stack:** .NET 8、WPF、xUnit、POSIX shell、PowerShell 7、Ubuntu WSL、Docker Compose。

---

## File map

- `tools/FlashMall.Launcher/FlashMall.Launcher.csproj`: WPF 项目与单文件发布配置。
- `tools/FlashMall.Launcher/App.xaml(.cs)`: 应用入口、依赖组装、启动参数。
- `tools/FlashMall.Launcher/MainWindow.xaml(.cs)`: 纯视图与窗口关闭取消。
- `tools/FlashMall.Launcher/Models/*`: 设置、控制事件、项目状态和命令枚举。
- `tools/FlashMall.Launcher/Services/*`: WSL 参数生成、进程执行、事件解析、设置存储与 URL 白名单。
- `tools/FlashMall.Launcher/ViewModels/*`: 异步命令和主窗口状态机。
- `tools/FlashMall.Launcher.Tests/*`: 不启动真实 WSL 的单元测试。
- `scripts/local/flash-mall-control.sh`: 桌面程序唯一调用的 WSL 控制协议。
- `scripts/local/test-flash-mall-control.sh`: shell 协议测试。
- `scripts/local/install-desktop-launcher.ps1`: 发布程序、写设置、创建桌面快捷方式。
- `scripts/local/test-install-desktop-launcher.ps1`: 安装脚本 dry-run 回归。
- `docs/CURRENT_PROJECT.md`: 记录新的推荐本地启动入口。

### Task 1: 建立可测试的 .NET 项目骨架

**Files:**
- Create: `tools/FlashMall.Launcher/FlashMall.Launcher.csproj`
- Create: `tools/FlashMall.Launcher/App.xaml`
- Create: `tools/FlashMall.Launcher/App.xaml.cs`
- Create: `tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj`
- Create: `tools/FlashMall.Launcher/Models/ControlCommand.cs`
- Create: `tools/FlashMall.Launcher/Models/LauncherSettings.cs`
- Create: `tools/FlashMall.Launcher.Tests/LauncherSettingsTests.cs`

- [ ] **Step 1: Write the failing defaults test**

```csharp
using FlashMall.Launcher.Models;
using Xunit;

namespace FlashMall.Launcher.Tests;

public sealed class LauncherSettingsTests
{
    [Fact]
    public void DefaultsTargetUbuntuHertzWorkspace()
    {
        var settings = LauncherSettings.Default;
        Assert.Equal("Ubuntu", settings.Distro);
        Assert.Equal("/home/mildred/code/flash-mall", settings.Workspace);
        Assert.Equal(180, settings.WaitTimeoutSeconds);
    }
}
```

- [ ] **Step 2: Run the test to verify RED**

Run: `dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj`

Expected: FAIL because the projects and `LauncherSettings` do not exist.

- [ ] **Step 3: Create the two projects and minimal models**

```xml
<!-- tools/FlashMall.Launcher/FlashMall.Launcher.csproj -->
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>WinExe</OutputType>
    <TargetFramework>net8.0-windows</TargetFramework>
    <UseWPF>true</UseWPF>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <PublishSingleFile>true</PublishSingleFile>
    <SelfContained>false</SelfContained>
  </PropertyGroup>
</Project>
```

```xml
<!-- tools/FlashMall.Launcher/App.xaml -->
<Application x:Class="FlashMall.Launcher.App" xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml" />
```

```csharp
namespace FlashMall.Launcher;
public partial class App : System.Windows.Application { }
```

```xml
<!-- tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj -->
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup><TargetFramework>net8.0-windows</TargetFramework><ImplicitUsings>enable</ImplicitUsings><Nullable>enable</Nullable><IsPackable>false</IsPackable></PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.NET.Test.Sdk" Version="17.11.1" />
    <PackageReference Include="xunit" Version="2.9.2" />
    <PackageReference Include="xunit.runner.visualstudio" Version="2.8.2" />
    <ProjectReference Include="../FlashMall.Launcher/FlashMall.Launcher.csproj" />
  </ItemGroup>
</Project>
```

```csharp
namespace FlashMall.Launcher.Models;
public enum ControlCommand { Start, Rebuild, RebuildService, Stop, Status, Logs }
public sealed record LauncherSettings(string Distro, string Workspace, int WaitTimeoutSeconds)
{
    public static LauncherSettings Default { get; } = new("Ubuntu", "/home/mildred/code/flash-mall", 180);
}
```

- [ ] **Step 4: Run GREEN and commit**

Run: `dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj`

Expected: PASS.

```bash
git add tools/FlashMall.Launcher tools/FlashMall.Launcher.Tests
git commit -m "build: initialize desktop launcher projects"
```

### Task 2: 建立稳定的 WSL 控制协议

**Files:**
- Create: `scripts/local/flash-mall-control.sh`
- Create: `scripts/local/test-flash-mall-control.sh`
- Modify: `scripts/local/start-compose-all.sh`

- [ ] **Step 1: Write failing protocol tests**

```sh
#!/usr/bin/env sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
control="$root/scripts/local/flash-mall-control.sh"
trap 'rm -f /tmp/flash-mall-control.out' EXIT

help=$(sh "$control" help)
printf '%s' "$help" | grep -q 'rebuild-service SERVICE'

if sh "$control" rebuild-service unknown >/tmp/flash-mall-control.out 2>&1; then
  echo "unknown service unexpectedly accepted" >&2
  exit 1
fi
grep -q 'invalid_service' /tmp/flash-mall-control.out

if sh "$control" status --wait-timeout nope >/tmp/flash-mall-control.out 2>&1; then
  echo "invalid timeout unexpectedly accepted" >&2
  exit 1
fi
grep -q 'invalid_timeout' /tmp/flash-mall-control.out

events=$(FLASH_MALL_CONTROL_SOURCE_ONLY=1 sh -c '. "$1"; emit progress preflight info "quote: \"safe\""' sh "$control")
printf '%s' "$events" | grep -q '"type":"progress"'
printf '%s' "$events" | grep -q 'quote: \\"safe\\"'
```

- [ ] **Step 2: Run RED**

Run: `sh scripts/local/test-flash-mall-control.sh`

Expected: FAIL because the control script does not exist.

- [ ] **Step 3: Implement command validation and JSON events**

The script must use this command table and must not start the legacy service:

```sh
business_services="auth-api product-rpc order-rpc inventory-kitex hertz-gateway"

is_business_service() {
  case "$1" in
    auth-api|product-rpc|order-rpc|inventory-kitex|hertz-gateway) return 0 ;;
    *) return 1 ;;
  esac
}

is_log_service() {
  case "$1" in
    auth-api|product-rpc|order-rpc|inventory-kitex|hertz-gateway|mysql|redis|rabbitmq|etcd|dtm|jaeger) return 0 ;;
    *) return 1 ;;
  esac
}

json_escape() {
  awk 'BEGIN{ORS=""} {if(NR>1)printf "\\n"; gsub(/\\/,"\\\\"); gsub(/\"/,"\\\""); gsub(/\r/,"\\r"); gsub(/\t/,"\\t"); printf "%s",$0}'
}

emit() {
  type=$(printf '%s' "$1" | json_escape)
  phase=$(printf '%s' "$2" | json_escape)
  level=$(printf '%s' "$3" | json_escape)
  message=$(printf '%s' "$4" | json_escape)
  service=$(printf '%s' "${5:-}" | json_escape)
  code=$(printf '%s' "${6:-}" | json_escape)
  printf '{"type":"%s","phase":"%s","level":"%s","message":"%s","service":"%s","code":"%s"}\n' "$type" "$phase" "$level" "$message" "$service" "$code"
}

emit_service_statuses() {
  (cd "$repo_root/deploy" && docker compose ps --all --format '{{.Service}}|{{.State}}|{{.Status}}') |
    while IFS='|' read -r name state detail; do
      [ -z "$name" ] || emit service status info "$state: $detail" "$name"
    done
}
```

Implement `main` with exact delegation:

```sh
usage() {
  cat <<'EOF'
Usage: flash-mall-control.sh COMMAND [SERVICE] [--wait-timeout SECONDS]
Commands:
  start
  rebuild
  rebuild-service SERVICE
  stop
  status
  logs [SERVICE]
EOF
}

main() {
  script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
  repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
  . "$script_dir/compose-env.sh"
  command="${1:-help}"
  [ "$#" -eq 0 ] || shift
  service=""
  wait_timeout=180

  case "$command" in
    rebuild-service)
      service="${1:-}"
      [ "$#" -eq 0 ] || shift
      ;;
    logs)
      if [ "$#" -gt 0 ] && [ "${1#--}" = "$1" ]; then service="$1"; shift; fi
      ;;
  esac
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --wait-timeout) shift; wait_timeout="${1:-}"; [ -n "$wait_timeout" ] || exit 2 ;;
      *) emit error validation error "unknown_option"; exit 2 ;;
    esac
    shift
  done
  case "$wait_timeout" in ''|*[!0-9]*) emit error validation error "invalid_timeout"; exit 2 ;; esac
  if [ "$wait_timeout" -lt 30 ] || [ "$wait_timeout" -gt 900 ]; then emit error validation error "invalid_timeout"; exit 2; fi

  error_emitted=0
  current_phase="$command"
  trap 'code=$?; if [ "$code" -ne 0 ] && [ "$error_emitted" -eq 0 ]; then emit error "$current_phase" error "Command failed" "" command_failed; fi' EXIT

case "$command" in
  start)
    emit progress preflight info "checking WSL Docker"
    if ! timeout 10 docker info >/dev/null 2>&1; then emit error preflight error "WSL Docker is unreachable" "" docker_unreachable; error_emitted=1; exit 3; fi
    missing=""
    for name in $business_services; do
      image="flash-mall/$name:$FLASH_MALL_IMAGE_TAG"
      docker image inspect "$image" >/dev/null 2>&1 || missing="$missing $image"
    done
    if [ -n "$missing" ]; then emit error preflight error "Missing images:$missing" "" images_missing; error_emitted=1; exit 4; fi
    emit progress startup info "starting existing images"
    "$script_dir/start-compose-all.sh" --no-build --wait-timeout "$wait_timeout"
    emit_service_statuses
    emit state ready info "Flash Mall is ready"
    ;;
  rebuild)
    emit progress build info "building Hertz runtime services"
    # shellcheck disable=SC2086
    "$script_dir/build-compose-images.sh" --tag "$FLASH_MALL_IMAGE_TAG" $business_services
    "$script_dir/start-compose-all.sh" --no-build --wait-timeout "$wait_timeout"
    emit_service_statuses
    emit state ready info "Flash Mall is ready"
    ;;
  rebuild-service)
    is_business_service "$service" || { emit error validation error "invalid_service"; error_emitted=1; exit 2; }
    "$script_dir/rebuild-compose-service.sh" --tag "$FLASH_MALL_IMAGE_TAG" "$service"
    "$script_dir/health-compose.sh" --wait "$wait_timeout" --logs-on-failure
    emit state ready info "$service rebuilt"
    ;;
  stop)
    emit progress stopping info "stopping compose stack"
    "$script_dir/stop-compose-all.sh"
    emit state stopped info "Flash Mall is stopped"
    ;;
  status)
    container_count=$(cd "$repo_root/deploy" && docker compose ps --all -q | awk 'NF{count++} END{print count+0}')
    if [ "$container_count" -eq 0 ]; then emit state stopped info "Flash Mall is stopped"; exit 0; fi
    emit_service_statuses
    if "$script_dir/health-compose.sh"; then emit state ready info "Flash Mall is ready"; else emit state partial warn "Flash Mall is not healthy"; error_emitted=1; exit 1; fi
    ;;
  logs)
    cd "$repo_root/deploy"
    if [ -n "$service" ]; then is_log_service "$service" || { emit error validation error "invalid_service"; error_emitted=1; exit 2; }; docker compose logs --tail 160 "$service"; else docker compose logs --tail 80 $business_services; fi
    ;;
  help) usage ;;
  *) emit error validation error "unknown_command"; error_emitted=1; usage >&2; exit 2 ;;
esac
}

if [ "${FLASH_MALL_CONTROL_SOURCE_ONLY:-0}" != "1" ]; then
  main "$@"
fi
```

Keep child command output as raw log lines; only lifecycle events are JSON. Add the usage text listing all six commands, then run `chmod +x scripts/local/flash-mall-control.sh scripts/local/test-flash-mall-control.sh`.

Also remove the stale `entry-api: http://127.0.0.1:8888` success line from `start-compose-all.sh` and add `jaeger: http://127.0.0.1:16686`; the default Compose profile does not start entry-api.

- [ ] **Step 4: Run shell tests and syntax checks**

Run: `sh -n scripts/local/flash-mall-control.sh && sh scripts/local/test-flash-mall-control.sh`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/local/flash-mall-control.sh scripts/local/test-flash-mall-control.sh scripts/local/start-compose-all.sh
git commit -m "feat: add WSL launcher control protocol"
```

### Task 3: 实现参数化 WSL 执行与事件解析

**Files:**
- Create: `tools/FlashMall.Launcher/Models/LauncherEvent.cs`
- Create: `tools/FlashMall.Launcher/Models/ControlRunResult.cs`
- Create: `tools/FlashMall.Launcher/Services/WslCommandBuilder.cs`
- Create: `tools/FlashMall.Launcher/Services/LauncherEventParser.cs`
- Create: `tools/FlashMall.Launcher/Services/IControlRunner.cs`
- Create: `tools/FlashMall.Launcher/Services/WslControlRunner.cs`
- Create: `tools/FlashMall.Launcher.Tests/WslCommandBuilderTests.cs`
- Create: `tools/FlashMall.Launcher.Tests/LauncherEventParserTests.cs`

- [ ] **Step 1: Write failing builder and parser tests**

```csharp
[Fact]
public void BuilderUsesArgumentListWithoutShellConcatenation()
{
    var psi = WslCommandBuilder.Build(LauncherSettings.Default, ControlCommand.RebuildService, "order-rpc");
    Assert.Equal("wsl.exe", psi.FileName);
    Assert.Equal(new[] { "-d", "Ubuntu", "--cd", "/home/mildred/code/flash-mall", "--", "./scripts/local/flash-mall-control.sh", "rebuild-service", "order-rpc", "--wait-timeout", "180" }, psi.ArgumentList);
    Assert.True(psi.RedirectStandardOutput);
    Assert.True(psi.RedirectStandardError);
}

[Fact]
public void ParserSeparatesEventsFromRawLogs()
{
    Assert.True(LauncherEventParser.TryParse("{\"type\":\"state\",\"phase\":\"ready\",\"level\":\"info\",\"message\":\"ok\"}", out var evt));
    Assert.Equal("ready", evt.Phase);
    Assert.False(LauncherEventParser.TryParse("docker build output", out _));
}
```

- [ ] **Step 2: Run RED**

Run: `dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj --filter 'WslCommandBuilderTests|LauncherEventParserTests'`

Expected: FAIL because the services do not exist.

- [ ] **Step 3: Implement the public contracts**

```csharp
namespace FlashMall.Launcher.Models;
public sealed record LauncherEvent(string Type, string Phase, string Level, string Message, string? Service = null, string? Code = null);

public sealed record ControlRunResult(int ExitCode, IReadOnlyList<string> StandardError);
```

```csharp
public interface IControlRunner
{
    Task<ControlRunResult> RunAsync(
        LauncherSettings settings,
        ControlCommand command,
        string? service,
        Action<LauncherEvent> onEvent,
        Action<string> onLog,
        CancellationToken cancellationToken);
}
```

`WslCommandBuilder.Build` must reject unknown service names and create `ProcessStartInfo` with `UseShellExecute=false`, redirects enabled, `CreateNoWindow=true`, and one `ArgumentList` item per argument. `RebuildService` accepts only the five business services; `Logs` additionally accepts mysql, redis, rabbitmq, etcd, dtm, and jaeger. Map enum values to `start`, `rebuild`, `rebuild-service`, `stop`, `status`, and `logs`.

`LauncherEventParser.TryParse` must use `System.Text.Json`, return false for malformed or non-JSON lines, and require non-empty `Type`, `Phase`, `Level`, and `Message`.

- [ ] **Step 4: Implement asynchronous execution**

```csharp
using var process = new Process { StartInfo = WslCommandBuilder.Build(settings, command, service) };
process.Start();
var stdout = Task.Run(async () => {
    while (await process.StandardOutput.ReadLineAsync(cancellationToken) is { } line)
        if (LauncherEventParser.TryParse(line, out var evt)) onEvent(evt); else onLog(line);
}, cancellationToken);
var errors = new List<string>();
var stderr = Task.Run(async () => {
    while (await process.StandardError.ReadLineAsync(cancellationToken) is { } line) { errors.Add(line); onLog(line); }
}, cancellationToken);
try {
    await Task.WhenAll(process.WaitForExitAsync(cancellationToken), stdout, stderr);
} catch (OperationCanceledException) {
    if (!process.HasExited) process.Kill(entireProcessTree: true);
    throw;
}
return new ControlRunResult(process.ExitCode, errors);
```

- [ ] **Step 5: Run GREEN and commit**

Run: `dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj`

Expected: PASS.

```bash
git add tools/FlashMall.Launcher tools/FlashMall.Launcher.Tests
git commit -m "feat: execute launcher commands through WSL"
```

### Task 4: 实现设置存储、URL 白名单和有限状态机

**Files:**
- Create: `tools/FlashMall.Launcher/Models/ProjectState.cs`
- Create: `tools/FlashMall.Launcher/Models/LauncherErrorKind.cs`
- Create: `tools/FlashMall.Launcher/Models/ServiceStatusItem.cs`
- Create: `tools/FlashMall.Launcher/Services/ILauncherSettingsStore.cs`
- Create: `tools/FlashMall.Launcher/Services/ILocalUrlService.cs`
- Create: `tools/FlashMall.Launcher/Services/ILauncherLogStore.cs`
- Create: `tools/FlashMall.Launcher/Services/IUiDispatcher.cs`
- Create: `tools/FlashMall.Launcher/Services/LauncherSettingsStore.cs`
- Create: `tools/FlashMall.Launcher/Services/LocalUrlService.cs`
- Create: `tools/FlashMall.Launcher/Services/LauncherLogStore.cs`
- Create: `tools/FlashMall.Launcher/Services/WpfUiDispatcher.cs`
- Create: `tools/FlashMall.Launcher/Services/LauncherErrorClassifier.cs`
- Create: `tools/FlashMall.Launcher/Services/StartupArguments.cs`
- Create: `tools/FlashMall.Launcher/ViewModels/AsyncCommand.cs`
- Create: `tools/FlashMall.Launcher/ViewModels/MainWindowViewModel.cs`
- Create: `tools/FlashMall.Launcher.Tests/MainWindowViewModelTests.cs`
- Create: `tools/FlashMall.Launcher.Tests/LocalUrlServiceTests.cs`
- Create: `tools/FlashMall.Launcher.Tests/LauncherErrorClassifierTests.cs`
- Create: `tools/FlashMall.Launcher.Tests/LauncherLogStoreTests.cs`
- Create: `tools/FlashMall.Launcher.Tests/StartupArgumentsTests.cs`
- Create: `tools/FlashMall.Launcher.Tests/TestDoubles.cs`

- [ ] **Step 1: Write failing state and URL tests**

```csharp
[Fact]
public async Task StartTransitionsThroughBusyToReady()
{
    var runner = new FakeRunner(new LauncherEvent("state", "ready", "info", "ready"), 0);
    var vm = new MainWindowViewModel(runner, new MemorySettingsStore(), new FakeUrlService(), new MemoryLogStore(), new ImmediateDispatcher(), LauncherSettings.Default);
    await vm.ExecuteAsync(ControlCommand.Start);
    Assert.Equal(ProjectState.Ready, vm.State);
    Assert.False(vm.IsBusy);
}

[Theory]
[InlineData("http://127.0.0.1:8889/shop", true)]
[InlineData("http://127.0.0.1:8889/admin", true)]
[InlineData("https://example.com", false)]
public void OnlyDeclaredLocalUrlsAreAllowed(string url, bool allowed) =>
    Assert.Equal(allowed, new LocalUrlService().IsAllowed(url));

[Theory]
[InlineData("docker_unreachable", "", LauncherErrorKind.DockerUnavailable)]
[InlineData("", "bind: address already in use", LauncherErrorKind.PortConflict)]
[InlineData("", "gateway health not ready", LauncherErrorKind.HealthTimeout)]
public void ErrorsAreClassified(string code, string log, LauncherErrorKind expected) =>
    Assert.Equal(expected, LauncherErrorClassifier.Classify(code, log));

[Fact]
public void ExplicitStartupArgumentsOverrideSavedSettings()
{
    var actual = StartupArguments.Apply(LauncherSettings.Default, new[] { "--distro", "Debian", "--workspace", "/srv/flash-mall" });
    Assert.Equal("Debian", actual.Distro);
    Assert.Equal("/srv/flash-mall", actual.Workspace);
}

[Fact]
public void LogStoreWritesInsideConfiguredRoot()
{
    var root = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
    try {
        var store = new LauncherLogStore(root);
        store.Append("diagnostic line");
        Assert.Contains("diagnostic line", File.ReadAllText(Directory.GetFiles(root).Single()));
    } finally { if (Directory.Exists(root)) Directory.Delete(root, true); }
}
```

Use these deterministic test doubles:

```csharp
internal sealed class FakeRunner(LauncherEvent evt, int exitCode) : IControlRunner
{
    public Task<ControlRunResult> RunAsync(LauncherSettings settings, ControlCommand command, string? service,
        Action<LauncherEvent> onEvent, Action<string> onLog, CancellationToken cancellationToken)
    {
        onEvent(evt);
        return Task.FromResult(new ControlRunResult(exitCode, Array.Empty<string>()));
    }
}

internal sealed class MemorySettingsStore : ILauncherSettingsStore
{
    public LauncherSettings Value { get; private set; } = LauncherSettings.Default;
    public LauncherSettings Load() => Value;
    public Task SaveAsync(LauncherSettings settings, CancellationToken cancellationToken = default)
    { Value = settings; return Task.CompletedTask; }
}

internal sealed class FakeUrlService : ILocalUrlService
{
    public string? Opened { get; private set; }
    public bool IsAllowed(string url) => url.StartsWith("http://127.0.0.1:", StringComparison.Ordinal);
    public void Open(string url) { if (!IsAllowed(url)) throw new InvalidOperationException(); Opened = url; }
}

internal sealed class MemoryLogStore : ILauncherLogStore
{
    public List<string> Lines { get; } = new();
    public string DirectoryPath => "memory";
    public void Append(string line) => Lines.Add(line);
    public void OpenDirectory() { }
}

internal sealed class ImmediateDispatcher : IUiDispatcher
{
    public void Post(Action action) => action();
}
```

- [ ] **Step 2: Run RED**

Run: `dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj --filter 'MainWindowViewModelTests|LocalUrlServiceTests|LauncherErrorClassifierTests|LauncherLogStoreTests|StartupArgumentsTests'`

Expected: FAIL because the state machine does not exist.

- [ ] **Step 3: Implement settings and state contracts**

```csharp
public enum ProjectState { Stopped, Starting, Building, Ready, Partial, Stopping, Failed }
public enum LauncherErrorKind { None, EnvironmentMissing, DockerUnavailable, PortConflict, ImagesMissing, BuildFailed, ContainerExited, HealthTimeout, Unknown }
public sealed record ServiceStatusItem(string Name, string State, string Detail);
```

```csharp
public interface ILauncherSettingsStore
{
    LauncherSettings Load();
    Task SaveAsync(LauncherSettings settings, CancellationToken cancellationToken = default);
}

public interface ILocalUrlService
{
    bool IsAllowed(string url);
    void Open(string url);
}

public interface ILauncherLogStore
{
    string DirectoryPath { get; }
    void Append(string line);
    void OpenDirectory();
}

public interface IUiDispatcher
{
    void Post(Action action);
}
```

Store settings as JSON at `Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "FlashMall", "launcher.json")`. Validate distro and workspace are non-empty and timeout is between 30 and 900 seconds; invalid files fall back to `LauncherSettings.Default` and are preserved as `.invalid` for diagnosis.

`LauncherLogStore` writes UTF-8 lines to `%LOCALAPPDATA%\FlashMall\logs\launcher-YYYYMMDD.log`, creates the directory lazily, and opens only that fixed directory through `explorer.exe`. Test it with a constructor-supplied temporary root so the test never touches the real profile.

`LauncherErrorClassifier.Classify(code, combinedLog)` must prefer protocol codes (`docker_unreachable`, `images_missing`) and otherwise match bounded phrases for `address already in use`, `exited`, `build failed`, and `health not ready`. It returns `Unknown` rather than guessing when no rule matches.

`MainWindowViewModel` has the exact constructor `(IControlRunner runner, ILauncherSettingsStore settingsStore, ILocalUrlService urls, ILauncherLogStore logs, IUiDispatcher dispatcher, LauncherSettings settings)`. Runner callbacks must enter through `dispatcher.Post` before changing properties or `ObservableCollection`; `WpfUiDispatcher` uses synchronous `Dispatcher.Invoke` (or runs directly when `CheckAccess()` is true), while tests use `ImmediateDispatcher`. `ExecuteAsync` must reject re-entry while `IsBusy`, map commands to initial states, append raw logs to both the UI and log store with a 5,000-line UI cap, consume `state/ready`, `state/stopped`, `state/partial`, and `error/*` events, and set `Failed` on a non-zero exit without a more specific state event. For `type=service`, upsert `ServiceStatusItem` by event `Service`, using the first segment of `Message` as state and the full message as detail.

Expose bindable `State`, `StateText`, `PhaseText`, `LogText`, `IsBusy`, `Distro`, `Workspace`, `SelectedService`, the five-name `Services` list, and `ObservableCollection<ServiceStatusItem> ServiceStatuses`. Expose `StartCommand`, `RebuildCommand`, `StopCommand`, `RefreshCommand`, `RebuildServiceCommand`, `LoadLogsCommand`, `OpenUrlCommand`, `SaveSettingsCommand`, and `OpenLogDirectoryCommand`. `CancelActiveOperation()` cancels only the current runner token. Every `AsyncCommand` raises `CanExecuteChanged` when `IsBusy` changes; start/rebuild/stop/service actions are disabled while busy, while URL and log-directory actions remain independent.

- [ ] **Step 4: Run GREEN and commit**

Run: `dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj`

Expected: PASS.

```bash
git add tools/FlashMall.Launcher tools/FlashMall.Launcher.Tests
git commit -m "feat: add launcher state and settings model"
```

### Task 5: 构建 WPF 主窗口

**Files:**
- Modify: `tools/FlashMall.Launcher/App.xaml`
- Modify: `tools/FlashMall.Launcher/App.xaml.cs`
- Create: `tools/FlashMall.Launcher/MainWindow.xaml`
- Create: `tools/FlashMall.Launcher/MainWindow.xaml.cs`

- [ ] **Step 1: Add an application smoke test**

```csharp
[Fact]
public void MainWindowTypeCanBeLoaded()
{
    var type = typeof(FlashMall.Launcher.MainWindow);
    Assert.Equal("Flash Mall 控制中心", MainWindow.WindowTitle);
    Assert.NotNull(type);
}
```

- [ ] **Step 2: Run RED**

Run: `dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj --filter MainWindowTypeCanBeLoaded`

Expected: FAIL because `MainWindow` does not exist.

- [ ] **Step 3: Implement application composition**

`App.OnStartup` must load settings, construct `WslControlRunner`, `LocalUrlService`, and `MainWindowViewModel`, assign `DataContext`, show the window, then asynchronously run `Status`. Parse optional `--distro` and `--workspace` argument pairs before loading saved settings; explicit arguments win for the current run.

`MainWindow` must expose `public const string WindowTitle = "Flash Mall 控制中心"` and cancel only the active runner token during `Closing`; it must never call `stop` merely because the window closes.

```csharp
protected override async void OnStartup(StartupEventArgs e)
{
    base.OnStartup(e);
    var store = new LauncherSettingsStore();
    var settings = StartupArguments.Apply(store.Load(), e.Args);
    var vm = new MainWindowViewModel(new WslControlRunner(), store, new LocalUrlService(), new LauncherLogStore(), new WpfUiDispatcher(Dispatcher), settings);
    var window = new MainWindow { DataContext = vm };
    window.Closing += (_, _) => vm.CancelActiveOperation();
    window.Show();
    await vm.ExecuteAsync(ControlCommand.Status);
}

public partial class MainWindow : Window
{
    public const string WindowTitle = "Flash Mall 控制中心";
    public MainWindow() => InitializeComponent();
}
```

- [ ] **Step 4: Implement the complete view**

Use a two-column WPF layout:

```xml
<Window Title="Flash Mall 控制中心" Width="1040" Height="680" MinWidth="900" MinHeight="580">
  <Grid Margin="24">
    <Grid.ColumnDefinitions><ColumnDefinition Width="360"/><ColumnDefinition Width="24"/><ColumnDefinition Width="*"/></Grid.ColumnDefinitions>
    <StackPanel Grid.Column="0">
      <TextBlock Text="Flash Mall" FontSize="28" FontWeight="Bold"/>
      <Border Margin="0,20,0,16" Padding="16" CornerRadius="10">
        <StackPanel><TextBlock Text="{Binding StateText}" FontSize="20"/><TextBlock Text="{Binding PhaseText}" TextWrapping="Wrap"/></StackPanel>
      </Border>
      <Button Content="启动项目" Command="{Binding StartCommand}" Height="44"/>
      <Button Content="重新构建并启动" Command="{Binding RebuildCommand}" Height="44" Margin="0,8,0,0"/>
      <Button Content="停止项目" Command="{Binding StopCommand}" Height="40" Margin="0,8,0,0"/>
      <WrapPanel Margin="0,18,0,0">
        <Button Content="商城" Command="{Binding OpenUrlCommand}" CommandParameter="http://127.0.0.1:8889/shop"/>
        <Button Content="后台" Command="{Binding OpenUrlCommand}" CommandParameter="http://127.0.0.1:8889/admin"/>
        <Button Content="RabbitMQ" Command="{Binding OpenUrlCommand}" CommandParameter="http://127.0.0.1:15672"/>
        <Button Content="Jaeger" Command="{Binding OpenUrlCommand}" CommandParameter="http://127.0.0.1:16686"/>
      </WrapPanel>
      <Expander Header="高级操作" Margin="0,20,0,0">
        <StackPanel>
          <ComboBox ItemsSource="{Binding Services}" SelectedItem="{Binding SelectedService}"/>
          <Button Content="重建选中服务" Command="{Binding RebuildServiceCommand}"/>
          <Button Content="查看选中服务日志" Command="{Binding LoadLogsCommand}"/>
          <ItemsControl ItemsSource="{Binding ServiceStatuses}">
            <ItemsControl.ItemTemplate><DataTemplate><StackPanel Orientation="Horizontal"><TextBlock Text="{Binding Name}" FontWeight="SemiBold"/><TextBlock Text="{Binding Detail}" Margin="8,0,0,0"/></StackPanel></DataTemplate></ItemsControl.ItemTemplate>
          </ItemsControl>
          <TextBox Text="{Binding Distro}"/><TextBox Text="{Binding Workspace}"/>
          <Button Content="保存设置" Command="{Binding SaveSettingsCommand}"/>
        </StackPanel>
      </Expander>
    </StackPanel>
    <Grid Grid.Column="2">
      <Grid.RowDefinitions><RowDefinition Height="Auto"/><RowDefinition Height="*"/></Grid.RowDefinitions>
      <DockPanel><TextBlock Text="运行日志（可选择并复制）" FontSize="18" FontWeight="SemiBold"/><Button DockPanel.Dock="Right" Content="打开日志目录" Command="{Binding OpenLogDirectoryCommand}"/></DockPanel>
      <TextBox Grid.Row="1" Text="{Binding LogText}" IsReadOnly="True" FontFamily="Consolas" TextWrapping="NoWrap" VerticalScrollBarVisibility="Auto" HorizontalScrollBarVisibility="Auto"/>
    </Grid>
  </Grid>
</Window>
```

Add styles and state colors in `Window.Resources`; do not introduce a third-party UI package.

- [ ] **Step 5: Verify tests and Release build**

Run: `dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj && dotnet build tools/FlashMall.Launcher/FlashMall.Launcher.csproj -c Release`

Expected: PASS and build exit 0.

- [ ] **Step 6: Commit**

```bash
git add tools/FlashMall.Launcher tools/FlashMall.Launcher.Tests
git commit -m "feat: build Flash Mall desktop control center"
```

### Task 6: 发布程序并创建桌面快捷方式

**Files:**
- Create: `scripts/local/install-desktop-launcher.ps1`
- Create: `scripts/local/test-install-desktop-launcher.ps1`

- [ ] **Step 1: Write failing dry-run test**

```powershell
$result = & "$PSScriptRoot/install-desktop-launcher.ps1" -DryRun -Distro Ubuntu -Workspace /home/mildred/code/flash-mall | ConvertFrom-Json
if ($result.distro -ne 'Ubuntu') { throw 'unexpected distro' }
if ($result.workspace -ne '/home/mildred/code/flash-mall') { throw 'unexpected workspace' }
if ($result.executable -notlike '*\.runtime\launcher\FlashMall.Launcher.exe') { throw 'unexpected executable' }
if ($result.shortcut -notlike '*\Desktop\Flash Mall 控制中心.lnk') { throw 'unexpected shortcut' }
```

- [ ] **Step 2: Run RED**

Run: `pwsh -NoProfile -File scripts/local/test-install-desktop-launcher.ps1`

Expected: FAIL because the installer does not exist.

- [ ] **Step 3: Implement dry-run and installation**

The installer parameters are:

```powershell
param(
  [string]$Distro = 'Ubuntu',
  [string]$Workspace = '/home/mildred/code/flash-mall',
  [switch]$DryRun,
  [switch]$NoShortcut
)
```

Resolve the repository from `$PSScriptRoot\..\..`, publish with:

```powershell
dotnet publish "$repoRoot\tools\FlashMall.Launcher\FlashMall.Launcher.csproj" `
  -c Release -r win-x64 --self-contained false `
  -p:PublishSingleFile=true -o "$repoRoot\.runtime\launcher"
```

Before publishing, verify `dotnet --list-runtimes` contains `Microsoft.WindowsDesktop.App 8.`. Create the shortcut with `WScript.Shell`; set target to the published exe, arguments to `--distro "Ubuntu" --workspace "/home/mildred/code/flash-mall"`, working directory to the repository, and description to `启动并管理 Flash Mall WSL 本地环境`.

`-DryRun` must perform no writes and output one JSON object with `distro`, `workspace`, `publishDirectory`, `executable`, and `shortcut`.

- [ ] **Step 4: Run GREEN, publish, and verify shortcut target**

Run:

```powershell
pwsh -NoProfile -File scripts/local/test-install-desktop-launcher.ps1
pwsh -NoProfile -File scripts/local/install-desktop-launcher.ps1 -NoShortcut
Test-Path .runtime\launcher\FlashMall.Launcher.exe
```

Expected: test passes and `Test-Path` returns `True`.

- [ ] **Step 5: Commit**

```bash
git add scripts/local/install-desktop-launcher.ps1 scripts/local/test-install-desktop-launcher.ps1
git commit -m "feat: install desktop launcher shortcut"
```

### Task 7: 文档与完整自动验证

**Files:**
- Modify: `docs/CURRENT_PROJECT.md`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Add launcher checks to fast CI**

Add this `changes.outputs` entry and path filter:

```yaml
outputs:
  desktop_launcher: ${{ steps.filter.outputs.desktop_launcher }}
# inside filters: |
desktop_launcher:
  - 'tools/FlashMall.Launcher/**'
  - 'tools/FlashMall.Launcher.Tests/**'
  - 'scripts/local/flash-mall-control.sh'
  - 'scripts/local/test-flash-mall-control.sh'
  - 'scripts/local/start-compose-all.sh'
  - 'scripts/local/install-desktop-launcher.ps1'
  - 'scripts/local/test-install-desktop-launcher.ps1'
  - '.github/workflows/ci.yml'
```

Add a Windows job guarded by that output:

```yaml
desktop-launcher:
  needs: changes
  if: needs.changes.outputs.desktop_launcher == 'true'
  runs-on: windows-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-dotnet@v4
      with:
        dotnet-version: '8.0.x'
    - run: dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj -c Release
    - shell: bash
      run: sh -n scripts/local/flash-mall-control.sh && sh scripts/local/test-flash-mall-control.sh
    - shell: pwsh
      run: ./scripts/local/test-install-desktop-launcher.ps1
    - run: dotnet publish tools/FlashMall.Launcher/FlashMall.Launcher.csproj -c Release -r win-x64 --self-contained false -p:PublishSingleFile=true
```

Add `desktop-launcher` to `fast-ci.needs`, expose `DESKTOP_RESULT: ${{ needs.desktop-launcher.result }}`, and include `$DESKTOP_RESULT` in the existing result loop so only `success` or `skipped` is accepted.

- [ ] **Step 2: Document the recommended entry**

Add a “Windows 桌面控制中心” section to `docs/CURRENT_PROJECT.md` with these exact commands:

```powershell
# 首次安装或更新启动器
pwsh -NoProfile -File scripts/local/install-desktop-launcher.ps1

# 不创建快捷方式，只验证发布
pwsh -NoProfile -File scripts/local/install-desktop-launcher.ps1 -NoShortcut
```

State explicitly that the shortcut controls Ubuntu WSL Docker, Hertz is on port 8889, default stop preserves volumes, and legacy `entry-api` is excluded.

- [ ] **Step 3: Run all static and unit verification**

Run:

```powershell
dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj -c Release
dotnet publish tools/FlashMall.Launcher/FlashMall.Launcher.csproj -c Release -r win-x64 --self-contained false -p:PublishSingleFile=true -o .runtime/launcher
pwsh -NoProfile -File scripts/local/test-install-desktop-launcher.ps1
wsl.exe -d Ubuntu --cd /home/mildred/code/flash-mall -- sh -n scripts/local/flash-mall-control.sh
wsl.exe -d Ubuntu --cd /home/mildred/code/flash-mall -- sh scripts/local/test-flash-mall-control.sh
git diff --check
```

Expected: every command exits 0.

- [ ] **Step 4: Commit CI and documentation**

```bash
git add .github/workflows/ci.yml docs/CURRENT_PROJECT.md
git commit -m "ci: verify desktop launcher"
```

### Task 8: 实际安装与端到端启动验收

**Files:**
- No tracked file changes expected.

- [ ] **Step 1: Capture the pre-test environment**

Run:

```powershell
Get-Process 'Docker Desktop' -ErrorAction SilentlyContinue
wsl.exe -d Ubuntu -- bash -lc 'docker info >/dev/null && docker volume ls --format "{{.Name}}" | sort'
```

Record the volume list. Do not start Docker Desktop; Ubuntu Docker must already be reachable or the test stops with the launcher’s environment error.

- [ ] **Step 2: Install the real desktop shortcut and launch the window**

Run:

```powershell
pwsh -NoProfile -File scripts/local/install-desktop-launcher.ps1
$exe = Resolve-Path .runtime\launcher\FlashMall.Launcher.exe
$process = Start-Process -FilePath $exe -PassThru
Start-Sleep -Seconds 3
$process.Refresh()
if ($process.HasExited) { throw 'launcher exited unexpectedly' }
```

Expected: a visible window titled `Flash Mall 控制中心` and a desktop shortcut with the same name.

- [ ] **Step 3: Exercise rebuild, health, and isolation through the control boundary**

Run:

```bash
scripts/local/flash-mall-control.sh rebuild --wait-timeout 240
curl --noproxy '*' -fsS http://127.0.0.1:8889/api/system/health | grep -q '"overall":true'
curl --noproxy '*' -fsS http://127.0.0.1:8889/shop >/dev/null
curl --noproxy '*' -fsS http://127.0.0.1:8889/admin >/dev/null
if docker ps --format '{{.Names}}' | grep -qx entry-api; then echo 'legacy entry-api is running' >&2; exit 1; fi
```

Expected: all checks pass and only the Hertz runtime stack is active.

- [ ] **Step 4: Verify stop preserves data and repeatability**

Run:

```bash
before=$(docker volume ls --format '{{.Name}}' | sort)
scripts/local/flash-mall-control.sh stop
after=$(docker volume ls --format '{{.Name}}' | sort)
test "$before" = "$after"
scripts/local/flash-mall-control.sh start --wait-timeout 180
scripts/local/flash-mall-control.sh status
scripts/local/flash-mall-control.sh stop
```

Expected: volume lists are identical, second start becomes healthy, and final stop succeeds.

- [ ] **Step 5: Close only the launcher and verify no orphaned test process**

Run:

```powershell
if (-not $process.HasExited) { $process.CloseMainWindow() | Out-Null; $process.WaitForExit(10000) | Out-Null }
if (Get-Process FlashMall.Launcher -ErrorAction SilentlyContinue) { throw 'launcher process still running' }
```

Expected: launcher exits, Compose remains stopped from Step 4, and Docker Desktop was never started.

### Task 9: Final repository verification and handoff

- [ ] Run `git status --short` and confirm only intended tracked changes are committed.
- [ ] Run `dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj -c Release`.
- [ ] Run `go test ./... -count=1` to ensure the launcher work did not regress the application.
- [ ] Run `git diff codex/arch-hertz-kitex...HEAD --check` and review the complete diff.
- [ ] Push `codex/desktop-launcher`, create a Chinese PR targeting `codex/arch-hertz-kitex`, monitor CI, and do not target `main`.
