package utils

import (
	"reflect"
	"testing"

	"codeberg.org/n30w/jasima/chat"
	"codeberg.org/n30w/jasima/memory"
)

func TestFixedQueue_ToSlice(t *testing.T) {
	type testCase[T any] struct {
		name    string
		q       *FixedQueue[T]
		want    []T
		wantErr bool
	}

	correctTranscript := memory.TranscriptGeneration{
		chat.SystemLayer:    []memory.Message{},
		chat.PhoneticsLayer: []memory.Message{{Text: "Hello there!"}},
	}

	q, _ := NewFixedQueue[memory.Generation](1)
	q.Enqueue(
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
