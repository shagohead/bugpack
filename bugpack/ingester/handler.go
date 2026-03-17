package ingester

import (
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-faster/jx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/shagohead/bugpack/bugpack/batcher"
	"github.com/shagohead/bugpack/bugpack/envelope"
)

func New[E any](projects ProjectResolver, batcher batcher.Batcher[E]) http.Handler {
	return &handler[E]{projects: projects, batcher: batcher}
}

type ProjectResolver interface {
	Project(key string) Project
}

type Project interface {
	envelope.Filter
	Name() string
}

type handler[E any] struct {
	projects ProjectResolver
	batcher  batcher.Batcher[E]
}

// ServeHTTP implements http.Handler.
func (h *handler[E]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid method", http.StatusMethodNotAllowed)
		return
	}
	auth := r.Header.Get(sentryAuthHeader)
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(attribute.String("envelope.sentry_auth", auth))
	pkey := projectKey(auth)
	if pkey == "" {
		http.Error(w, "missing sentry auth header", http.StatusBadRequest)
		return
	}
	span.SetAttributes(attribute.String("envelope.project_key", pkey))
	project := h.projects.Project(pkey)
	if project == nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	var body io.Reader
	cenc := r.Header.Get("Content-Encoding")
	span.SetAttributes(attribute.String("envelope.encoding", cenc))
	switch cenc {
	case "gzip":
		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer zr.Close()
		body = zr
	case "":
		body = r.Body
	default:
		http.Error(w, "unsupported encoding", http.StatusBadRequest)
		return
	}

	event := new(envelope.Envelope)
	dec := jx.GetDecoder()
	defer jx.PutDecoder(dec)
	dec.Reset(body)
	if err := event.Decode(dec); err != nil {
		if errors.Is(err, envelope.ErrNonEventType) {
			span.SetAttributes(attribute.String("envelope.type", event.Type))
			return
		}
		span.SetStatus(codes.Error, "decoding error")
		span.RecordError(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	span.SetAttributes(
		attribute.String("envelope.level", event.Level),
		attribute.String("envelope.release", event.Release),
		attribute.String("envelope.environment", event.Environment),
	)
	span.SetStatus(codes.Ok, "")
	event.Project = project.Name()
	event.ClientIP = clientIP(r)
	if !project.Filter(event) {
		span.AddEvent("filtered out")
		return
	}
	h.batcher.Batch(event)
}

const (
	sentryAuthHeader = "X-Sentry-Auth"
	sentryAuthPrefix = "Sentry "
	sentryAuthParam  = "sentry_key="
	sentryAuthNone   = ""
)

func clientIP(r *http.Request) string {
	v := r.Header.Get("X-Forwarded-For")
	if v == "" {
		return r.RemoteAddr
	}
	i := strings.LastIndexByte(v, ',')
	if i == -1 {
		return r.RemoteAddr
	}
	ip := strings.TrimSpace(v[i+1:])
	if len(ip) < 7 {
		return r.RemoteAddr
	}
	return ip
}

// Decode projectkey from sentry auth header.
func projectKey(s string) string {
	if !strings.HasPrefix(s, sentryAuthPrefix) {
		return sentryAuthNone
	}
	i := strings.Index(s, sentryAuthParam)
	if i < len(sentryAuthPrefix) {
		return sentryAuthNone
	}
	least := s[i+len(sentryAuthParam):]
	comma := strings.IndexRune(least, ',')
	if comma < 0 {
		return least
	}
	return least[:comma]
}
