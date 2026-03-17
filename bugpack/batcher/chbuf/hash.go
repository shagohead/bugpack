package chbuf

import (
	"fmt"
	"iter"
	"slices"
	"unsafe"

	"github.com/zeebo/xxh3"

	"github.com/shagohead/bugpack/bugpack/envelope"
)

func (e *Envelope) parentHash(x *envelope.Exception) uint64 {
	if x.Mechanism.Parent >= 1 {
		for n, p := range e.Exception {
			if x.Mechanism.Parent == p.Mechanism.ID {
				return e.excHash[n]
			}
		}
	}
	return 0
}

type hashField uint8

const (
	fieldLevel hashField = iota
	fieldMessage
	fieldExcParent
	fieldExcModule
	fieldExcType
	fieldExcValue
	fieldFrameLineNum
	fieldFrameInApp
	fieldFrameFilename
	fieldFrameAbsPath
	fieldFrameModule
	fieldFrameFunction
	fieldFrameCtxLine
	fieldFramePreCtx
	fieldFramePostCtx
)

func (e *Envelope) messageFields() iter.Seq2[hashField, any] {
	return func(yield func(hashField, any) bool) {
		if !yield(fieldLevel, e.Level) {
			return
		}
		if !yield(fieldMessage, e.Message) {
			return
		}
	}
}

type fieldValue struct {
	f hashField
	v any
}

func (e *Envelope) exceptionFields(x *envelope.Exception) iter.Seq2[hashField, any] {
	return func(yield func(hashField, any) bool) {
		for _, s := range []fieldValue{
			{f: fieldLevel, v: e.Level},
			{f: fieldExcParent, v: e.parentHash(x)},
			{f: fieldExcModule, v: x.Module},
			{f: fieldExcType, v: x.Type},
			{f: fieldExcValue, v: x.Value},
		} {
			if !yield(s.f, s.v) {
				return
			}
		}
		for _, f := range x.Frames {
			for _, s := range []fieldValue{
				// TODO: Add Vars encoded as string.
				{f: fieldFrameLineNum, v: f.LineNum},
				{f: fieldFrameInApp, v: f.InApp},
				{f: fieldFrameFilename, v: f.Filename},
				{f: fieldFrameAbsPath, v: f.AbsPath},
				{f: fieldFrameModule, v: f.Module},
				{f: fieldFrameFunction, v: f.Function},
				{f: fieldFrameCtxLine, v: f.CtxLine},
			} {
				if !yield(s.f, s.v) {
					return
				}
			}
			for _, l := range f.PreCtx {
				if !yield(fieldFramePreCtx, l) {
					return
				}
			}
			for _, l := range f.PostCtx {
				if !yield(fieldFramePostCtx, l) {
					return
				}
			}
		}
	}
}

func (e *Envelope) calcHash() {
	buf := make([]byte, 16)
	if len(e.Message) != 0 {
		e.msgHash = hashTuple(buf, e.messageFields())
	}
	if len(e.Exception) != 0 {
		e.excHash = make([]uint64, len(e.Exception))
		slices.SortFunc(e.Exception, func(a, b envelope.Exception) int {
			// FIXME: Ordering is not determined when ID is missing or equals.
			return a.Mechanism.ID - b.Mechanism.ID
		})
		for i, x := range e.Exception {
			e.excHash[i] = hashTuple(buf, e.exceptionFields(&x))
		}
	}
}

func hashTuple(b []byte, src iter.Seq2[hashField, any]) uint64 {
	var x uint64
	for f, v := range src {
		y := hashFieldValue(b, f, v)
		if x == 0 {
			x = y
		} else {
			x = hashCombine(b, x, y)
		}
	}
	return x
}

func hashCombine(b []byte, x, y uint64) uint64 {
	copy(b[:8], (*[8]byte)(unsafe.Pointer(&x))[:])
	copy(b[8:], (*[8]byte)(unsafe.Pointer(&y))[:])
	return hashBytes(b)
}

func hashFieldValue(b []byte, f hashField, v any) uint64 {
	b[0] = byte(f)
	sum := hashValue(v)
	copy(b[1:9], (*[8]byte)(unsafe.Pointer(&sum))[:])
	return hashBytes(b[:9])
}

func hashValue(v any) uint64 {
	var b []byte
	switch t := v.(type) {
	case uint64:
		b = (*[unsafe.Sizeof(t)]byte)(unsafe.Pointer(&t))[:]
	case int:
		b = (*[unsafe.Sizeof(t)]byte)(unsafe.Pointer(&t))[:]
	case bool:
		b = (*[unsafe.Sizeof(t)]byte)(unsafe.Pointer(&t))[:]
	case string:
		b = unsafe.Slice(unsafe.StringData(t), len(t))
	default:
		panic(fmt.Sprintf("hashValue for type %T is unsupported", v))
	}
	return hashBytes(b)
}

func hashBytes(s []byte) uint64 {
	return xxh3.Hash(s)
}
