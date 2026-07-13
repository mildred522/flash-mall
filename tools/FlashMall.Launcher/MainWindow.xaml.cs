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
}
