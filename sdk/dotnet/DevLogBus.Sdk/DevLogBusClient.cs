namespace DanSherwin.DevLogBus;

using System.Net.Http;
using System.Text;
using System.Text.Json;

public delegate bool DevLogBusRecordFilter(DevLogBusRecord record);

public delegate DevLogBusRecord DevLogBusRecordRedactor(DevLogBusRecord record);

public sealed class DevLogBusClientOptions
{
    public string Endpoint { get; init; } = DevLogBusClient.DefaultHttpEndpoint;
    public string Source { get; init; } = "";
    public TimeSpan Timeout { get; init; } = TimeSpan.FromSeconds(2);
    public DevLogBusRecordFilter? Filter { get; init; }
    public DevLogBusRecordRedactor? Redactor { get; init; }
    public HttpClient? HttpClient { get; init; }
}

public sealed record DevLogBusPublishResult(int Published, bool Filtered);

public sealed class DevLogBusClient : IDisposable
{
    public const string DefaultHttpEndpoint = "http://127.0.0.1:7423";
    public const string RedactedValue = "[REDACTED]";

    private readonly string endpoint;
    private readonly string source;
    private readonly DevLogBusRecordFilter? filter;
    private readonly DevLogBusRecordRedactor? redactor;
    private readonly HttpClient httpClient;
    private readonly bool ownsHttpClient;

    public DevLogBusClient(DevLogBusClientOptions? options = null)
    {
        options ??= new DevLogBusClientOptions();
        endpoint = TrimEndpoint(options.Endpoint);
        source = options.Source ?? "";
        filter = options.Filter;
        redactor = options.Redactor;

        if (options.HttpClient is null)
        {
            var timeout = options.Timeout > TimeSpan.Zero ? options.Timeout : TimeSpan.FromSeconds(2);
            httpClient = new HttpClient { Timeout = timeout };
            ownsHttpClient = true;
        }
        else
        {
            httpClient = options.HttpClient;
        }
    }

    public string Endpoint => endpoint;

    public string Source => source;

    public Task<DevLogBusPublishResult> PublishMessageAsync(
        string level,
        string message,
        IReadOnlyDictionary<string, object?>? attrs = null,
        CancellationToken cancellationToken = default)
    {
        return PublishAsync(new DevLogBusRecord
        {
            Source = source,
            Level = level,
            Message = message,
            Attrs = attrs,
        }, cancellationToken);
    }

    public Task<DevLogBusPublishResult> PublishRawAttrsAsync(
        string level,
        string message,
        string attrsJson,
        CancellationToken cancellationToken = default)
    {
        return PublishAsync(new DevLogBusRecord
        {
            Source = source,
            Level = level,
            Message = message,
            AttrsJson = attrsJson,
        }, cancellationToken);
    }

    public async Task<DevLogBusPublishResult> PublishAsync(
        DevLogBusRecord input,
        CancellationToken cancellationToken = default)
    {
        var record = string.IsNullOrWhiteSpace(input.Source) && !string.IsNullOrWhiteSpace(source)
            ? input with { Source = source }
            : input;

        if (filter is not null && !filter(record))
        {
            return new DevLogBusPublishResult(0, true);
        }
        if (redactor is not null)
        {
            record = redactor(record);
        }

        using var content = new StringContent(record.ToJson(), Encoding.UTF8, "application/json");
        using var response = await httpClient.PostAsync($"{endpoint}/api/records", content, cancellationToken).ConfigureAwait(false);
        if (!response.IsSuccessStatusCode)
        {
            throw new HttpRequestException($"DevLogBus publish failed: HTTP {(int)response.StatusCode}");
        }
        return await ReadResultAsync(response, cancellationToken).ConfigureAwait(false);
    }

    public DevLogBusLogger Logger(string? source = null)
    {
        return new DevLogBusLogger(this, string.IsNullOrWhiteSpace(source) ? this.source : source);
    }

    public static DevLogBusRecordFilter DropSources(params string[] sources)
    {
        var blocked = sources
            .Where(source => !string.IsNullOrWhiteSpace(source))
            .Select(source => source.Trim())
            .ToHashSet(StringComparer.Ordinal);
        return record => !blocked.Contains(record.Source);
    }

    public static DevLogBusRecordRedactor RedactMessage()
    {
        return record => record with { Message = RedactedValue };
    }

    public void Dispose()
    {
        if (ownsHttpClient)
        {
            httpClient.Dispose();
        }
    }

    private static async Task<DevLogBusPublishResult> ReadResultAsync(
        HttpResponseMessage response,
        CancellationToken cancellationToken)
    {
        var body = await response.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false);
        if (string.IsNullOrWhiteSpace(body))
        {
            return new DevLogBusPublishResult(1, false);
        }

        using var doc = JsonDocument.Parse(body);
        var published = doc.RootElement.TryGetProperty("published", out var value) && value.TryGetInt32(out var count)
            ? count
            : 1;
        return new DevLogBusPublishResult(published, false);
    }

    private static string TrimEndpoint(string? endpoint)
    {
        var value = string.IsNullOrWhiteSpace(endpoint) ? DefaultHttpEndpoint : endpoint.Trim();
        return value.TrimEnd('/');
    }
}
