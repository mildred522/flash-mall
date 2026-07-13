using FlashMall.Launcher.Models;
using FlashMall.Launcher.Services;

namespace FlashMall.Launcher.Tests;

internal sealed class FakeRunner(LauncherEvent evt, int exitCode) : IControlRunner
{
    public Task<ControlRunResult> RunAsync(
        LauncherSettings settings,
        ControlCommand command,
        string? service,
        Action<LauncherEvent> onEvent,
        Action<string> onLog,
        CancellationToken cancellationToken)
    {
        onEvent(evt);
        return Task.FromResult(new ControlRunResult(exitCode, Array.Empty<string>()));
    }
}

internal sealed class BlockingRunner : IControlRunner
{
    public TaskCompletionSource Completion { get; } = new(TaskCreationOptions.RunContinuationsAsynchronously);
    public int Calls { get; private set; }

    public async Task<ControlRunResult> RunAsync(
        LauncherSettings settings,
        ControlCommand command,
        string? service,
        Action<LauncherEvent> onEvent,
        Action<string> onLog,
        CancellationToken cancellationToken)
    {
        Calls++;
        await Completion.Task.WaitAsync(cancellationToken);
        return new ControlRunResult(0, Array.Empty<string>());
    }
}

internal sealed class MemorySettingsStore : ILauncherSettingsStore
{
    public LauncherSettings Value { get; private set; } = LauncherSettings.Default;
    public LauncherSettings Load() => Value;
    public Task SaveAsync(LauncherSettings settings, CancellationToken cancellationToken = default)
    {
        Value = settings;
        return Task.CompletedTask;
    }
}

internal sealed class FakeUrlService : ILocalUrlService
{
    public string? Opened { get; private set; }
    public bool IsAllowed(string url) => url.StartsWith("http://127.0.0.1:", StringComparison.Ordinal);
    public void Open(string url)
    {
        if (!IsAllowed(url)) throw new InvalidOperationException();
        Opened = url;
    }
}

internal sealed class MemoryLogStore : ILauncherLogStore
{
    public List<string> Lines { get; } = [];
    public string DirectoryPath => "memory";
    public void Append(string line) => Lines.Add(line);
    public void OpenDirectory() { }
}

internal sealed class ImmediateDispatcher : IUiDispatcher
{
    public void Post(Action action) => action();
}
