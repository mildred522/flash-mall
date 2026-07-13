using System.Text.Json;
using FlashMall.Launcher.Models;

namespace FlashMall.Launcher.Services;

public static class LauncherEventParser
{
    private static readonly JsonSerializerOptions Options = new()
    {
        PropertyNameCaseInsensitive = true,
    };

    public static bool TryParse(string line, out LauncherEvent evt)
    {
        evt = null!;
        if (string.IsNullOrWhiteSpace(line) || line[0] != '{')
        {
            return false;
        }

        try
        {
            var parsed = JsonSerializer.Deserialize<LauncherEvent>(line, Options);
            if (parsed is null ||
                string.IsNullOrWhiteSpace(parsed.Type) ||
                string.IsNullOrWhiteSpace(parsed.Phase) ||
                string.IsNullOrWhiteSpace(parsed.Level) ||
                string.IsNullOrWhiteSpace(parsed.Message))
            {
                return false;
            }

            evt = parsed;
            return true;
        }
        catch (JsonException)
        {
            return false;
        }
    }
}
