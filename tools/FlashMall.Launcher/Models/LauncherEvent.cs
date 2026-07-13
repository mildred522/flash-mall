namespace FlashMall.Launcher.Models;

public sealed record LauncherEvent(
    string Type,
    string Phase,
    string Level,
    string Message,
    string? Service = null,
    string? Code = null);
