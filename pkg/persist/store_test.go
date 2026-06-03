package persist

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A simple counter state for testing.
type testState struct {
	Counter int      `json:"counter"`
	Items   []string `json:"items"`
}

func (s *testState) snapshot() ([]byte, error) {
	return json.Marshal(s)
}

func (s *testState) loadSnapshot(data []byte) error {
	s.Items = nil
	return json.Unmarshal(data, s)
}

func (s *testState) replay(entry []byte) error {
	var op struct {
		OP    string `json:"op"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(entry, &op); err != nil {
		return err
	}
	switch op.OP {
	case "inc":
		s.Counter++
	case "add":
		s.Items = append(s.Items, op.Value)
	case "remove":
		for i, v := range s.Items {
			if v == op.Value {
				s.Items = append(s.Items[:i], s.Items[i+1:]...)
				break
			}
		}
	}
	return nil
}

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "persist-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestFreshStart(t *testing.T) {
	dir := tempDir(t)
	state := &testState{}

	store, err := OpenStore(StoreConfig{
		Dir:          dir,
		Snapshot:     state.snapshot,
		LoadSnapshot: state.loadSnapshot,
		Replay:       state.replay,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Append some entries.
	store.Append([]byte(`{"op":"inc"}`))
	store.Append([]byte(`{"op":"add","value":"hello"}`))
	store.Append([]byte(`{"op":"inc"}`))
	store.Append([]byte(`{"op":"add","value":"world"}`))

	// Snapshot to persist.
	if err := store.Snapshot(); err != nil {
		t.Fatal(err)
	}

	// Verify snapshot file exists.
	if _, err := os.Stat(filepath.Join(dir, "snapshot.json")); err != nil {
		t.Fatal("snapshot.json not created:", err)
	}
	// Verify journal is truncated.
	fi, err := os.Stat(filepath.Join(dir, "journal.log"))
	if err != nil {
		t.Fatal("journal.log gone:", err)
	}
	if fi.Size() != 0 {
		t.Fatal("journal should be empty after snapshot, got", fi.Size(), "bytes")
	}
}

func TestRecovery(t *testing.T) {
	dir := tempDir(t)

	// Phase 1: create state and persist.
	state1 := &testState{}
	store1, err := OpenStore(StoreConfig{
		Dir:          dir,
		Snapshot:     state1.snapshot,
		LoadSnapshot: state1.loadSnapshot,
		Replay:       state1.replay,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Append AND update in-memory state (real app does both).
	applyAndAppend := func(entry string) {
		b := []byte(entry)
		store1.Append(b)
		state1.replay(b)
	}
	applyAndAppend(`{"op":"inc"}`)
	applyAndAppend(`{"op":"add","value":"a"}`)
	applyAndAppend(`{"op":"add","value":"b"}`)
	applyAndAppend(`{"op":"inc"}`)

	// Snapshot saves: counter=2, items=["a","b"]
	if err := store1.Snapshot(); err != nil {
		t.Fatal(err)
	}
	// Append entries AFTER snapshot.
	applyAndAppend(`{"op":"add","value":"c"}`)
	applyAndAppend(`{"op":"inc"}`)
	store1.Close()

	// Phase 2: recover into new state.
	state2 := &testState{}
	store2, err := OpenStore(StoreConfig{
		Dir:          dir,
		Snapshot:     state2.snapshot,
		LoadSnapshot: state2.loadSnapshot,
		Replay:       state2.replay,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	// After recovery: snapshot(counter=2, items=[a,b]) + journal(add "c", inc)
	// → counter=3, items=[a,b,c]
	if state2.Counter != 3 {
		t.Fatalf("expected counter=3, got %d", state2.Counter)
	}
	if len(state2.Items) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(state2.Items), state2.Items)
	}
	if state2.Items[0] != "a" || state2.Items[1] != "b" || state2.Items[2] != "c" {
		t.Fatalf("expected [a b c], got %v", state2.Items)
	}
}

func TestRecoveryJournalOnly(t *testing.T) {
	dir := tempDir(t)

	// Phase 1: create state and persist, but crash before snapshot.
	state1 := &testState{}
	store1, err := OpenStore(StoreConfig{
		Dir:          dir,
		Snapshot:     state1.snapshot,
		LoadSnapshot: state1.loadSnapshot,
		Replay:       state1.replay,
	})
	if err != nil {
		t.Fatal(err)
	}
	store1.Append([]byte(`{"op":"inc"}`))
	store1.Append([]byte(`{"op":"add","value":"x"}`))
	store1.Append([]byte(`{"op":"add","value":"y"}`))
	store1.Close()

	// Remove snapshot (if any initial snapshot was taken).
	os.Remove(filepath.Join(dir, "snapshot.json"))

	// Phase 2: recover from journal only.
	state2 := &testState{}
	store2, err := OpenStore(StoreConfig{
		Dir:          dir,
		Snapshot:     state2.snapshot,
		LoadSnapshot: state2.loadSnapshot,
		Replay:       state2.replay,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	if state2.Counter != 1 {
		t.Fatalf("journal-only: expected counter=1, got %d", state2.Counter)
	}
	if len(state2.Items) != 2 || state2.Items[0] != "x" || state2.Items[1] != "y" {
		t.Fatalf("journal-only: expected [x y], got %v", state2.Items)
	}
}

func TestIdempotentReplay(t *testing.T) {
	dir := tempDir(t)

	state1 := &testState{}
	store1, err := OpenStore(StoreConfig{
		Dir:          dir,
		Snapshot:     state1.snapshot,
		LoadSnapshot: state1.loadSnapshot,
		Replay:       state1.replay,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Append AND update in-memory state.
	entry := []byte(`{"op":"add","value":"foo"}`)
	store1.Append(entry)
	state1.replay(entry)

	// Snapshot saves items=["foo"]. Journal is rotated (empty).
	store1.Snapshot()
	store1.Close()

	// Simulate the crash window: snapshot.json exists with ["foo"],
	// but journal.log somehow still contains entries from before the
	// snapshot was taken (because crash happened between snapshot rename
	// and journal rotation). We manually reproduce this by appending
	// entries that overlap with the snapshot.
	f, _ := os.OpenFile(filepath.Join(dir, "journal.log"), os.O_APPEND|os.O_WRONLY, 0644)
	f.Write([]byte(`{"op":"add","value":"foo"}` + "\n")) // already in snapshot → duplicate
	f.Write([]byte(`{"op":"add","value":"bar"}` + "\n")) // new
	f.Write([]byte(`{"op":"inc"}` + "\n"))               // new
	f.Close()

	// Recover: snapshot has ["foo"], journal replays [add foo, add bar, inc].
	// Without idempotency: items = ["foo", "foo", "bar"].
	state2 := &testState{}
	store2, err := OpenStore(StoreConfig{
		Dir:          dir,
		Snapshot:     state2.snapshot,
		LoadSnapshot: state2.loadSnapshot,
		Replay:       state2.replay,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	// Non-idempotent handler: foo appears twice.
	if state2.Counter != 1 {
		t.Fatalf("expected counter=1, got %d", state2.Counter)
	}
	if len(state2.Items) != 3 { // foo (from snapshot), foo (duplicate replay), bar
		t.Fatalf("expected 3 items (duplicate foo), got %d: %v", len(state2.Items), state2.Items)
	}
}

func TestAutoSnapshot(t *testing.T) {
	dir := tempDir(t)
	state := &testState{}

	store, err := OpenStore(StoreConfig{
		Dir:              dir,
		Snapshot:         state.snapshot,
		LoadSnapshot:     state.loadSnapshot,
		Replay:           state.replay,
		SnapshotInterval: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	store.Append([]byte(`{"op":"inc"}`))
	store.StartAutoSnapshot()

	// Wait for at least one snapshot cycle.
	time.Sleep(500 * time.Millisecond)

	// Snapshot file should exist.
	if _, err := os.Stat(filepath.Join(dir, "snapshot.json")); os.IsNotExist(err) {
		t.Fatal("snapshot not created by auto-snapshot")
	}
}

func TestClose(t *testing.T) {
	dir := tempDir(t)
	state := &testState{}

	store, _ := OpenStore(StoreConfig{
		Dir:          dir,
		Snapshot:     state.snapshot,
		LoadSnapshot: state.loadSnapshot,
		Replay:       state.replay,
	})
	store.Append([]byte(`{"op":"inc"}`))
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	// Append after close should fail.
	if err := store.Append([]byte(`{"op":"inc"}`)); err == nil {
		t.Fatal("expected error on Append after Close, got nil")
	}
}
