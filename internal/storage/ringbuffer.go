package storage

import "sync"

// RingBuffer is a generic thread-safe ring buffer that stores a fixed number of items.
// When the buffer is full, adding a new item overwrites the oldest item.
// All operations are O(1) except GetAll() which is O(n) where n is the current size.
type RingBuffer[T any] struct {
	sync.RWMutex
	items        []T
	capacity     int
	head         int // next write position (wraps at capacity)
	size         int // current number of items
	totalWritten int // monotonically increasing count of all items ever added
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
	rb.totalWritten++

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
	rb.totalWritten = 0
}

// GetRange returns items between start and end positions (inclusive).
// Positions are absolute (monotonically increasing) values from CurrentPosition.
// Returns nil if the range is invalid, empty, or has been evicted from the buffer.
func (rb *RingBuffer[T]) GetRange(start, end int) []T {
	rb.RLock()
	defer rb.RUnlock()

	if rb.size == 0 || start < 0 || end < start {
		return nil
	}

	// The oldest position still in the buffer
	oldestPos := max(rb.totalWritten-rb.size, 0)

	// Clamp range to what's available
	start = max(start, oldestPos)
	if end >= rb.totalWritten {
		end = rb.totalWritten - 1
	}
	if start > end {
		return nil
	}

	rangeSize := end - start + 1
	result := make([]T, rangeSize)
	for i := 0; i < rangeSize; i++ {
		result[i] = rb.items[(start+i)%rb.capacity]
	}

	return result
}

// CurrentPosition returns the total number of items ever added to the buffer.
// This is a monotonically increasing value used by snapshots to bookmark a point
// in time. Use with GetRange to retrieve items between two positions.
func (rb *RingBuffer[T]) CurrentPosition() int {
	rb.RLock()
	defer rb.RUnlock()
	return rb.totalWritten
}
