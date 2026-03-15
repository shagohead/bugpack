package chbuf

import (
	"slices"
	"testing"

	"github.com/shagohead/bugpack/bugpack/envelope"
)

func Test_calcHash(t *testing.T) {
	var sums []uint64

	checksum := func(t *testing.T, e *Envelope, sum func(*Envelope) uint64, repr func(*Envelope) any) {
		e.calcHash()
		s := sum(e)
		v := repr(e)
		{ // Checking algorithm determinism.
			e.calcHash()
			if n := sum(e); n != s {
				t.Errorf("not matched checksum %v to %v for %+v", n, s, v)
			}
		}
		if slices.Contains(sums, s) {
			t.Errorf("collision checksum: %v for %+v", s, v)
			return
		}
		t.Logf("calculated hash %v", s)
		sums = append(sums, s)
	}

	t.Run("message", func(t *testing.T) {
		for _, e := range []envelope.Event{
			{Level: "", Message: ""},
			{Level: "abc", Message: ""},
			{Level: "", Message: "abc"},
		} {
			checksum(
				t, &Envelope{Envelope: &envelope.Envelope{Event: e}},
				func(e *Envelope) uint64 { return e.msgHash },
				func(e *Envelope) any { return e.Envelope.Event },
			)
		}
	})

	t.Run("exception", func(t *testing.T) {
		for _, e := range []envelope.Exception{
			{},
			{Module: "a", Type: ""},
			{Module: "", Type: "a"},
			{Frames: envelope.Array[envelope.Frame, *envelope.Frame]{
				{PreCtx: []string{"a", ""}},
			}},
			{Frames: envelope.Array[envelope.Frame, *envelope.Frame]{
				{PreCtx: []string{"", "a"}},
			}},
			{Frames: envelope.Array[envelope.Frame, *envelope.Frame]{
				{PostCtx: []string{"", "a"}},
			}},
		} {
			checksum(
				t, &Envelope{Envelope: &envelope.Envelope{Event: envelope.Event{
					Exception: envelope.Array[envelope.Exception, *envelope.Exception]{e},
				}}},
				func(e *Envelope) uint64 { return e.excHash[0] },
				func(e *Envelope) any { return e.Envelope.Exception[0] },
			)
		}
	})
}
