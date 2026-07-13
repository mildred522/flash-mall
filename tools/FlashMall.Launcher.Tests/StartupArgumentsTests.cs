using FlashMall.Launcher.Models;
using FlashMall.Launcher.Services;

namespace FlashMall.Launcher.Tests;

public sealed class StartupArgumentsTests
{
    [Fact]
    public void ExplicitStartupArgumentsOverrideSavedSettings()
    {
        var actual = StartupArguments.Apply(
            LauncherSettings.Default,
            ["--distro", "Debian", "--workspace", "/srv/flash-mall"]);

        Assert.Equal("Debian", actual.Distro);
        Assert.Equal("/srv/flash-mall", actual.Workspace);
    }

    [Fact]
    public void InvalidArgumentsAreRejected() =>
        Assert.Throws<ArgumentException>(() => StartupArguments.Apply(LauncherSettings.Default, ["--workspace"]));
}
