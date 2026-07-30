using System.ComponentModel;
using System.Windows;
using System.Windows.Controls;
using FlashMall.Launcher.ViewModels;

namespace FlashMall.Launcher;

public partial class MainWindow : Window
{
    public const string WindowTitle = "Flash Mall 控制中心";

    public MainWindow()
    {
        InitializeComponent();
        Closing += OnClosing;
    }

    private void OnClosing(object? sender, CancelEventArgs e)
    {
        if (DataContext is MainWindowViewModel viewModel)
        {
            viewModel.CancelActiveOperation();
        }
    }

    private void LogTextBox_OnTextChanged(object sender, TextChangedEventArgs e)
    {
        if (sender is TextBox textBox)
        {
            textBox.ScrollToEnd();
        }
    }

    private async void ResetDemo_OnClick(object sender, RoutedEventArgs e)
    {
        if (DataContext is not MainWindowViewModel viewModel || viewModel.IsBusy)
        {
            return;
        }
        var answer = MessageBox.Show(
            "将备份并清空本地 MySQL、Redis 业务数据，然后恢复固定演示数据。上传图片和构建缓存会保留。是否继续？",
            "确认重置演示环境",
            MessageBoxButton.YesNo,
            MessageBoxImage.Warning);
        if (answer == MessageBoxResult.Yes)
        {
            await viewModel.ResetDemoCommand.ExecuteAsync(null);
        }
    }
}
