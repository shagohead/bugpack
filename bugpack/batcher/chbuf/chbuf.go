package chbuf

import (
	"context"
	"encoding/json"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/proto"

	"github.com/shagohead/bugpack/bugpack/batcher"
	"github.com/shagohead/bugpack/bugpack/envelope"
)

func Factory(conn Conn) func() batcher.Buffer {
	return func() batcher.Buffer {
		return &buffer{con: conn, val: makeValues()}
	}
}

type Conn interface {
	Do(context.Context, ch.Query) error
}

type buffer struct {
	con Conn
	val *values
}

const limit = 1000

// Append implements batcher.Buffer.
func (b *buffer) Append(e *envelope.Envelope) bool {
	b.val.append(e)
	return b.val.timestamp.Rows() >= limit
}

// Empty implements batcher.Buffer.
func (b *buffer) Empty() bool {
	return b.val.timestamp.Rows() == 0
}

const query = `INSERT INTO issue_event VALUES`

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
		level:        new(proto.ColStr).LowCardinality(),
		message:      new(proto.ColStr),
		excModule:    new(proto.ColStr),
		excType:      new(proto.ColStr),
		excValue:     new(proto.ColStr),
		excFrames:    proto.NewArray(newColFrame()),
		parents:      proto.NewArray(newColParent()),
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
		{Name: "Level", Data: v.level},
		{Name: "Message", Data: v.message},
		{Name: "Exception", Data: proto.ColTuple{
			proto.Named(v.excModule, "Module"),
			proto.Named(v.excType, "Type"),
			proto.Named(v.excValue, "Value"),
			proto.Named(v.excFrames, "Frames"),
		}},
		{Name: "Parents", Data: v.parents},
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
	level        *proto.ColLowCardinality[string]
	message      *proto.ColStr
	excModule    *proto.ColStr
	excType      *proto.ColStr
	excValue     *proto.ColStr
	excFrames    *proto.ColArr[envelope.Frame]
	parents      *proto.ColArr[parent]
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

func (v *values) append(e *envelope.Envelope) {
	if e.Message != "" {
		v.appendMessage(e)
	}
	for _, exc := range e.Exception {
		v.appendException(e, &exc)
	}
}

func (v *values) appendBase(e *envelope.Envelope) {
	v.project.Append(e.Project)
	v.level.Append(e.Level)
	v.clientip.Append(e.ClientIP)
	v.sdkName.Append(e.SDK.Name)
	v.sdkVersion.Append(e.SDK.Version)
	v.platform.Append(e.Platform)
	v.serverName.Append(e.ServerName)
	v.environment.Append(e.Environment)
	v.release.Append(e.Release)
	v.tags.Append(e.Tags)
	v.trace.Append(e.TraceID)
	v.span.Append(e.SpanID)
	v.timestamp.Append(e.Timestamp)
	v.appendUser(e)
	v.appendContext(e)
	v.appendRequest(e)
}

func (v *values) appendRequest(e *envelope.Envelope) {
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

func (v *values) appendUser(e *envelope.Envelope) {
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

func (v *values) appendContext(e *envelope.Envelope) {
	data, _ := json.Marshal(e.Contexts)
	if data == nil {
		data = emptyBytesObj
	}
	v.context.Append(data)
}

var emptyBytesObj = []byte(`{}`)

const emptyStr = ""

func (v *values) appendMessage(e *envelope.Envelope) {
	v.appendBase(e)
	v.message.Append(e.Message)
	v.excModule.Append(emptyStr)
	v.excType.Append(emptyStr)
	v.excValue.Append(emptyStr)
	v.excFrames.Append(nil)
	v.parents.Append(nil)
}

func (v *values) appendException(e *envelope.Envelope, exc *envelope.Exception) {
	v.appendBase(e)
	v.message.Append(emptyStr)
	v.excModule.Append(exc.Module)
	v.excType.Append(exc.Type)
	v.excValue.Append(exc.Value)
	v.excFrames.Append(exc.Frames)
	var parents []parent
	if exc.Mechanism.Parent >= 1 {
		// TODO: Make parents.
		// parents = append(parents, parent{})
	}
	v.parents.Append(parents)
}
