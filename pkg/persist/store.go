// Package persist provides a generic journal + snapshot persistence store.
// It is model-agnostic — callers control serialization and replay via callbacks.
//
// On-disk layout:
//
//	persistlog/<service>/
//	  snapshot.json    ← latest full-state snapshot
//	  journal.log      ← append-only operation log since last snapshot
//
// Recovery: load snapshot → replay journal. The Replay handler must be
// idempotent because a crash between snapshot rename and journal truncation
// may cause entries already reflected in the snapshot to be replayed.
package persist

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aceld/zinx/zlog"
)

// StateSnapshot returns the serialized full state for snapshotting.
type StateSnapshot func() ([]byte, error)

// SnapshotLoader rebuilds in-memory state from snapshot bytes during recovery.
type SnapshotLoader func(data []byte) error

// ReplayHandler applies a single journal entry during recovery. Must be idempotent.
type ReplayHandler func(entry []byte) error

// StoreConfig holds configuration for a Store.
type StoreConfig struct {
	Dir              string
	Snapshot         StateSnapshot
	LoadSnapshot     SnapshotLoader
	Replay           ReplayHandler
	SnapshotInterval time.Duration // auto-snapshot interval; default 30s if zero
	MaxJournalSize   int64         // safety valve in bytes; default 256MB if zero
}

const (
	defaultSnapshotInterval = 30 * time.Second
	defaultMaxJournalSize   = 256 * 1024 * 1024 // 256 MiB
	snapshotName            = "snapshot.json"
	journalName             = "journal.log"
)

// Store manages journal + snapshot persistence for a single service.
type Store struct {
	cfg StoreConfig

	mu      sync.Mutex
	journal *os.File
	buf     *bufio.Writer
	closed  bool

	snapshotPath string
	journalPath  string
}

// OpenStore initializes or recovers state from the given directory.
// Recovery flow: load snapshot via LoadSnapshot → replay journal via Replay.
func OpenStore(cfg StoreConfig) (*Store, error) {
	if cfg.Snapshot == nil {
		return nil, errors.New("persist: Snapshot is required")
	}
	if cfg.LoadSnapshot == nil {
		return nil, errors.New("persist: LoadSnapshot is required")
	}
	if cfg.Replay == nil {
		return nil, errors.New("persist: Replay is required")
	}
	if cfg.SnapshotInterval <= 0 {
		cfg.SnapshotInterval = defaultSnapshotInterval
	}
	if cfg.MaxJournalSize <= 0 {
		cfg.MaxJournalSize = defaultMaxJournalSize
	}

	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, fmt.Errorf("persist: create dir %s: %w", cfg.Dir, err)
	}

	s := &Store{
		cfg:          cfg,
		snapshotPath: filepath.Join(cfg.Dir, snapshotName),
		journalPath:  filepath.Join(cfg.Dir, journalName),
	}

	// 1. Load snapshot
	if data, err := os.ReadFile(s.snapshotPath); err == nil {
		if err := cfg.LoadSnapshot(data); err != nil {
			zlog.Ins().ErrorF("persist: snapshot load failed, will rebuild from journal: %v", err)
		} else {
			zlog.Ins().InfoF("persist: snapshot loaded from %s (%d bytes)", s.snapshotPath, len(data))
		}
	}

	// 2. Replay journal
	if err := s.replayJournal(); err != nil {
		zlog.Ins().ErrorF("persist: journal replay error: %v", err)
	}

	// 3. Open journal for writing
	f, err := os.OpenFile(s.journalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("persist: open journal: %w", err)
	}
	s.journal = f
	s.buf = bufio.NewWriter(f)

	return s, nil
}

// Append writes one entry to the journal. Thread-safe.
// Each entry is written as a single line (entry + '\n').
func (s *Store) Append(entry []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("persist: store is closed")
	}

	buf := make([]byte, len(entry)+1)
	copy(buf, entry)
	buf[len(entry)] = '\n'

	if _, err := s.buf.Write(buf); err != nil {
		return fmt.Errorf("persist: append: %w", err)
	}
	return s.buf.Flush()
}

// Snapshot atomically writes the current state and truncates the journal.
func (s *Store) Snapshot() error {
	data, err := s.cfg.Snapshot()
	if err != nil {
		return fmt.Errorf("persist: serialize snapshot: %w", err)
	}

	tmpPath := s.snapshotPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("persist: write snapshot: %w", err)
	}
	if err := os.Rename(tmpPath, s.snapshotPath); err != nil {
		return fmt.Errorf("persist: rename snapshot: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("persist: store is closed")
	}

	// Rotate journal: close, delete old file, create fresh empty one.
	// If we crash after delete but before snapshot completes, replay is
	// idempotent (snapshot + journal replay may double-apply).
	if s.journal != nil {
		s.buf.Flush()
		s.journal.Close()
		os.Remove(s.journalPath)

		f, err := os.OpenFile(s.journalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("persist: recreate journal: %w", err)
		}
		s.journal = f
		s.buf.Reset(f)
	}

	return nil
}

// StartAutoSnapshot runs Snapshot() at the configured interval.
// Also enforces the MaxJournalSize safety valve.
func (s *Store) StartAutoSnapshot() {
	interval := s.cfg.SnapshotInterval
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if s.closed {
				return
			}

			size := s.journalSize()
			if size > s.cfg.MaxJournalSize {
				zlog.Ins().ErrorF("persist: journal size %d exceeds limit %d, forcing snapshot",
					size, s.cfg.MaxJournalSize)
			}

			if err := s.Snapshot(); err != nil {
				zlog.Ins().ErrorF("persist: auto snapshot failed: %v", err)
			}
		}
	}()
}

// Close flushes and closes the journal.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true

	if s.buf != nil {
		s.buf.Flush()
	}
	if s.journal != nil {
		return s.journal.Close()
	}
	return nil
}

// journalSize returns the current journal file size in bytes.
func (s *Store) journalSize() int64 {
	fi, err := os.Stat(s.journalPath)
	if err != nil {
		return 0
	}
	return fi.Size()
}

// replayJournal reads the journal line by line and calls Replay for each entry.
func (s *Store) replayJournal() error {
	f, err := os.Open(s.journalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		// Copy the line because scanner reuses its buffer.
		entry := make([]byte, len(line))
		copy(entry, line)
		if err := s.cfg.Replay(entry); err != nil {
			return fmt.Errorf("persist: replay entry %d: %w", count, err)
		}
		count++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("persist: journal scan: %w", err)
	}

	if count > 0 {
		zlog.Ins().InfoF("persist: replayed %d journal entries", count)
	}
	return nil
}
