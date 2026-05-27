using System;
using System.Net.WebSockets;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using Pb;

namespace CardWarClient;

public class ChatClient : IDisposable
{
    private readonly ClientWebSocket _ws = new();
    private readonly long _playerId;
    private readonly string _secret;
    private readonly string _gatewayUrl;

    public event Action<ChatResp>? OnChatResp;
    public event Action<string>? OnError;

    public ChatClient(long playerId, string gatewayHost, int gatewayPort, string jwtSecret)
    {
        _playerId = playerId;
        _secret = jwtSecret;
        _gatewayUrl = $"ws://{gatewayHost}:{gatewayPort}/ws";
    }

    public async Task ConnectAsync()
    {
        var token = JwtHelper.Generate(_playerId, _secret);
        var uri = new Uri($"{_gatewayUrl}?token={Uri.EscapeDataString(token)}");

        await _ws.ConnectAsync(uri, CancellationToken.None);
        Console.WriteLine($"[Client {_playerId}] Connected");

        // Start receive loop and wait for it to be ready
        var recvStarted = new TaskCompletionSource();
        _ = Task.Run(() => ReceiveLoop(recvStarted));
        await recvStarted.Task;
    }

    /// <summary>Send Ping (msgID=1) to test WebSocket round-trip. Gateway echoes Pong (msgID=2).</summary>
    public async Task SendPing()
    {
        // DataPack BE: [msgID=1][dataLen=0]
        var frame = new byte[] { 0, 0, 0, 1, 0, 0, 0, 0 };
        Console.WriteLine("[DEBUG] Sending Ping (msgId=1)");
        await _ws.SendAsync(frame, WebSocketMessageType.Binary, true, CancellationToken.None);
    }

    public async Task SendChat(string content, long targetPlayerId = 0)
    {
        var chatReq = new ChatReq
        {
            Content = content,
            TargetPlayerId = targetPlayerId
        };
        await SendMessage(5, chatReq);
    }

    private async Task SendMessage(uint msgId, IMessage message)
    {
        // Zinx DataPack (default): BigEndian, msgID first
        // Format: [4B msgID BE][4B dataLen BE][protobuf body]
        var body = message.ToByteArray();
        var frame = new byte[8 + body.Length];
        frame[0] = (byte)(msgId >> 24);
        frame[1] = (byte)(msgId >> 16);
        frame[2] = (byte)(msgId >> 8);
        frame[3] = (byte)(msgId);
        frame[4] = (byte)(body.Length >> 24);
        frame[5] = (byte)(body.Length >> 16);
        frame[6] = (byte)(body.Length >> 8);
        frame[7] = (byte)(body.Length);
        Array.Copy(body, 0, frame, 8, body.Length);
        Console.WriteLine($"[DEBUG] Sending msgId={msgId} bodyLen={body.Length}");
        await _ws.SendAsync(frame, WebSocketMessageType.Binary, true, CancellationToken.None);
    }

    private async Task ReceiveLoop(TaskCompletionSource recvStarted)
    {
        var buffer = new byte[4096];
        recvStarted.SetResult(); // signal that we're ready to receive
        Console.WriteLine("[DEBUG] Receive loop started");

        try
        {
            while (_ws.State == WebSocketState.Open)
            {
                Console.WriteLine("[DEBUG] Waiting for data...");
                var result = await _ws.ReceiveAsync(buffer, CancellationToken.None);
                Console.WriteLine($"[DEBUG] Received {result.Count} bytes, type={result.MessageType}");

                if (result.MessageType == WebSocketMessageType.Close)
                    break;
                if (result.Count < 8) continue;

                // Zinx DataPack: [4B msgID BE][4B dataLen BE][body...]
                var msgId   = (uint)(buffer[0] << 24 | buffer[1] << 16 | buffer[2] << 8 | buffer[3]);
                var dataLen = (uint)(buffer[4] << 24 | buffer[5] << 16 | buffer[6] << 8 | buffer[7]);
                var body    = new byte[dataLen];
                Array.Copy(buffer, 8, body, 0, (int)dataLen);
                Console.WriteLine($"[DEBUG] DataPack msgId={msgId} dataLen={dataLen}");
                HandleMessage(msgId, body);
            }
        }
        catch (WebSocketException ex)
        {
            Console.WriteLine($"[DEBUG] WebSocket exception: {ex.Message}");
            OnError?.Invoke($"WebSocket error: {ex.Message}");
        }
        catch (Exception ex)
        {
            Console.WriteLine($"[DEBUG] Unexpected exception: {ex}");
            OnError?.Invoke($"Error: {ex.Message}");
        }

        Console.WriteLine("[DEBUG] Receive loop exited");
    }

    private void HandleMessage(uint msgId, byte[] body)
    {
        switch (msgId)
        {
            case 6: // MsgIdChatResp
                var msg = ChatResp.Parser.ParseFrom(body);
                if (msg.SenderPlayerId == -1)
                    Console.WriteLine($"[Error] {msg.Content}");
                else
                    OnChatResp?.Invoke(msg);
                break;
            case 2: // MsgIdPong
                Console.WriteLine("[DEBUG] Pong received");
                break;
            default:
                Console.WriteLine($"[DEBUG] Unknown msgId={msgId}");
                break;
        }
    }

    public void Dispose() => _ws.Dispose();
}
