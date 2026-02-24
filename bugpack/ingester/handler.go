package ingester

import (
	"compress/gzip"
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

func New(projects ProjectResolver, batcher batcher.Batcher) http.Handler {
	return &handler{projects: projects, batcher: batcher}
}

type ProjectResolver interface {
	Project(key string) Project
}

type Project interface {
	envelope.Filter
	Name() string
}

type handler struct {
	projects ProjectResolver
	batcher  batcher.Batcher
}

// ServeHTTP implements http.Handler.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
