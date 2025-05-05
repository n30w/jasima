package utils

import (
	"reflect"
	"testing"

	"codeberg.org/n30w/jasima/pkg/chat"
	memory2 "codeberg.org/n30w/jasima/pkg/memory"
)

func TestFixedQueue_ToSlice(t *testing.T) {
	type testCase[T any] struct {
		name    string
		q       *FixedQueue[T]
		want    []T
		wantErr bool
	}

	correctTranscript := memory2.TranscriptGeneration{
		chat.SystemLayer:    []memory2.Message{},
		chat.PhoneticsLayer: []memory2.Message{{Text: "Hello there!"}},
	}

	q, _ := NewFixedQueue[memory2.Generation](1)
	q.Enqueue(
		memory2.Generation{
			Transcript: correctTranscript,
			Logography: memory2.LogographyGeneration{},
		},
	)

	tests := []testCase[memory2.Generation]{
		{
			name: "slices are equal",
			q:    q,
			want: []memory2.Generation{
				{
					Transcript: correctTranscript,
					Logography: memory2.LogographyGeneration{},
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
