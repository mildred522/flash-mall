using System.Text.Json;
using System.IO;
using FlashMall.Launcher.Models;

namespace FlashMall.Launcher.Services;

public sealed class LauncherSettingsStore : ILauncherSettingsStore
{
    private static readonly JsonSerializerOptions JsonOptions = new() { WriteIndented = true };
    private readonly string _path;

    public LauncherSettingsStore(string? path = null)
    {
        _path = path ?? Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "FlashMall",
            "launcher.json");
    }

    public LauncherSettings Load()
    {
        if (!File.Exists(_path))
        {
            return LauncherSettings.Default;
        }

        try
        {
            var settings = JsonSerializer.Deserialize<LauncherSettings>(File.ReadAllText(_path), JsonOptions);
            return IsValid(settings) ? settings! : PreserveInvalidAndReturnDefault();
        }
        catch (JsonException)
        {
            return PreserveInvalidAndReturnDefault();
        }
        catch (IOException)
        {
            return LauncherSettings.Default;
        }
        catch (UnauthorizedAccessException)
        {
            return LauncherSettings.Default;
        }
    }

    public async Task SaveAsync(LauncherSettings settings, CancellationToken cancellationToken = default)
    {
        if (!IsValid(settings))
        {
            throw new ArgumentException("Launcher settings are invalid.", nameof(settings));
        }

        var directory = Path.GetDirectoryName(_path)
            ?? throw new InvalidOperationException("Settings path does not have a parent directory.");
        Directory.CreateDirectory(directory);
        var temporaryPath = _path + ".tmp";
        var json = JsonSerializer.Serialize(settings, JsonOptions);
        await File.WriteAllTextAsync(temporaryPath, json, cancellationToken);
        File.Move(temporaryPath, _path, overwrite: true);
    }

    private static bool IsValid(LauncherSettings? settings) =>
        settings is not null &&
        !string.IsNullOrWhiteSpace(settings.Distro) &&
        !string.IsNullOrWhiteSpace(settings.Workspace) &&
        settings.WaitTimeoutSeconds is >= 30 and <= 900;

    private LauncherSettings PreserveInvalidAndReturnDefault()
    {
        try
        {
            File.Move(_path, _path + ".invalid", overwrite: true);
        }
        catch (IOException)
        {
            // Falling back is more important than rotating an unreadable diagnostic file.
        }
        catch (UnauthorizedAccessException)
        {
            // Falling back is more important than rotating an unreadable diagnostic file.
        }

        return LauncherSettings.Default;
    }
}
