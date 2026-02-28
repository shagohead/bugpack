package envelope

import "time"

type Envelope struct {
	Meta
	Header
	Event
	Project  string
	ClientIP string
}

type Meta struct {
	EventID string
	SentAt  time.Time
}

type Header struct {
	Type        string
	ContentType string
	Length      int
}

// TODO: Prepare Envelope by buffer and for buffer.

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
	TraceID     string
	SpanID      string
	Message     string
	Exception   Array[Exception, *Exception]
	Request     *Request
	Timestamp   time.Time
	// TODO: Add breadcrumbs decoding.
	// Breadcrumbs Array[Breadcrumb, *Breadcrumb]
}

type SDK struct {
	Name    string
	Version string
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
	Module    string
	Type      string
	Value     string
	Mechanism Mechanism
	Frames    Array[Frame, *Frame]
}

type Mechanism struct {
	ID     int
	Parent int
	Group  bool
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

type Filter interface {
	// Filter envelope. Returns true for saving, false if not.
	Filter(e *Envelope) bool
}
