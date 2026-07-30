using FlashMall.Launcher.Models;
using FlashMall.Launcher.Services;

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
        Assert.Equal(RunProfile.Development, settings.RunProfile);
        Assert.False(settings.ObservabilityEnabled);
    }

    [Fact]
    public void LegacySettingsGainSafeProfileDefaults()
    {
        var path = Path.GetTempFileName();
        try
        {
            File.WriteAllText(path, """
                {"Distro":"Ubuntu","Workspace":"/tmp/flash-mall","WaitTimeoutSeconds":120}
                """);

            var settings = new LauncherSettingsStore(path).Load();

            Assert.Equal(RunProfile.Development, settings.RunProfile);
            Assert.False(settings.ObservabilityEnabled);
        }
        finally
        {
            File.Delete(path);
        }
    }
}
