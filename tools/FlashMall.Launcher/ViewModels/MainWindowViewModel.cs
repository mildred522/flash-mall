using System.Collections.ObjectModel;
using System.ComponentModel;
using System.IO;
using System.Runtime.CompilerServices;
using FlashMall.Launcher.Models;
using FlashMall.Launcher.Services;

namespace FlashMall.Launcher.ViewModels;

public sealed class MainWindowViewModel : INotifyPropertyChanged
{
    private const int MaximumUiLogLines = 5_000;
    private static readonly string[] BusinessServices =
        ["auth-api", "product-rpc", "order-rpc", "inventory-kitex", "hertz-gateway"];

    private readonly IControlRunner _runner;
    private readonly ILauncherSettingsStore _settingsStore;
    private readonly ILocalUrlService _urls;
    private readonly ILauncherLogStore _logs;
    private readonly IUiDispatcher _dispatcher;
    private readonly Queue<string> _uiLogLines = new();
    private CancellationTokenSource? _activeCancellation;
    private ProjectState _state = ProjectState.Stopped;
    private string _phaseText = "正在读取项目状态…";
    private string _logText = string.Empty;
    private bool _isBusy;
    private string _distro;
    private string _workspace;
    private int _waitTimeoutSeconds;
    private string _selectedService;

    public MainWindowViewModel(
        IControlRunner runner,
        ILauncherSettingsStore settingsStore,
        ILocalUrlService urls,
        ILauncherLogStore logs,
        IUiDispatcher dispatcher,
        LauncherSettings settings)
    {
        _runner = runner;
        _settingsStore = settingsStore;
        _urls = urls;
        _logs = logs;
        _dispatcher = dispatcher;
        _distro = settings.Distro;
        _workspace = settings.Workspace;
        _waitTimeoutSeconds = settings.WaitTimeoutSeconds;
        _selectedService = BusinessServices[0];

        StartCommand = BusyCommand(ControlCommand.Start);
        RebuildCommand = BusyCommand(ControlCommand.Rebuild);
        StopCommand = BusyCommand(ControlCommand.Stop);
        RefreshCommand = BusyCommand(ControlCommand.Status);
        RebuildServiceCommand = new AsyncCommand(
            _ => ExecuteAsync(ControlCommand.RebuildService, SelectedService),
            _ => !IsBusy && !string.IsNullOrWhiteSpace(SelectedService));
        LoadLogsCommand = new AsyncCommand(
            _ => ExecuteAsync(ControlCommand.Logs, SelectedService),
            _ => !IsBusy && !string.IsNullOrWhiteSpace(SelectedService));
        OpenUrlCommand = new AsyncCommand(parameter =>
        {
            OpenUrl(parameter as string);
            return Task.CompletedTask;
        });
        SaveSettingsCommand = new AsyncCommand(_ => SaveSettingsAsync(), _ => !IsBusy);
        OpenLogDirectoryCommand = new AsyncCommand(_ =>
        {
            OpenLogDirectory();
            return Task.CompletedTask;
        });
    }

    public event PropertyChangedEventHandler? PropertyChanged;

    public ProjectState State
    {
        get => _state;
        private set
        {
            if (SetField(ref _state, value))
            {
                OnPropertyChanged(nameof(StateText));
            }
        }
    }

    public string StateText => State switch
    {
        ProjectState.Stopped => "已停止",
        ProjectState.Starting => "正在启动",
        ProjectState.Building => "正在构建",
        ProjectState.Ready => "运行正常",
        ProjectState.Partial => "部分可用",
        ProjectState.Stopping => "正在停止",
        ProjectState.Failed => "操作失败",
        _ => State.ToString(),
    };

    public string PhaseText
    {
        get => _phaseText;
        private set => SetField(ref _phaseText, value);
    }

    public string LogText
    {
        get => _logText;
        private set => SetField(ref _logText, value);
    }

    public bool IsBusy
    {
        get => _isBusy;
        private set
        {
            if (SetField(ref _isBusy, value))
            {
                RaiseCommandStates();
            }
        }
    }

    public string Distro
    {
        get => _distro;
        set => SetField(ref _distro, value);
    }

    public string Workspace
    {
        get => _workspace;
        set => SetField(ref _workspace, value);
    }

    public int WaitTimeoutSeconds
    {
        get => _waitTimeoutSeconds;
        set => SetField(ref _waitTimeoutSeconds, value);
    }

    public string SelectedService
    {
        get => _selectedService;
        set
        {
            if (SetField(ref _selectedService, value))
            {
                RebuildServiceCommand.RaiseCanExecuteChanged();
                LoadLogsCommand.RaiseCanExecuteChanged();
            }
        }
    }

    public IReadOnlyList<string> Services => BusinessServices;
    public ObservableCollection<ServiceStatusItem> ServiceStatuses { get; } = [];

    public AsyncCommand StartCommand { get; }
    public AsyncCommand RebuildCommand { get; }
    public AsyncCommand StopCommand { get; }
    public AsyncCommand RefreshCommand { get; }
    public AsyncCommand RebuildServiceCommand { get; }
    public AsyncCommand LoadLogsCommand { get; }
    public AsyncCommand OpenUrlCommand { get; }
    public AsyncCommand SaveSettingsCommand { get; }
    public AsyncCommand OpenLogDirectoryCommand { get; }

    public async Task ExecuteAsync(ControlCommand command, string? service = null)
    {
        if (IsBusy)
        {
            return;
        }

        IsBusy = true;
        State = InitialState(command, State);
        PhaseText = InitialPhase(command);
        var cancellation = new CancellationTokenSource();
        _activeCancellation = cancellation;

        try
        {
            var settings = CurrentSettings();
            var result = await _runner.RunAsync(
                settings,
                command,
                service,
                evt => _dispatcher.Post(() => HandleEvent(evt)),
                line => _dispatcher.Post(() => AppendLog(line)),
                cancellation.Token);

            _dispatcher.Post(() =>
            {
                if (result.ExitCode != 0 && State is not ProjectState.Partial and not ProjectState.Failed)
                {
                    State = ProjectState.Failed;
                    PhaseText = DescribeError(LauncherErrorClassifier.Classify(
                        null,
                        string.Join(Environment.NewLine, result.StandardError)));
                }
            });
        }
        catch (OperationCanceledException)
        {
            _dispatcher.Post(() => PhaseText = "当前操作已取消；已启动的 Docker 服务不会因此停止。");
        }
        catch (Exception exception)
        {
            _dispatcher.Post(() =>
            {
                AppendLog(exception.ToString());
                State = ProjectState.Failed;
                PhaseText = DescribeError(LauncherErrorClassifier.Classify(null, exception.Message));
            });
        }
        finally
        {
            _dispatcher.Post(() => IsBusy = false);
            if (ReferenceEquals(_activeCancellation, cancellation))
            {
                _activeCancellation = null;
            }
            cancellation.Dispose();
        }
    }

    public void CancelActiveOperation() => _activeCancellation?.Cancel();

    private AsyncCommand BusyCommand(ControlCommand command) =>
        new(_ => ExecuteAsync(command), _ => !IsBusy);

    private LauncherSettings CurrentSettings() =>
        new(Distro.Trim(), Workspace.Trim(), WaitTimeoutSeconds);

    private async Task SaveSettingsAsync()
    {
        try
        {
            await _settingsStore.SaveAsync(CurrentSettings());
            PhaseText = "启动设置已保存。";
        }
        catch (Exception exception)
        {
            AppendLog(exception.Message);
            PhaseText = "设置无效：WSL 发行版和项目路径不能为空，超时必须为 30–900 秒。";
        }
    }

    private void OpenUrl(string? url)
    {
        try
        {
            if (string.IsNullOrWhiteSpace(url)) throw new ArgumentException("Missing URL.");
            _urls.Open(url);
        }
        catch (Exception exception)
        {
            AppendLog(exception.Message);
            PhaseText = "无法打开该本地地址。";
        }
    }

    private void OpenLogDirectory()
    {
        try
        {
            _logs.OpenDirectory();
        }
        catch (Exception exception)
        {
            AppendLog(exception.Message);
            PhaseText = "无法打开日志目录。";
        }
    }

    private void HandleEvent(LauncherEvent evt)
    {
        AppendLog($"[{evt.Level}] {evt.Message}");
        PhaseText = evt.Message;

        if (evt.Type.Equals("service", StringComparison.OrdinalIgnoreCase) &&
            !string.IsNullOrWhiteSpace(evt.Service))
        {
            UpsertService(evt);
            return;
        }

        if (evt.Type.Equals("state", StringComparison.OrdinalIgnoreCase))
        {
            var marker = string.IsNullOrWhiteSpace(evt.Code) ? evt.Phase : evt.Code;
            State = marker.ToLowerInvariant() switch
            {
                "ready" => ProjectState.Ready,
                "stopped" => ProjectState.Stopped,
                "partial" => ProjectState.Partial,
                _ => State,
            };
            return;
        }

        if (evt.Type.Equals("error", StringComparison.OrdinalIgnoreCase))
        {
            var kind = LauncherErrorClassifier.Classify(evt.Code, evt.Message);
            if (State != ProjectState.Partial)
            {
                State = ProjectState.Failed;
            }
            PhaseText = DescribeError(kind);
        }
    }

    private void UpsertService(LauncherEvent evt)
    {
        var state = !string.IsNullOrWhiteSpace(evt.Code)
            ? evt.Code
            : evt.Message.Split(':', 2, StringSplitOptions.TrimEntries)[0];
        var next = new ServiceStatusItem(evt.Service!, state, evt.Message);
        var existing = ServiceStatuses
            .Select((item, index) => (item, index))
            .FirstOrDefault(pair => pair.item.Name.Equals(evt.Service, StringComparison.Ordinal));
        if (existing.item is null)
        {
            ServiceStatuses.Add(next);
        }
        else
        {
            ServiceStatuses[existing.index] = next;
        }
    }

    private void AppendLog(string line)
    {
        if (string.IsNullOrEmpty(line)) return;
        try
        {
            _logs.Append(line);
        }
        catch (IOException)
        {
            // Keep the live UI usable when the optional disk log cannot be written.
        }
        catch (UnauthorizedAccessException)
        {
            // Keep the live UI usable when the optional disk log cannot be written.
        }

        _uiLogLines.Enqueue(line);
        while (_uiLogLines.Count > MaximumUiLogLines)
        {
            _uiLogLines.Dequeue();
        }
        LogText = string.Join(Environment.NewLine, _uiLogLines);
    }

    private static ProjectState InitialState(ControlCommand command, ProjectState current) => command switch
    {
        ControlCommand.Start => ProjectState.Starting,
        ControlCommand.Rebuild or ControlCommand.RebuildService => ProjectState.Building,
        ControlCommand.Stop => ProjectState.Stopping,
        _ => current,
    };

    private static string InitialPhase(ControlCommand command) => command switch
    {
        ControlCommand.Start => "正在检查环境并启动已有镜像…",
        ControlCommand.Rebuild => "正在重新构建业务服务…",
        ControlCommand.RebuildService => "正在重新构建选中的服务…",
        ControlCommand.Stop => "正在停止服务并保留数据卷…",
        ControlCommand.Status => "正在刷新服务状态…",
        ControlCommand.Logs => "正在读取服务日志…",
        _ => "正在执行操作…",
    };

    private static string DescribeError(LauncherErrorKind kind) => kind switch
    {
        LauncherErrorKind.EnvironmentMissing => "找不到 WSL 环境，请检查发行版和项目路径。",
        LauncherErrorKind.DockerUnavailable => "WSL 内的 Docker 未运行或无法访问。",
        LauncherErrorKind.PortConflict => "本地端口已被占用，请查看日志定位冲突进程。",
        LauncherErrorKind.ImagesMissing => "本地镜像不完整，请使用“重新构建并启动”。",
        LauncherErrorKind.BuildFailed => "镜像构建失败，请查看右侧日志。",
        LauncherErrorKind.ContainerExited => "服务容器异常退出，请查看服务日志。",
        LauncherErrorKind.HealthTimeout => "服务未在限定时间内就绪，请查看健康检查日志。",
        _ => "操作未完成，请查看右侧日志。",
    };

    private void RaiseCommandStates()
    {
        StartCommand?.RaiseCanExecuteChanged();
        RebuildCommand?.RaiseCanExecuteChanged();
        StopCommand?.RaiseCanExecuteChanged();
        RefreshCommand?.RaiseCanExecuteChanged();
        RebuildServiceCommand?.RaiseCanExecuteChanged();
        LoadLogsCommand?.RaiseCanExecuteChanged();
        SaveSettingsCommand?.RaiseCanExecuteChanged();
    }

    private bool SetField<T>(ref T field, T value, [CallerMemberName] string? propertyName = null)
    {
        if (EqualityComparer<T>.Default.Equals(field, value)) return false;
        field = value;
        OnPropertyChanged(propertyName);
        return true;
    }

    private void OnPropertyChanged([CallerMemberName] string? propertyName = null) =>
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
}
