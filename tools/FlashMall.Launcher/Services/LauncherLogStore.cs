using System.Diagnostics;
using System.IO;
using System.Text;

namespace FlashMall.Launcher.Services;

public sealed class LauncherLogStore : ILauncherLogStore
{
    private readonly object _writeLock = new();

    public LauncherLogStore(string? directoryPath = null)
    {
        DirectoryPath = directoryPath ?? Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "FlashMall",
            "logs");
    }

    public string DirectoryPath { get; }

    public void Append(string line)
    {
        lock (_writeLock)
        {
            Directory.CreateDirectory(DirectoryPath);
            var path = Path.Combine(DirectoryPath, $"launcher-{DateTime.Now:yyyyMMdd}.log");
            File.AppendAllText(
                path,
                $"{DateTimeOffset.Now:O} {line}{Environment.NewLine}",
                new UTF8Encoding(encoderShouldEmitUTF8Identifier: false));
        }
    }

    public void OpenDirectory()
    {
        Directory.CreateDirectory(DirectoryPath);
        Process.Start(new ProcessStartInfo("explorer.exe")
        {
            UseShellExecute = false,
            ArgumentList = { DirectoryPath },
        });
    }
}
