package config

import (
	"io"
	"log/slog"

	"github.com/shagohead/bugpack/bugpack/batcher"
	"github.com/shagohead/bugpack/bugpack/envelope"
	"github.com/shagohead/bugpack/bugpack/ingester"
)

type LogFormat string

const (
	LogFormatJSON = "json"
	LogFormatText = "text"
)

type Config struct {
	LogFormat    LogFormat           `yaml:"log_format"`
	LogLevel     slog.Level          `yaml:"log_level"`
	ServerAddr   string              `yaml:"server_addr"`
	ListenPrefix string              `yaml:"listen_prefix"`
	HealthPath   string              `yaml:"health_path"`
	Projects     map[string]*project `yaml:"projects"`
	Filters      map[string]*filter  `yaml:"filters"`
	Batcher      batcher.Config      `yaml:"batcher"`
}

// Project implements ingester.ProjectResolver.
func (c *Config) Project(key string) ingester.Project {
	if p, ok := c.Projects[key]; ok {
		return p
	}
	return nil
}

type project struct {
	Rename  string     `yaml:"rename"`
	Filters []*pfilter `yaml:"filters"`
}

// Filter implements ingester.Project.
func (p *project) Filter(e *envelope.Envelope) bool {
	for _, f := range p.Filters {
		if !f.filter.Filter(e) {
			return false
		}
	}
	return true
}

// Name implements ingester.Project.
func (p *project) Name() string {
	return p.Rename
}

type filter struct {
	impl envelope.Filter
}

// Filter implements envelope.Filter.
func (f *filter) Filter(e *envelope.Envelope) bool {
	return f.impl.Filter(e)
}

type pfilter struct {
	*filter
	lazyRef string
}

func (c *Config) configure(src io.Reader) error {
	c.LogFormat = LogFormatJSON
	c.LogLevel = slog.LevelInfo
	c.ServerAddr = ":8080"
	c.HealthPath = "/healtz"
	c.Batcher = batcher.DefaultConfig
	return c.unmarshal(src)
}
