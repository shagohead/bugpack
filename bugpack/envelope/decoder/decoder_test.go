package decoder

import (
	"io"
	"log"
	"os"
	"path"
	"testing"

	"github.com/go-faster/jx"
)

var (
	envelopeArrayExceptions []byte
	envelopeObjectException []byte
)

func init() {
	for key, dst := range map[string]*[]byte{
		"envelope_object_exception": &envelopeObjectException,
		"envelope_array_exceptions": &envelopeArrayExceptions,
	} {
		f, err := os.Open(path.Join("testdata", key+".json"))
		if err != nil {
			log.Fatal(err)
		}
		*dst, err = io.ReadAll(f)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func TestDecode(t *testing.T) {
	d := jx.GetDecoder()
	t.Run("object-exception", func(t *testing.T) {
		e := new(Envelope)
		d.ResetBytes(envelopeObjectException)
		if err := e.Decode(d); err != nil {
			t.Fatal(err)
		}
		t.Logf("decoded envelope: %+v", *e)
	})
	t.Run("array-exceptions", func(t *testing.T) {
		e := new(Envelope)
		d.ResetBytes(envelopeArrayExceptions)
		if err := e.Decode(d); err != nil {
			t.Fatal(err)
		}
		t.Logf("decoded envelope: %+v", *e)
	})
}

func BenchmarkDecode(b *testing.B) {
	d := jx.GetDecoder()
	b.Run("object-exception", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			d.ResetBytes(envelopeObjectException)
			if err := (&Envelope{}).Decode(d); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("array-exceptions", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			d.ResetBytes(envelopeArrayExceptions)
			if err := (&Envelope{}).Decode(d); err != nil {
				b.Fatal(err)
			}
		}
	})
}
