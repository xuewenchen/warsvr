using System;
using System.Collections.Generic;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace CardWarClient;

/// <summary>
/// Minimal JWT generator using HMACSHA256. No external dependencies.
/// </summary>
public static class JwtHelper
{
    public static string Generate(long playerId, string secret)
    {
        var header = Base64UrlEncode(JsonSerializer.Serialize(new { alg = "HS256", typ = "JWT" }));
        var payload = Base64UrlEncode(JsonSerializer.Serialize(new Dictionary<string, object>
        {
            ["playerId"] = (double)playerId, // JSON number → float64 for JWT compatibility
            ["iat"] = DateTimeOffset.UtcNow.ToUnixTimeSeconds()
        }));

        var signingInput = $"{header}.{payload}";
        using var hmac = new HMACSHA256(Encoding.UTF8.GetBytes(secret));
        var signature = Base64UrlEncode(hmac.ComputeHash(Encoding.UTF8.GetBytes(signingInput)));

        return $"{signingInput}.{signature}";
    }

    private static string Base64UrlEncode(byte[] data)
    {
        return Convert.ToBase64String(data).TrimEnd('=').Replace('+', '-').Replace('/', '_');
    }

    private static string Base64UrlEncode(string raw)
    {
        return Base64UrlEncode(Encoding.UTF8.GetBytes(raw));
    }
}
