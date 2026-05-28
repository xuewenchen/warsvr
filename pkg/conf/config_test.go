package conf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	data := `
services:
  gateway:
    - id: gw-1
      tcp_listen: 0.0.0.0:8999
      ws_listen: 0.0.0.0:9000
  chatsvr:
    - id: cs-1
      listen: 0.0.0.0:8001

gateway:
  jwt_secret: "test-secret"
  routes:
    chatsvr:
      forward: [5]
      route_key: playerId
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Load(path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if GlobalConfig.Gateway.JWTSecret != "test-secret" {
		t.Errorf("expected test-secret, got %s", GlobalConfig.Gateway.JWTSecret)
	}
	rc, ok := GlobalConfig.Gateway.Routes["chatsvr"]
	if !ok {
		t.Fatal("chatsvr route not found")
	}
	if len(rc.Forward) != 1 || rc.Forward[0] != 5 {
		t.Errorf("expected forward [5], got %v", rc.Forward)
	}
	if rc.RouteKey != "playerId" {
		t.Errorf("expected route_key playerId, got %s", rc.RouteKey)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	err := Load("/nonexistent/path/config.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseHostPort_Valid(t *testing.T) {
	host, port := ParseHostPort("192.168.1.1:8080")
	if host != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %s", host)
	}
	if port != 8080 {
		t.Errorf("expected 8080, got %d", port)
	}
}

func TestLookupServer_ByID(t *testing.T) {
	servers := []ServerNode{
		{ID: "s1", Listen: "0.0.0.0:8001"},
		{ID: "s2", Listen: "0.0.0.0:8002"},
	}
	s := LookupServer(servers, "s2", "test")
	if s.ID != "s2" {
		t.Errorf("expected s2, got %s", s.ID)
	}
}

func TestLookupServer_DefaultFirst(t *testing.T) {
	servers := []ServerNode{
		{ID: "first", Listen: "0.0.0.0:8001"},
	}
	s := LookupServer(servers, "", "test")
	if s.ID != "first" {
		t.Errorf("expected first, got %s", s.ID)
	}
}
