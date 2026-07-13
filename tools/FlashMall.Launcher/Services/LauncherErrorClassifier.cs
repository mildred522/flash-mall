using FlashMall.Launcher.Models;

namespace FlashMall.Launcher.Services;

public static class LauncherErrorClassifier
{
    public static LauncherErrorKind Classify(string? code, string? combinedLog)
    {
        var normalizedCode = code?.Trim().ToLowerInvariant();
        if (normalizedCode == "docker_unreachable") return LauncherErrorKind.DockerUnavailable;
        if (normalizedCode == "images_missing") return LauncherErrorKind.ImagesMissing;

        var log = combinedLog ?? string.Empty;
        if (log.Contains("address already in use", StringComparison.OrdinalIgnoreCase))
            return LauncherErrorKind.PortConflict;
        if (log.Contains("health not ready", StringComparison.OrdinalIgnoreCase) ||
            log.Contains("health check failed", StringComparison.OrdinalIgnoreCase))
            return LauncherErrorKind.HealthTimeout;
        if (log.Contains("build failed", StringComparison.OrdinalIgnoreCase))
            return LauncherErrorKind.BuildFailed;
        if (log.Contains("exited", StringComparison.OrdinalIgnoreCase))
            return LauncherErrorKind.ContainerExited;
        if (log.Contains("wsl", StringComparison.OrdinalIgnoreCase) &&
            log.Contains("not found", StringComparison.OrdinalIgnoreCase))
            return LauncherErrorKind.EnvironmentMissing;
        return LauncherErrorKind.Unknown;
    }
}
