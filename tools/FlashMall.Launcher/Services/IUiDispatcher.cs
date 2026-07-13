namespace FlashMall.Launcher.Services;

public interface IUiDispatcher
{
    void Post(Action action);
}
