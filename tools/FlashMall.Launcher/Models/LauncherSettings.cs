namespace FlashMall.Launcher.Models;

public sealed record LauncherSettings(string Distro, string Workspace, int WaitTimeoutSeconds)
{
    public static LauncherSettings Default { get; } = new(
        "Ubuntu",
        "/home/mildred/code/flash-mall",
        180);
}
