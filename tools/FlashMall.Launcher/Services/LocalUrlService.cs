using System.Diagnostics;

namespace FlashMall.Launcher.Services;

public sealed class LocalUrlService : ILocalUrlService
{
    private static readonly HashSet<string> AllowedUrls = new(StringComparer.Ordinal)
    {
        "http://127.0.0.1:8889/shop",
        "http://127.0.0.1:8889/admin",
        "http://127.0.0.1:15672",
        "http://127.0.0.1:16686",
    };

    public bool IsAllowed(string url) => AllowedUrls.Contains(url);

    public void Open(string url)
    {
        if (!IsAllowed(url))
        {
            throw new InvalidOperationException("Only declared Flash Mall local URLs may be opened.");
        }

        Process.Start(new ProcessStartInfo(url) { UseShellExecute = true });
    }
}
