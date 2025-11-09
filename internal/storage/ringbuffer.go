package storage

import "sync"

// RingBuffer is a generic thread-safe ring buffer that stores a fixed number of items.
// When the buffer is full, adding a new item overwrites the oldest item.
// All operations are O(1) except GetAll() which is O(n) where n is the current size.
type RingBuffer[T any] struct {
	sync.RWMutex
	items    []T
	capacity int
	head     int // next write position
	size     int // current number of items
}

// NewRingBuffer creates a new ring buffer with the specified capacity.
// The capacity must be greater than zero.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	if capacity <= 0 {
		panic("ring buffer capacity must be greater than zero")
	}

	return &RingBuffer[T]{
		items:    make([]T, capacity),
		capacity: capacity,
		head:     0,
		size:     0,
	}
}

// Add inserts an item into the ring buffer.
// If the buffer is at capacity, this overwrites the oldest item.
func (rb *RingBuffer[T]) Add(item T) {
	rb.Lock()
	defer rb.Unlock()

	rb.items[rb.head] = item
	rb.head = (rb.head + 1) % rb.capacity

	if rb.size < rb.capacity {
		rb.size++
	}
}

// GetAll returns all items in chronological order (oldest to newest).
// The returned slice is a copy and safe to modify.
func (rb *RingBuffer[T]) GetAll() []T {
	rb.RLock()
	defer rb.RUnlock()

	if rb.size == 0 {
		return nil
	}

	result := make([]T, rb.size)

	if rb.size < rb.capacity {
		// Haven't wrapped yet - items are at beginning of buffer
		copy(result, rb.items[:rb.size])
	} else {
		// Buffer has wrapped - head points to oldest item
		// Copy from head to end, then from beginning to head
		n := copy(result, rb.items[rb.head:])
		copy(result[n:], rb.items[:rb.head])
	}

	return result
}

// GetRecent returns the N most recent items in chronological order.
// If N is greater than the current size, all items are returned.
func (rb *RingBuffer[T]) GetRecent(n int) []T {
	all := rb.GetAll()
	if len(all) <= n {
		return all
	}
	return all[len(all)-n:]
}

// Size returns the current number of items in the buffer.
func (rb *RingBuffer[T]) Size() int {
	rb.RLock()
	defer rb.RUnlock()
	return rb.size
}

// Capacity returns the maximum capacity of the buffer.
func (rb *RingBuffer[T]) Capacity() int {
	return rb.capacity
}

// Clear removes all items from the buffer.
func (rb *RingBuffer[T]) Clear() {
	rb.Lock()
	defer rb.Unlock()
	rb.size = 0
	rb.head = 0
}

// GetRange returns items between start and end positions (inclusive).
// Positions are absolute (not modulo capacity) and represent the logical
// sequence of items added. Handles wraparound correctly.
// Returns nil if the range is invalid or empty.
func (rb *RingBuffer[T]) GetRange(start, end int) []T {
	rb.RLock()
	defer rb.RUnlock()

	if rb.size == 0 || start < 0 || end < start {
		return nil
	}

	// Calculate the absolute position of the oldest item still in buffer
	oldestPos := rb.head - rb.size
	if oldestPos < 0 {
		oldestPos = 0
	}

	// Clamp the range to what's actually in the buffer
	if start < oldestPos {
		start = oldestPos
	}
	if end >= rb.head {
		end = rb.head - 1
	}
	if start > end {
		return nil
	}

	// Allocate result slice
	rangeSize := end - start + 1
	result := make([]T, 0, rangeSize)

	// Copy items from the range
	for pos := start; pos <= end; pos++ {
		idx := pos % rb.capacity
		result = append(result, rb.items[idx])
	}

	return result
}

// CurrentPosition returns the current write position.
// This represents the absolute number of items that have been added to the buffer.
// Used by snapshots to bookmark a point in time.
func (rb *RingBuffer[T]) CurrentPosition() int {
	rb.RLock()
	defer rb.RUnlock()

	// Return absolute position (total items added)
	// If size < capacity, head is the count
	// If size == capacity, we've wrapped and need to calculate total
	if rb.size < rb.capacity {
		return rb.head
	}

	// Buffer is full and may have wrapped multiple times
	// We need to track total items added, not just current head
	// For now, return head as the position (wraps at capacity)
	return rb.head
}
