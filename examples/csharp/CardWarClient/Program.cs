using System;
using System.Threading.Tasks;

namespace CardWarClient;

class Program
{
    private const string GatewayHost = "127.0.0.1";
    private const int GatewayPort = 9000;
    private const string JwtSecret = "change-me-in-production";

    static async Task Main(string[] args)
    {
        var playerId = 1L;
        if (args.Length > 0) long.TryParse(args[0], out playerId);

        Console.WriteLine($"=== CardWar C# Client (Player {playerId}) ===");
        Console.WriteLine($"Gateway: ws://{GatewayHost}:{GatewayPort}/ws");
        Console.WriteLine();

        using var client = new ChatClient(playerId, GatewayHost, GatewayPort, JwtSecret);
        client.OnChatResp += msg =>
        {
            var now = DateTime.Now.ToString("HH:mm:ss");
            if (msg.TargetPlayerId != 0)
                Console.WriteLine($"[{now}] [Private] {msg.SenderPlayerId} -> {msg.TargetPlayerId}: {msg.Content}");
            else
                Console.WriteLine($"[{now}] [Global] {msg.SenderPlayerId}: {msg.Content}");
        };
        client.OnError += err => Console.WriteLine($"[{DateTime.Now:HH:mm:ss}] [Error] {err}");

        try
        {
            await client.ConnectAsync();
        }
        catch (Exception ex)
        {
            Console.WriteLine($"Connection failed: {ex.Message}");
            return;
        }

        // Step 1: Ping to verify WebSocket round-trip
        Console.WriteLine(">>> Step 1: Ping test (Gateway local echo)...");
        await client.SendPing();
        await Task.Delay(1000);

        // Step 2: Send chat to verify full chain
        Console.WriteLine(">>> Step 2: Chat test (Gateway -> ChatSvr)...");
        await client.SendChat("Hello from C#!");
        await Task.Delay(2000);

        Console.WriteLine(">>> Ready. Sending every 5s (Ctrl+C to stop)");
        Console.WriteLine();

        var rng = new Random();
        var others = new[] { 2L, 3L, 4L };
        for (int i = 0; ; i++)
        {
            if (i % 3 == 0)
                await client.SendChat($"Hello #{i} from C# client {playerId}");
            else
                await client.SendChat($"Private msg #{i}", others[rng.Next(others.Length)]);

            await Task.Delay(5000);
        }
    }
}
