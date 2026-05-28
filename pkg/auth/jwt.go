package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateJWT creates a signed JWT token for the given playerId.
func GenerateJWT(playerID int64, secret string) (string, error) {
	claims := jwt.MapClaims{
		"playerId": float64(playerID),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateJWT validates a JWT token and returns the playerId from its claims.
func ValidateJWT(tokenString, secret string) (int64, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return 0, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, fmt.Errorf("invalid token claims")
	}

	pid, ok := claims["playerId"].(float64)
	if !ok {
		pid, ok = claims["user_id"].(float64)
	}
	if !ok || pid == 0 {
		return 0, fmt.Errorf("player_id not found in token")
	}
	return int64(pid), nil
}
