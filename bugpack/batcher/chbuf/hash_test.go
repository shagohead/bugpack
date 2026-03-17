package chbuf

import (
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/go-faster/jx"
	"github.com/google/go-cmp/cmp"

	"github.com/shagohead/bugpack/bugpack/envelope"
)

func Test_calcHash(t *testing.T) {
	var sums []uint64

	checksum := func(t *testing.T, e *Envelope, sum func(*Envelope) uint64, repr func(*Envelope) any) {
		e.calcHash()
		s := sum(e)
		v := repr(e)
		{ // Hash again and compare with previous sum.
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
			{Level: "error", Message: ""},
			{Level: "err", Message: "or"},
			{Level: "er", Message: "ror"},
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

func TestHashDetermenism(t *testing.T) {
	type sum struct {
		msg uint64
		exc []uint64
	}

	for name, sum := range map[string]sum{
		"envelope_object_exception": {exc: []uint64{12263778202603919805}},
		"envelope_array_exceptions": {exc: []uint64{3493830303312952130, 16100857950136530739, 942250274704261214}},
	} {
		t.Run(name, func(t *testing.T) {
			f, err := os.Open(fmt.Sprintf("../../envelope/testdata/%s.json", name))
			if err != nil {
				t.Fatalf("%s: opening error: %v", name, err)
			}
			t.Cleanup(func() { f.Close() })
			e := new(envelope.Envelope)
			d := new(jx.Decoder)
			d.Reset(f)
			if err := e.Decode(d); err != nil {
				t.Fatalf("%s: decoding error: %v", name, err)
			}
			w := Bufferer(nil).Envelope(e)
			if w.msgHash != sum.msg {
				t.Errorf("%s: msgHash = %v, want %v", name, w.msgHash, sum.msg)
			}
			if d := cmp.Diff(sum.exc, w.excHash); d != "" {
				t.Errorf("%s: excHash diff:\n%s", name, d)
			}
		})
	}
}
