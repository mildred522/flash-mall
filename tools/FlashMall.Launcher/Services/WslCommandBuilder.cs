using System.Diagnostics;
using System.Text;
using FlashMall.Launcher.Models;

namespace FlashMall.Launcher.Services;

public static class WslCommandBuilder
{
    private static readonly HashSet<string> BusinessServices =
    [
        "auth-api", "product-rpc", "order-rpc", "inventory-kitex", "hertz-gateway",
    ];

    private static readonly HashSet<string> LogServices =
    [
        .. BusinessServices,
        "mysql", "mysql-init", "redis", "redis-init", "rabbitmq", "etcd", "dtm", "jaeger",
    ];

    public static ProcessStartInfo Build(
        LauncherSettings settings,
        ControlCommand command,
        string? service = null)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(settings.Distro);
        ArgumentException.ThrowIfNullOrWhiteSpace(settings.Workspace);
        if (settings.WaitTimeoutSeconds is < 30 or > 900)
        {
            throw new ArgumentOutOfRangeException(
                nameof(settings),
                "Wait timeout must be between 30 and 900 seconds.");
        }

        ValidateService(command, service);

        var commandName = command switch
        {
            ControlCommand.Start => "start",
            ControlCommand.Rebuild => "rebuild",
            ControlCommand.RebuildService => "rebuild-service",
            ControlCommand.Stop => "stop",
            ControlCommand.Status => "status",
            ControlCommand.Logs => "logs",
            _ => throw new ArgumentOutOfRangeException(nameof(command), command, "Unknown control command."),
        };

        var startInfo = new ProcessStartInfo
        {
            FileName = "wsl.exe",
            UseShellExecute = false,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            StandardOutputEncoding = Encoding.UTF8,
            StandardErrorEncoding = Encoding.UTF8,
            CreateNoWindow = true,
        };

        AddArguments(
            startInfo,
            "-d", settings.Distro,
            "--cd", settings.Workspace,
            "--", "./scripts/local/flash-mall-control.sh", commandName);
        if (!string.IsNullOrWhiteSpace(service))
        {
            startInfo.ArgumentList.Add(service);
        }

        AddArguments(startInfo, "--wait-timeout", settings.WaitTimeoutSeconds.ToString());
        return startInfo;
    }

    private static void ValidateService(ControlCommand command, string? service)
    {
        if (command == ControlCommand.RebuildService)
        {
            if (string.IsNullOrWhiteSpace(service) || !BusinessServices.Contains(service))
            {
                throw new ArgumentException("A supported business service is required.", nameof(service));
            }

            return;
        }

        if (command == ControlCommand.Logs)
        {
            if (!string.IsNullOrWhiteSpace(service) && !LogServices.Contains(service))
            {
                throw new ArgumentException("The requested log service is not supported.", nameof(service));
            }

            return;
        }

        if (!string.IsNullOrWhiteSpace(service))
        {
            throw new ArgumentException("This command does not accept a service.", nameof(service));
        }
    }

    private static void AddArguments(ProcessStartInfo startInfo, params string[] values)
    {
        foreach (var value in values)
        {
            startInfo.ArgumentList.Add(value);
        }
    }
}
