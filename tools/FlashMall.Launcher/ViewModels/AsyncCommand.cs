using System.Windows.Input;

namespace FlashMall.Launcher.ViewModels;

public sealed class AsyncCommand(
    Func<object?, Task> execute,
    Predicate<object?>? canExecute = null) : ICommand
{
    public event EventHandler? CanExecuteChanged;

    public bool CanExecute(object? parameter) => canExecute?.Invoke(parameter) ?? true;

    public async void Execute(object? parameter) => await execute(parameter);

    public Task ExecuteAsync(object? parameter = null) => execute(parameter);

    public void RaiseCanExecuteChanged() => CanExecuteChanged?.Invoke(this, EventArgs.Empty);
}
