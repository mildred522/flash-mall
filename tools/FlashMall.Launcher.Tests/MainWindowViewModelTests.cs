using FlashMall.Launcher.Models;
using FlashMall.Launcher.Services;
using FlashMall.Launcher.ViewModels;

namespace FlashMall.Launcher.Tests;

public sealed class MainWindowViewModelTests
{
    [Fact]
    public async Task StartTransitionsThroughBusyToReady()
    {
        var runner = new FakeRunner(new LauncherEvent("state", "ready", "info", "ready"), 0);
        var vm = CreateViewModel(runner);

        await vm.ExecuteAsync(ControlCommand.Start);

        Assert.Equal(ProjectState.Ready, vm.State);
        Assert.False(vm.IsBusy);
    }

    [Fact]
    public async Task ExecuteRejectsConcurrentReentry()
    {
        var runner = new BlockingRunner();
        var vm = CreateViewModel(runner);
        var first = vm.ExecuteAsync(ControlCommand.Rebuild);

        await vm.ExecuteAsync(ControlCommand.Status);
        runner.Completion.SetResult();
        await first;

        Assert.Equal(1, runner.Calls);
    }

    [Fact]
    public async Task ServiceEventsAreUpserted()
    {
        var runner = new FakeRunner(
            new LauncherEvent("service", "status", "info", "running: healthy", "hertz-gateway", "running"),
            0);
        var vm = CreateViewModel(runner);

        await vm.ExecuteAsync(ControlCommand.Status);

        var status = Assert.Single(vm.ServiceStatuses);
        Assert.Equal("hertz-gateway", status.Name);
        Assert.Equal("running", status.State);
    }

    private static MainWindowViewModel CreateViewModel(IControlRunner runner) =>
        new(
            runner,
            new MemorySettingsStore(),
            new FakeUrlService(),
            new MemoryLogStore(),
            new ImmediateDispatcher(),
            LauncherSettings.Default);
}
