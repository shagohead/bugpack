package main

import (
	"fmt"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

func main() {
	if err := run(); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func run() error {
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         os.Getenv("DSN"),
		Debug:       true,
		Environment: "local",
	}); err != nil {
		return err
	}
	defer sentry.CurrentHub().Flush(time.Second)
	defer sentry.Recover()

	if len(os.Args) < 2 {
		return fmt.Errorf("missing command argument")
	}

	// Set TraceID & SpanID for consistent JSON output across test calls.
	sentry.CurrentHub().Scope().SetPropagationContext(sentry.PropagationContext{
		TraceID:                sentry.TraceID{},
		SpanID:                 sentry.SpanID{},
		ParentSpanID:           sentry.SpanID{},
		DynamicSamplingContext: sentry.DynamicSamplingContext{},
	})

	switch os.Args[1] {
	case "panic":
		raisePanic()
	case "captureMessage":
		captureMessage()
	case "captureException":
		captureException()
	case "captureExceptionMany":
		captureExceptionMany()
	case "captureExceptionNested":
		captureExceptionNested()
	case "captureExceptionScoped":
		captureExceptionScoped()
	case "captureExceptionWrapped":
		captureExceptionWrapped()
	default:
		return fmt.Errorf("unknown command argument %q", os.Args[1])
	}
	return nil
}

func raisePanic() {
	panic("manual panic from raisePanic")
}

func captureMessage() {
	sentry.CaptureMessage("manual text message")
}

func captureException() {
	sentry.CaptureException(&ManualError{Prop: 42})
}

func captureExceptionMany() {
	sentry.CaptureException(&ManualError{Prop: 3})
	sentry.CaptureException(&ManualError{Prop: 42})
}

func captureExceptionNested() {
	captureExceptionNestedChild()
}

func captureExceptionNestedChild() {
	func() {
		sentry.CaptureException(&ManualError{Prop: 42})
	}()
}

// TODO: Научиться аттачить скоуп/ивент к трейсу open-telemetry.

func captureExceptionScoped() {
	hub := sentry.CurrentHub().Clone()
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("tag_key", "tag_value")
		scope.SetContext("scope_ctx_key", map[string]any{
			"ctx_key_int":    1,
			"ctx_key_string": "string value",
			"ctx_key_float":  3.14,
		})
		scope.SetExtra("scope_extra_int", 42)
		scope.SetExtra("scope_extra_string", "extra string")
		scope.SetUser(sentry.User{
			ID:        "125844",
			Email:     "mail@domain.com",
			IPAddress: "127.0.0.1",
			Username:  "example",
			Name:      "Some User",
			Data:      map[string]string{"origin": "source code"},
		})
		scope.SetLevel(sentry.LevelWarning)
		hub.CaptureException(fmt.Errorf("scoped error"))
	})
}

func captureExceptionWrapped() {
	origin := fmt.Errorf("origin exception")
	middle := fmt.Errorf("middle exception: %w", origin)
	sentry.CaptureException(fmt.Errorf("inner exception: %w", middle))
}

type ManualError struct {
	Prop int
}

// Error implements error.
func (m *ManualError) Error() string {
	return fmt.Sprintf("ManualError Property: %d", m.Prop)
}
