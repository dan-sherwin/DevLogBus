namespace DevLogBus.Sdk.Tests;

using System.Net;
using System.Net.Sockets;
using System.Text;
using DanSherwin.DevLogBus;

public static class Program
{
    public static async Task Main()
    {
        NormalizesLevels();
        BuildsRecordJson();
        RejectsNonObjectAttrsJson();
        await FiltersBeforeHttpPublish();
        await PublishesToHttpEndpoint();
        Console.WriteLine("DevLogBus .NET SDK tests passed");
    }

    private static void NormalizesLevels()
    {
        AssertEqual("WARN", DevLogBusRecord.NormalizeLevel("warning"), "warning should normalize");
        AssertEqual("DEBUG", DevLogBusRecord.NormalizeLevel("dbg"), "dbg should normalize");
        AssertEqual("CUSTOM", DevLogBusRecord.NormalizeLevel("custom"), "custom should uppercase");
    }

    private static void BuildsRecordJson()
    {
        var json = new DevLogBusRecord
        {
            Time = DateTimeOffset.Parse("2026-05-31T12:00:00Z"),
            Source = "dotnet_test",
            Level = "warning",
            Message = "quote \" newline\n",
            Attrs = new Dictionary<string, object?>
            {
                ["request"] = new Dictionary<string, object?> { ["id"] = "req-1" },
            },
        }.ToJson();

        AssertTrue(json.Contains("\"level\":\"WARN\"", StringComparison.Ordinal), "level should be WARN");
        AssertTrue(json.Contains("\"source\":\"dotnet_test\"", StringComparison.Ordinal), "source should be encoded");
        AssertTrue(
            json.Contains("quote \\\" newline\\n", StringComparison.Ordinal) ||
            json.Contains("quote \\u0022 newline\\n", StringComparison.Ordinal),
            "message should be escaped");
        AssertTrue(json.Contains("\"attrs\":{\"request\":{\"id\":\"req-1\"}}", StringComparison.Ordinal), "attrs should be embedded");
    }

    private static void RejectsNonObjectAttrsJson()
    {
        AssertThrows<InvalidOperationException>(() => new DevLogBusRecord
        {
            Source = "dotnet_test",
            Message = "hello",
            AttrsJson = "\"nope\"",
        }.ToJson(), "AttrsJson should require object JSON");
    }

    private static async Task FiltersBeforeHttpPublish()
    {
        using var client = new DevLogBusClient(new DevLogBusClientOptions
        {
            Endpoint = "http://127.0.0.1:1",
            Source = "hidden",
            Filter = DevLogBusClient.DropSources("hidden"),
        });

        var result = await client.PublishMessageAsync("INFO", "drop me");

        AssertEqual(0, result.Published, "filtered publish count");
        AssertTrue(result.Filtered, "result should be filtered");
    }

    private static async Task PublishesToHttpEndpoint()
    {
        using var server = await TestServer.Start();
        using var client = new DevLogBusClient(new DevLogBusClientOptions
        {
            Endpoint = server.Endpoint,
            Source = "dotnet_test",
            Redactor = DevLogBusClient.RedactMessage(),
        });

        var result = await client.PublishMessageAsync("INFO", "secret");
        var request = await server.Request;

        AssertEqual(1, result.Published, "published count");
        AssertTrue(!result.Filtered, "result should not be filtered");
        AssertTrue(request.Contains(DevLogBusClient.RedactedValue, StringComparison.Ordinal), "request should contain redacted value");
    }

    private static void AssertEqual<T>(T want, T got, string message)
        where T : notnull
    {
        if (!EqualityComparer<T>.Default.Equals(want, got))
        {
            throw new InvalidOperationException($"{message}: got {got}, want {want}");
        }
    }

    private static void AssertTrue(bool ok, string message)
    {
        if (!ok)
        {
            throw new InvalidOperationException(message);
        }
    }

    private static void AssertThrows<TException>(Action action, string message)
        where TException : Exception
    {
        try
        {
            action();
        }
        catch (TException)
        {
            return;
        }
        throw new InvalidOperationException($"{message}: expected {typeof(TException).Name}");
    }

    private sealed class TestServer : IDisposable
    {
        private readonly TcpListener listener;

        private TestServer(TcpListener listener, string endpoint, Task<string> request)
        {
            this.listener = listener;
            Endpoint = endpoint;
            Request = request;
        }

        public string Endpoint { get; }

        public Task<string> Request { get; }

        public static Task<TestServer> Start()
        {
            var listener = new TcpListener(IPAddress.Loopback, 0);
            listener.Start();
            var port = ((IPEndPoint)listener.LocalEndpoint).Port;
            var request = Task.Run(() => AcceptOne(listener));
            return Task.FromResult(new TestServer(listener, $"http://127.0.0.1:{port}", request));
        }

        public void Dispose()
        {
            listener.Stop();
        }

        private static async Task<string> AcceptOne(TcpListener listener)
        {
            using var client = await listener.AcceptTcpClientAsync();
            await using var stream = client.GetStream();
            var bytes = new List<byte>();
            var buffer = new byte[1024];

            while (!HeaderComplete(bytes))
            {
                var read = await stream.ReadAsync(buffer, 0, buffer.Length);
                if (read == 0)
                {
                    break;
                }
                bytes.AddRange(buffer.Take(read));
            }

            var headerEnd = HeaderEnd(bytes);
            var headers = Encoding.UTF8.GetString(bytes.Take(headerEnd).ToArray());
            var contentLength = ContentLength(headers);
            while (bytes.Count < headerEnd + contentLength)
            {
                var read = await stream.ReadAsync(buffer, 0, buffer.Length);
                if (read == 0)
                {
                    break;
                }
                bytes.AddRange(buffer.Take(read));
            }

            var request = Encoding.UTF8.GetString(bytes.ToArray());
            var response = Encoding.UTF8.GetBytes(
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 15\r\nConnection: close\r\n\r\n{\"published\":1}");
            await stream.WriteAsync(response, 0, response.Length);
            return request;
        }

        private static bool HeaderComplete(List<byte> bytes)
        {
            return HeaderEnd(bytes) >= 0;
        }

        private static int HeaderEnd(List<byte> bytes)
        {
            for (var i = 0; i <= bytes.Count - 4; i++)
            {
                if (bytes[i] == '\r' && bytes[i + 1] == '\n' && bytes[i + 2] == '\r' && bytes[i + 3] == '\n')
                {
                    return i + 4;
                }
            }
            return -1;
        }

        private static int ContentLength(string headers)
        {
            foreach (var line in headers.Split("\r\n"))
            {
                if (!line.StartsWith("Content-Length:", StringComparison.OrdinalIgnoreCase))
                {
                    continue;
                }
                return int.Parse(line["Content-Length:".Length..].Trim());
            }
            return 0;
        }
    }
}
