using FlashMall.Launcher.Models;
using FlashMall.Launcher.Services;

namespace FlashMall.Launcher.Tests;

public sealed class WslCommandBuilderTests
{
    [Fact]
    public void BuilderUsesArgumentListWithoutShellConcatenation()
    {
        var psi = WslCommandBuilder.Build(LauncherSettings.Default, ControlCommand.RebuildService, "order-rpc");

        Assert.Equal("wsl.exe", psi.FileName);
        Assert.Equal(
            new[]
            {
                "-d", "Ubuntu", "--cd", "/home/mildred/code/flash-mall", "--",
                "./scripts/local/flash-mall-control.sh", "rebuild-service", "order-rpc",
                "--wait-timeout", "180",
            },
            psi.ArgumentList);
        Assert.True(psi.RedirectStandardOutput);
        Assert.True(psi.RedirectStandardError);
        Assert.False(psi.UseShellExecute);
        Assert.True(psi.CreateNoWindow);
    }

    [Theory]
    [InlineData(ControlCommand.RebuildService, "mysql")]
    [InlineData(ControlCommand.Logs, "entry-api")]
    [InlineData(ControlCommand.Logs, "unknown")]
    public void BuilderRejectsServicesOutsideTheCommandAllowList(ControlCommand command, string service) =>
        Assert.Throws<ArgumentException>(() => WslCommandBuilder.Build(LauncherSettings.Default, command, service));

    [Fact]
    public void LogsAllowsInfrastructureServices()
    {
        var psi = WslCommandBuilder.Build(LauncherSettings.Default, ControlCommand.Logs, "mysql");

        Assert.Contains("mysql", psi.ArgumentList);
    }
}
