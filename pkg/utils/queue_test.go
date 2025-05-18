package utils

import (
	"reflect"
	"testing"

	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/memory"
)

func TestDynamicFixedQueue_ToSlice(t *testing.T) {
	type testCase[T any] struct {
		name    string
		q       *dynamicFixedQueue[T]
		want    []T
		wantErr bool
	}

	correctTranscript := memory.TranscriptGeneration{
		chat.SystemLayer:    []memory.Message{},
		chat.PhoneticsLayer: []memory.Message{{Text: "Hello there!"}},
	}

	uq, _ := newQueue[memory.Generation](1)

	q := &dynamicFixedQueue[memory.Generation]{queue: uq}
	_ = q.Enqueue(
		memory.Generation{
			Transcript: correctTranscript,
			Logography: memory.LogographyGeneration{},
		},
	)

	tests := []testCase[memory.Generation]{
		{
			name: "slices are equal",
			q:    q,
			want: []memory.Generation{
				{
					Transcript: correctTranscript,
					Logography: memory.LogographyGeneration{},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got, err := tt.q.ToSlice()

				if (err != nil) != tt.wantErr {
					t.Errorf(
						"ToSlice() error = %v, wantErr %v",
						err,
						tt.wantErr,
					)
					return
				}

				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("ToSlice() got = %v, want %v", got, tt.want)
				}

				// Assert that the underlying queues were NOT touched.

				for _, v := range q.data {
					if !reflect.DeepEqual(v.Transcript, correctTranscript) {
						t.Errorf(
							"underlying Queue data dissimilar: got = %v,"+
								" want %v", v.Transcript,
							correctTranscript,
						)
					}
				}

				for _, v := range tt.q.data {
					if !reflect.DeepEqual(v.Transcript, correctTranscript) {
						t.Errorf(
							"underlying Queue data dissimilar: got = %v,"+
								" want %v", v.Transcript,
							correctTranscript,
						)
					}
				}
			},
		)
	}
}

func TestDynamicFixedQueue_Enqueue(t *testing.T) {
	type intTestCase struct {
		name      string
		initial   []int
		capacity  int
		items     []int
		wantSlice []int
		wantErr   bool
	}

	tests := []intTestCase{
		{
			name:      "enqueue single item",
			initial:   []int{},
			capacity:  3,
			items:     []int{1},
			wantSlice: []int{1},
			wantErr:   false,
		},
		{
			name:      "enqueue multiple items within capacity",
			initial:   []int{1},
			capacity:  3,
			items:     []int{2, 3},
			wantSlice: []int{1, 2, 3},
			wantErr:   false,
		},
		{
			name:      "overwrite when full",
			initial:   []int{1, 2, 3},
			capacity:  3,
			items:     []int{4},
			wantSlice: []int{2, 3, 4},
			wantErr:   false,
		},
		{
			name:      "error when items exceed capacity",
			initial:   []int{1},
			capacity:  3,
			items:     []int{2, 3, 4, 5},
			wantSlice: []int{1},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				q, err := NewDynamicFixedQueue[int](tt.capacity)
				if err != nil {
					t.Fatalf("failed to create queue: %v", err)
				}

				for _, v := range tt.initial {
					if err := q.Enqueue(v); err != nil {
						t.Fatalf("setup Enqueue error: %v", err)
					}
				}

				err = q.Enqueue(tt.items...)
				if (err != nil) != tt.wantErr {
					t.Errorf("Enqueue() error = %v, wantErr %v", err, tt.wantErr)
				}
				got, _ := q.ToSlice()
				if !reflect.DeepEqual(got, tt.wantSlice) {
					t.Errorf("contents = %v, want %v", got, tt.wantSlice)
				}
			},
		)
	}
}
