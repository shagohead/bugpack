package chbuf

import (
	"github.com/ClickHouse/ch-go/proto"
)

type parent struct {
	Type  string
	Value string
}

func newColParent() *colParent {
	c := new(colParent)
	c.tuple = proto.ColTuple{
		proto.Named(&c.ExcType, "Type"),
		proto.Named(&c.ExcValue, "Value"),
	}
	return c
}

type colParent struct {
	tuple proto.ColTuple

	ExcType  proto.ColStr
	ExcValue proto.ColStr
}

// Append implements proto.ColumnOf.
func (c *colParent) Append(v parent) {
	c.ExcType.Append(v.Type)
	c.ExcValue.Append(v.Value)
}

// AppendArr implements proto.ColumnOf.
func (c *colParent) AppendArr(v []parent) {
	for _, f := range v {
		c.Append(f)
	}
}

// DecodeColumn implements proto.ColumnOf.
func (c *colParent) DecodeColumn(r *proto.Reader, rows int) error {
	return c.tuple.DecodeColumn(r, rows)
}

// EncodeColumn implements proto.ColumnOf.
func (c *colParent) EncodeColumn(b *proto.Buffer) {
	c.tuple.EncodeColumn(b)
}

// Reset implements proto.ColumnOf.
func (c *colParent) Reset() {
	c.tuple.Reset()
}

// Row implements proto.ColumnOf.
func (c *colParent) Row(i int) parent {
	return parent{
		Type:  c.ExcType.Row(i),
		Value: c.ExcValue.Row(i),
	}
}

// Rows implements proto.ColumnOf.
func (c *colParent) Rows() int {
	return c.tuple.Rows()
}

// Type implements proto.ColumnOf.
func (c *colParent) Type() proto.ColumnType {
	return c.tuple.Type()
}

// WriteColumn implements proto.ColumnOf.
func (c *colParent) WriteColumn(w *proto.Writer) {
	c.tuple.WriteColumn(w)
}

var _ proto.ColumnOf[parent] = (*colParent)(nil)
