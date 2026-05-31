namespace DanSherwin.DevLogBus;

using System.Globalization;
using System.Text;
using System.Text.Json;

public sealed record DevLogBusRecord
{
    public DateTimeOffset? Time { get; init; }
    public string Level { get; init; } = "INFO";
    public string Source { get; init; } = "";
    public string Message { get; init; } = "";
    public IReadOnlyDictionary<string, object?>? Attrs { get; init; }
    public string? AttrsJson { get; init; }

    public string ToJson()
    {
        Validate();

        using var stream = new MemoryStream();
        using (var writer = new Utf8JsonWriter(stream))
        {
            writer.WriteStartObject();
            writer.WriteString("time", FormatTime(Time));
            writer.WriteString("level", NormalizeLevel(Level));
            writer.WriteString("source", Source.Trim());
            writer.WriteString("message", Message);

            if (!string.IsNullOrWhiteSpace(AttrsJson))
            {
                writer.WritePropertyName("attrs");
                writer.WriteRawValue(AttrsJson);
            }
            else if (Attrs is { Count: > 0 })
            {
                writer.WritePropertyName("attrs");
                JsonSerializer.Serialize(writer, Attrs);
            }

            writer.WriteEndObject();
        }
        return Encoding.UTF8.GetString(stream.ToArray());
    }

    public static string NormalizeLevel(string? level)
    {
        var value = (level ?? "").Trim();
        return value.ToLowerInvariant() switch
        {
            "debug" or "dbg" => "DEBUG",
            "" or "info" => "INFO",
            "warn" or "warning" => "WARN",
            "error" or "err" => "ERROR",
            _ => value.ToUpperInvariant(),
        };
    }

    private void Validate()
    {
        if (string.IsNullOrWhiteSpace(Source))
        {
            throw new InvalidOperationException("DevLogBus source is required");
        }
        if (Message.Length == 0)
        {
            throw new InvalidOperationException("DevLogBus message is required");
        }
        if (string.IsNullOrWhiteSpace(AttrsJson))
        {
            return;
        }

        using var doc = JsonDocument.Parse(AttrsJson);
        if (doc.RootElement.ValueKind != JsonValueKind.Object)
        {
            throw new InvalidOperationException("DevLogBus AttrsJson must be a JSON object");
        }
    }

    private static string FormatTime(DateTimeOffset? time)
    {
        return (time ?? DateTimeOffset.UtcNow)
            .UtcDateTime
            .ToString("O", CultureInfo.InvariantCulture);
    }
}
