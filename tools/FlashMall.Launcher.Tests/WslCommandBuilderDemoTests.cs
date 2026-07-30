using FlashMall.Launcher.Models;
using FlashMall.Launcher.Services;

namespace FlashMall.Launcher.Tests;

public sealed class WslCommandBuilderDemoTests
{
    [Fact]
    public void VerifyDemoUsesConfiguredProfile()
    {
        var settings = LauncherSettings.Default with { RunProfile = RunProfile.Interview };

        var command = WslCommandBuilder.Build(settings, ControlCommand.VerifyDemo);

        Assert.Contains("verify-demo", command.ArgumentList);
        Assert.Contains("interview", command.ArgumentList);
    }

    [Fact]
    public void ResetDemoCarriesExplicitConfirmation()
    {
        var command = WslCommandBuilder.Build(LauncherSettings.Default, ControlCommand.ResetDemo);

        Assert.Contains("reset-demo", command.ArgumentList);
        Assert.Contains("--confirm-reset", command.ArgumentList);
    }

    [Fact]
    public void ObservabilitySettingEnablesComposeProfile()
    {
        var settings = LauncherSettings.Default with { ObservabilityEnabled = true };

        var command = WslCommandBuilder.Build(settings, ControlCommand.Start);

        Assert.Contains("--observability", command.ArgumentList);
    }
}
