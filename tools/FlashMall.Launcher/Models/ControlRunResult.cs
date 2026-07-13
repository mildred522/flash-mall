namespace FlashMall.Launcher.Models;

public sealed record ControlRunResult(int ExitCode, IReadOnlyList<string> StandardError);
