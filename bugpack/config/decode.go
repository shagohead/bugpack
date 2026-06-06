package config

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/shagohead/bugpack/bugpack/envelope"
	"github.com/shagohead/bugpack/bugpack/envelope/filters"
)

func Load(filePath string) (*Config, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	c := new(Config)
	if err = c.configure(f); err != nil {
		return nil, fmt.Errorf("configure from %s: %v", filePath, err)
	}
	return c, nil
}

type ctxKey int

const (
	ctxDecoder ctxKey = iota
)

func decoder(ctx context.Context) *yaml.Decoder {
	if v := ctx.Value(ctxDecoder); v != nil {
		return v.(*yaml.Decoder)
	}
	panic("cannot be used out of *Config.unmarshal()")
}

func (c *Config) unmarshal(src io.Reader) error {
	d := yaml.NewDecoder(src)
	ctx := context.WithValue(context.Background(), ctxDecoder, d)
	if err := d.DecodeContext(ctx, c); err != nil {
		return err
	}
	for key, p := range c.Projects {
		if p == nil {
			p = new(project)
			c.Projects[key] = p
		}
		if p.Rename == "" {
			p.Rename = key
		}
		for i, pf := range p.Filters {
			if pf.lazyRef != "" {
				if c.Filters == nil {
					return fmt.Errorf("cannot resolve filter %q from nil mapping", pf.lazyRef)
				}
				f, ok := c.Filters[pf.lazyRef]
				if !ok {
					return fmt.Errorf("filter named %q not found", pf.lazyRef)
				}
				p.Filters[i].filter = f
			}
		}
	}
	return nil
}

// UnmarshalYAML implements yaml.NodeUnmarshalerContext.
func (l *LogFormat) UnmarshalYAML(ctx context.Context, s ast.Node) error {
	if t := s.Type(); t != ast.StringType {
		return fmt.Errorf("unexpected LogFormat node type: %s", t)
	}
	switch v := s.String(); LogFormat(v) {
	case LogFormatJSON, LogFormatText:
		*l = LogFormat(v)
		return nil
	default:
		return fmt.Errorf("invalid LogFormat value %q; expected text or json", v)
	}
}

var _ yaml.NodeUnmarshalerContext = (*LogFormat)(nil)

type baseFilter struct {
	Type string `yaml:"type"`
}

type arrayFilter struct {
	Filters []*filter `yaml:"filters"`
}

type fieldFilter struct {
	Field string `yaml:"field"`
	Value any    `yaml:"value"`
}

// UnmarshalYAML implements yaml.NodeUnmarshalerContext.
func (f *filter) UnmarshalYAML(ctx context.Context, s ast.Node) error {
	if s.Type() != ast.MappingType {
		return fmt.Errorf("unexpected filter node type: %s", s.Type())
	}
	d := decoder(ctx)
	base := baseFilter{}
	if err := d.DecodeFromNodeContext(ctx, s, &base); err != nil {
		return err
	}
	if base.Type == "and" {
		arr := arrayFilter{}
		if err := d.DecodeFromNodeContext(ctx, s, &arr); err != nil {
			return err
		}
		sub := make([]envelope.Filter, len(arr.Filters))
		for i, s := range arr.Filters {
			sub[i] = s.impl
		}
		f.impl = filters.NewAND(sub)
		return nil
	}
	cmp, err := filters.FieldCmp(base.Type)
	if err != nil {
		return err
	}
	field := fieldFilter{}
	if err := d.DecodeFromNodeContext(ctx, s, &field); err != nil {
		return err
	}
	f.impl = filters.NewField(cmp, field.Field, field.Value)
	return nil
}

var _ yaml.NodeUnmarshalerContext = (*filter)(nil)

// UnmarshalYAML implements yaml.NodeUnmarshalerContext.
func (p *pfilter) UnmarshalYAML(ctx context.Context, s ast.Node) error {
	switch s.Type() {
	case ast.MappingType:
		p.filter = new(filter)
		return decoder(ctx).DecodeFromNodeContext(ctx, s, p.filter)
	case ast.StringType:
		p.lazyRef = s.String()
	default:
		return fmt.Errorf("unexpected project filter node type: %s", s.Type())
	}
	return nil
}

var _ yaml.NodeUnmarshalerContext = (*pfilter)(nil)
