package utils

import (
	"reflect"
	"testing"

	"codeberg.org/n30w/jasima/chat"
	"codeberg.org/n30w/jasima/memory"
)

// Generation contains all generational information related to a single
// iteration of a conlang's development.
type Generation struct {
	Transcript     []memory.Message
	Logography     LogographyGeneration
	Specifications chat.LayerMessageSet
}

type ts interface {
	Generation | string
}

type LogographyGeneration map[string]string

func TestFixedQueue_ToSlice(t *testing.T) {
	type testCase[T ts] struct {
		name    string
		q       *FixedQueue[T]
		want    []T
		wantErr bool
	}

	correctTranscript := []memory.Message{
		{Text: "Hello World"},
	}

	q, _ := NewFixedQueue[Generation](1)
	q.Enqueue(
		Generation{Transcript: correctTranscript, Logography: LogographyGeneration{}},
	)

	tests := []testCase[Generation]{
		{
			name: "string",
			q:    q,
			want: []Generation{
				{
					Transcript: correctTranscript,
					Logography: LogographyGeneration{},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				// fmt.Printf("%#v\n", q)
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
