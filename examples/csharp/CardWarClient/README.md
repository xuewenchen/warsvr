# CardWar C# Client

Minimal C# client for the CardWar Gateway. Connects via WebSocket with JWT authentication and sends/receives protobuf chat messages.

## Requirements

- .NET 8.0 SDK
- No external JWT dependency (uses `HMACSHA256` from `System.Security.Cryptography`)

## Protocol

- **Connection**: `ws://host:port/ws?token=<JWT>`
- **Auth**: JWT signed with HS256, claims `{"playerId": <int64>, "iat": <unix_seconds>}`
- **Framing**: Zinx DataPack 8-byte header (BigEndian): `[4B msgID BE][4B dataLen BE]` + protobuf body
- **Messages**: `ChatReq` (msgID=5), `ChatResp` (msgID=6), `Pong` (msgID=2)

## Quick Start

```bash
dotnet run -- 1     # Player 1 (default)
dotnet run -- 2     # Player 2
dotnet run -- 3     # Player 3
```

## Usage in your own code

```csharp
using var client = new ChatClient(playerId, "127.0.0.1", 19000, "jwt-secret");
client.OnChatResp += msg => Console.WriteLine($"[{msg.SenderPlayerId}] {msg.Content}");
await client.ConnectAsync();
await client.SendChat("hello world");              // global
await client.SendChat("hey", targetPlayerId: 42);  // private
```

## Message IDs

| MsgID | Name | Direction |
|---|---|---|
| 5 | ChatReq | Client → Gateway |
| 6 | ChatResp | Gateway → Client |
| 1 | Ping | Client → Gateway |
| 2 | Pong | Gateway → Client |
