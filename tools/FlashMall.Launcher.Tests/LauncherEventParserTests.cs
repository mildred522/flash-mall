using FlashMall.Launcher.Services;

namespace FlashMall.Launcher.Tests;

public sealed class LauncherEventParserTests
{
    [Fact]
    public void ParserSeparatesEventsFromRawLogs()
    {
        Assert.True(LauncherEventParser.TryParse(
            "{\"type\":\"state\",\"phase\":\"ready\",\"level\":\"info\",\"message\":\"ok\"}",
            out var evt));
        Assert.Equal("ready", evt.Phase);
        Assert.False(LauncherEventParser.TryParse("docker build output", out _));
    }

    [Theory]
    [InlineData("{}")]
    [InlineData("{\"type\":\"state\",\"phase\":\"ready\",\"level\":\"info\"}")]
    [InlineData("{not-json}")]
    public void ParserRequiresACompleteEvent(string line) =>
        Assert.False(LauncherEventParser.TryParse(line, out _));
}
