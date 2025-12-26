package decoder

import (
	"fmt"
	"time"

	"github.com/go-faster/errors"
	"github.com/go-faster/jx"
)

type Envelope struct {
	Meta
	Event
}

func (e *Envelope) Decode(d *jx.Decoder) error {
	// decode meta
	if err := d.ObjBytes(); err != nil {
		return errors.Wrap(err, "meta")
	}
	if err := d.ObjBytes(); err != nil {
		return errors.Wrap(err, "event header")
	}
	if err := d.ObjBytes(); err != nil {
		return errors.Wrap(err, "event")
	}
}

type Meta struct {
	EventID string
	SentAt  time.Time
	SDK     struct {
		Name    string
		Version string
		Trace   struct {
			Environment string
			TraceID     string
		}
	}
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
	Timestamp   time.Time
}

type SDK struct {
	Name    string
	Version string
}

type Exception struct {
	Module string
	Type   string
	Value  string
	Frames Array[Frame, *Frame]
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
			err = decodeMap(d, &e.Extra, "")
		case "user":
			err = decodeMap(d, &e.User, "")
		case "tags":
			err = decodeStrMap(d, &e.Tags)
		case "exception":
			err = e.Exception.Decode(d)
		case "timestamp":
			var s string
			s, err = d.Str()
			if err != nil {
				break
			}
			e.Timestamp, err = time.Parse(time.RFC3339Nano, s)
		default:
			err = d.Skip()
		}
		if err != nil {
			err = errors.Wrap(err, string(key))
		}
		return err
	})
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

func (e *Exception) Decode(d *jx.Decoder) error {
	return d.ObjBytes(func(d *jx.Decoder, key []byte) (err error) {
		switch string(key) {
		case "module":
			e.Module, err = d.Str()
		case "type":
			e.Type, err = d.Str()
		case "value":
			e.Value, err = d.Str()
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
			err = decodeContextLines(d, &f.PreCtx)
		case "post_context":
			err = decodeContextLines(d, &f.PostCtx)
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

func decodeMap(d *jx.Decoder, dst *map[string]any, prefix string) error {
	return d.ObjBytes(func(d *jx.Decoder, key []byte) (err error) {
		if *dst == nil {
			*dst = make(map[string]any)
		}
		var v any
		switch d.Next() {
		// case jx.Array:
		// case jx.Bool:
		// case jx.Invalid:
		// case jx.Null:
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

func decodeContextLines(d *jx.Decoder, dst *[]string) error {
	return d.Arr(func(d *jx.Decoder) error {
		s, err := d.Str()
		if err != nil {
			return err
		}
		*dst = append(*dst, s)
		return nil
	})
}
