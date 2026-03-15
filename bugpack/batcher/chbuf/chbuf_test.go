package chbuf

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/proto"

	"github.com/shagohead/bugpack/bugpack/envelope"
	"github.com/shagohead/bugpack/chtest/chmigrated"
)

func TestFlush(t *testing.T) {
	conn := chmigrated.Database(t, "chbuf_flush")
	bufferer := Bufferer(conn)
	buf := bufferer.Buffer()

	event := func(t *testing.T) *envelope.Envelope {
		return &envelope.Envelope{
			Project: t.Name(),
			Event: envelope.Event{
				SDK:         envelope.SDK{Name: "chbuf_test.go", Version: "0.1.0"},
				Platform:    "go",
				ServerName:  "computer.local",
				Environment: "production",
				Release:     "v0.1.0",
				Level:       "errorLevel",
				Contexts:    map[string]any{"browser": "chromium", "retries": 1},
				Extra:       map[string]any{"drink": "coffee", "magic_value": 42},
				User:        map[string]any{"id": "125844", "email": "mail@domain.com", "data": map[string]any{"age": 37}},
				Tags:        map[string]string{"operation": "TestFlush"},
				EventID:     "df2d4860-0a08-4f32-bc7f-55a57df85c64",
				TraceID:     "2864edc1-d16d-4ffc-b4a7-471f308ac789",
				SpanID:      "25c259fa-3ec7-4e30-bac4-fe7f638d84ed",
				Timestamp:   time.Now(),
			},
		}
	}

	t.Run("message", func(t *testing.T) {
		event := event(t)
		event.Message = "testing message event"
		buf.Append(bufferer.Envelope(event))
		if err := buf.Flush(t.Context()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("request", func(t *testing.T) {
		event := event(t)
		event.Message = "writing request"
		event.Request = &envelope.Request{
			URL:         "https://github.com",
			Method:      http.MethodPost,
			Data:        "username=shagohead&repo=bugpack",
			QueryString: "username=shagohead",
			Cookies:     "usage=coffee",
			Headers:     map[string]string{"Referer": "https://github.com/shagohead"},
			Environ:     map[string]string{"host": "local"},
		}
		buf.Append(bufferer.Envelope(event))
		if err := buf.Flush(t.Context()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("exception/frames", func(t *testing.T) {
		event := event(t)
		event.Exception = append(event.Exception, envelope.Exception{
			Type:  "*main.CustomError",
			Value: "Exception Value",
			Frames: envelope.Array[envelope.Frame, *envelope.Frame]{
				{
					AbsPath:  "/file.go",
					Module:   "main",
					Function: "main",
					LineNum:  10,
					CtxLine:  "\tif err := run(); err != nil {",
					PreCtx:   []string{"", "\"quoted\"", "", "// comment", "\ttabbed"},
					PostCtx:  []string{"a", "b", "c", "d", "f"},
					InApp:    true,
				},
				{
					AbsPath:  "/file.go",
					Function: "run",
					Module:   "main",
					LineNum:  20,
					CtxLine:  "\tfunctionCall()",
					PreCtx:   []string{"a1", "b1", "c1", "d1", "f1"},
					PostCtx:  []string{"a2", "b2", "c2", "d2", "f2"},
					InApp:    true,
				},
				{
					AbsPath:  "/file.go",
					Function: "captureExceptionMany",
					Module:   "main",
					LineNum:  30,
					CtxLine:  "\tsentry.CaptureException(\u0026CustomError{Prop: 42})",
					PreCtx:   []string{"a3", "b3", "c3", "d3", "f3"},
					PostCtx:  []string{"a4", "b4", "c4", "d4", "f4"},
					InApp:    true,
					Vars:     map[string]any{"error": "captureExceptionMany()"},
				},
			},
		})
		buf.Append(bufferer.Envelope(event))
		if err := buf.Flush(t.Context()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("exception/parents", func(t *testing.T) {
		event := event(t)
		event.Exception = append(event.Exception, envelope.Exception{
			Type: "*main.ParentError", Value: "Top Error",
			Mechanism: envelope.Mechanism{
				ID: 1, Group: true,
			},
		})
		event.Exception = append(event.Exception, envelope.Exception{
			Type: "*main.ChildError", Value: "Middle Error",
			Mechanism: envelope.Mechanism{
				ID: 2, Parent: 1, Group: true,
			},
		})
		event.Exception = append(event.Exception, envelope.Exception{
			Type: "*main.ChildError", Value: "Floor Error",
			Mechanism: envelope.Mechanism{
				ID: 3, Parent: 2, Group: true,
			},
		})
		buf.Append(bufferer.Envelope(event))
		if err := buf.Flush(t.Context()); err != nil {
			t.Fatal(err)
		}
	})

	t.Cleanup(func() {
		t.Log("Reading writed data")
		row := new(proto.ColStr)
		if err := conn.Do(context.Background(), ch.Query{
			Body:   "SELECT toJSONString(tuple(*)) row FROM issue_event",
			Result: proto.Results{{Name: "row", Data: row}},
			OnResult: func(ctx context.Context, block proto.Block) error {
				for i := range block.Rows {
					t.Logf("row %d from block: %s", i, row.Row(i))
				}
				return nil
			},
		}); err != nil {
			t.Fatal(err)
		}
	})
}
