package chbuf

import (
	"context"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/chpool"
	"github.com/ClickHouse/ch-go/proto"

	"github.com/shagohead/bugpack/bugpack/batcher"
	"github.com/shagohead/bugpack/bugpack/envelope"
)

func Factory(pool *chpool.Pool) func() batcher.Buffer {
	return func() batcher.Buffer {
		return &buffer{pool: pool}
	}
}

const capacity = 1000

type buffer struct {
	pool *chpool.Pool
	buf  [capacity]*envelope.Envelope
	cur  int
}

// Append implements batcher.Buffer.
func (b *buffer) Append(e *envelope.Envelope) bool {
	b.buf[b.cur] = e
	b.cur++
	return b.cur == capacity
}

// Empty implements batcher.Buffer.
func (b *buffer) Empty() bool {
	return b.cur == 0
}

const flushingQuery = `
INSERT INTO ...
`

// Flush implements batcher.Buffer.
func (b *buffer) Flush(ctx context.Context) error {
	if err := b.pool.Do(ctx, ch.Query{
		Body:  flushingQuery,
		Input: proto.Input{},
		OnInput: func(ctx context.Context) error {
			panic("TODO")
		},
	}); err != nil {
		return err
	}
	b.cur = 0
	return nil
}
