using FlashMall.Launcher.Models;

namespace FlashMall.Launcher.Tests;

public sealed class LauncherSettingsTests
{
    [Fact]
    public void DefaultsTargetUbuntuHertzWorkspace()
    {
        var settings = LauncherSettings.Default;

        Assert.Equal("Ubuntu", settings.Distro);
        Assert.Equal("/home/mildred/code/flash-mall", settings.Workspace);
        Assert.Equal(180, settings.WaitTimeoutSeconds);
    }
}
