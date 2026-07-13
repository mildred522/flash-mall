using FlashMall.Launcher.Services;

namespace FlashMall.Launcher.Tests;

public sealed class LocalUrlServiceTests
{
    [Theory]
    [InlineData("http://127.0.0.1:8889/shop", true)]
    [InlineData("http://127.0.0.1:8889/admin", true)]
    [InlineData("http://127.0.0.1:15672", true)]
    [InlineData("http://127.0.0.1:16686", true)]
    [InlineData("https://example.com", false)]
    [InlineData("http://127.0.0.1:8889/shop/../../evil", false)]
    public void OnlyDeclaredLocalUrlsAreAllowed(string url, bool allowed) =>
        Assert.Equal(allowed, new LocalUrlService().IsAllowed(url));
}
