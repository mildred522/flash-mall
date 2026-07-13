namespace FlashMall.Launcher;

using System.Windows;
using FlashMall.Launcher.Models;
using FlashMall.Launcher.Services;
using FlashMall.Launcher.ViewModels;

public partial class App : Application
{
    protected override async void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);

        var settingsStore = new LauncherSettingsStore();
        var savedSettings = settingsStore.Load();
        LauncherSettings settings;
        try
        {
            settings = StartupArguments.Apply(savedSettings, e.Args);
        }
        catch (ArgumentException exception)
        {
            MessageBox.Show(
                $"启动参数无效，将使用已保存的设置。\n\n{exception.Message}",
                FlashMall.Launcher.MainWindow.WindowTitle,
                MessageBoxButton.OK,
                MessageBoxImage.Warning);
            settings = savedSettings;
        }

        var viewModel = new MainWindowViewModel(
            new WslControlRunner(),
            settingsStore,
            new LocalUrlService(),
            new LauncherLogStore(),
            new WpfUiDispatcher(Dispatcher),
            settings);
        var window = new FlashMall.Launcher.MainWindow { DataContext = viewModel };
        MainWindow = window;
        window.Show();
        await viewModel.ExecuteAsync(ControlCommand.Status);
    }
}
