package chbuf

import (
	"context"
	"encoding/json"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/proto"

	"github.com/shagohead/bugpack/bugpack/batcher"
	"github.com/shagohead/bugpack/bugpack/envelope"
)

func Bufferer(conn Conn) batcher.Bufferer[*Envelope] {
	return &bufferer{conn: conn}
}

type bufferer struct {
	conn Conn
}

// Buffer implements batcher.Bufferer.
func (b *bufferer) Buffer() batcher.Buffer[*Envelope] {
	return &buffer{con: b.conn, val: makeValues()}
}

// Envelope implements batcher.Bufferer.
func (b *bufferer) Envelope(e *envelope.Envelope) *Envelope {
	d := &Envelope{Envelope: e}
	d.calcHash()
	d.ctxjson, _ = json.Marshal(e.Contexts)
	if d.ctxjson == nil {
		d.ctxjson = emptyBytesObj
	}
	return d
}

var _ batcher.Bufferer[*Envelope] = (*bufferer)(nil)

type Envelope struct {
	*envelope.Envelope
	msgHash uint64
	excHash []uint64
	ctxjson []byte
}

type Conn interface {
	Do(context.Context, ch.Query) error
}

type buffer struct {
	con Conn
	val *values
}

// Append implements batcher.Buffer.
func (b *buffer) Append(e *Envelope) bool {
	b.val.append(e)
	return b.val.timestamp.Rows() >= limit
}

const limit = 1000

// Empty implements batcher.Buffer.
func (b *buffer) Empty() bool {
	return b.val.timestamp.Rows() == 0
}

const query = `INSERT INTO issue_event (
	Project,
	IssueHash,
	Level,
	Message,
	Exception,
	ClientIP,
	SDK,
	Platform,
	ServerName,
	Environment,
	Release,
	User,
	UserData,
	Context,
	Tags,
	Request,
	TraceID,
	SpanID,
	Timestamp
) VALUES`

// Flush implements batcher.Buffer.
func (b *buffer) Flush(ctx context.Context) error {
	if b.val.timestamp.Rows() == 0 {
		return nil
	}
	if err := b.con.Do(ctx, ch.Query{Body: query, Input: b.val.input}); err != nil {
		return err
	}
	b.val.input.Reset()
	return nil
}

func makeValues() *values {
	v := &values{
		project:      new(proto.ColStr).LowCardinality(),
		hash:         new(proto.ColUInt64),
		level:        new(proto.ColStr).LowCardinality(),
		message:      new(proto.ColStr),
		excParent:    new(proto.ColUInt64),
		excModule:    new(proto.ColStr),
		excType:      new(proto.ColStr),
		excValue:     new(proto.ColStr),
		excFrames:    proto.NewArray(newColFrame()),
		clientip:     new(proto.ColStr),
		sdkName:      new(proto.ColStr).LowCardinality(),
		sdkVersion:   new(proto.ColStr).LowCardinality(),
		platform:     new(proto.ColStr).LowCardinality(),
		serverName:   new(proto.ColStr).LowCardinality(),
		environment:  new(proto.ColStr).LowCardinality(),
		release:      new(proto.ColStr).LowCardinality(),
		userID:       new(proto.ColStr),
		userIP:       new(proto.ColStr),
		userEmail:    new(proto.ColStr),
		userUsername: new(proto.ColStr),
		userName:     new(proto.ColStr),
		userData:     new(proto.ColJSONBytes),
		context:      new(proto.ColJSONBytes),
		tags:         proto.NewMap(new(proto.ColStr), new(proto.ColStr)),
		reqURL:       new(proto.ColStr),
		reqMethod:    new(proto.ColStr).LowCardinality(),
		reqData:      new(proto.ColStr),
		reqQuery:     new(proto.ColStr),
		reqCookies:   new(proto.ColStr),
		reqHeaders:   proto.NewMap(new(proto.ColStr), new(proto.ColStr)),
		reqEnviron:   proto.NewMap(new(proto.ColStr), new(proto.ColStr)),
		trace:        new(proto.ColStr),
		span:         new(proto.ColStr),
		timestamp:    new(proto.ColDateTime64).WithPrecision(proto.PrecisionMicro),
	}
	v.input = proto.Input{
		{Name: "Project", Data: v.project},
		{Name: "IssueHash", Data: v.hash},
		{Name: "Level", Data: v.level},
		{Name: "Message", Data: v.message},
		{Name: "Exception", Data: proto.ColTuple{
			proto.Named(v.excParent, "ParentHash"),
			proto.Named(v.excModule, "Module"),
			proto.Named(v.excType, "Type"),
			proto.Named(v.excValue, "Value"),
			proto.Named(v.excFrames, "Frames"),
		}},
		{Name: "ClientIP", Data: v.clientip},
		{Name: "SDK", Data: proto.ColTuple{
			proto.Named(v.sdkName, "Name"),
			proto.Named(v.sdkVersion, "Version"),
		}},
		{Name: "Platform", Data: v.platform},
		{Name: "ServerName", Data: v.serverName},
		{Name: "Environment", Data: v.environment},
		{Name: "Release", Data: v.release},
		{Name: "User", Data: proto.ColTuple{
			proto.Named(v.userID, "ID"),
			proto.Named(v.userIP, "IP"),
			proto.Named(v.userEmail, "Email"),
			proto.Named(v.userUsername, "Username"),
			proto.Named(v.userName, "Name"),
		}},
		{Name: "UserData", Data: v.userData},
		{Name: "Context", Data: v.context},
		{Name: "Tags", Data: v.tags},
		{Name: "Request", Data: proto.ColTuple{
			proto.Named(v.reqURL, "URL"),
			proto.Named(v.reqMethod, "Method"),
			proto.Named(v.reqData, "Data"),
			proto.Named(v.reqQuery, "QueryString"),
			proto.Named(v.reqCookies, "Cookies"),
			proto.ColNamed[map[string]string]{ColumnOf: v.reqHeaders, Name: "Headers"},
			proto.ColNamed[map[string]string]{ColumnOf: v.reqEnviron, Name: "Environ"},
		}},
		{Name: "TraceID", Data: v.trace},
		{Name: "SpanID", Data: v.span},
		{Name: "Timestamp", Data: v.timestamp},
	}
	return v
}

type values struct {
	input        proto.Input
	project      *proto.ColLowCardinality[string]
	hash         *proto.ColUInt64
	level        *proto.ColLowCardinality[string]
	message      *proto.ColStr
	excParent    *proto.ColUInt64
	excModule    *proto.ColStr
	excType      *proto.ColStr
	excValue     *proto.ColStr
	excFrames    *proto.ColArr[envelope.Frame]
	clientip     *proto.ColStr
	sdkName      *proto.ColLowCardinality[string]
	sdkVersion   *proto.ColLowCardinality[string]
	platform     *proto.ColLowCardinality[string]
	serverName   *proto.ColLowCardinality[string]
	environment  *proto.ColLowCardinality[string]
	release      *proto.ColLowCardinality[string]
	userID       *proto.ColStr
	userIP       *proto.ColStr
	userEmail    *proto.ColStr
	userUsername *proto.ColStr
	userName     *proto.ColStr
	userData     *proto.ColJSONBytes
	context      *proto.ColJSONBytes
	tags         *proto.ColMap[string, string]
	reqURL       *proto.ColStr
	reqMethod    *proto.ColLowCardinality[string]
	reqData      *proto.ColStr
	reqQuery     *proto.ColStr
	reqCookies   *proto.ColStr
	reqHeaders   *proto.ColMap[string, string]
	reqEnviron   *proto.ColMap[string, string]
	trace        *proto.ColStr
	span         *proto.ColStr
	timestamp    *proto.ColDateTime64
}

func (v *values) append(e *Envelope) {
	if e.Message != "" {
		v.appendMessage(e)
	}
	for i := range e.Exception {
		v.appendException(e, i)
	}
}

func (v *values) appendBase(e *Envelope) {
	v.project.Append(e.Project)
	v.level.Append(e.Level)
	v.clientip.Append(e.ClientIP)
	v.sdkName.Append(e.SDK.Name)
	v.sdkVersion.Append(e.SDK.Version)
	v.platform.Append(e.Platform)
	v.serverName.Append(e.ServerName)
	v.environment.Append(e.Environment)
	v.release.Append(e.Release)
	v.context.Append(e.ctxjson)
	v.tags.Append(e.Tags)
	v.trace.Append(e.TraceID)
	v.span.Append(e.SpanID)
	v.timestamp.Append(e.Timestamp)
	v.appendUser(e)
	v.appendRequest(e)
}

func (v *values) appendRequest(e *Envelope) {
	if e.Request == nil {
		v.reqURL.Append(emptyStr)
		v.reqMethod.Append(emptyStr)
		v.reqData.Append(emptyStr)
		v.reqQuery.Append(emptyStr)
		v.reqCookies.Append(emptyStr)
		v.reqHeaders.Append(nil)
		v.reqEnviron.Append(nil)
		return
	}
	v.reqURL.Append(e.Request.URL)
	v.reqMethod.Append(e.Request.Method)
	v.reqData.Append(e.Request.Data)
	v.reqQuery.Append(e.Request.QueryString)
	v.reqCookies.Append(e.Request.Cookies)
	v.reqHeaders.Append(e.Request.Headers)
	v.reqEnviron.Append(e.Request.Environ)
}

func (v *values) appendUser(e *Envelope) {
	for key, dst := range map[string]*proto.ColStr{
		"id":         v.userID,
		"ip_address": v.userIP,
		"email":      v.userEmail,
		"username":   v.userUsername,
		"name":       v.userName,
	} {
		var s string
		if a, ok := e.User[key]; ok {
			s, _ = a.(string)
		}
		dst.Append(s)
	}
	var data []byte
	if a, ok := e.User["data"]; ok {
		data, _ = json.Marshal(a)
	}
	if data == nil {
		data = emptyBytesObj
	}
	v.userData.Append(data)
}

var emptyBytesObj = []byte(`{}`)

const emptyStr = ""

func (v *values) appendMessage(e *Envelope) {
	v.appendBase(e)
	v.hash.Append(e.msgHash)
	v.message.Append(e.Message)
	v.excModule.Append(emptyStr)
	v.excType.Append(emptyStr)
	v.excValue.Append(emptyStr)
	v.excFrames.Append(nil)
	v.excParent.Append(0)
}

func (v *values) appendException(e *Envelope, i int) {
	v.appendBase(e)
	v.hash.Append(e.excHash[i])
	v.message.Append(emptyStr)
	x := e.Exception[i]
	v.excModule.Append(x.Module)
	v.excType.Append(x.Type)
	v.excValue.Append(x.Value)
	v.excFrames.Append(x.Frames)
	v.excParent.Append(e.parentHash(&x))
}
