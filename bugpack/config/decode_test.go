package config

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/shagohead/bugpack/bugpack/envelope"
	"github.com/shagohead/bugpack/bugpack/envelope/filters"
)

func TestUnmarshall(t *testing.T) {
	filterCmpOpt := cmpopts.EquateComparable(filters.Field{})

	assertJoins := func(fn ...func(t *testing.T, c *Config)) func(t *testing.T, c *Config) {
		return func(t *testing.T, c *Config) {
			for _, fn := range fn {
				fn(t, c)
			}
		}
	}

	assertFilters := func(want map[string]envelope.Filter) func(t *testing.T, c *Config) {
		return func(t *testing.T, c *Config) {
			for fkey, want := range want {
				f, ok := c.Filters[fkey]
				if !ok {
					t.Errorf("filter %q not found in config", fkey)
					continue
				}
				if diff := cmp.Diff(want, f.impl, filterCmpOpt); diff != "" {
					t.Errorf("filter %s not match:\n%s", fkey, diff)
				}
			}
			for fkey := range c.Filters {
				if _, ok := want[fkey]; !ok {
					t.Errorf("config got unexpected filter %q", fkey)
				}
			}
		}
	}

	assertProjectFilters := func(want map[string][]envelope.Filter) func(t *testing.T, c *Config) {
		return func(t *testing.T, c *Config) {
			for pkey, want := range want {
				p, ok := c.Projects[pkey]
				if !ok {
					t.Errorf("project %q not found in config", pkey)
					continue
				}
				for i, want := range want {
					if i >= len(p.Filters) {
						t.Errorf("project filter %s[%d] not found", pkey, i)
						continue
					}
					pf := p.Filters[i]
					if pf.filter == nil {
						t.Errorf("nil project filter %s[%d]", pkey, i)
						continue
					}
					if diff := cmp.Diff(want, pf.filter.impl, filterCmpOpt); diff != "" {
						t.Errorf("project filter %s[%d] not match:\n%s", pkey, i, diff)
					}
				}
				if n := len(p.Filters) - len(want); n > 0 {
					t.Errorf("project has %d extra filters", n)
				}
			}
			for pkey := range c.Projects {
				if _, ok := want[pkey]; !ok {
					t.Errorf("config got unexpected project %q", pkey)
				}
			}
		}
	}

	for _, tt := range []struct {
		name   string
		input  string
		assert func(t *testing.T, c *Config)
		fails  bool
	}{
		{
			name: "Filter/equals",
			input: `---
filters:
  testFilter: {type: equals, field: level, value: warning}`,
			assert: assertFilters(map[string]envelope.Filter{
				"testFilter": filters.NewField(filters.CmpEquals, "level", "warning"),
			}),
		},
		{
			name: "Filter/contains",
			input: `---
filters:
  testFilter: {type: contains, field: level, value: warn}`,
			assert: assertFilters(map[string]envelope.Filter{
				"testFilter": filters.NewField(filters.CmpContains, "level", "warn"),
			}),
		},
		{
			name: "Filter/and",
			input: `---
filters:
  testArr:
    type: and
    filters:
      - {type: equals, field: environment, value: production}
      - {type: contains, field: level, value: warn}`,
			assert: assertFilters(map[string]envelope.Filter{
				"testArr": filters.NewAND([]envelope.Filter{
					filters.NewField(filters.CmpEquals, "environment", "production"),
					filters.NewField(filters.CmpContains, "level", "warn"),
				}),
			}),
		},
		{
			name: "Filters",
			input: `---
filters:
  first: {type: equals, field: level, value: warning}
  second:
    type: contains
    field: environment
    value: prod`,
			assert: assertFilters(map[string]envelope.Filter{
				"first":  filters.NewField(filters.CmpEquals, "level", "warning"),
				"second": filters.NewField(filters.CmpContains, "environment", "prod"),
			}),
		},
		{
			name: "ProjectFilter/ref",
			input: `---
filters:
  testFilter: {type: equals, field: level, value: warning}
projects:
  testProject:
    filters:
      - testFilter`,
			assert: assertJoins(
				assertFilters(map[string]envelope.Filter{
					"testFilter": filters.NewField(filters.CmpEquals, "level", "warning"),
				}),
				assertProjectFilters(map[string][]envelope.Filter{
					"testProject": {filters.NewField(filters.CmpEquals, "level", "warning")},
				}),
			),
		},
		{
			name: "ProjectFilter/ref/unresolved",
			input: `---
projects:
  testProject:
    filters:
      - testFilter`,
			fails: true,
		},
		{
			name: "ProjectFilter/spec/oneline",
			input: `---
projects:
  testProject:
    filters:
      - {type: equals, field: level, value: warning}`,
			assert: assertProjectFilters(map[string][]envelope.Filter{
				"testProject": {filters.NewField(filters.CmpEquals, "level", "warning")},
			}),
		},
		{
			name: "ProjectFilter/spec/multiline",
			input: `---
projects:
  testProject:
    filters:
      - type: equals
        field: level
        value: warning`,
			assert: assertProjectFilters(map[string][]envelope.Filter{
				"testProject": {filters.NewField(filters.CmpEquals, "level", "warning")},
			}),
		},
		{
			name: "ProjectFilter/spec/anchors",
			input: `---
template: &tpl
  type: contains
  field: level
projects:
  anchored:
    filters:
      - &rfilter {type: equals, field: level, value: warning}
  aliased:
    filters:
      - *rfilter
  merged:
    filters:
      - <<: *tpl
        value: warn`,
			assert: assertProjectFilters(map[string][]envelope.Filter{
				"anchored": {filters.NewField(filters.CmpEquals, "level", "warning")},
				"aliased":  {filters.NewField(filters.CmpEquals, "level", "warning")},
				"merged":   {filters.NewField(filters.CmpContains, "level", "warn")},
			}),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := new(Config)
			err := c.unmarshal(strings.NewReader(tt.input))
			if err != nil {
				if !tt.fails {
					t.Errorf("%s unmarshal: %v", tt.name, err)
				}
				return
			}
			if tt.fails {
				t.Errorf("%s unexpected successful", tt.name)
			}
			if tt.assert != nil {
				tt.assert(t, c)
			}
		})
	}
}
