package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-key"

func TestGenerateAndValidate(t *testing.T) {
	pid := int64(12345)
	token, err := GenerateJWT(pid, testSecret)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	got, err := ValidateJWT(token, testSecret)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}
	if got != pid {
		t.Errorf("expected playerId %d, got %d", pid, got)
	}
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	token, _ := GenerateJWT(1, testSecret)
	_, err := ValidateJWT(token, "wrong-secret")
	if err == nil {
		t.Fatal("expected error with wrong secret")
	}
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	claims := jwt.MapClaims{
		"playerId": float64(42),
		"iat":      time.Now().Add(-2 * time.Hour).Unix(),
		"exp":      time.Now().Add(-1 * time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, _ := tok.SignedString([]byte(testSecret))

	_, err := ValidateJWT(token, testSecret)
	if err == nil {
		t.Fatal("expected error with expired token")
	}
}

func TestValidateJWT_NoPlayerId(t *testing.T) {
	claims := jwt.MapClaims{"iat": time.Now().Unix()}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, _ := tok.SignedString([]byte(testSecret))

	_, err := ValidateJWT(token, testSecret)
	if err == nil {
		t.Fatal("expected error with missing playerId")
	}
}

func TestValidateJWT_GarbageToken(t *testing.T) {
	_, err := ValidateJWT("not.a.token", testSecret)
	if err == nil {
		t.Fatal("expected error with garbage token")
	}
}

func TestGenerateJWT_LargePlayerId(t *testing.T) {
	pid := int64(1 << 53) // max safe integer in float64
	token, err := GenerateJWT(pid, testSecret)
	if err != nil {
		t.Fatalf("GenerateJWT with large pid failed: %v", err)
	}
	got, _ := ValidateJWT(token, testSecret)
	if got != pid {
		t.Errorf("expected %d, got %d", pid, got)
	}
}

func BenchmarkGenerateJWT(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		GenerateJWT(12345, testSecret)
	}
}

func BenchmarkValidateJWT(b *testing.B) {
	token, _ := GenerateJWT(12345, testSecret)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ValidateJWT(token, testSecret)
	}
}
