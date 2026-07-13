using FlashMall.Launcher.Models;

namespace FlashMall.Launcher.Services;

public interface ILauncherSettingsStore
{
    LauncherSettings Load();
    Task SaveAsync(LauncherSettings settings, CancellationToken cancellationToken = default);
}
