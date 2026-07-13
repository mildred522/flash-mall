namespace FlashMall.Launcher.Models;

public enum LauncherErrorKind
{
    None,
    EnvironmentMissing,
    DockerUnavailable,
    PortConflict,
    ImagesMissing,
    BuildFailed,
    ContainerExited,
    HealthTimeout,
    Unknown,
}
