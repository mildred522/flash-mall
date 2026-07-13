using FlashMall.Launcher.Models;

namespace FlashMall.Launcher.Services;

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
