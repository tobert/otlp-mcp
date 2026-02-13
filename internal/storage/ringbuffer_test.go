package storage

import (
	"sync"
	"testing"
)

// TestRingBufferBasic tests basic add and retrieval operations.
func TestRingBufferBasic(t *testing.T) {
	rb := NewRingBuffer[int](3)

	// Test initial state
	if rb.Size() != 0 {
		t.Fatalf("expected size 0, got %d", rb.Size())
	}
	if rb.Capacity() != 3 {
		t.Fatalf("expected capacity 3, got %d", rb.Capacity())
	}

	// Add items
	rb.Add(1)
	rb.Add(2)
	rb.Add(3)

	if rb.Size() != 3 {
		t.Fatalf("expected size 3, got %d", rb.Size())
	}

	all := rb.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected 3 items, got %d", len(all))
	}

	// Verify order (oldest to newest)
	expected := []int{1, 2, 3}
	for i, val := range all {
		if val != expected[i] {
			t.Errorf("at index %d: expected %d, got %d", i, expected[i], val)
		}
	}
}

// TestRingBufferWrapping tests that the buffer correctly wraps around.
func TestRingBufferWrapping(t *testing.T) {
	rb := NewRingBuffer[int](3)

	// Fill the buffer
	rb.Add(1)
	rb.Add(2)
	rb.Add(3)

	// Add one more, should evict 1
	rb.Add(4)

	all := rb.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected 3 items after wrap, got %d", len(all))
	}

	expected := []int{2, 3, 4}
	for i, val := range all {
		if val != expected[i] {
			t.Errorf("at index %d: expected %d, got %d", i, expected[i], val)
		}
	}

	// Add more items
	rb.Add(5)
	rb.Add(6)

	all = rb.GetAll()
	expected = []int{4, 5, 6}
	for i, val := range all {
		if val != expected[i] {
			t.Errorf("at index %d: expected %d, got %d", i, expected[i], val)
		}
	}
}

// TestRingBufferGetRecent tests the GetRecent method.
func TestRingBufferGetRecent(t *testing.T) {
	rb := NewRingBuffer[int](10)

	for i := 0; i < 5; i++ {
		rb.Add(i)
	}

	// Get last 3
	recent := rb.GetRecent(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent items, got %d", len(recent))
	}

	expected := []int{2, 3, 4}
	for i, val := range recent {
		if val != expected[i] {
			t.Errorf("at index %d: expected %d, got %d", i, expected[i], val)
		}
	}

	// Request more than available
	recent = rb.GetRecent(10)
	if len(recent) != 5 {
		t.Fatalf("expected 5 items when requesting more than available, got %d", len(recent))
	}
}

// TestRingBufferClear tests the Clear method.
func TestRingBufferClear(t *testing.T) {
	rb := NewRingBuffer[int](5)

	rb.Add(1)
	rb.Add(2)
	rb.Add(3)

	if rb.Size() != 3 {
		t.Fatalf("expected size 3 before clear, got %d", rb.Size())
	}

	rb.Clear()

	if rb.Size() != 0 {
		t.Fatalf("expected size 0 after clear, got %d", rb.Size())
	}

	all := rb.GetAll()
	if all != nil {
		t.Fatalf("expected nil after clear, got %v", all)
	}

	// Should be able to add after clear
	rb.Add(10)
	if rb.Size() != 1 {
		t.Fatalf("expected size 1 after adding post-clear, got %d", rb.Size())
	}
}

// TestRingBufferConcurrent tests thread-safety under concurrent access.
func TestRingBufferConcurrent(t *testing.T) {
	rb := NewRingBuffer[int](1000)

	var wg sync.WaitGroup

	// Concurrent writers
	writers := 10
	writesPerWriter := 100

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			for j := 0; j < writesPerWriter; j++ {
				rb.Add(start*writesPerWriter + j)
			}
		}(i)
	}

	// Concurrent readers
	readers := 5
	readsPerReader := 100

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < readsPerReader; j++ {
				_ = rb.GetAll()
				_ = rb.GetRecent(10)
				_ = rb.Size()
			}
		}()
	}

	wg.Wait()

	// Should have exactly 1000 items (capacity)
	if rb.Size() != 1000 {
		t.Fatalf("expected 1000 items after concurrent writes, got %d", rb.Size())
	}

	// Verify we can still read
	all := rb.GetAll()
	if len(all) != 1000 {
		t.Fatalf("expected 1000 items in GetAll(), got %d", len(all))
	}
}

// TestRingBufferStrings tests the buffer with string types.
func TestRingBufferStrings(t *testing.T) {
	rb := NewRingBuffer[string](3)

	rb.Add("first")
	rb.Add("second")
	rb.Add("third")
	rb.Add("fourth") // Should evict "first"

	all := rb.GetAll()
	expected := []string{"second", "third", "fourth"}

	for i, val := range all {
		if val != expected[i] {
			t.Errorf("at index %d: expected %q, got %q", i, expected[i], val)
		}
	}
}

// TestRingBufferStructs tests the buffer with struct types.
func TestRingBufferStructs(t *testing.T) {
	type TestStruct struct {
		ID   int
		Name string
	}

	rb := NewRingBuffer[TestStruct](2)

	rb.Add(TestStruct{ID: 1, Name: "one"})
	rb.Add(TestStruct{ID: 2, Name: "two"})
	rb.Add(TestStruct{ID: 3, Name: "three"}) // Should evict first

	all := rb.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 items, got %d", len(all))
	}

	if all[0].ID != 2 || all[0].Name != "two" {
		t.Errorf("unexpected first item: %+v", all[0])
	}

	if all[1].ID != 3 || all[1].Name != "three" {
		t.Errorf("unexpected second item: %+v", all[1])
	}
}

// TestRingBufferPointers tests the buffer with pointer types.
func TestRingBufferPointers(t *testing.T) {
	rb := NewRingBuffer[*int](3)

	v1, v2, v3, v4 := 1, 2, 3, 4
	rb.Add(&v1)
	rb.Add(&v2)
	rb.Add(&v3)
	rb.Add(&v4) // Should evict v1

	all := rb.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected 3 items, got %d", len(all))
	}

	if *all[0] != 2 {
		t.Errorf("expected 2, got %d", *all[0])
	}
	if *all[1] != 3 {
		t.Errorf("expected 3, got %d", *all[1])
	}
	if *all[2] != 4 {
		t.Errorf("expected 4, got %d", *all[2])
	}
}

// TestRingBufferCurrentPosition verifies monotonic position tracking.
func TestRingBufferCurrentPosition(t *testing.T) {
	rb := NewRingBuffer[int](3)

	if rb.CurrentPosition() != 0 {
		t.Fatalf("expected position 0, got %d", rb.CurrentPosition())
	}

	rb.Add(10)
	rb.Add(20)
	rb.Add(30)
	if rb.CurrentPosition() != 3 {
		t.Fatalf("expected position 3, got %d", rb.CurrentPosition())
	}

	// Wrap the buffer
	rb.Add(40)
	rb.Add(50)
	if rb.CurrentPosition() != 5 {
		t.Fatalf("expected position 5 after wrap, got %d", rb.CurrentPosition())
	}

	// Keep wrapping
	rb.Add(60)
	rb.Add(70)
	rb.Add(80)
	if rb.CurrentPosition() != 8 {
		t.Fatalf("expected position 8 after multiple wraps, got %d", rb.CurrentPosition())
	}
}

// TestRingBufferGetRange tests GetRange with absolute positions.
func TestRingBufferGetRange(t *testing.T) {
	rb := NewRingBuffer[int](5)

	// Add 5 items (positions 0-4)
	for i := 1; i <= 5; i++ {
		rb.Add(i * 10)
	}

	// Get all items via range
	result := rb.GetRange(0, 4)
	expected := []int{10, 20, 30, 40, 50}
	if len(result) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("at %d: expected %d, got %d", i, expected[i], v)
		}
	}

	// Get subset
	result = rb.GetRange(2, 3)
	expected = []int{30, 40}
	if len(result) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("at %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

// TestRingBufferGetRangeAfterWrap tests GetRange correctness after the buffer wraps.
func TestRingBufferGetRangeAfterWrap(t *testing.T) {
	rb := NewRingBuffer[int](3)

	// Add 6 items to wrap twice (capacity 3)
	for i := 1; i <= 6; i++ {
		rb.Add(i * 10)
	}

	// Position is now 6. Buffer contains items at positions 3,4,5 (values 40,50,60)
	if rb.CurrentPosition() != 6 {
		t.Fatalf("expected position 6, got %d", rb.CurrentPosition())
	}

	// Get all available items
	result := rb.GetRange(3, 5)
	expected := []int{40, 50, 60}
	if len(result) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("at %d: expected %d, got %d", i, expected[i], v)
		}
	}

	// Request evicted range — should clamp to available
	result = rb.GetRange(0, 5)
	if len(result) != 3 {
		t.Fatalf("expected 3 items (clamped), got %d", len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("at %d: expected %d, got %d", i, expected[i], v)
		}
	}

	// Fully evicted range — should return nil
	result = rb.GetRange(0, 2)
	if result != nil {
		t.Fatalf("expected nil for fully evicted range, got %v", result)
	}
}

// TestRingBufferSnapshotWorkflow simulates the snapshot use case:
// take snapshot, add items, take another snapshot, get range between them.
func TestRingBufferSnapshotWorkflow(t *testing.T) {
	rb := NewRingBuffer[int](5)

	// Add some initial data
	for i := 1; i <= 3; i++ {
		rb.Add(i)
	}

	// "Snapshot 1" at position 3
	snap1 := rb.CurrentPosition()

	// Add more data
	for i := 4; i <= 7; i++ {
		rb.Add(i)
	}

	// "Snapshot 2" at position 7
	snap2 := rb.CurrentPosition()

	// Get items between snapshots (should be 4,5,6,7)
	result := rb.GetRange(snap1, snap2-1)
	expected := []int{4, 5, 6, 7}
	if len(result) != len(expected) {
		t.Fatalf("expected %d items between snapshots, got %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("at %d: expected %d, got %d", i, expected[i], v)
		}
	}

	// Add more data to wrap the buffer (capacity 5, so items 1-2 get evicted)
	for i := 8; i <= 12; i++ {
		rb.Add(i)
	}

	// "Snapshot 3" at position 12
	snap3 := rb.CurrentPosition()

	// Range between snap2 and snap3 should return items 8-11 (7 was at snap2)
	result = rb.GetRange(snap2, snap3-1)
	expected = []int{8, 9, 10, 11, 12}
	if len(result) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("at %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

// TestRingBufferEmpty tests operations on an empty buffer.
func TestRingBufferEmpty(t *testing.T) {
	rb := NewRingBuffer[int](5)

	all := rb.GetAll()
	if all != nil {
		t.Errorf("expected nil from GetAll() on empty buffer, got %v", all)
	}

	recent := rb.GetRecent(3)
	if recent != nil {
		t.Errorf("expected nil from GetRecent() on empty buffer, got %v", recent)
	}

	if rb.Size() != 0 {
		t.Errorf("expected size 0, got %d", rb.Size())
	}
}
