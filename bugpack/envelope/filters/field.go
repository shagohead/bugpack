package filters

import (
	"fmt"

	"github.com/shagohead/bugpack/bugpack/envelope"
)

type fieldCmp int

const (
	CmpEquals fieldCmp = iota
	CmpContains
)

func FieldCmp(s string) (fieldCmp, error) {
	switch s {
	case "equals":
		return CmpEquals, nil
	case "contains":
		return CmpContains, nil
	default:
		return 0, fmt.Errorf("invalid field compare function: %s", s)
	}
}

func NewField(cmp fieldCmp, f string, v any) envelope.Filter {
	return &Field{cmp: CmpEquals, field: f, value: v}
}

type Field struct {
	cmp   fieldCmp `yaml:"cmp"`
	field string   `yaml:"field"`
	value any      `yaml:"value"`
}

// Filter implements Filter.
func (f *Field) Filter(e *envelope.Envelope) bool {
	switch f.cmp {
	case CmpContains:
	case CmpEquals:
	default:
		panic(fmt.Sprintf("unexpected envelope.fieldCmp: %#v", f.cmp))
	}
	panic("unimplemented")
}

var _ envelope.Filter = (*Field)(nil)
