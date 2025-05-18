package utils

import (
	"fmt"

	"github.com/pkg/errors"
)

// Queue is a FIFO queue backed by a circular buffer that holds items
// of a type T.
type Queue[T any] interface {
	Enqueue(...T) error
	Dequeue() (T, error)
	ToSlice() ([]T, error)
}

// NewStaticFixedQueue returns a fixed-capacity FIFO queue backed by a circular
// buffer. The queue will not accept new items if it is full.
func NewStaticFixedQueue[T any](capacity int) (Queue[T], error) {
	q, err := newQueue[T](capacity)
	if err != nil {
		return nil, err
	}

	return &staticFixedQueue[T]{queue: q}, nil
}

// NewDynamicFixedQueue returns a fixed-capacity FIFO queue backed by a
// circular buffer. The queue will accept new items if it is full, overwriting
// the head of the queue with its successor for every new item.
func NewDynamicFixedQueue[T any](capacity int) (Queue[T], error) {
	q, err := newQueue[T](capacity)
	if err != nil {
		return nil, err
	}

	return &dynamicFixedQueue[T]{queue: q}, nil
}

type queue[T any] struct {
	// data is the underlying slice that holds elements.
	data []T

	// head is the index of the next element to dequeue at.
	head int

	// tail is the index of the next slot to enqueue at.
	tail int

	// size is the current number of elements in the queue.
	size int

	// capacity is the maximum number of elements allowed to queue.
	capacity int
}

func newQueue[T any](capacity int) (*queue[T], error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("queue capacity must be > 0; got %d", capacity)
	}

	return &queue[T]{
		data:     make([]T, capacity),
		capacity: capacity,
	}, nil
}

// Enqueue adds item to the back of the queue.
func (q *queue[T]) Enqueue(items ...T) error {
	l := len(items)

	if l > q.capacity {
		return fmt.Errorf("total items of %d to enqueue exceeds maximum capacity %d", l, q.capacity)
	}

	if l == 0 {
		return errors.New("no items provided to enqueue")
	}

	for i := range items {
		q.enqueue(items[i])
	}

	return nil
}

func (q *queue[T]) enqueue(item T) {
	q.data[q.tail] = item
	q.tail = (q.tail + 1) % q.capacity
	q.size++
}

// Dequeue removes and returns the front element.
// Returns an error if the queue is empty.
func (q *queue[T]) Dequeue() (T, error) {
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
func (q *queue[T]) Peek() (T, error) {
	var zero T
	if q.IsEmpty() {
		return zero, fmt.Errorf("peek on empty queue")
	}
	return q.data[q.head], nil
}

// IsEmpty reports whether the queue has no elements.
func (q *queue[T]) IsEmpty() bool {
	return q.size == 0
}

// IsFull reports whether the queue is at capacity.
func (q *queue[T]) IsFull() bool {
	return q.size == q.capacity
}

// Size returns the number of elements currently in the queue.
func (q *queue[T]) Size() int {
	return q.size
}

// Capacity returns the fixed maximum number of elements.
func (q *queue[T]) Capacity() int {
	return q.capacity
}

// ToSlice generates a slice representation from the queue. The queue is not
// emptied. The values are only deep copied.
func (q *queue[T]) ToSlice() ([]T, error) {
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

// dynamicFixedQueue is a fixed-capacity FIFO queue backed by a circular buffer. This
// implementation is modified from a ChatGPT o4-mini-high implementation. See:
// https://chatgpt.com/share/68103f68-dab0-800c-b74c-0c8453d2a5d2
type dynamicFixedQueue[T any] struct {
	*queue[T]
}

func (q *dynamicFixedQueue[T]) Enqueue(items ...T) error {
	if q.IsFull() {
		_, err := q.Dequeue()
		if err != nil {
			return errors.Wrap(err, "failed to overwrite item in queue")
		}
	}

	return q.queue.Enqueue(items...)
}

type staticFixedQueue[T any] struct {
	*queue[T]
}

func (q *staticFixedQueue[T]) Enqueue(items ...T) error {
	if q.IsFull() {
		return errors.New("queue is full")
	}

	return q.queue.Enqueue(items...)
}

// QueueFromSlice generates a queue from a slice.
func QueueFromSlice[T any](s []T) (Queue[T], error) {
	errMsg := "failed to create slice from queue"
	l := len(s)
	q, err := NewStaticFixedQueue[T](l)
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
