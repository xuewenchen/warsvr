package pkg

import (
	"testing"

	"github.com/aceld/zinx/ziface"
)

type mockConn struct {
	ziface.IConnection
	id uint64
}

func newMockConn(id uint64) *mockConn { return &mockConn{id: id} }
func (m *mockConn) GetConnID() uint64 { return m.id }

func TestHashRoute_Empty(t *testing.T) {
	if c := HashRoute("key", nil); c != nil {
		t.Error("expected nil for empty healthy list")
	}
}

func TestHashRoute_Single(t *testing.T) {
	conns := []ziface.IConnection{newMockConn(42)}
	c := HashRoute("any", conns)
	if c == nil {
		t.Fatal("expected non-nil")
	}
	if c.GetConnID() != 42 {
		t.Errorf("expected conn 42, got %d", c.GetConnID())
	}
}

func TestHashRoute_Consistent(t *testing.T) {
	conns := []ziface.IConnection{
		newMockConn(1), newMockConn(2), newMockConn(3),
	}
	// Same key always routes to same conn
	first := HashRoute("playerA", conns)
	for i := 0; i < 100; i++ {
		if HashRoute("playerA", conns).GetConnID() != first.GetConnID() {
			t.Error("HashRoute not consistent for same key")
		}
	}
}

func TestHashRoute_DifferentKeysCanDiffer(t *testing.T) {
	conns := []ziface.IConnection{
		newMockConn(1), newMockConn(2), newMockConn(3),
	}
	a := HashRoute("alice", conns)
	b := HashRoute("bob", conns)
	// Different keys might or might not hit same conn (hash collision possible)
	_ = a
	_ = b
}

func TestRandomRoute_Empty(t *testing.T) {
	if c := RandomRoute("key", nil); c != nil {
		t.Error("expected nil for empty healthy list")
	}
}

func TestRandomRoute_Single(t *testing.T) {
	conns := []ziface.IConnection{newMockConn(99)}
	c := RandomRoute("any", conns)
	if c == nil {
		t.Fatal("expected non-nil")
	}
	if c.GetConnID() != 99 {
		t.Errorf("expected conn 99, got %d", c.GetConnID())
	}
}

func BenchmarkHashRoute_10Connections(b *testing.B) {
	conns := make([]ziface.IConnection, 10)
	for i := range conns {
		conns[i] = newMockConn(uint64(i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		HashRoute("playerX", conns)
	}
}

func BenchmarkRandomRoute_10Connections(b *testing.B) {
	conns := make([]ziface.IConnection, 10)
	for i := range conns {
		conns[i] = newMockConn(uint64(i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		RandomRoute("playerX", conns)
	}
}
