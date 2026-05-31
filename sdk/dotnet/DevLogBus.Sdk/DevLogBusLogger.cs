namespace DanSherwin.DevLogBus;

public sealed class DevLogBusLogger
{
    private readonly DevLogBusClient client;
    private readonly string source;

    internal DevLogBusLogger(DevLogBusClient client, string source)
    {
        this.client = client;
        this.source = source;
    }

    public Task<DevLogBusPublishResult> DebugAsync(
        string message,
        IReadOnlyDictionary<string, object?>? attrs = null,
        CancellationToken cancellationToken = default)
    {
        return PublishAsync("DEBUG", message, attrs, cancellationToken);
    }

    public Task<DevLogBusPublishResult> InfoAsync(
        string message,
        IReadOnlyDictionary<string, object?>? attrs = null,
        CancellationToken cancellationToken = default)
    {
        return PublishAsync("INFO", message, attrs, cancellationToken);
    }

    public Task<DevLogBusPublishResult> WarnAsync(
        string message,
        IReadOnlyDictionary<string, object?>? attrs = null,
        CancellationToken cancellationToken = default)
    {
        return PublishAsync("WARN", message, attrs, cancellationToken);
    }

    public Task<DevLogBusPublishResult> ErrorAsync(
        string message,
        IReadOnlyDictionary<string, object?>? attrs = null,
        CancellationToken cancellationToken = default)
    {
        return PublishAsync("ERROR", message, attrs, cancellationToken);
    }

    private Task<DevLogBusPublishResult> PublishAsync(
        string level,
        string message,
        IReadOnlyDictionary<string, object?>? attrs,
        CancellationToken cancellationToken)
    {
        return client.PublishAsync(new DevLogBusRecord
        {
            Source = source,
            Level = level,
            Message = message,
            Attrs = attrs,
        }, cancellationToken);
    }
}
