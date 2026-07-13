using FlashMall.Launcher.Models;
using FlashMall.Launcher.Services;

namespace FlashMall.Launcher.Tests;

public sealed class LauncherErrorClassifierTests
{
    [Theory]
    [InlineData("docker_unreachable", "", LauncherErrorKind.DockerUnavailable)]
    [InlineData("images_missing", "", LauncherErrorKind.ImagesMissing)]
    [InlineData("", "bind: address already in use", LauncherErrorKind.PortConflict)]
    [InlineData("", "container exited with code 2", LauncherErrorKind.ContainerExited)]
    [InlineData("", "docker build failed", LauncherErrorKind.BuildFailed)]
    [InlineData("", "gateway health not ready", LauncherErrorKind.HealthTimeout)]
    [InlineData("", "unrelated failure", LauncherErrorKind.Unknown)]
    public void ErrorsAreClassified(string code, string log, LauncherErrorKind expected) =>
        Assert.Equal(expected, LauncherErrorClassifier.Classify(code, log));
}
