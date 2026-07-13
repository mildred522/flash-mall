namespace FlashMall.Launcher.Services;

public interface ILocalUrlService
{
    bool IsAllowed(string url);
    void Open(string url);
}
