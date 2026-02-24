package filters

import "github.com/shagohead/bugpack/bugpack/envelope"

func NewAND(filters []envelope.Filter) envelope.Filter {
	return &AND{Filters: filters}
}

type AND struct {
	Filters []envelope.Filter `yaml:"filters"`
}

// Filter implements Filter.
func (a *AND) Filter(e *envelope.Envelope) bool {
	for _, f := range a.Filters {
		if !f.Filter(e) {
			return false
		}
	}
	return true
}

var _ envelope.Filter = (*AND)(nil)
