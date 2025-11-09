package storage

import "sync"

// RingBuffer is a generic thread-safe ring buffer that stores a fixed number of items.
// When the buffer is full, adding a new item overwrites the oldest item.
// All operations are O(1) except GetAll() which is O(n) where n is the current size.
type RingBuffer[T any] struct {
	mu       sync.RWMutex
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
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.items[rb.head] = item
	rb.head = (rb.head + 1) % rb.capacity

	if rb.size < rb.capacity {
		rb.size++
	}
}

// GetAll returns all items in chronological order (oldest to newest).
// The returned slice is a copy and safe to modify.
func (rb *RingBuffer[T]) GetAll() []T {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

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
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

// Capacity returns the maximum capacity of the buffer.
func (rb *RingBuffer[T]) Capacity() int {
	return rb.capacity
}

// Clear removes all items from the buffer.
func (rb *RingBuffer[T]) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.size = 0
	rb.head = 0
}
