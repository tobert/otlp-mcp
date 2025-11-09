package storage

import (
	"testing"
)

func TestSnapshotManagerCreate(t *testing.T) {
	sm := NewSnapshotManager()

	err := sm.Create("snap1", 100, 200, 300)
	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	// Should fail with duplicate name
	err = sm.Create("snap1", 100, 200, 300)
	if err == nil {
		t.Error("expected error creating duplicate snapshot")
	}

	if sm.Count() != 1 {
		t.Errorf("expected 1 snapshot, got %d", sm.Count())
	}
}

func TestSnapshotManagerGet(t *testing.T) {
	sm := NewSnapshotManager()

	err := sm.Create("snap1", 100, 200, 300)
	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	snap, err := sm.Get("snap1")
	if err != nil {
		t.Fatalf("failed to get snapshot: %v", err)
	}

	if snap.Name != "snap1" {
		t.Errorf("expected name 'snap1', got %q", snap.Name)
	}
	if snap.TracePos != 100 {
		t.Errorf("expected trace pos 100, got %d", snap.TracePos)
	}
	if snap.LogPos != 200 {
		t.Errorf("expected log pos 200, got %d", snap.LogPos)
	}
	if snap.MetricPos != 300 {
		t.Errorf("expected metric pos 300, got %d", snap.MetricPos)
	}

	// Should fail with non-existent name
	_, err = sm.Get("nonexistent")
	if err == nil {
		t.Error("expected error getting non-existent snapshot")
	}
}

func TestSnapshotManagerList(t *testing.T) {
	sm := NewSnapshotManager()

	sm.Create("snap1", 100, 200, 300)
	sm.Create("snap2", 150, 250, 350)
	sm.Create("snap3", 200, 300, 400)

	names := sm.List()
	if len(names) != 3 {
		t.Errorf("expected 3 snapshot names, got %d", len(names))
	}

	// Check that all names are present
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	if !nameMap["snap1"] || !nameMap["snap2"] || !nameMap["snap3"] {
		t.Error("missing expected snapshot names")
	}
}

func TestSnapshotManagerDelete(t *testing.T) {
	sm := NewSnapshotManager()

	sm.Create("snap1", 100, 200, 300)
	sm.Create("snap2", 150, 250, 350)

	if sm.Count() != 2 {
		t.Errorf("expected 2 snapshots, got %d", sm.Count())
	}

	err := sm.Delete("snap1")
	if err != nil {
		t.Fatalf("failed to delete snapshot: %v", err)
	}

	if sm.Count() != 1 {
		t.Errorf("expected 1 snapshot after delete, got %d", sm.Count())
	}

	// Should fail with non-existent name
	err = sm.Delete("snap1")
	if err == nil {
		t.Error("expected error deleting non-existent snapshot")
	}
}

func TestSnapshotManagerClear(t *testing.T) {
	sm := NewSnapshotManager()

	sm.Create("snap1", 100, 200, 300)
	sm.Create("snap2", 150, 250, 350)
	sm.Create("snap3", 200, 300, 400)

	if sm.Count() != 3 {
		t.Errorf("expected 3 snapshots, got %d", sm.Count())
	}

	sm.Clear()

	if sm.Count() != 0 {
		t.Errorf("expected 0 snapshots after clear, got %d", sm.Count())
	}
}

func TestSnapshotManagerConcurrent(t *testing.T) {
	sm := NewSnapshotManager()

	// Create snapshots concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			sm.Create(string(rune('a'+n)), n*10, n*20, n*30)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	if sm.Count() != 10 {
		t.Errorf("expected 10 snapshots, got %d", sm.Count())
	}
}
