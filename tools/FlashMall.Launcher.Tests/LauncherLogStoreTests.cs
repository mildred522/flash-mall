using FlashMall.Launcher.Services;

namespace FlashMall.Launcher.Tests;

public sealed class LauncherLogStoreTests
{
    [Fact]
    public void LogStoreWritesInsideConfiguredRoot()
    {
        var root = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        try
        {
            var store = new LauncherLogStore(root);
            store.Append("diagnostic line");

            Assert.Contains("diagnostic line", File.ReadAllText(Assert.Single(Directory.GetFiles(root))));
        }
        finally
        {
            if (Directory.Exists(root)) Directory.Delete(root, true);
        }
    }
}
