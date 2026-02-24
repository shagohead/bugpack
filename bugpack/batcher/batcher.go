// batcher synchronously buffer events, schedules them for DBMS flushing and then reuse this buffers again.
package batcher

import (
	"context"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/go-faster/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/shagohead/bugpack/bugpack/envelope"
)

// Batcher accumulates events in buffer, which will be enqueued for flushing into DBMS.
type Batcher interface {
	// Batch the event. Safe for concurrent use.
	Batch(e *envelope.Envelope)

	// Start goroutines and wait they until they are finished.
	Serve() error

	// Gracefull shutdown of batcher. [Serve] will wait until already scheduled buffers will be flushed.
	Shutdown()
}

type Buffer interface {
	// Append event into buffer. Should return true if buffer is full.
	Append(*envelope.Envelope) bool

	// Flush buffer into DBMS and reset it's underlying resources for reuse.
	// This method should concern about context lifetime.
	Flush(context.Context) error

	// Is buffer empty or not.
	Empty() bool
}

// Default production config.
var DefaultConfig = Config{
	FlushWorkers: 4,
	BatchTimeout: time.Second * 10,
	FlushTimeout: time.Minute * 5,
	RetryBackoff: ConfigBackoff{
		Enable:              true,
		InitialInterval:     time.Second * 5,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxInterval:         backoff.DefaultMaxInterval,
	},
}

type Config struct {
	FlushContext context.Context
	FlushWorkers int           `yaml:"flush_workers"`
	BatchTimeout time.Duration `yaml:"batch_timeout"`
	FlushTimeout time.Duration `yaml:"flush_timeout"`
	RetryBackoff ConfigBackoff `yaml:"retry_backoff"`
}

type ConfigBackoff struct {
	Enable              bool          `yaml:"enable"`
	InitialInterval     time.Duration `yaml:"initial_interval"`
	RandomizationFactor float64       `yaml:"randomization_factor"`
	Multiplier          float64       `yaml:"multiplier"`
	MaxInterval         time.Duration `yaml:"max_interval"`
}

func New(factory func() Buffer, config Config) Batcher {
	if config.BatchTimeout == 0 || config.FlushWorkers == 0 {
		panic("BatchTimeout and FlushWorkers cannot be zero")
	}
	if config.FlushContext == nil {
		config.FlushContext = context.Background()
	} else {
		config.FlushContext = context.WithoutCancel(config.FlushContext)
	}
	ctx, cancel := context.WithCancel(config.FlushContext)
	return &batcher{
		conf:      config,
		pool:      &sync.Pool{New: func() any { return factory() }},
		done:      make(chan struct{}),
		events:    make(chan *envelope.Envelope),
		toflash:   make(chan Buffer),
		tracer:    otel.GetTracerProvider().Tracer("bugpack/batcher"),
		ctxflash:  ctx,
		stopflash: cancel,
	}
}

// batcher accumulates events into buffers, which then will be routed into flushing workers.
type batcher struct {
	conf      Config
	pool      *sync.Pool
	done      chan struct{}
	events    chan *envelope.Envelope
	toflash   chan Buffer     // Transmitter from receiver to workers. Closed with receiver.
	ctxflash  context.Context // Flushing context. Canceled after first worker error.
	stopflash func()
	tracer    trace.Tracer
}

// Batch implements Batcher.
func (b *batcher) Batch(e *envelope.Envelope) {
	select {
	case b.events <- e:
	case <-b.done:
	case <-b.ctxflash.Done():
	}
}

// Serve implements Batcher.
func (b *batcher) Serve() error {
	var wg, cg sync.WaitGroup
	errs := make(chan error, 1) // Collect first error from workers.

	wg.Add(b.conf.FlushWorkers)
	for range b.conf.FlushWorkers {
		go func() {
			defer wg.Done()
			for buf := range b.toflash {
				if err := b.flush(b.ctxflash, buf); err != nil {
					select {
					case errs <- err:
					default:
					}
					return
				}
			}
		}()
	}

	var rerr error
	wdone := make(chan struct{}) // Will be closed after workers are done.
	cg.Go(func() {
		wg.Wait()
		close(wdone)
	})
	cg.Go(func() {
		select {
		case rerr = <-errs: // Stop flushing after receiving first worker error.
			b.stopflash()
		case <-wdone:
		}
	})
	cg.Go(b.receive)
	cg.Wait()
	return rerr
}

// Shutdown implements Batcher.
func (b *batcher) Shutdown() {
	close(b.done)
}

func (b *batcher) receive() {
	c := b.pool.Get().(Buffer)
	t := time.NewTimer(b.conf.BatchTimeout)
	defer func() {
		close(b.toflash)
		t.Stop()
	}()
	var stop bool
	for !stop {
		t.Reset(b.conf.BatchTimeout)
		select {
		case <-t.C:
		case <-b.done:
			stop = true
		case e := <-b.events:
			if !c.Append(e) {
				continue
			}
		case <-b.ctxflash.Done():
			return
		}
		if c.Empty() {
			continue
		}
		select {
		case b.toflash <- c:
			c = b.pool.Get().(Buffer)
		case <-b.ctxflash.Done():
			return
		}
	}
}

const (
	otelError    = attribute.Key("error")
	otelInterval = attribute.Key("interval")
	otelRetry    = attribute.Key("retry")
	otelRetries  = attribute.Key("retries_count")
)

// flush buffer into DBMS.
// On error, if retrying enabled, will do it with exponential backoffs.
func (b *batcher) flush(ctx context.Context, buf Buffer) (err error) {
	ctx, span := b.tracer.Start(ctx, "batcher.flush")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()
	var (
		retry    int
		timer    *time.Timer // Reusable timer for consequent retries.
		rbackoff backoff.BackOff
	)
	if b.conf.FlushTimeout > 0 {
		var cancel func()
		deadline := time.Now().Add(b.conf.FlushTimeout)
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	for {
		span.SetAttributes(otelRetries.Int(retry))
		err = buf.Flush(ctx)
		if err == nil {
			b.pool.Put(buf)
			span.SetStatus(codes.Ok, "")
			return nil
		}
		if !b.conf.RetryBackoff.Enable {
			return err
		}
		// Delaying timer and backoff allocated only if needed.
		if rbackoff == nil {
			rbackoff = &backoff.ExponentialBackOff{
				InitialInterval:     b.conf.RetryBackoff.InitialInterval,
				RandomizationFactor: b.conf.RetryBackoff.RandomizationFactor,
				Multiplier:          b.conf.RetryBackoff.Multiplier,
				MaxInterval:         b.conf.RetryBackoff.MaxInterval,
			}
			timer = time.NewTimer(0)
			defer timer.Stop() // GC will do the job, but i want to free resources explicitly.
		}
		delay := rbackoff.NextBackOff()
		if deadline, ok := ctx.Deadline(); ok && deadline.Before(time.Now().Add(delay)) {
			return errors.Wrap(err, "out of time")
		}
		timer.Reset(delay)
		retry++
		span.AddEvent("Waiting for next flushing attempt", trace.WithAttributes(
			otelRetry.Int(retry),
			otelInterval.String(delay.String()),
			otelError.String(err.Error()),
		))
		select {
		case <-ctx.Done():
			return errors.Wrap(err, "retry canceled")
		case <-timer.C:
		}
	}
}
