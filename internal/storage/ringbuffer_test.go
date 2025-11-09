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
