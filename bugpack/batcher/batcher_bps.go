//go:build ignore

package main

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"time"

	"github.com/shagohead/bugpack/bugpack/batcher"
	"github.com/shagohead/bugpack/bugpack/envelope"
)

func main() {
	var (
		capacity int
		parallel int
		details  bool
	)
	flag.IntVar(&capacity, "c", 1000, "Buffers capacity")
	flag.IntVar(&parallel, "p", 1, "Parallel senders")
	flag.BoolVar(&details, "d", false, "Print detailed counters from senders")
	flag.Parse()

	batcher := batcher.New(&bufferer{capacity: capacity}, batcher.DefaultConfig)
	go func() {
		batcher.Serve()
	}()
	event := new(envelope.Envelope)
	counters := make(chan int, parallel)
	var wg sync.WaitGroup
	start := time.Now()
	for range parallel {
		wg.Go(func() {
			var c int
			s := time.Now().Add(time.Second)
			for {
				for range 1000 {
					batcher.Batch(event)
					c++
				}
				if !time.Now().Before(s) {
					counters <- c
					return
				}
			}
		})
	}
	wg.Wait()
	duration := time.Now().Sub(start)
	close(counters)
	var total int
	for c := range counters {
		if details {
			fmt.Printf("Calls by one sender: %v\n", c)
		}
		total += c
	}
	fmt.Printf("Measured Batch calls: %v/%v\n", total, duration)
}

type bufferer struct {
	capacity int
}

// Buffer implements batcher.Bufferer.
func (b *bufferer) Buffer() batcher.Buffer[*envelope.Envelope] {
	buf := make(arrbuffer, 0, b.capacity)
	return &buf
}

// Envelope implements batcher.Bufferer.
func (b *bufferer) Envelope(e *envelope.Envelope) *envelope.Envelope {
	return e
}

var _ batcher.Bufferer[*envelope.Envelope] = (*bufferer)(nil)

type arrbuffer []*envelope.Envelope

// Append implements Buffer.
func (a *arrbuffer) Append(e *envelope.Envelope) bool {
	*a = append(*a, e)
	return len(*a) == cap(*a)
}

// Envelope implements Buffer.
func (b *arrbuffer) Envelope(e *envelope.Envelope) *envelope.Envelope {
	return e
}

// Empty implements Buffer.
func (a *arrbuffer) Empty() bool {
	return len(*a) == 0
}

// Flush implements Buffer.
func (a *arrbuffer) Flush(context.Context) error {
	*a = (*a)[:0]
	return nil
}
