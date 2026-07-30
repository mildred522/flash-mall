namespace FlashMall.Launcher.Models;

public sealed record LauncherSettings(
    string Distro,
    string Workspace,
    int WaitTimeoutSeconds,
    RunProfile RunProfile = RunProfile.Development,
    bool ObservabilityEnabled = false)
{
    public static LauncherSettings Default { get; } = new(
        "Ubuntu",
        "/home/mildred/code/flash-mall",
        180,
        RunProfile.Development,
        false);
}
