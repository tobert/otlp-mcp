package storage

import (
	"fmt"
	"sync"
	"time"
)

// SnapshotManager manages named snapshots of buffer positions.
// Snapshots provide lightweight bookmarks for time-based queries.
type SnapshotManager struct {
	sync.RWMutex
	snapshots map[string]*Snapshot
}

// Snapshot represents a point-in-time bookmark across all storage buffers.
// Each snapshot is only 24 bytes (3 ints) making them extremely lightweight.
type Snapshot struct {
	Name      string
	CreatedAt time.Time
	TracePos  int // Position in trace buffer
	LogPos    int // Position in log buffer
	MetricPos int // Position in metric buffer
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager() *SnapshotManager {
	return &SnapshotManager{
		snapshots: make(map[string]*Snapshot),
	}
}

// Create creates a new snapshot with the specified name and positions.
// Returns an error if a snapshot with the same name already exists.
func (sm *SnapshotManager) Create(name string, tracePos, logPos, metricPos int) error {
	sm.Lock()
	defer sm.Unlock()

	if _, exists := sm.snapshots[name]; exists {
		return fmt.Errorf("snapshot %q already exists", name)
	}

	sm.snapshots[name] = &Snapshot{
		Name:      name,
		CreatedAt: time.Now(),
		TracePos:  tracePos,
		LogPos:    logPos,
		MetricPos: metricPos,
	}

	return nil
}

// Get retrieves a snapshot by name.
// Returns an error if the snapshot does not exist.
func (sm *SnapshotManager) Get(name string) (*Snapshot, error) {
	sm.RLock()
	defer sm.RUnlock()

	snap, exists := sm.snapshots[name]
	if !exists {
		return nil, fmt.Errorf("snapshot %q not found", name)
	}

	// Return a copy to avoid concurrent modification
	return &Snapshot{
		Name:      snap.Name,
		CreatedAt: snap.CreatedAt,
		TracePos:  snap.TracePos,
		LogPos:    snap.LogPos,
		MetricPos: snap.MetricPos,
	}, nil
}

// List returns the names of all snapshots.
func (sm *SnapshotManager) List() []string {
	sm.RLock()
	defer sm.RUnlock()

	names := make([]string, 0, len(sm.snapshots))
	for name := range sm.snapshots {
		names = append(names, name)
	}

	return names
}

// Delete removes a snapshot by name.
// Returns an error if the snapshot does not exist.
func (sm *SnapshotManager) Delete(name string) error {
	sm.Lock()
	defer sm.Unlock()

	if _, exists := sm.snapshots[name]; !exists {
		return fmt.Errorf("snapshot %q not found", name)
	}

	delete(sm.snapshots, name)
	return nil
}

// Clear removes all snapshots.
func (sm *SnapshotManager) Clear() {
	sm.Lock()
	defer sm.Unlock()

	sm.snapshots = make(map[string]*Snapshot)
}

// Count returns the number of snapshots.
func (sm *SnapshotManager) Count() int {
	sm.RLock()
	defer sm.RUnlock()

	return len(sm.snapshots)
}
