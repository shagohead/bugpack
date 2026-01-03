package decoder

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/go-faster/errors"
	"github.com/go-faster/jx"
)

type Envelope struct {
	Meta
	Header
	Event
}

func (e *Envelope) Decode(d *jx.Decoder) (err error) {
	if err = e.Meta.Decode(d); err != nil {
		return errors.Wrap(err, "meta object")
	}
	if err = e.Header.Decode(d); err != nil {
		return errors.Wrap(err, "header object")
	}
	if err = e.Event.Decode(d); err != nil {
		return errors.Wrap(err, "event object")
	}
	return nil
}

type Meta struct {
	EventID string
	SentAt  time.Time
}

func (m *Meta) Decode(d *jx.Decoder) error {
	return d.ObjBytes(func(d *jx.Decoder, key []byte) (err error) {
		switch string(key) {
		case "event_id":
			m.EventID, err = d.Str()
		case "sent_at":
			var s string
			s, err = d.Str()
			if err != nil {
				break
			}
			m.SentAt, err = time.Parse(time.RFC3339Nano, s)
		default:
			err = d.Skip()
		}
		if err != nil {
			err = errors.Wrap(err, string(key))
		}
		return err
	})
}

type Header struct {
	Type        string
	ContentType string
	Length      int
}

func (h *Header) Decode(d *jx.Decoder) error {
	return d.ObjBytes(func(d *jx.Decoder, key []byte) (err error) {
		switch string(key) {
		case "type":
			h.Type, err = d.Str()
		case "content_type":
			h.ContentType, err = d.Str()
		case "length":
			h.Length, err = d.Int()
		default:
			err = d.Skip()
		}
		if err != nil {
			err = errors.Wrap(err, string(key))
		}
		return err
	})
}

// TODO: использовать unique для SDK, Platform, и прочих низко-кардинальных значений.

type Event struct {
	SDK         SDK
	Platform    string
	ServerName  string
	Environment string
	Release     string
	Level       string
	Contexts    map[string]any
	Extra       map[string]any
	User        map[string]any
	Tags        map[string]string
	EventID     string
	Message     string
	Exception   Array[Exception, *Exception]
	Request     *Request
	Timestamp   time.Time
	// Breadcrumbs Array[Breadcrumb, *Breadcrumb]
}

func (e *Event) Decode(d *jx.Decoder) error {
	return d.ObjBytes(func(d *jx.Decoder, key []byte) (err error) {
		switch string(key) {
		case "sdk":
			err = e.SDK.Decode(d)
		case "platform":
			e.Platform, err = d.Str()
		case "server_name":
			e.ServerName, err = d.Str()
		case "environment":
			e.Environment, err = d.Str()
		case "release":
			e.Release, err = d.Str()
		case "level":
			e.Level, err = d.Str()
		case "event_id":
			e.EventID, err = d.Str()
		case "message":
			e.Message, err = d.Str()
		case "contexts":
			err = decodeMap(d, &e.Contexts, "")
		case "extra":
			err = e.decodeExtra(d)
		case "user":
			err = decodeMap(d, &e.User, "")
		case "tags":
			err = decodeStrMap(d, &e.Tags)
		case "exception":
			err = e.decodeException(d)
		case "timestamp":
			err = e.decodeTimestamp(d)
		case "request":
			err = e.decodeRequest(d)
		default:
			err = d.Skip()
		}
		if err != nil {
			err = errors.Wrap(err, string(key))
		}
		return err
	})
}

func (e *Event) decodeTimestamp(d *jx.Decoder) error {
	switch d.Next() {
	case jx.Number:
		raw, err := d.Raw()
		if err != nil {
			return err
		}
		dot := bytes.IndexRune(raw, '.')
		if dot < 1 {
			n, err := d.Int64()
			if err != nil {
				return err
			}
			e.Timestamp = time.Unix(n, 0)
			return nil
		}
		s, err := strconv.ParseInt(string(raw[0:dot]), 10, 64)
		if err != nil {
			return err
		}
		n, err := strconv.ParseInt(string(raw[dot+1:]), 10, 64)
		if err != nil {
			return err
		}
		if l := len(raw[dot+1:]); l < 4 {
			n *= 1000_000
		} else if l < 7 {
			n *= 1000
		}
		e.Timestamp = time.Unix(s, n)
		return nil
	case jx.String:
		var s string
		s, err := d.Str()
		if err != nil {
			return err
		}
		e.Timestamp, err = time.Parse(time.RFC3339Nano, s)
		return err
	default:
		return fmt.Errorf("unexpected `timestamp` type: %s", d.Next().String())
	}
}

func (e *Event) decodeExtra(d *jx.Decoder) error {
	e.Extra = make(map[string]any)
	return d.Obj(func(d *jx.Decoder, key string) error {
		r, err := d.Raw()
		if err != nil {
			return errors.Wrap(err, key)
		}
		// NOTE: Возможно надо скалярные типы приводить к соответствующим в Go.
		// А возможно надо приводить значение к строке.
		// Решение нужно будет принять во время реализации хранения этих данных в СУБД.
		// e.Extra[key] = r.String()
		e.Extra[key] = []byte(r)
		return nil
	})
}

func (e *Event) decodeException(d *jx.Decoder) error {
	switch d.Next() {
	case jx.Array:
		return e.Exception.Decode(d)
	case jx.Object:
		return d.ObjBytes(func(d *jx.Decoder, key []byte) (err error) {
			switch string(key) {
			case "values":
				err = e.Exception.Decode(d)
			default:
				err = d.Skip()
			}
			if err != nil {
				err = errors.Wrap(err, string(key))
			}
			return err
		})
	default:
		return fmt.Errorf("unexpected `exception` type: %s", d.Next().String())
	}
}

type SDK struct {
	Name    string
	Version string
}

func (s *SDK) Decode(d *jx.Decoder) error {
	return d.ObjBytes(func(d *jx.Decoder, key []byte) (err error) {
		switch string(key) {
		case "name":
			s.Name, err = d.Str()
		case "version":
			s.Version, err = d.Str()
		default:
			err = d.Skip()
		}
		if err != nil {
			err = errors.Wrap(err, string(key))
		}
		return err
	})
}

type Request struct {
	URL         string
	Method      string
	Data        string
	QueryString string
	Cookies     string
	Headers     map[string]string
	Environ     map[string]string
}

func (e *Event) decodeRequest(d *jx.Decoder) error {
	var r *Request
	return d.ObjBytes(func(d *jx.Decoder, key []byte) (err error) {
		if r == nil {
			e.Request = new(Request)
			r = e.Request
		}
		switch string(key) {
		case "url":
			r.URL, err = d.Str()
		case "method":
			r.Method, err = d.Str()
		case "data":
			r.Data, err = d.Str()
		case "query_string":
			r.QueryString, err = d.Str()
		case "cookies":
			r.Cookies, err = d.Str()
		case "headers":
			err = decodeStrMap(d, &r.Headers)
		case "env":
			err = decodeStrMap(d, &r.Environ)
		default:
			err = d.Skip()
		}
		if err != nil {
			err = errors.Wrap(err, string(key))
		}
		return err
	})
}

type Decoder interface {
	Decode(*jx.Decoder) error
}

type Array[T any, PT interface {
	*T
	Decoder
}] []T

func (a *Array[T, PT]) Decode(d *jx.Decoder) error {
	return d.Arr(func(d *jx.Decoder) error {
		var i T
		var pt PT = &i
		if err := pt.Decode(d); err != nil {
			return errors.Wrapf(err, "[%d]", len(*a))
		}
		*a = append(*a, i)
		return nil
	})
}

type Level string

const (
	LevelDebug   Level = "debug"
	LevelInfo    Level = "info"
	LevelWarning Level = "warning"
	LevelError   Level = "error"
	LevelFatal   Level = "fatal"
)

type Breadcrumb struct {
	Type      string
	Category  string
	Message   string
	Data      map[string]any
	Level     Level
	Timestamp time.Time
}

type Exception struct {
	Module string
	Type   string
	Value  string
	Frames Array[Frame, *Frame]
}

func (e *Exception) Decode(d *jx.Decoder) error {
	return d.ObjBytes(func(d *jx.Decoder, key []byte) (err error) {
		switch string(key) {
		case "module":
			if d.Next() == jx.Null {
				err = d.Skip()
				break
			}
			e.Module, err = d.Str()
		case "type":
			e.Type, err = d.Str()
		case "value":
			e.Value, err = d.Str()
		case "stacktrace":
			err = e.decodeStacktrace(d)
		default:
			err = d.Skip()
		}
		if err != nil {
			err = errors.Wrap(err, string(key))
		}
		return err
	})
}

func (e *Exception) decodeStacktrace(d *jx.Decoder) error {
	return d.ObjBytes(func(d *jx.Decoder, key []byte) (err error) {
		switch string(key) {
		case "frames":
			err = e.Frames.Decode(d)
		default:
			err = d.Skip()
		}
		if err != nil {
			err = errors.Wrap(err, string(key))
		}
		return err
	})
}

type Frame struct {
	Filename string
	AbsPath  string
	Module   string
	Function string
	LineNum  int
	CtxLine  string
	PreCtx   []string
	PostCtx  []string
	Vars     map[string]any
	InApp    bool
}

func (f *Frame) Decode(d *jx.Decoder) error {
	return d.ObjBytes(func(d *jx.Decoder, key []byte) (err error) {
		switch string(key) {
		case "filename":
			f.Filename, err = d.Str()
		case "abs_path":
			f.AbsPath, err = d.Str()
		case "module":
			f.Module, err = d.Str()
		case "function":
			f.Function, err = d.Str()
		case "lineno":
			f.LineNum, err = d.Int()
		case "context_line":
			f.CtxLine, err = d.Str()
		case "pre_context":
			err = f.decodeContextLines(d, &f.PreCtx)
		case "post_context":
			err = f.decodeContextLines(d, &f.PostCtx)
		case "in_app":
			f.InApp, err = d.Bool()
		case "vars":
			err = decodeMap(d, &f.Vars, "")
		default:
			err = d.Skip()
		}
		if err != nil {
			err = errors.Wrap(err, string(key))
		}
		return err
	})
}

func (*Frame) decodeContextLines(d *jx.Decoder, dst *[]string) error {
	return d.Arr(func(d *jx.Decoder) error {
		s, err := d.Str()
		if err != nil {
			return err
		}
		*dst = append(*dst, s)
		return nil
	})
}

func decodeMap(d *jx.Decoder, dst *map[string]any, prefix string) error {
	return d.ObjBytes(func(d *jx.Decoder, key []byte) (err error) {
		if *dst == nil {
			*dst = make(map[string]any)
		}
		var v any
		switch d.Next() {
		case jx.Null:
			err = d.Skip()
		case jx.Bool:
			v, err = d.Bool()
		case jx.Array:
			v, err = d.Raw()
		case jx.Number:
			var f float64
			f, err = d.Float64()
			if err == nil {
				if float64(int64(f)) == f {
					v = int(f)
				} else {
					v = f
				}
			}
		case jx.Object:
			err = decodeMap(d, dst, fmt.Sprintf("%s%s.", prefix, key))
			if err == nil {
				return nil
			}
		case jx.String:
			v, err = d.Str()
		default:
			err = fmt.Errorf("unexpected jx.Type %s", d.Next().String())
		}
		if err != nil {
			return errors.Wrap(err, string(key))
		}
		(*dst)[fmt.Sprintf("%s%s", prefix, key)] = v
		return nil
	})
}

func decodeStrMap(d *jx.Decoder, dst *map[string]string) error {
	return d.ObjBytes(func(d *jx.Decoder, key []byte) error {
		if *dst == nil {
			*dst = make(map[string]string)
		}
		s, err := d.Str()
		if err != nil {
			return errors.Wrap(err, string(key))
		}
		(*dst)[string(key)] = s
		return err
	})
}
