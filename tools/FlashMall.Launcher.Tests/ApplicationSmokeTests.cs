namespace FlashMall.Launcher.Tests;

public sealed class ApplicationSmokeTests
{
    [Fact]
    public void MainWindowTypeCanBeLoaded()
    {
        var type = typeof(FlashMall.Launcher.MainWindow);

        Assert.Equal("Flash Mall 控制中心", FlashMall.Launcher.MainWindow.WindowTitle);
        Assert.NotNull(type);
    }
}
