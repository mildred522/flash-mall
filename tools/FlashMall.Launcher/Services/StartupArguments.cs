using FlashMall.Launcher.Models;

namespace FlashMall.Launcher.Services;

public static class StartupArguments
{
    public static LauncherSettings Apply(LauncherSettings settings, IReadOnlyList<string> arguments)
    {
        var distro = settings.Distro;
        var workspace = settings.Workspace;
        var timeout = settings.WaitTimeoutSeconds;

        for (var index = 0; index < arguments.Count; index++)
        {
            var option = arguments[index];
            if (index + 1 >= arguments.Count)
            {
                throw new ArgumentException($"{option} requires a value.", nameof(arguments));
            }

            var value = arguments[++index];
            switch (option)
            {
                case "--distro": distro = value; break;
                case "--workspace": workspace = value; break;
                case "--wait-timeout" when int.TryParse(value, out var parsed): timeout = parsed; break;
                case "--wait-timeout":
                    throw new ArgumentException("--wait-timeout requires an integer.", nameof(arguments));
                default:
                    throw new ArgumentException($"Unknown option: {option}", nameof(arguments));
            }
        }

        var actual = new LauncherSettings(distro, workspace, timeout);
        if (string.IsNullOrWhiteSpace(actual.Distro) ||
            string.IsNullOrWhiteSpace(actual.Workspace) ||
            actual.WaitTimeoutSeconds is < 30 or > 900)
        {
            throw new ArgumentException("Startup settings are invalid.", nameof(arguments));
        }

        return actual;
    }
}
