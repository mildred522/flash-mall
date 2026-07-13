using System.Diagnostics;
using FlashMall.Launcher.Models;

namespace FlashMall.Launcher.Services;

public sealed class WslControlRunner : IControlRunner
{
    public async Task<ControlRunResult> RunAsync(
        LauncherSettings settings,
        ControlCommand command,
        string? service,
        Action<LauncherEvent> onEvent,
        Action<string> onLog,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(onEvent);
        ArgumentNullException.ThrowIfNull(onLog);

        using var process = new Process
        {
            StartInfo = WslCommandBuilder.Build(settings, command, service),
        };
        if (!process.Start())
        {
            throw new InvalidOperationException("Could not start wsl.exe.");
        }

        var errors = new List<string>();
        var stdout = ReadStandardOutputAsync(process, onEvent, onLog, cancellationToken);
        var stderr = ReadStandardErrorAsync(process, errors, onLog, cancellationToken);

        try
        {
            await Task.WhenAll(process.WaitForExitAsync(cancellationToken), stdout, stderr);
        }
        catch (OperationCanceledException)
        {
            if (!process.HasExited)
            {
                process.Kill(entireProcessTree: true);
                await process.WaitForExitAsync(CancellationToken.None);
            }

            throw;
        }

        return new ControlRunResult(process.ExitCode, errors);
    }

    private static async Task ReadStandardOutputAsync(
        Process process,
        Action<LauncherEvent> onEvent,
        Action<string> onLog,
        CancellationToken cancellationToken)
    {
        while (await process.StandardOutput.ReadLineAsync(cancellationToken) is { } line)
        {
            if (LauncherEventParser.TryParse(line, out var evt))
            {
                onEvent(evt);
            }
            else
            {
                onLog(line);
            }
        }
    }

    private static async Task ReadStandardErrorAsync(
        Process process,
        ICollection<string> errors,
        Action<string> onLog,
        CancellationToken cancellationToken)
    {
        while (await process.StandardError.ReadLineAsync(cancellationToken) is { } line)
        {
            errors.Add(line);
            onLog(line);
        }
    }
}
