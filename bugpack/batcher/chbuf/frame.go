package chbuf

import (
	"github.com/ClickHouse/ch-go/proto"

	"github.com/shagohead/bugpack/bugpack/envelope"
)

func newColFrame() *colFrame {
	c := &colFrame{
		PreCtx:  new(proto.ColStr).Array(),
		PostCtx: new(proto.ColStr).Array(),
	}
	c.tuple = proto.ColTuple{
		proto.Named(&c.Filename, "Filename"),
		proto.Named(&c.AbsPath, "AbsPath"),
		proto.Named(&c.Module, "Module"),
		proto.Named(&c.Function, "Function"),
		proto.Named(&c.LineNum, "LineNum"),
		proto.Named(&c.CtxLine, "CtxLine"),
		proto.Named(c.PreCtx, "PreCtx"),
		proto.Named(c.PostCtx, "PostCtx"),
		proto.Named(&c.Vars, "Vars"),
		proto.Named(&c.InApp, "InApp"),
	}
	return c
}

type colFrame struct {
	tuple proto.ColTuple

	Filename proto.ColStr
	AbsPath  proto.ColStr
	Module   proto.ColStr
	Function proto.ColStr
	LineNum  proto.ColUInt32
	CtxLine  proto.ColStr
	PreCtx   *proto.ColArr[string]
	PostCtx  *proto.ColArr[string]
	Vars     proto.ColStr
	InApp    proto.ColBool
}

// Append implements proto.ColumnOf.
func (c *colFrame) Append(v envelope.Frame) {
	c.Filename.Append(v.Filename)
	c.AbsPath.Append(v.AbsPath)
	c.Module.Append(v.Module)
	c.Function.Append(v.Function)
	c.LineNum.Append(uint32(v.LineNum))
	c.CtxLine.Append(v.CtxLine)
	c.PreCtx.Append(v.PreCtx)
	c.PostCtx.Append(v.PostCtx)
	c.Vars.Append("") // TODO: Vars map encoded to JSON string.
	c.InApp.Append(v.InApp)
}

// AppendArr implements proto.ColumnOf.
func (c *colFrame) AppendArr(v []envelope.Frame) {
	for _, f := range v {
		c.Append(f)
	}
}

// DecodeColumn implements proto.ColumnOf.
func (c *colFrame) DecodeColumn(r *proto.Reader, rows int) error {
	return c.tuple.DecodeColumn(r, rows)
}

// EncodeColumn implements proto.ColumnOf.
func (c *colFrame) EncodeColumn(b *proto.Buffer) {
	c.tuple.EncodeColumn(b)
}

// Reset implements proto.ColumnOf.
func (c *colFrame) Reset() {
	c.tuple.Reset()
}

// Row implements proto.ColumnOf.
func (c *colFrame) Row(i int) envelope.Frame {
	return envelope.Frame{
		Filename: c.Filename.Row(i),
		AbsPath:  c.AbsPath.Row(i),
		Module:   c.Module.Row(i),
		Function: c.Function.Row(i),
		LineNum:  int(c.LineNum[i]),
		CtxLine:  c.CtxLine.Row(i),
		PreCtx:   c.PreCtx.Row(i),
		PostCtx:  c.PostCtx.Row(i),
		// Vars:     map[string]any{},
		InApp: c.InApp.Row(i),
	}
}

// Rows implements proto.ColumnOf.
func (c *colFrame) Rows() int {
	return c.tuple.Rows()
}

// Type implements proto.ColumnOf.
func (c *colFrame) Type() proto.ColumnType {
	return c.tuple.Type()
}

// WriteColumn implements proto.ColumnOf.
func (c *colFrame) WriteColumn(w *proto.Writer) {
	c.tuple.WriteColumn(w)
}

var _ proto.ColumnOf[envelope.Frame] = (*colFrame)(nil)
