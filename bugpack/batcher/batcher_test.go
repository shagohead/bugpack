package batcher

import (
	"context"
	"io"
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-faster/errors"

	"github.com/shagohead/bugpack/bugpack/envelope/decoder"
)

type action int // buffer tracking actions.

const (
	actionAppend action = iota
	actionFlush
	actionCancel
)

type flush struct {
	d time.Duration // Delay before flush returns.
	e error         // Returning error.
}

type buffer struct {
	len int
	cap int
	act chan action // Notifications of buffer activity.
	fsh chan flush  // Behaviour commands for Flash calls.
}

// Append implements Buffer.
func (b *buffer) Append(e *decoder.Envelope) bool {
	b.act <- actionAppend
	b.len++
	return b.len == b.cap
}

// Empty implements Buffer.
func (b *buffer) Empty() bool {
	return b.len == 0
}

// Flush implements Buffer.
func (b *buffer) Flush(ctx context.Context) error {
	b.act <- actionFlush
	b.len = 0
	var fsh flush
	select {
	case fsh = <-b.fsh:
	default:
	}
	if fsh.d > 0 {
		select {
		case <-time.After(fsh.d):
		case <-ctx.Done():
			b.act <- actionCancel
			if fsh.e == nil {
				fsh.e = ctx.Err()
			}
		}
	}
	return fsh.e
}

func factory(len, cap int, act chan action, fsh chan flush) func() Buffer {
	return func() Buffer {
		return &buffer{len: len, cap: cap, act: act, fsh: fsh}
	}
}

func TestBatcher(t *testing.T) {
	minuteRetry := ConfigBackoff{
		Enable:              true,
		InitialInterval:     time.Minute,
		RandomizationFactor: 0,
		Multiplier:          1,
		MaxInterval:         time.Minute,
	}
	for _, tt := range []struct {
		name string
		conf Config
		blen int           // Initial buffer length.
		bcap int           // Buffer capacity.
		flsh []flush       // Behaviour for Flush calls.
		send int           // Count of Batch calls.
		wait time.Duration // Wait duration after `send` Batch calls.
		shut bool          // Call Shutdown before test assertions.
		werr error         // Want Shutdown error.
		wapp int           // Want Append calls.
		wfsh int           // Want Flush calls.
		wcan int           // Want canceled Flush'es.
	}{
		{
			name: "shutdown without events",
			shut: true,
		},
		{
			name: "shutdown wait flush",
			bcap: 2, send: 1, shut: true, wapp: 1, wfsh: 1,
		},
		{
			name: "accumulate without flash",
			bcap: 2, send: 1, wapp: 1, wfsh: 0,
		},
		{
			name: "flush on full buffer",
			bcap: 2, send: 2, wapp: 2, wfsh: 1,
		},
		{
			name: "flush on timeout",
			bcap: 2, send: 1, wait: time.Second * 2, wapp: 1, wfsh: 1,
		},
		{
			name: "skip empty buffer",
			bcap: 1, send: 0, wait: time.Second * 2, wfsh: 0,
		},
		{
			name: "cancel flushing by first error",
			bcap: 1, send: 1, wapp: 1, wfsh: 1,
			flsh: []flush{{e: io.EOF}}, werr: io.EOF,
		},
		{
			name: "cancel flushing by concurrent error",
			conf: Config{FlushWorkers: 2},
			bcap: 1, send: 2, wapp: 2, wfsh: 2, wcan: 1,
			flsh: []flush{{d: time.Second}, {e: io.EOF}}, werr: io.EOF,
		},
		{
			name: "flush timed out",
			conf: Config{FlushTimeout: time.Second},
			bcap: 1, send: 1, wait: time.Second, wapp: 1, wfsh: 1, wcan: 1,
			flsh: []flush{{d: time.Minute}}, werr: context.DeadlineExceeded,
		},
		{
			name: "retry successfull",
			conf: Config{RetryBackoff: minuteRetry},
			bcap: 1, send: 1, wait: time.Second * 61, wapp: 1, wfsh: 2,
			flsh: []flush{{e: io.EOF}},
		},
		{
			name: "retrying delay timed out",
			conf: Config{FlushTimeout: time.Second, RetryBackoff: minuteRetry},
			bcap: 1, send: 1, wait: time.Second, wapp: 1, wfsh: 1,
			flsh: []flush{{e: io.EOF}}, werr: errors.Wrap(io.EOF, "out of time"),
		},
		{
			name: "retrying flush timed out",
			conf: Config{FlushTimeout: time.Second * 61, RetryBackoff: minuteRetry},
			bcap: 1, send: 1, wait: time.Second * 61, wapp: 1, wfsh: 2, wcan: 1,
			flsh: []flush{{e: io.EOF}, {d: time.Hour}}, werr: errors.Wrap(context.DeadlineExceeded, "out of time"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				a := make(chan action)
				var appends, flushes, cancels int
				go func() {
					for a := range a {
						switch a {
						case actionAppend:
							appends++
						case actionFlush:
							flushes++
						case actionCancel:
							cancels++
						}
					}
				}()
				if tt.conf.BatchTimeout == 0 {
					tt.conf.BatchTimeout = time.Second
				}
				if tt.conf.FlushWorkers == 0 {
					tt.conf.FlushWorkers = 1
				}
				var flsh chan flush
				if tt.flsh != nil {
					flsh = make(chan flush, len(tt.flsh))
					for _, f := range tt.flsh {
						flsh <- f
					}
				}
				b := New(factory(tt.blen, tt.bcap, a, flsh), tt.conf)
				var err error
				go func() {
					err = b.Serve()
				}()
				synctest.Wait()
				for range tt.send {
					b.Batch(&decoder.Envelope{})
				}
				if tt.wait > 0 {
					time.Sleep(tt.wait)
				}
				synctest.Wait()
				if tt.shut {
					b.Shutdown()
				}
				synctest.Wait()
				if g, w := strerrs(err, tt.werr); g != w {
					t.Errorf("Shutdown error: %s, want: %s", g, w)
				}
				if appends != tt.wapp {
					t.Errorf("Append calls: %v, want: %v", appends, tt.wapp)
				}
				if flushes != tt.wfsh {
					t.Errorf("Flush calls: %v, want: %v", flushes, tt.wfsh)
				}
				if cancels != tt.wcan {
					t.Errorf("Canceled flushes: %v, want: %v", cancels, tt.wcan)
				}
				if !tt.shut { // Finilize blocking goroutines after all behaviour checks.
					b.Shutdown()
				}
				synctest.Wait()
				close(a)
			})
		})
	}
}

func TestPoolBufferReuse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := make(chan action)   // Just for using [buffer] type.
		n := make(chan struct{}) // Channel for counting New() calls.
		var buffers int
		go func() {
			for {
				select {
				case <-a:
				case <-n:
					buffers++
				case <-t.Context().Done():
					return
				}
			}
		}()
		f := func() Buffer {
			n <- struct{}{}
			return &buffer{cap: 1, act: a} // cap: 1 event == 1 buffer.
		}
		b := New(f, Config{FlushWorkers: 1, BatchTimeout: time.Second})
		go b.Serve()
		c := 20 // Sended events count.
		for range c {
			b.Batch(&decoder.Envelope{})
		}
		b.Shutdown()
		synctest.Wait()
		// Noop flush have very short period of buffer retention.
		// The actual number of buffers created depends of pool implementation.
		// To prove buffer re-usage we need only check that the number of New()
		// calls is a way less than the number of Get() calls.
		if buffers > c/2 {
			t.Fatalf("Created buffers: %v, expected no more than a half of %v", buffers, c)
		}
	})
}

func strerrs(got, want error) (string, string) {
	g, w := "(nil)", "(nil)"
	if got != nil {
		g = got.Error()
	}
	if want != nil {
		w = want.Error()
	}
	return g, w
}
