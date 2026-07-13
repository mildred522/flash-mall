namespace FlashMall.Launcher.Services;

public interface ILauncherLogStore
{
    string DirectoryPath { get; }
    void Append(string line);
    void OpenDirectory();
}
