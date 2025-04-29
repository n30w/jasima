package utils

import (
	"fmt"

	"github.com/pkg/errors"
)

// FixedQueue is a fixed-capacity FIFO queue backed by a circular buffer. This
// implementation is modified from a ChatGPT o4-mini-high implementation. See:
// https://chatgpt.com/share/68103f68-dab0-800c-b74c-0c8453d2a5d2
type FixedQueue[T any] struct {
	data     []T // underlying slice of capacity
	head     int // index of next element to dequeue
	tail     int // index of next slot to enqueue
	size     int // current number of elements
	capacity int // max number of elements
}

// NewFixedQueue creates a new FixedQueue with the given capacity.
// Returns an error if capacity ≤ 0.
func NewFixedQueue[T any](capacity int) (*FixedQueue[T], error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("queue capacity must be > 0; got %d", capacity)
	}
	return &FixedQueue[T]{
		data:     make([]T, capacity),
		capacity: capacity,
	}, nil
}

// Enqueue adds item to the back of the queue. If the queue is full,
// it will overwrite the first element in the queue.
func (q *FixedQueue[T]) Enqueue(item T) error {
	if q.IsFull() {
		_, err := q.Dequeue()
		if err != nil {
			return errors.Wrap(err, "failed to enqueue full queue")
		}
	}
	q.data[q.tail] = item
	q.tail = (q.tail + 1) % q.capacity
	q.size++
	return nil
}

// Dequeue removes and returns the front element.
// Returns an error if the queue is empty.
func (q *FixedQueue[T]) Dequeue() (T, error) {
	var zero T
	if q.IsEmpty() {
		return zero, fmt.Errorf("dequeue from empty queue")
	}
	item := q.data[q.head]
	// zero out the slot (optional, to avoid holding references)
	var empty T
	q.data[q.head] = empty
	q.head = (q.head + 1) % q.capacity
	q.size--
	return item, nil
}

// Peek returns the front element without removing it.
// Returns an error if the queue is empty.
func (q *FixedQueue[T]) Peek() (T, error) {
	var zero T
	if q.IsEmpty() {
		return zero, fmt.Errorf("peek on empty queue")
	}
	return q.data[q.head], nil
}

// IsEmpty reports whether the queue has no elements.
func (q *FixedQueue[T]) IsEmpty() bool {
	return q.size == 0
}

// IsFull reports whether the queue is at capacity.
func (q *FixedQueue[T]) IsFull() bool {
	return q.size == q.capacity
}

// Size returns the number of elements currently in the queue.
func (q *FixedQueue[T]) Size() int {
	return q.size
}

// Capacity returns the fixed maximum number of elements.
func (q *FixedQueue[T]) Capacity() int {
	return q.capacity
}

// ToSlice generates a slice from a queue. The queue is not emptied. The values
// are only copied.
func (q *FixedQueue[T]) ToSlice() ([]T, error) {
	arr := make([]T, q.size)
	if q.IsEmpty() {
		return arr, nil
	}

	// Make a deep copy of the data.

	qCopy := *q

	qCopy.data = make([]T, q.size)

	copy(qCopy.data, q.data)

	s := qCopy.Size()

	for i := range s {
		v, err := qCopy.Dequeue()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create slice from queue")
		}

		// Add to the front of the array first.

		arr[i] = v
	}

	return arr, nil
}

// QueueFromSlice generates a queue from a slice.
func QueueFromSlice[T any](s []T) (*FixedQueue[T], error) {
	errMsg := "failed to create slice from queue"
	l := len(s)
	q, err := NewFixedQueue[T](l)
	if err != nil {
		return nil, errors.Wrap(err, errMsg)
	}

	for _, v := range s {
		err := q.Enqueue(v)
		if err != nil {
			return nil, errors.Wrap(err, errMsg)
		}
	}

	return q, nil
}
